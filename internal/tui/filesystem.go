package tui

import (
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/sector-f/ssh-cli-forum/internal/service"
)

// A filesystem object describes a forum resource. It has no knowledge of the
// path at which it is exposed; paths belong to pathNode instead.
type filesystemObject interface {
	Commands() []command
}

// Directories provide lookup and enumeration independently of their location
// in the filesystem tree.
type directory interface {
	filesystemObject
	Lookup(virtualFilesystem, string) (filesystemObject, error)
	ReadDir(virtualFilesystem) ([]filesystemEntry, error)
}

// Displayable resources provide their own content independently of whether
// they can also contain child resources.
type displayable interface {
	filesystemObject
	Display(virtualFilesystem) (string, error)
}

var (
	_ directory = (*rootDirectory)(nil)

	_ directory   = (*boardsDirectory)(nil)
	_ displayable = (*boardsDirectory)(nil)

	_ directory   = (*boardObject)(nil)
	_ displayable = (*boardObject)(nil)

	_ displayable = (*topicObject)(nil)

	_ directory = (*usersDirectory)(nil)

	_ displayable = (*userObject)(nil)
)

type usersDirectory struct{}

func (d usersDirectory) Commands() []command { return nil }

func (d usersDirectory) Lookup(fs virtualFilesystem, name string) (filesystemObject, error) {
	user, err := fs.model.users.GetUserByUsername(fs.model.ctx, name)
	if err != nil {
		return nil, err
	}
	return userObject{user: user}, nil
}

func (d usersDirectory) ReadDir(fs virtualFilesystem) ([]filesystemEntry, error) {
	users, err := fs.model.users.ListUsers(fs.model.ctx)
	if err != nil {
		return nil, err
	}

	entries := make([]filesystemEntry, 0, len(users))
	for _, user := range users {
		entries = append(entries, filesystemEntry{
			name:   user.Username,
			object: userObject{user: user},
		})
	}
	return entries, nil
}

type userObject struct {
	user service.User
}

func (u userObject) Commands() []command { return nil }

func (u userObject) Display(fs virtualFilesystem) (string, error) {
	return fmt.Sprintf(
		"Username: %s\nCreated at: %s\n",
		u.user.Username,
		u.user.CreatedAt.Format(time.DateTime),
	), nil
}

// pathNode is one name in the virtual filesystem tree. The same object can be
// wrapped by different nodes without the object carrying path or parent state.
type pathNode struct {
	name   string
	object filesystemObject
	parent *pathNode
}

func (n *pathNode) Path() string {
	if n == nil || n.parent == nil {
		return "/"
	}
	return path.Join(n.parent.Path(), n.name)
}

func (n *pathNode) Parent() *pathNode {
	if n == nil {
		return nil
	}
	return n.parent
}

type filesystemEntry struct {
	name   string
	object filesystemObject
}

type virtualFilesystem struct {
	model *sessionModel
	root  *pathNode
}

type rootDirectory struct{}

func (rootDirectory) Commands() []command { return nil }

func (rootDirectory) Lookup(_ virtualFilesystem, name string) (filesystemObject, error) {
	switch name {
	case "boards":
		return boardsDirectory{}, nil
	case "users":
		return usersDirectory{}, nil
	default:
		return nil, fmt.Errorf("no such entry: %s", name)
	}
}

func (rootDirectory) ReadDir(_ virtualFilesystem) ([]filesystemEntry, error) {
	return []filesystemEntry{
		{name: "boards", object: boardsDirectory{}},
		{name: "users", object: usersDirectory{}},
	}, nil
}

type boardsDirectory struct{}

func (boardsDirectory) Commands() []command { return nil }

// Display returns the board listing in the same tabular format used by the
// former board-list command.
func (boardsDirectory) Display(fs virtualFilesystem) (string, error) {
	if fs.model == nil || fs.model.boards == nil {
		return "", fmt.Errorf("forum service is unavailable")
	}
	boards, err := fs.model.boards.ListBoards(fs.model.ctx)
	if err != nil {
		return "", err
	}
	if len(boards) == 0 {
		return "No boards have been created.", nil
	}

	var content strings.Builder
	content.WriteString("SLUG                 BOARD                         STATUS\n")
	for _, board := range boards {
		fmt.Fprintf(&content, "%-20s %-29s %s\n", board.Slug, board.Name, board.Status)
		if board.Description != "" {
			fmt.Fprintf(&content, "  %s\n", board.Description)
		}
	}
	return strings.TrimRight(content.String(), "\n"), nil
}

