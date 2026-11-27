package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/sector-f/ssh-cli-forum/internal/service"
)

func TestGooseInitialMigration(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "initial.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := store.Migrate(); err != nil {
		t.Fatal(err)
	}
	versions, err := store.MigrationVersions()
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 1 || versions[0] != 1 {
		t.Fatalf("applied migration versions = %v, want [1]", versions)
	}
	if err := store.Migrate(); err != nil {
		t.Fatalf("second migrate should be a no-op: %v", err)
	}
	var channels int
	if err := store.db.QueryRow("SELECT COUNT(*) FROM chat_channels WHERE slug = 'global'").Scan(&channels); err != nil {
		t.Fatal(err)
	}
	if channels != 1 {
		t.Fatalf("global chat channel count = %d, want 1", channels)
	}
}

func TestOpenDoesNotApplyMigrations(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "unmigrated.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	var tables int
	err = store.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'users'").Scan(&tables)
	if err != nil {
		t.Fatal(err)
	}
	if tables != 0 {
		t.Fatal("Open created application tables before Migrate")
	}
}

func TestForumWriteOperations(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "forum.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(); err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	if err := store.CreateBoardWithDetails(ctx, "general", "General", "General discussion", 0, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec("INSERT INTO invites(code, created_at, expires_at) VALUES('test-invite', ?, ?)", nowText(), "9999-12-31T23:59:59Z"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`INSERT INTO users(username, pubkey, invite_code, role, status, created_at)
		VALUES('alice', x'01', 'test-invite', 'member', 'active', ?)`, nowText()); err != nil {
		t.Fatal(err)
	}

	board, err := store.GetBoardBySlug(ctx, "general")
	if err != nil {
		t.Fatal(err)
	}
	appSession := service.NewSession(1, store, store, nil)
	threadID, err := appSession.CreateThreadWithBody(ctx, "Test topic", "Initial post", board.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := appSession.CreateComment(ctx, threadID, 1, "A reply"); err != nil {
		t.Fatal(err)
	}

	topics, total, err := store.ListThreadsPage(ctx, board.ID, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(topics) != 1 || topics[0].ReplyCount != 1 {
		t.Fatalf("unexpected topic listing: total=%d topics=%+v", total, topics)
	}

	if err := appSession.DeleteThread(ctx, threadID); err != nil {
		t.Fatal(err)
	}
	if _, err := appSession.GetThreadByID(ctx, threadID); err != service.ErrThreadNotFound {
		t.Fatalf("deleted topic lookup error = %v, want ErrThreadNotFound", err)
	}
}
