package sqlite

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db *sql.DB
}

// Service is retained as a source-compatible alias for previous callers.
type Service = Store

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite3", path+"?_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	if _, err := s.db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return err
	}
	if _, err := s.db.Exec("PRAGMA journal_mode = WAL"); err != nil {
		return err
	}
	if _, err := s.db.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		return err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmts := []string{
		`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS invites (
			code TEXT PRIMARY KEY,
			created_at DATETIME NOT NULL,
			expires_at DATETIME NOT NULL,
			is_admin BOOLEAN NOT NULL DEFAULT 0,
			revoked_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			pubkey BLOB NOT NULL,
			invite_code TEXT NOT NULL UNIQUE,
			is_admin BOOLEAN NOT NULL DEFAULT 0,
			display_name TEXT NOT NULL DEFAULT '',
			role TEXT NOT NULL DEFAULT 'member',
			status TEXT NOT NULL DEFAULT 'active',
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_seen_at TEXT,
			FOREIGN KEY (invite_code) REFERENCES invites(code)
		)`,
		`CREATE TABLE IF NOT EXISTS boards (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			slug TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			sort_order INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'active',
			created_by INTEGER,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (created_by) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS threads (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			board_id INTEGER NOT NULL,
			author_id INTEGER NOT NULL,
			body TEXT NOT NULL,
			is_pinned BOOLEAN NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'open',
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_activity_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			deleted_at TEXT,
			FOREIGN KEY (board_id) REFERENCES boards(id),
			FOREIGN KEY (author_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS comments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			thread_id INTEGER NOT NULL,
			author_id INTEGER NOT NULL,
			body TEXT NOT NULL,
			posted_at DATETIME NOT NULL,
			updated_at TEXT,
			deleted_at TEXT,
			FOREIGN KEY (thread_id) REFERENCES threads(id),
			FOREIGN KEY (author_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS moderation_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			actor_id INTEGER NOT NULL,
			action TEXT NOT NULL,
			target_type TEXT NOT NULL,
			target_id INTEGER NOT NULL,
			reason TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (actor_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS chat_channels (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			slug TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			requires_membership BOOLEAN NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS chat_channel_memberships (
			channel_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			joined_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			left_at TEXT,
			PRIMARY KEY (channel_id, user_id),
			FOREIGN KEY (channel_id) REFERENCES chat_channels(id),
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS chat_messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			channel_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			kind TEXT NOT NULL DEFAULT 'text',
			body TEXT NOT NULL,
			created_at TEXT NOT NULL,
			FOREIGN KEY (channel_id) REFERENCES chat_channels(id),
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
	}
	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("create schema: %w", err)
		}
	}

	// Upgrade databases created by the original prototype without discarding data.
	columns := map[string]map[string]string{
		"invites": {"revoked_at": "TEXT"},
		"users": {
			"display_name": "TEXT NOT NULL DEFAULT ''",
			"role":         "TEXT NOT NULL DEFAULT 'member'",
			"status":       "TEXT NOT NULL DEFAULT 'active'",
			"created_at":   "TEXT",
			"last_seen_at": "TEXT",
		},
		"boards": {
			"slug":        "TEXT NOT NULL DEFAULT ''",
			"description": "TEXT NOT NULL DEFAULT ''",
			"sort_order":  "INTEGER NOT NULL DEFAULT 0",
			"status":      "TEXT NOT NULL DEFAULT 'active'",
			"created_by":  "INTEGER",
			"created_at":  "TEXT",
			"updated_at":  "TEXT",
		},
		"threads": {
			"is_pinned":        "BOOLEAN NOT NULL DEFAULT 0",
			"status":           "TEXT NOT NULL DEFAULT 'open'",
			"created_at":       "TEXT",
			"updated_at":       "TEXT",
			"last_activity_at": "TEXT",
			"deleted_at":       "TEXT",
		},
		"comments": {"updated_at": "TEXT", "deleted_at": "TEXT"},
	}
	for table, defs := range columns {
		for name, definition := range defs {
			exists, err := hasColumn(tx, table, name)
			if err != nil {
				return err
			}
			if !exists {
				if _, err := tx.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, name, definition)); err != nil {
					return fmt.Errorf("add %s.%s: %w", table, name, err)
				}
			}
		}
	}

	// Preserve the old administrator flag while introducing explicit roles.
	if _, err := tx.Exec(`UPDATE users SET role = CASE WHEN is_admin = 1 THEN 'admin' ELSE 'member' END WHERE role IS NULL OR role = '' OR (is_admin = 1 AND role = 'member')`); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE users SET created_at = COALESCE(created_at, CURRENT_TIMESTAMP)`); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE boards SET created_at = COALESCE(created_at, CURRENT_TIMESTAMP), updated_at = COALESCE(updated_at, CURRENT_TIMESTAMP)`); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE threads SET created_at = COALESCE(created_at, (SELECT MIN(posted_at) FROM comments WHERE comments.thread_id = threads.id), CURRENT_TIMESTAMP),
		updated_at = COALESCE(updated_at, CURRENT_TIMESTAMP),
		last_activity_at = COALESCE(last_activity_at, (SELECT MAX(posted_at) FROM comments WHERE comments.thread_id = threads.id), created_at, CURRENT_TIMESTAMP)`); err != nil {
		return err
	}

	// Give existing boards stable path components.
	rows, err := tx.Query("SELECT id, name FROM boards WHERE slug = '' OR slug IS NULL ORDER BY id")
	if err != nil {
		return err
	}
	type legacyBoard struct {
		id   int
		name string
	}
	var legacy []legacyBoard
	for rows.Next() {
		var b legacyBoard
		if err := rows.Scan(&b.id, &b.name); err != nil {
			rows.Close()
			return err
		}
		legacy = append(legacy, b)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, b := range legacy {
		base := slugify(b.name)
		if base == "" {
			base = fmt.Sprintf("board-%d", b.id)
		}
		slug := base
		for suffix := 2; ; suffix++ {
			var count int
			if err := tx.QueryRow("SELECT COUNT(*) FROM boards WHERE slug = ? AND id <> ?", slug, b.id).Scan(&count); err != nil {
				return err
			}
			if count == 0 {
				break
			}
			slug = fmt.Sprintf("%s-%d", base, suffix)
		}
		if _, err := tx.Exec("UPDATE boards SET slug = ? WHERE id = ?", slug, b.id); err != nil {
			return err
		}
	}

	indexes := []string{
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username_nocase ON users(lower(username))",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_boards_slug ON boards(slug)",
		"CREATE INDEX IF NOT EXISTS idx_boards_status_order ON boards(status, sort_order, name)",
		"CREATE INDEX IF NOT EXISTS idx_threads_board_activity ON threads(board_id, is_pinned DESC, last_activity_at DESC)",
		"CREATE INDEX IF NOT EXISTS idx_threads_visible ON threads(status, deleted_at)",
		"CREATE INDEX IF NOT EXISTS idx_comments_thread_posted ON comments(thread_id, posted_at)",
		"CREATE INDEX IF NOT EXISTS idx_invites_expiry ON invites(expires_at)",
		"CREATE INDEX IF NOT EXISTS idx_modlog_created ON moderation_log(created_at DESC)",
		"CREATE INDEX IF NOT EXISTS idx_chat_messages_channel_id ON chat_messages(channel_id, id DESC)",
		"CREATE INDEX IF NOT EXISTS idx_chat_memberships_user_channel ON chat_channel_memberships(user_id, channel_id, left_at)",
	}
	for _, stmt := range indexes {
		if _, err := tx.Exec(stmt); err != nil {
			return err
		}
	}
	if _, err := tx.Exec("INSERT OR IGNORE INTO chat_channels(slug, name, requires_membership) VALUES ('global', 'Global', 0)"); err != nil {
		return err
	}
	if _, err := tx.Exec("INSERT OR IGNORE INTO schema_migrations(version) VALUES (1), (2)"); err != nil {
		return err
	}
	return tx.Commit()
}

func hasColumn(tx *sql.Tx, table, column string) (bool, error) {
	rows, err := tx.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull, pk int
		var defaultValue interface{}
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			return false, err
		}
		if strings.EqualFold(name, column) {
			return true, nil
		}
	}
	return false, rows.Err()
}

func (s *Store) MigrationVersions() ([]int, error) {
	rows, err := s.db.Query("SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var versions []int
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		versions = append(versions, v)
	}
	return versions, rows.Err()
}

func (s *Store) Backup(destination string) error {
	if strings.TrimSpace(destination) == "" {
		return fmt.Errorf("backup destination is required")
	}
	_, err := s.db.Exec("VACUUM INTO ?", destination)
	if err != nil {
		return fmt.Errorf("create backup: %w", err)
	}
	return nil
}

func nowText() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	dash := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		case r == '-' || r == '_' || r == ' ':
			if b.Len() > 0 && !dash {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}
