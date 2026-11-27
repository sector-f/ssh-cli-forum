package sqlite

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/sector-f/ssh-cli-forum/internal/service"
	"go.uber.org/zap"
)

func TestLegacyDatabaseMigrationPreservesForumData(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`CREATE TABLE invites (code TEXT PRIMARY KEY, created_at DATETIME NOT NULL, expires_at DATETIME NOT NULL, is_admin BOOLEAN NOT NULL DEFAULT 0)`,
		`CREATE TABLE users (id INTEGER PRIMARY KEY AUTOINCREMENT, username TEXT NOT NULL UNIQUE, pubkey BLOB NOT NULL, invite_code TEXT NOT NULL UNIQUE, is_admin BOOLEAN NOT NULL DEFAULT 0, FOREIGN KEY (invite_code) REFERENCES invites(code))`,
		`CREATE TABLE boards (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL UNIQUE)`,
		`CREATE TABLE threads (id INTEGER PRIMARY KEY AUTOINCREMENT, title TEXT NOT NULL, board_id INTEGER NOT NULL, author_id INTEGER NOT NULL, body TEXT NOT NULL, FOREIGN KEY (board_id) REFERENCES boards(id), FOREIGN KEY (author_id) REFERENCES users(id))`,
		`CREATE TABLE comments (id INTEGER PRIMARY KEY AUTOINCREMENT, thread_id INTEGER NOT NULL, author_id INTEGER NOT NULL, body TEXT NOT NULL, posted_at DATETIME NOT NULL, FOREIGN KEY (thread_id) REFERENCES threads(id), FOREIGN KEY (author_id) REFERENCES users(id))`,
	}
	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().UTC()
	if _, err := db.Exec("INSERT INTO invites(code, created_at, expires_at) VALUES('legacy-invite', ?, ?)", now, now.Add(24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO users(username, pubkey, invite_code) VALUES('alice', ?, 'legacy-invite')", []byte("public-key")); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO boards(name) VALUES('General Discussion')"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO threads(title, board_id, author_id, body) VALUES('Legacy topic', 1, 1, 'Original body')"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO comments(thread_id, author_id, body, posted_at) VALUES(1, 1, 'Legacy reply', ?)", now); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := Open(zap.NewNop(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	board, err := store.GetBoardBySlug("general-discussion")
	if err != nil {
		t.Fatal(err)
	}
	if board.Name != "General Discussion" || board.Status != service.BoardActive {
		t.Fatalf("unexpected migrated board: %+v", board)
	}
	thread, err := store.GetThreadByID(1)
	if err != nil {
		t.Fatal(err)
	}
	if thread.Body != "Original body" || len(thread.Comments) != 1 || thread.Comments[0].Body != "Legacy reply" {
		t.Fatalf("legacy topic data was not preserved: %+v", thread)
	}
}

func TestForumWriteOperations(t *testing.T) {
	store, err := Open(zap.NewNop(), filepath.Join(t.TempDir(), "service.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := store.CreateBoardWithDetails("general", "General", "General discussion", 0, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec("INSERT INTO invites(code, created_at, expires_at) VALUES('test-invite', ?, ?)", nowText(), "9999-12-31T23:59:59Z"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`INSERT INTO users(username, pubkey, invite_code, role, status, created_at)
		VALUES('alice', x'01', 'test-invite', 'moderator', 'active', ?)`, nowText()); err != nil {
		t.Fatal(err)
	}

	board, err := store.GetBoardBySlug("general")
	if err != nil {
		t.Fatal(err)
	}
	id, err := store.CreateThreadWithBody("Test topic", "Initial post", board.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateComment(id, 1, "A reply"); err != nil {
		t.Fatal(err)
	}

	topics, total, err := store.ListThreadsPage(board.ID, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(topics) != 1 || topics[0].ReplyCount != 1 {
		t.Fatalf("unexpected topic listing: total=%d topics=%+v", total, topics)
	}

	appSession := service.NewSession(1, store, store, nil)
	if err := appSession.SetThreadStatus(id, 1, service.ThreadLocked, "test lock"); err != nil {
		t.Fatal(err)
	}
	if err := appSession.CreateComment(id, 1, "A reply after locking"); !errors.Is(err, service.ErrThreadLocked) {
		t.Fatalf("expected locked topic error, got %v", err)
	}

	if err := appSession.DeleteThread(id, 1, false, ""); err != nil {
		t.Fatal(err)
	}
	if err := appSession.SetThreadStatus(id, 1, service.ThreadOpen, "restore"); err != nil {
		t.Fatal(err)
	}
	if _, err := appSession.GetThreadByID(id); err != nil {
		t.Fatalf("restored topic should be readable: %v", err)
	}
}
