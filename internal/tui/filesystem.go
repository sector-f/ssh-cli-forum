package tui

import (
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/sector-f/ssh-cli-forum/internal/service"
)

// A location is a node in the forum's virtual filesystem. Its path is a
// presentation of the node and its ancestry, not the source of its context.
type location interface {
	Path() string
	Parent() location
	Commands() []command
}

type rootLocation struct{}
type boardsLocation struct{}
type recentLocation struct{}

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

func (rootLocation) Path() string                { return "/" }
func (rootLocation) Parent() location            { return nil }
func (rootLocation) Commands() []command         { return nil }
func (boardsLocation) Path() string              { return "/boards" }
func (boardsLocation) Parent() location          { return rootLocation{} }
func (boardsLocation) Commands() []command       { return nil }
func (recentLocation) Path() string              { return "/recent" }
func (recentLocation) Parent() location          { return rootLocation{} }
func (recentLocation) Commands() []command       { return nil }
func (boardLocation) Parent() location           { return boardsLocation{} }
func (boardLocation) Commands() []command        { return boardCommands() }
func (l threadLocation) Parent() location        { return l.parent }
func (threadLocation) Commands() []command       { return threadCommands() }

func (l boardLocation) Path() string {
	return path.Join("/boards", l.board.Slug)
}

func (l threadLocation) Path() string {
	if l.parent == nil {
		return path.Join("/", strconv.Itoa(l.id))
	}
	return path.Join(l.parent.Path(), strconv.Itoa(l.id))
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

// Resolve interprets paths by traversing location children. Relative paths are
// anchored at the current location; absolute paths start at the filesystem root.
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
			next, err := fs.child(current, component)
			if err != nil {
				return nil, err
			}
			current = next
		}
	}
	return current, nil
}

func (fs virtualFilesystem) child(parent location, name string) (location, error) {
	m := fs.model
	switch parent := parent.(type) {
	case rootLocation:
		switch name {
		case "boards":
			return boardsLocation{}, nil
		case "recent":
			return recentLocation{}, nil
		default:
			return nil, fmt.Errorf("no such location: %s", path.Join(parent.Path(), name))
		}
	case boardsLocation:
		board, err := m.boards.GetBoardBySlug(m.ctx, name)
		if err != nil {
			return nil, err
		}
		return boardLocation{board: board}, nil
	case boardLocation:
		id, err := strconv.Atoi(name)
		if err != nil {
			return nil, service.ErrThreadNotFound
		}
		thread, err := m.boards.GetThreadByID(m.ctx, id)
		if err != nil {
			return nil, err
		}
		if thread.BoardID != parent.board.ID {
			return nil, service.ErrThreadNotFound
		}
		return threadLocation{id: id, parent: parent}, nil
	case recentLocation:
		id, err := strconv.Atoi(name)
		if err != nil {
			return nil, service.ErrThreadNotFound
		}
		recent, err := m.boards.RecentThreads(m.ctx, recentLocationLimit)
		if err != nil {
			return nil, err
		}
		for _, thread := range recent {
			if thread.ID == id {
				return threadLocation{id: id, parent: parent}, nil
			}
		}
		return nil, service.ErrThreadNotFound
	case threadLocation:
		return nil, fmt.Errorf("%s is not a directory", parent.Path())
	default:
		return nil, fmt.Errorf("invalid location")
	}
}

const recentLocationLimit = 100

func (fs virtualFilesystem) Children(loc location) ([]locationEntry, error) {
	m := fs.model
	switch loc := loc.(type) {
	case rootLocation:
		return []locationEntry{
			{name: "boards", location: boardsLocation{}},
			{name: "recent", location: recentLocation{}},
		}, nil
	case boardsLocation:
		boards, err := m.boards.ListBoards(m.ctx)
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
	case boardLocation:
		threads, _, err := m.boards.ListThreadsPage(m.ctx, loc.board.ID, 1, 50)
		if err != nil {
			return nil, err
		}
		entries := make([]locationEntry, 0, len(threads))
		for _, thread := range threads {
			entries = append(entries, locationEntry{
				name: strconv.Itoa(thread.ID),
				location: threadLocation{id: thread.ID, parent: loc},
			})
		}
		return entries, nil
	case recentLocation:
		threads, err := m.boards.RecentThreads(m.ctx, recentLocationLimit)
		if err != nil {
			return nil, err
		}
		entries := make([]locationEntry, 0, len(threads))
		for _, thread := range threads {
			entries = append(entries, locationEntry{
				name: strconv.Itoa(thread.ID),
				location: threadLocation{id: thread.ID, parent: loc},
			})
		}
		return entries, nil
	case threadLocation:
		return nil, fmt.Errorf("%s is not a directory", loc.Path())
	default:
		return nil, fmt.Errorf("invalid location")
	}
}

// Paths returns path completions for all currently enumerable locations.
func (fs virtualFilesystem) Paths() []string {
	// Root collections are useful as both absolute destinations and
	// directory prefixes. Nested directories use a trailing slash only.
	paths := []string{"/", "/boards", "/recent"}
	var walk func(location)
	walk = func(parent location) {
		entries, err := fs.Children(parent)
		if err != nil {
			return
		}
		for _, entry := range entries {
			entryPath := entry.location.Path()
			if isDirectory(entry.location) {
				switch entry.location.(type) {
				case boardsLocation, recentLocation:
					// Already included above.
				default:
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
	switch loc.(type) {
	case rootLocation, boardsLocation, recentLocation, boardLocation:
		return true
	default:
		return false
	}
}
