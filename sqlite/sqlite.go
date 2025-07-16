package sqlite

import (
	"database/sql"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/ssh"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sector-f/ssh-cli-forum/forum"
	"go.uber.org/zap"
)

type Service struct {
	logger   *zap.Logger
	db       *sql.DB
	programs map[string]*tea.Program
	mu       sync.Mutex
}

type Invite struct {
	Code      string
	CreatedAt time.Time
	ExpiresAt time.Time
	IsAdmin   bool
}

type User struct {
	ID         int
	Username   string
	PubKey     ssh.PublicKey
	InviteCode string
	IsAdmin    bool
}

type Board struct {
	ID   int
	Name string
}

type Thread struct {
	ID       int
	Title    string
	BoardID  int
	AuthorID int
	Body     string
}

type Comment struct {
	ID       int
	ThreadID int
	AuthorID int
	Body     string
	PostedAt time.Time
}

// Open opens a connection and creates tables if they don't exist.
func Open(logger *zap.Logger, path string) (forum.Service, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	s := &Service{logger: logger, db: db, programs: make(map[string]*tea.Program)}

	if err := s.createTables(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Service) createTables() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS invites (
			code TEXT PRIMARY KEY,
			created_at DATETIME NOT NULL,
			expires_at DATETIME NOT NULL,
			is_admin BOOLEAN NOT NULL DEFAULT 0
		);`,

		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			pubkey BLOB NOT NULL,
			invite_code TEXT NOT NULL UNIQUE,
			is_admin BOOLEAN NOT NULL DEFAULT 0,
			FOREIGN KEY (invite_code) REFERENCES invites(code)
		);`,

		`CREATE TABLE IF NOT EXISTS boards (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE
		);`,

		`CREATE TABLE IF NOT EXISTS threads (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			board_id INTEGER NOT NULL,
			author_id INTEGER NOT NULL,
			body TEXT NOT NULL,
			FOREIGN KEY (board_id) REFERENCES boards(id),
			FOREIGN KEY (author_id) REFERENCES users(id)
		);`,

		`CREATE TABLE IF NOT EXISTS comments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			thread_id INTEGER NOT NULL,
			author_id INTEGER NOT NULL,
			body TEXT NOT NULL,
			posted_at DATETIME NOT NULL,
			FOREIGN KEY (thread_id) REFERENCES threads(id),
			FOREIGN KEY (author_id) REFERENCES users(id)
		);`,
	}

	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}

	return nil
}
