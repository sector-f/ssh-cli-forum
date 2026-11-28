package tui

import (
	"testing"

	"github.com/sector-f/ssh-cli-forum/internal/service"
)

func TestFilesystemResolveUsesTypedParents(t *testing.T) {
	model := &sessionModel{}
	fs := model.filesystem()

	got, err := fs.Resolve(rootLocation{}, "/boards")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got.(boardsLocation); !ok {
		t.Fatalf("Resolve(/boards) returned %T, want boardsLocation", got)
	}

	got, err = fs.Resolve(got, "../recent")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got.(recentLocation); !ok {
		t.Fatalf("Resolve(../recent) returned %T, want recentLocation", got)
	}

	board := boardLocation{board: service.Board{ID: 7, Slug: "general"}}
	topicFromBoard := threadLocation{id: 42, parent: board}
	got, err = fs.Resolve(topicFromBoard, "..")
	if err != nil {
		t.Fatal(err)
	}
	if got.Path() != board.Path() {
		t.Fatalf("parent of %s = %s, want %s", topicFromBoard.Path(), got.Path(), board.Path())
	}

	recent := recentLocation{}
	topicFromRecent := threadLocation{id: 42, parent: recent}
	got, err = fs.Resolve(topicFromRecent, "..")
	if err != nil {
		t.Fatal(err)
	}
	if got.Path() != recent.Path() {
		t.Fatalf("parent of %s = %s, want %s", topicFromRecent.Path(), got.Path(), recent.Path())
	}
}

func TestFilesystemResolveDoesNotTraverseThroughTopic(t *testing.T) {
	model := &sessionModel{}
	topic := threadLocation{
		id: 42,
		parent: boardLocation{board: service.Board{ID: 7, Slug: "general"}},
	}
	if _, err := model.filesystem().Resolve(topic, "replies"); err == nil {
		t.Fatal("expected a topic location to reject child traversal")
	}
}

func TestFilesystemResolveNormalizesParentTraversalByWalkingTree(t *testing.T) {
	model := &sessionModel{}
	got, err := model.filesystem().Resolve(rootLocation{}, "boards/../recent")
	if err != nil {
		t.Fatal(err)
	}
	if got.Path() != "/recent" {
		t.Fatalf("Resolve(boards/../recent) = %s, want /recent", got.Path())
	}

	topic := threadLocation{
		id: 42,
		parent: boardLocation{board: service.Board{ID: 7, Slug: "general"}},
	}
	got, err = model.filesystem().Resolve(topic, "../..")
	if err != nil {
		t.Fatal(err)
	}
	if got.Path() != "/boards" {
		t.Fatalf("Resolve(../..) = %s, want /boards", got.Path())
	}

	got, err = model.filesystem().Resolve(topic, "../../..")
	if err != nil {
		t.Fatal(err)
	}
	if got.Path() != "/" {
		t.Fatalf("Resolve(../../..) = %s, want /", got.Path())
	}
}
