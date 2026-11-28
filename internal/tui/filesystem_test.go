package tui

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"strings"
	"testing"

	"github.com/sector-f/ssh-cli-forum/internal/service"
)

func TestFilesystemResolveBuildsPathsOutsideObjects(t *testing.T) {
	root := &pathNode{object: rootDirectory{}}
	fs := virtualFilesystem{root: root}

	boards, err := fs.Resolve(root, "/boards")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := boards.object.(boardsDirectory); !ok {
		t.Fatalf("Resolve(/boards) returned %T, want boardsDirectory", boards.object)
	}
	if boards.Path() != "/boards" || boards.Parent() != root {
		t.Fatalf("boards path node has unexpected path or parent: path=%q parent=%p", boards.Path(), boards.Parent())
	}

	board := boardObject{board: service.Board{ID: 7, Slug: "general"}}
	boardPath := &pathNode{name: "general", object: board, parent: boards}
	topic := topicObject{id: 42}
	topicPath := &pathNode{name: "42", object: topic, parent: boardPath}

	if boardPath.Path() != "/boards/general" || topicPath.Path() != "/boards/general/42" {
		t.Fatalf("unexpected paths: board=%q topic=%q", boardPath.Path(), topicPath.Path())
	}
	if topicPath.Parent() != boardPath || boardPath.Parent() != boards {
		t.Fatal("path tree did not preserve parent links")
	}

	// The resource values do not contain path or parent state.
	if got := board.board.Slug; got != "general" {
		t.Fatalf("board object changed unexpectedly: %q", got)
	}
	if topic.id != 42 {
		t.Fatalf("topic object changed unexpectedly: %d", topic.id)
	}
}

func TestFilesystemResolveDoesNotTraverseThroughTopic(t *testing.T) {
	fs := virtualFilesystem{}
	root := fs.Root()
	boards := &pathNode{name: "boards", object: boardsDirectory{}, parent: root}
	board := &pathNode{
		name: "general", object: boardObject{board: service.Board{ID: 7, Slug: "general"}},
		parent: boards,
	}
	topic := &pathNode{name: "42", object: topicObject{id: 42}, parent: board}

	if _, err := fs.Resolve(topic, "replies"); err == nil {
		t.Fatal("expected a topic object to reject child traversal")
	}
}

func TestFilesystemResolveNormalizesParentTraversalByWalkingPathTree(t *testing.T) {
	fs := virtualFilesystem{}
	root := fs.Root()

	got, err := fs.Resolve(root, "boards/..")
	if err != nil {
		t.Fatal(err)
	}
	if got.Path() != "/" {
		t.Fatalf("Resolve(boards/..) = %s, want /", got.Path())
	}

	boards := &pathNode{name: "boards", object: boardsDirectory{}, parent: root}
	board := &pathNode{
		name: "general", object: boardObject{board: service.Board{ID: 7, Slug: "general"}},
		parent: boards,
	}
	topic := &pathNode{name: "42", object: topicObject{id: 42}, parent: board}

	got, err = fs.Resolve(topic, "../..")
	if err != nil {
		t.Fatal(err)
	}
	if got.Path() != "/boards" {
		t.Fatalf("Resolve(../..) = %s, want /boards", got.Path())
	}

	got, err = fs.Resolve(topic, "../../..")
	if err != nil {
		t.Fatal(err)
	}
	if got.Path() != "/" {
		t.Fatalf("Resolve(../../..) = %s, want /", got.Path())
	}
}

func TestFilesystemTraversalUsesDirectoryInterface(t *testing.T) {
	fs := virtualFilesystem{}
	root := fs.Root()

	customObject := testDirectoryObject{}
	nestedObject := testDirectoryObject{}
	leafObject := testLeafObject{}
	nestedObject.children = map[string]filesystemObject{"leaf": leafObject}
	customObject.children = map[string]filesystemObject{"nested": nestedObject}

	custom := &pathNode{name: "custom", object: customObject, parent: root}
	got, err := fs.Resolve(custom, "nested/leaf")
	if err != nil {
		t.Fatal(err)
	}
	if got.Path() != "/custom/nested/leaf" {
		t.Fatalf("Resolve returned %s, want /custom/nested/leaf", got.Path())
	}

	entries, err := fs.Children(custom)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].name != "nested" {
		t.Fatalf("Children(custom) = %#v, want nested entry", entries)
	}

	parent, err := fs.Resolve(got, "..")
	if err != nil {
		t.Fatal(err)
	}
	if parent.Path() != "/custom/nested" {
		t.Fatalf("parent of %s = %s, want /custom/nested", got.Path(), parent.Path())
	}
}

