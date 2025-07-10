package sqlite

import (
	"database/sql"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Service struct {
	db *sql.DB
}

type Invite struct {
	Code string
}

type User struct {
	Username   string
	PubKey     string
	InviteCode string
	IsAdmin    bool
}

type Board struct {
	Name string
}

type Thread struct {
	Title    string
	BoardID  int
	AuthorID int
}

type Comment struct {
	ThreadID int
	AuthorID int
	Body     string
	PostedAt time.Time
}

// Open opens a connection and creates tables if they don't exist.
func Open(path string) (*Service, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	s := &Service{db: db}

	if err := s.createTables(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Service) createTables() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS invites (
			code TEXT PRIMARY KEY
		);`,

		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			pubkey TEXT NOT NULL,
			invite_code TEXT,
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