func (boardsDirectory) Lookup(fs virtualFilesystem, name string) (filesystemObject, error) {
	board, err := fs.model.boards.GetBoardBySlug(fs.model.ctx, name)
	if err != nil {
		return nil, err
	}
	return boardObject{board: board}, nil
}

func (boardsDirectory) ReadDir(fs virtualFilesystem) ([]filesystemEntry, error) {
	boards, err := fs.model.boards.ListBoards(fs.model.ctx)
	if err != nil {
		return nil, err
	}
	entries := make([]filesystemEntry, 0, len(boards))
	for _, board := range boards {
		entries = append(entries, filesystemEntry{
			name: board.Slug,
			object: boardObject{board: service.Board{
				ID: board.ID, Slug: board.Slug, Name: board.Name,
				Description: board.Description, Status: board.Status,
			}},
		})
	}
	return entries, nil
}

type boardObject struct {
	board service.Board
}

func (boardObject) Commands() []command { return boardCommands() }

func (l boardObject) Lookup(fs virtualFilesystem, name string) (filesystemObject, error) {
	id, err := strconv.Atoi(name)
	if err != nil {
		return nil, service.ErrThreadNotFound
	}
	thread, err := fs.model.boards.GetThreadByID(fs.model.ctx, id)
	if err != nil {
		return nil, err
	}
	if thread.BoardID != l.board.ID {
		return nil, service.ErrThreadNotFound
	}
	return topicObject{id: id}, nil
}

func (l boardObject) ReadDir(fs virtualFilesystem) ([]filesystemEntry, error) {
	const pageSize = 100
	var entries []filesystemEntry
	for page := 1; ; page++ {
		threads, total, err := fs.model.boards.ListThreadsPage(fs.model.ctx, l.board.ID, page, pageSize)
		if err != nil {
			return nil, err
		}
		for _, thread := range threads {
			entries = append(entries, filesystemEntry{
				name:   strconv.Itoa(thread.ID),
				object: topicObject{id: thread.ID},
			})
		}
		if len(entries) >= total || len(threads) == 0 {
			break
		}
	}
	return entries, nil
}

// Display returns the board metadata followed by its topics. The directory
// entries remain independently available through ls and path navigation.
func (l boardObject) Display(fs virtualFilesystem) (string, error) {
	if fs.model == nil || fs.model.boards == nil {
		return "", fmt.Errorf("forum service is unavailable")
	}

	var content strings.Builder
	if l.board.Description != "" {
		fmt.Fprintf(&content, "%s — %s\n", l.board.Name, l.board.Description)
	} else {
		content.WriteString(l.board.Name + "\n")
	}
	fmt.Fprintf(&content, "Board status: %s\n", l.board.Status)

	const pageSize = 100
	var threads []service.ThreadListing
	total := 0
	for page := 1; ; page++ {
		nextPage, pageTotal, err := fs.model.boards.ListThreadsPage(fs.model.ctx, l.board.ID, page, pageSize)
		if err != nil {
			return "", err
		}
		if pageTotal > total {
			total = pageTotal
		}
		threads = append(threads, nextPage...)
		if len(threads) >= total || len(nextPage) == 0 {
			break
		}
	}

	if len(threads) == 0 {
		content.WriteString("No topics found.")
		return strings.TrimRight(content.String(), "\n"), nil
	}

	content.WriteString("ID    TOPIC                                            REPLIES  LAST ACTIVITY\n")
	for _, thread := range threads {
		title := thread.Title
		if len(title) > 46 {
			title = title[:43] + "..."
		}
		fmt.Fprintf(&content, "%-4d  %-46s %7d  %s\n",
			thread.ID, title, thread.ReplyCount, relativeTime(thread.LastActivityAt))
	}
	return strings.TrimRight(content.String(), "\n"), nil
}

type topicObject struct {
	id int
}

func (topicObject) Commands() []command { return threadCommands() }

