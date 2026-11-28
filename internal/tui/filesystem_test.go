package tui

import (
	"fmt"
	"path"
	"testing"

	"github.com/sector-f/ssh-cli-forum/internal/service"
)

func TestFilesystemResolveUsesTypedParents(t *testing.T) {
	fs := virtualFilesystem{}

	got, err := fs.Resolve(rootLocation{}, "/boards")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got.(boardsLocation); !ok {
		t.Fatalf("Resolve(/boards) returned %T, want boardsLocation", got)
	}

	got, err = fs.Resolve(got, "..")
	if err != nil {
		t.Fatal(err)
	}
	if got.Path() != "/" {
		t.Fatalf("parent of /boards = %s, want /", got.Path())
	}

	board := boardLocation{board: service.Board{ID: 7, Slug: "general"}}
	topic := threadLocation{id: 42, parent: board}
	got, err = fs.Resolve(topic, "..")
	if err != nil {
		t.Fatal(err)
	}
	if got.Path() != board.Path() {
		t.Fatalf("parent of %s = %s, want %s", topic.Path(), got.Path(), board.Path())
	}
}

func TestFilesystemResolveDoesNotTraverseThroughTopic(t *testing.T) {
	fs := virtualFilesystem{}
	topic := threadLocation{
		id: 42,
		parent: boardLocation{board: service.Board{ID: 7, Slug: "general"}},
	}
	if _, err := fs.Resolve(topic, "replies"); err == nil {
		t.Fatal("expected a topic location to reject child traversal")
	}
}

func TestFilesystemResolveNormalizesParentTraversalByWalkingTree(t *testing.T) {
	fs := virtualFilesystem{}
	got, err := fs.Resolve(rootLocation{}, "boards/..")
	if err != nil {
		t.Fatal(err)
	}
	if got.Path() != "/" {
		t.Fatalf("Resolve(boards/..) = %s, want /", got.Path())
	}

	topic := threadLocation{
		id: 42,
		parent: boardLocation{board: service.Board{ID: 7, Slug: "general"}},
	}
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

	custom := testDirectoryLocation{name: "custom", parent: rootLocation{}}
	nested := testDirectoryLocation{name: "nested", parent: custom}
	leaf := testLeafLocation{name: "leaf", parent: nested}
	nested.children = map[string]location{"leaf": leaf}
	custom.children = map[string]location{"nested": nested}

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

	parent, err := fs.Resolve(leaf, "..")
	if err != nil {
		t.Fatal(err)
	}
	if parent.Path() != nested.Path() {
		t.Fatalf("parent of %s = %s, want %s", leaf.Path(), parent.Path(), nested.Path())
	}
}

func TestFilesystemDoesNotExposeRecentAsAnAlternateTopicPath(t *testing.T) {
	if _, err := (virtualFilesystem{}).Resolve(rootLocation{}, "/recent/42"); err == nil {
		t.Fatal("expected /recent to be absent from the canonical location tree")
	}
}

type testDirectoryLocation struct {
	name     string
	parent   location
	children map[string]location
}

func (l testDirectoryLocation) Path() string {
	return path.Join(l.parent.Path(), l.name)
}

func (l testDirectoryLocation) Parent() location  { return l.parent }
func (testDirectoryLocation) Commands() []command { return nil }

func (l testDirectoryLocation) Lookup(_ virtualFilesystem, name string) (location, error) {
	child, ok := l.children[name]
	if !ok {
		return nil, fmt.Errorf("no such test child: %s", name)
	}
	return child, nil
}

func (l testDirectoryLocation) ReadDir(_ virtualFilesystem) ([]locationEntry, error) {
	entries := make([]locationEntry, 0, len(l.children))
	for name, child := range l.children {
		entries = append(entries, locationEntry{name: name, location: child})
	}
	return entries, nil
}

type testLeafLocation struct {
	name   string
	parent location
}

func (l testLeafLocation) Path() string {
	return path.Join(l.parent.Path(), l.name)
}

func (l testLeafLocation) Parent() location  { return l.parent }
func (testLeafLocation) Commands() []command { return nil }
