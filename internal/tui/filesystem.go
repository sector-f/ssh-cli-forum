package tui

import (
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/sector-f/ssh-cli-forum/internal/service"
)

// A location is a node in the forum's virtual filesystem. Each domain object
// has one canonical location; collections such as recent activity are exposed
// as commands rather than alternate paths to the same object.
type location interface {
	Path() string
	Parent() location
	Commands() []command
}

// A directory owns the rules for looking up and enumerating its children.
// The filesystem resolver only needs this interface, so adding a new directory
// does not require changes to path traversal.
type directory interface {
	location
	Lookup(virtualFilesystem, string) (location, error)
	ReadDir(virtualFilesystem) ([]locationEntry, error)
}

type rootLocation struct{}
type boardsLocation struct{}

type boardLocation struct {
	board service.Board
}

type threadLocation struct {
	id     int
	parent location
}

type locationEntry struct {
	name     string
	location location
}

type virtualFilesystem struct {
	model *sessionModel
}

func (rootLocation) Path() string          { return "/" }
func (rootLocation) Parent() location      { return nil }
func (rootLocation) Commands() []command   { return nil }
func (boardsLocation) Path() string        { return "/boards" }
func (boardsLocation) Parent() location    { return rootLocation{} }
func (boardsLocation) Commands() []command { return nil }
func (boardLocation) Parent() location     { return boardsLocation{} }
func (boardLocation) Commands() []command  { return boardCommands() }
func (l threadLocation) Parent() location  { return l.parent }
func (threadLocation) Commands() []command { return threadCommands() }

func (l boardLocation) Path() string {
	return path.Join("/boards", l.board.Slug)
}

func (l threadLocation) Path() string {
	if l.parent == nil {
		return path.Join("/", strconv.Itoa(l.id))
	}
	return path.Join(l.parent.Path(), strconv.Itoa(l.id))
}

func (rootLocation) Lookup(_ virtualFilesystem, name string) (location, error) {
	switch name {
	case "boards":
		return boardsLocation{}, nil
	default:
		return nil, fmt.Errorf("no such location: %s", path.Join("/", name))
	}
}

func (rootLocation) ReadDir(_ virtualFilesystem) ([]locationEntry, error) {
	return []locationEntry{{name: "boards", location: boardsLocation{}}}, nil
}

func (boardsLocation) Lookup(fs virtualFilesystem, name string) (location, error) {
	board, err := fs.model.boards.GetBoardBySlug(fs.model.ctx, name)
	if err != nil {
		return nil, err
	}
	return boardLocation{board: board}, nil
}

func (boardsLocation) ReadDir(fs virtualFilesystem) ([]locationEntry, error) {
	boards, err := fs.model.boards.ListBoards(fs.model.ctx)
	if err != nil {
		return nil, err
	}
	entries := make([]locationEntry, 0, len(boards))
	for _, board := range boards {
		entries = append(entries, locationEntry{
			name: board.Slug,
			location: boardLocation{board: service.Board{
				ID: board.ID, Slug: board.Slug, Name: board.Name,
				Description: board.Description, Status: board.Status,
			}},
		})
	}
	return entries, nil
}

func (l boardLocation) Lookup(fs virtualFilesystem, name string) (location, error) {
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
	return threadLocation{id: id, parent: l}, nil
}

func (l boardLocation) ReadDir(fs virtualFilesystem) ([]locationEntry, error) {
	threads, _, err := fs.model.boards.ListThreadsPage(fs.model.ctx, l.board.ID, 1, 50)
	if err != nil {
		return nil, err
	}
	entries := make([]locationEntry, 0, len(threads))
	for _, thread := range threads {
		entries = append(entries, locationEntry{
			name: strconv.Itoa(thread.ID),
			location: threadLocation{id: thread.ID, parent: l},
		})
	}
	return entries, nil
}

func (m *sessionModel) currentLocation() location {
	if m.location == nil {
		return rootLocation{}
	}
	return m.location
}

func (m *sessionModel) currentBoardLocation() (boardLocation, bool) {
	loc, ok := m.currentLocation().(boardLocation)
	return loc, ok
}

func (m *sessionModel) currentThreadLocation() (threadLocation, bool) {
	loc, ok := m.currentLocation().(threadLocation)
	return loc, ok
}

func (m *sessionModel) filesystem() virtualFilesystem {
	return virtualFilesystem{model: m}
}

// Resolve interprets paths by traversing directory-provided lookups. Relative
// paths are anchored at the current location; absolute paths start at the root.
func (fs virtualFilesystem) Resolve(base location, target string) (location, error) {
	if target == "" {
		return base, nil
	}
	if base == nil {
		base = rootLocation{}
	}

	current := base
	if path.IsAbs(target) {
		current = rootLocation{}
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
			dir, ok := current.(directory)
			if !ok {
				return nil, fmt.Errorf("%s is not a directory", current.Path())
			}
			next, err := dir.Lookup(fs, component)
			if err != nil {
				return nil, err
			}
			current = next
		}
	}
	return current, nil
}

// Children enumerates a location through its directory interface.
func (fs virtualFilesystem) Children(loc location) ([]locationEntry, error) {
	dir, ok := loc.(directory)
	if !ok {
		return nil, fmt.Errorf("%s is not a directory", loc.Path())
	}
	return dir.ReadDir(fs)
}

// Paths returns path completions for all currently enumerable locations.
func (fs virtualFilesystem) Paths() []string {
	paths := []string{"/"}
	var walk func(location)
	walk = func(parent location) {
		entries, err := fs.Children(parent)
		if err != nil {
			return
		}
		for _, entry := range entries {
			entryPath := entry.location.Path()
			if isDirectory(entry.location) {
				// Root collections are useful without a trailing slash;
				// nested directories use one to distinguish them from leaves.
				if parent.Path() == "/" {
					paths = append(paths, entryPath)
				} else {
					paths = append(paths, entryPath+"/")
				}
				walk(entry.location)
				continue
			}
			paths = append(paths, entryPath)
		}
	}
	walk(rootLocation{})
	return paths
}

func isDirectory(loc location) bool {
	_, ok := loc.(directory)
	return ok
}