// Display returns the topic's opening post and all replies. Replies are read in
// bounded pages so a topic's displayed content is not silently truncated.
func (l topicObject) Display(fs virtualFilesystem) (string, error) {
	if fs.model == nil || fs.model.boards == nil {
		return "", fmt.Errorf("forum service is unavailable")
	}
	thread, err := fs.model.boards.GetThreadByID(fs.model.ctx, l.id)
	if err != nil {
		return "", err
	}

	const pageSize = 100
	comments, total, err := fs.model.boards.ListCommentsPage(fs.model.ctx, l.id, 1, pageSize)
	if err != nil {
		return "", err
	}
	for page := 2; len(comments) < total; page++ {
		nextPage, updatedTotal, err := fs.model.boards.ListCommentsPage(fs.model.ctx, l.id, page, pageSize)
		if err != nil {
			return "", err
		}
		if updatedTotal > total {
			total = updatedTotal
		}
		if len(nextPage) == 0 {
			break
		}
		comments = append(comments, nextPage...)
	}

	var content strings.Builder
	fmt.Fprintf(&content, "[%d] %s\n", thread.ID, thread.Title)
	fmt.Fprintf(&content, "Started by %s · %s · %d replies\n",
		thread.Author.Username, thread.CreatedAt.Format("2006-01-02 15:04"), total)
	content.WriteString(strings.Repeat("-", 72) + "\n")
	content.WriteString(thread.Author.Username + ":\n")
	content.WriteString(thread.Body + "\n")
	content.WriteString(strings.Repeat("-", 72) + "\n")
	for _, comment := range comments {
		fmt.Fprintf(&content, "%s · %s\n",
			comment.Author.Username, comment.PostedAt.Format("2006-01-02 15:04"))
		content.WriteString(comment.Body + "\n")
		content.WriteString(strings.Repeat("-", 72) + "\n")
	}
	return strings.TrimRight(content.String(), "\n"), nil
}

// Resolve interprets paths by traversing directory-provided lookups. Relative
// paths are anchored at the supplied path node; absolute paths start at root.

func (m *sessionModel) currentLocation() *pathNode {
	if m.location == nil {
		return m.filesystem().Root()
	}
	return m.location
}

func (m *sessionModel) currentBoard() (boardObject, bool) {
	if m.currentLocation().object == nil {
		return boardObject{}, false
	}
	loc, ok := m.currentLocation().object.(boardObject)
	return loc, ok
}

func (m *sessionModel) currentTopic() (topicObject, bool) {
	if m.currentLocation().object == nil {
		return topicObject{}, false
	}
	loc, ok := m.currentLocation().object.(topicObject)
	return loc, ok
}

func (m *sessionModel) filesystem() virtualFilesystem {
	if m.filesystemRoot == nil {
		m.filesystemRoot = &pathNode{object: rootDirectory{}}
	}
	return virtualFilesystem{model: m, root: m.filesystemRoot}
}

func (fs virtualFilesystem) Root() *pathNode {
	if fs.root != nil {
		return fs.root
	}
	return &pathNode{object: rootDirectory{}}
}

func (fs virtualFilesystem) Resolve(base *pathNode, target string) (*pathNode, error) {
	if target == "" {
		return base, nil
	}
	if base == nil {
		base = fs.Root()
	}

	current := base
	if path.IsAbs(target) {
		current = fs.Root()
	}
	for _, component := range strings.Split(target, "/") {
		switch component {
		case "", ".":
			continue
		case "..":
			if parent := current.Parent(); parent != nil {
				current = parent
			}
		default:
			dir, ok := current.object.(directory)
			if !ok {
				return nil, fmt.Errorf("%s is not a directory", current.Path())
			}
			object, err := dir.Lookup(fs, component)
			if err != nil {
				return nil, err
			}
			current = &pathNode{name: component, object: object, parent: current}
		}
	}
	return current, nil
}

// Children enumerates a directory and attaches each returned object to a new
// path node. The directory object itself remains unaware of the resulting path.

func (fs virtualFilesystem) Children(parent *pathNode) ([]*pathNode, error) {
	dir, ok := parent.object.(directory)
	if !ok {
		return nil, fmt.Errorf("%s is not a directory", parent.Path())
	}
	entries, err := dir.ReadDir(fs)
	if err != nil {
		return nil, err
	}
	children := make([]*pathNode, 0, len(entries))
	for _, entry := range entries {
		children = append(children, &pathNode{
			name: entry.name, object: entry.object, parent: parent,
		})
	}
	return children, nil
}

// Paths returns path completions for all currently enumerable locations.

func (fs virtualFilesystem) Paths() []string {
	root := fs.Root()
	paths := []string{root.Path()}
	var walk func(*pathNode)
	walk = func(parent *pathNode) {
		children, err := fs.Children(parent)
		if err != nil {
			return
		}
		for _, child := range children {
			childPath := child.Path()
			if isDirectory(child.object) {
				// Root collections are useful without a trailing slash;
				// nested directories use one to distinguish them from leaves.
				if parent.Path() == "/" {
					paths = append(paths, childPath)
				} else {
					paths = append(paths, childPath+"/")
				}
				walk(child)
				continue
			}
			paths = append(paths, childPath)
		}
	}
	walk(root)
	return paths
}

func isDirectory(object filesystemObject) bool {
	_, ok := object.(directory)
	return ok
}
