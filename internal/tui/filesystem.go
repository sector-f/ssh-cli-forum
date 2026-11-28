package tui

import (
	"fmt"
	"path"
	"strconv"
	"strings"

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

type rootDirectory struct{}
type boardsDirectory struct{}

type boardObject struct {
	board service.Board
}

type topicObject struct {
	id int
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

func (rootDirectory) Commands() []command  { return nil }
func (boardsDirectory) Commands() []command { return nil }
func (boardObject) Commands() []command     { return boardCommands() }
func (topicObject) Commands() []command     { return threadCommands() }

func (fs virtualFilesystem) Root() *pathNode {
	if fs.root != nil {
		return fs.root
	}
	return &pathNode{object: rootDirectory{}}
}

func (rootDirectory) Lookup(_ virtualFilesystem, name string) (filesystemObject, error) {
	if name == "boards" {
		return boardsDirectory{}, nil
	}
	return nil, fmt.Errorf("no such entry: %s", name)
}

func (rootDirectory) ReadDir(_ virtualFilesystem) ([]filesystemEntry, error) {
	return []filesystemEntry{{name: "boards", object: boardsDirectory{}}}, nil
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
	threads, _, err := fs.model.boards.ListThreadsPage(fs.model.ctx, l.board.ID, 1, 50)
	if err != nil {
		return nil, err
	}
	entries := make([]filesystemEntry, 0, len(threads))
	for _, thread := range threads {
		entries = append(entries, filesystemEntry{
			name: strconv.Itoa(thread.ID),
			object: topicObject{id: thread.ID},
		})
	}
	return entries, nil
}

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

// Resolve interprets paths by traversing directory-provided lookups. Relative
// paths are anchored at the supplied path node; absolute paths start at root.
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
