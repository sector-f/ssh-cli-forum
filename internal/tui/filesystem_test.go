package tui

import (
	"context"
	"fmt"
	"path"
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
	boards []service.BoardListing
}

func (s *testBoardService) ListBoards(context.Context) ([]service.BoardListing, error) {
	return append([]service.BoardListing(nil), s.boards...), nil
}

func TestPathNodeUsesStandardPathJoining(t *testing.T) {
	root := &pathNode{object: rootDirectory{}}
	child := &pathNode{name: "boards", object: boardsDirectory{}, parent: root}
	if got, want := path.Join(child.Parent().Path(), child.name), "/boards"; got != want {
		t.Fatalf("joined path = %q, want %q", got, want)
	}
}