func TestBoardsDirectoryReadsCurrentBoardsOnEveryListing(t *testing.T) {
	boards := &testBoardService{
		boards: []service.BoardListing{{
			ID: 1, Slug: "general", Name: "General", Description: "General discussion", Status: service.BoardActive,
		}},
	}
	model := &sessionModel{boards: boards}
	fs := virtualFilesystem{model: model}
	root := fs.Root()
	boardsPath := &pathNode{name: "boards", object: boardsDirectory{}, parent: root}

	first, err := fs.Children(boardsPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || first[0].Path() != "/boards/general" {
		t.Fatalf("first listing = %#v, want /boards/general", first)
	}

	boards.boards = append(boards.boards, service.BoardListing{
		ID: 2, Slug: "development", Name: "Development", Status: service.BoardActive,
	})
	second, err := fs.Children(boardsPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 2 || second[1].Path() != "/boards/development" {
		t.Fatalf("second listing did not reflect the added board: %#v", second)
	}
}

func TestFilesystemDoesNotExposeRecentAsAnAlternateTopicPath(t *testing.T) {
	if _, err := (virtualFilesystem{}).Resolve(nil, "/recent/42"); err == nil {
		t.Fatal("expected /recent to be absent from the filesystem tree")
	}
}

type testDirectoryObject struct {
	children map[string]filesystemObject
}

func (testDirectoryObject) Commands() []command { return nil }

func (l testDirectoryObject) Lookup(_ virtualFilesystem, name string) (filesystemObject, error) {
	child, ok := l.children[name]
	if !ok {
		return nil, fmt.Errorf("no such test child: %s", name)
	}
	return child, nil
}

func (l testDirectoryObject) ReadDir(_ virtualFilesystem) ([]filesystemEntry, error) {
	entries := make([]filesystemEntry, 0, len(l.children))
	for name, child := range l.children {
		entries = append(entries, filesystemEntry{name: name, object: child})
	}
	return entries, nil
}

type testLeafObject struct{}

func (testLeafObject) Commands() []command { return nil }

type testBoardService struct {
	service.BoardService
	boards        []service.BoardListing
	threads       map[int][]service.ThreadListing
	boardDetails  map[string]service.Board
	threadDetails map[int]service.Thread
	comments      map[int][]service.Comment
}

func (s *testBoardService) ListBoards(context.Context) ([]service.BoardListing, error) {
	return append([]service.BoardListing(nil), s.boards...), nil
}

func (s *testBoardService) GetBoardBySlug(_ context.Context, slug string) (service.Board, error) {
	if board, ok := s.boardDetails[slug]; ok {
		return board, nil
	}
	for _, board := range s.boards {
		if board.Slug == slug {
			return service.Board{
				ID: board.ID, Slug: board.Slug, Name: board.Name,
				Description: board.Description, Status: board.Status,
			}, nil
		}
	}
	return service.Board{}, service.ErrBoardNotFound
}

func (s *testBoardService) GetBoardByID(_ context.Context, id int) (service.Board, error) {
	for _, board := range s.boardDetails {
		if board.ID == id {
			return board, nil
		}
	}
	for _, board := range s.boards {
		if board.ID == id {
			return service.Board{
				ID: board.ID, Slug: board.Slug, Name: board.Name,
				Description: board.Description, Status: board.Status,
			}, nil
		}
	}
	return service.Board{}, service.ErrBoardNotFound
}

func (s *testBoardService) ListThreadsPage(_ context.Context, boardID, page, pageSize int) ([]service.ThreadListing, int, error) {
	threads := s.threads[boardID]
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	start := (page - 1) * pageSize
	if start >= len(threads) {
		return nil, len(threads), nil
	}
	end := start + pageSize
	if end > len(threads) {
		end = len(threads)
	}
	return append([]service.ThreadListing(nil), threads[start:end]...), len(threads), nil
}

func (s *testBoardService) GetThreadByID(_ context.Context, id int) (service.Thread, error) {
	thread, ok := s.threadDetails[id]
	if !ok {
		return service.Thread{}, service.ErrThreadNotFound
	}
	return thread, nil
}

func (s *testBoardService) ListCommentsPage(_ context.Context, threadID, page, pageSize int) ([]service.Comment, int, error) {
	comments := s.comments[threadID]
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	start := (page - 1) * pageSize
	if start >= len(comments) {
		return nil, len(comments), nil
	}
	end := start + pageSize
	if end > len(comments) {
		end = len(comments)
	}
	return append([]service.Comment(nil), comments[start:end]...), len(comments), nil
}


func TestBoardReadDirReturnsEveryTopicAcrossPages(t *testing.T) {
	threads := make([]service.ThreadListing, 105)
	for i := range threads {
		threads[i] = service.ThreadListing{ID: i + 1}
	}
	boardService := &testBoardService{
		threads: map[int][]service.ThreadListing{7: threads},
	}
	fs := virtualFilesystem{model: &sessionModel{boards: boardService}}

	entries, err := (boardObject{board: service.Board{ID: 7}}).ReadDir(fs)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(threads) {
		t.Fatalf("ReadDir returned %d topics, want %d", len(entries), len(threads))
	}
	if entries[0].name != "1" || entries[len(entries)-1].name != strconv.Itoa(len(threads)) {
		t.Fatalf("ReadDir returned unexpected topic names: first=%q last=%q", entries[0].name, entries[len(entries)-1].name)
	}
}

func TestBoardsDirectoryDisplaysPreviousBoardTable(t *testing.T) {
	boardService := &testBoardService{
		boards: []service.BoardListing{
			{ID: 7, Slug: "general", Name: "General Discussion", Description: "A place for general topics.", Status: service.BoardActive},
			{ID: 8, Slug: "development", Name: "Development", Status: service.BoardArchived},
		},
	}
	fs := virtualFilesystem{model: &sessionModel{boards: boardService}}
	content, err := (boardsDirectory{}).Display(fs)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"SLUG                 BOARD                         STATUS",
		"general", "General Discussion", "active",
		"A place for general topics.", "development", "Development", "archived",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("boards directory display is missing %q:\n%s", want, content)
		}
	}
}

