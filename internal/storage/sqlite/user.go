package sqlite

import (
	"context"
	"bytes"
	"database/sql"
	"errors"
	"fmt"

	sqlite3 "github.com/mattn/go-sqlite3"
	"github.com/sector-f/ssh-cli-forum/internal/service"
)

func (s *Store) CreateUser(ctx context.Context, username string, pubKey []byte, inviteCode, role string) error {
	if len(pubKey) == 0 {
		return fmt.Errorf("public key is required")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO users (username, pubkey, invite_code, role, status, created_at)
		VALUES (?, ?, ?, ?, 'active', ?)`,
		username, pubKey, inviteCode, role, nowText())
	if err != nil && isUniqueConstraint(err) {
		var exists int
		if lookupErr := s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE username = ? COLLATE NOCASE)", username).Scan(&exists); lookupErr != nil {
			return fmt.Errorf("check conflicting username: %w", lookupErr)
		}
		if exists != 0 {
			return service.ErrUserExists
		}
		if lookupErr := s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE invite_code = ?)", inviteCode).Scan(&exists); lookupErr != nil {
			return fmt.Errorf("check redeemed invitation: %w", lookupErr)
		}
		if exists != 0 {
			return service.ErrInviteUsed
		}
	}
	return err
}

func (s *Store) BootstrapAdmin(ctx context.Context, username string, pubKey []byte) error {
	if len(pubKey) == 0 {
		return fmt.Errorf("public key is required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	code, err := randomToken(24)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO invites (code, created_at, expires_at) VALUES (?, ?, ?)", code, nowText(), "9999-12-31T23:59:59Z"); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO users (username, pubkey, invite_code, role, status, created_at)
		VALUES (?, ?, ?, 'admin', 'active', ?)`, username, pubKey, code, nowText()); err != nil {
		if isUniqueConstraint(err) {
			var exists int
			if lookupErr := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE username = ? COLLATE NOCASE)", username).Scan(&exists); lookupErr != nil {
				return fmt.Errorf("check conflicting username: %w", lookupErr)
			}
			if exists != 0 {
				return service.ErrUserExists
			}
		}
		return err
	}
	return tx.Commit()
}

func (s *Store) GetUserByID(ctx context.Context, id int) (service.User, error) {
	return scanUser(s.db.QueryRowContext(ctx, userSelect+" WHERE id = ?", id))
}

func (s *Store) GetUserByUsername(ctx context.Context, username string) (service.User, error) {
	return scanUser(s.db.QueryRowContext(ctx, userSelect+" WHERE username = ? COLLATE NOCASE", username))
}

const userSelect = `SELECT id, username, display_name, role, status, created_at, last_seen_at FROM users`

func scanUser(row *sql.Row) (service.User, error) {
	var u service.User
	var created, last sql.NullString
	if err := row.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Role, &u.Status, &created, &last); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return service.User{}, service.ErrUserNotFound
		}
		return service.User{}, err
	}
	u.CreatedAt, u.LastSeenAt = parseTime(created.String), parseTime(last.String)
	return u, nil
}

func (s *Store) AuthenticateUser(ctx context.Context, username string, givenKey []byte) (bool, error) {
	if len(givenKey) == 0 {
		return false, nil
	}
	var keyBytes []byte
	var status string
	err := s.db.QueryRowContext(ctx, "SELECT pubkey, status FROM users WHERE username = ? COLLATE NOCASE", username).Scan(&keyBytes, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("load account for authentication: %w", err)
	}
	if status == service.StatusSuspended || !bytes.Equal(keyBytes, givenKey) {
		return false, nil
	}
	if _, err := s.db.ExecContext(ctx, "UPDATE users SET last_seen_at = ? WHERE username = ? COLLATE NOCASE", nowText(), username); err != nil {
		return false, fmt.Errorf("update last-seen timestamp: %w", err)
	}
	return true, nil
}

func (s *Store) ListUsers(ctx context.Context) ([]service.User, error) {
	rows, err := s.db.QueryContext(ctx, userSelect + " ORDER BY username COLLATE NOCASE")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []service.User
	for rows.Next() {
		var u service.User
		var created, last sql.NullString
		if err := rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Role, &u.Status, &created, &last); err != nil {
			return nil, err
		}
		u.CreatedAt, u.LastSeenAt = parseTime(created.String), parseTime(last.String)
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *Store) SetUserStatus(ctx context.Context, username, status string) error {
	res, err := s.db.ExecContext(ctx, "UPDATE users SET status = ? WHERE username = ? COLLATE NOCASE", status, username)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err == nil && n == 0 {
		return service.ErrUserNotFound
	}
	return err
}


func isUniqueConstraint(err error) bool {
	var sqliteErr sqlite3.Error
	return errors.As(err, &sqliteErr) && sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique
}
