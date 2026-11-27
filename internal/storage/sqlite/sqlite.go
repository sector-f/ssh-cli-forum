package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"io/fs"
	"fmt"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite3", path+"?_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

//go:embed migrations/*.sql
var migrationFiles embed.FS

func (s *Store) migrationProvider() (*goose.Provider, error) {
	files, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		return nil, fmt.Errorf("open embedded migrations: %w", err)
	}
	provider, err := goose.NewProvider(
		goose.DialectSQLite3,
		s.db,
		files,
		goose.WithDisableGlobalRegistry(true),
	)
	if err != nil {
		return nil, fmt.Errorf("configure migrations: %w", err)
	}
	return provider, nil
}

// Migrate applies all pending embedded SQL migrations.
func (s *Store) Migrate() error {
	if _, err := s.db.Exec("PRAGMA journal_mode = WAL"); err != nil {
		return fmt.Errorf("enable WAL: %w", err)
	}
	provider, err := s.migrationProvider()
	if err != nil {
		return err
	}
	if _, err := provider.Up(context.Background()); err != nil {
		return fmt.Errorf("apply database migrations: %w", err)
	}
	return nil
}

// MigrationVersions returns the versions Goose has already applied.
func (s *Store) MigrationVersions() ([]int, error) {
	provider, err := s.migrationProvider()
	if err != nil {
		return nil, err
	}
	statuses, err := provider.Status(context.Background())
	if err != nil {
		return nil, fmt.Errorf("read migration status: %w", err)
	}
	versions := make([]int, 0, len(statuses))
	for _, status := range statuses {
		if status.State == goose.StateApplied {
			versions = append(versions, int(status.Source.Version))
		}
	}
	return versions, nil
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