func TestBoardAndTopicDisplayTheirOwnContent(t *testing.T) {
	threads := make([]service.ThreadListing, 105)
	for i := range threads {
		threads[i] = service.ThreadListing{
			ID: i + 1, Title: fmt.Sprintf("Topic %03d", i+1), ReplyCount: i,
		}
	}
	board := boardObject{board: service.Board{
		ID: 7, Slug: "general", Name: "General Discussion",
		Description: "A place for general topics.", Status: service.BoardActive,
	}}
	thread := service.Thread{
		ID: 42, BoardID: 7, Title: "A topic",
		Author: service.User{Username: "alice"}, Body: "Opening post",
	}
	comments := make([]service.Comment, 105)
	for i := range comments {
		comments[i] = service.Comment{
			ID: i + 1, ThreadID: 42, Body: fmt.Sprintf("Reply %03d", i+1),
			Author: service.User{Username: "bob"},
		}
	}
	boardService := &testBoardService{
		threads: map[int][]service.ThreadListing{7: threads},
		threadDetails: map[int]service.Thread{42: thread},
		comments: map[int][]service.Comment{42: comments},
	}
	fs := virtualFilesystem{model: &sessionModel{boards: boardService}}
	boardContent, err := board.Display(fs)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"General Discussion — A place for general topics.",
		"Board status: active", "ID    TOPIC", "Topic 001", "Topic 105",
	} {
		if !strings.Contains(boardContent, want) {
			t.Errorf("board content does not contain %q:\n%s", want, boardContent)
		}
	}

	topicContent, err := (topicObject{id: 42}).Display(fs)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"[42] A topic", "alice:", "Opening post", "105 replies", "Reply 001", "Reply 105"} {
		if !strings.Contains(topicContent, want) {
			t.Errorf("topic content does not contain %q", want)
		}
	}
}

func TestPathNodeUsesStandardPathJoining(t *testing.T) {
	root := &pathNode{object: rootDirectory{}}
	child := &pathNode{name: "boards", object: boardsDirectory{}, parent: root}
	if got, want := path.Join(child.Parent().Path(), child.name), "/boards"; got != want {
		t.Fatalf("joined path = %q, want %q", got, want)
	}
}
