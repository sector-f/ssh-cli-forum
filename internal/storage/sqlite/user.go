package sqlite

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"

	sqlite3 "github.com/mattn/go-sqlite3"
	"github.com/sector-f/ssh-cli-forum/internal/service"
)

func (s *Store) CreateUser(username string, pubKey []byte, inviteCode, role string) error {
	if len(pubKey) == 0 {
		return fmt.Errorf("public key is required")
	}
	_, err := s.db.Exec(`INSERT INTO users (username, pubkey, invite_code, is_admin, role, status, created_at)
		VALUES (?, ?, ?, ?, ?, 'active', ?)`,
		username, pubKey, inviteCode, role == service.RoleAdmin, role, nowText())
	if err != nil && isUniqueConstraint(err) {
		var exists int
		if lookupErr := s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE username = ? COLLATE NOCASE)", username).Scan(&exists); lookupErr != nil {
			return fmt.Errorf("check conflicting username: %w", lookupErr)
		}
		if exists != 0 {
			return service.ErrUserExists
		}
		if lookupErr := s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE invite_code = ?)", inviteCode).Scan(&exists); lookupErr != nil {
			return fmt.Errorf("check redeemed invitation: %w", lookupErr)
		}
		if exists != 0 {
			return service.ErrInviteUsed
		}
	}
	return err
}

func (s *Store) BootstrapAdmin(username string, pubKey []byte) error {
	if len(pubKey) == 0 {
		return fmt.Errorf("public key is required")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	code, err := randomToken(24)
	if err != nil {
		return err
	}
	if _, err := tx.Exec("INSERT INTO invites (code, created_at, expires_at, is_admin) VALUES (?, ?, ?, 1)", code, nowText(), "9999-12-31T23:59:59Z"); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO users (username, pubkey, invite_code, is_admin, role, status, created_at)
		VALUES (?, ?, ?, 1, 'admin', 'active', ?)`, username, pubKey, code, nowText()); err != nil {
		if isUniqueConstraint(err) {
			var exists int
			if lookupErr := tx.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE username = ? COLLATE NOCASE)", username).Scan(&exists); lookupErr != nil {
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

func (s *Store) GetUserByID(id int) (service.User, error) {
	return scanUser(s.db.QueryRow(userSelect+" WHERE id = ?", id))
}

func (s *Store) GetUserByUsername(username string) (service.User, error) {
	return scanUser(s.db.QueryRow(userSelect+" WHERE username = ? COLLATE NOCASE", username))
}

const userSelect = `SELECT id, username, display_name, role, status, is_admin, created_at, last_seen_at FROM users`

func scanUser(row *sql.Row) (service.User, error) {
	var u service.User
	var created, last sql.NullString
	if err := row.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Role, &u.Status, &u.IsAdmin, &created, &last); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return service.User{}, service.ErrUserNotFound
		}
		return service.User{}, err
	}
	u.CreatedAt, u.LastSeenAt = parseTime(created.String), parseTime(last.String)
	if u.Role == "" {
		u.Role = service.RoleMember
		if u.IsAdmin {
			u.Role = service.RoleAdmin
		}
	}
	u.IsAdmin = u.Role == service.RoleAdmin || u.IsAdmin
	return u, nil
}

func (s *Store) AuthenticateUser(username string, givenKey []byte) (bool, error) {
	if len(givenKey) == 0 {
		return false, nil
	}
	var keyBytes []byte
	var status string
	err := s.db.QueryRow("SELECT pubkey, status FROM users WHERE username = ? COLLATE NOCASE", username).Scan(&keyBytes, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("load account for authentication: %w", err)
	}
	if status == service.StatusSuspended || !bytes.Equal(keyBytes, givenKey) {
		return false, nil
	}
	if _, err := s.db.Exec("UPDATE users SET last_seen_at = ? WHERE username = ? COLLATE NOCASE", nowText(), username); err != nil {
		return false, fmt.Errorf("update last-seen timestamp: %w", err)
	}
	return true, nil
}

func (s *Store) ListUsers() ([]service.User, error) {
	rows, err := s.db.Query(userSelect + " ORDER BY username COLLATE NOCASE")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []service.User
	for rows.Next() {
		var u service.User
		var created, last sql.NullString
		if err := rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Role, &u.Status, &u.IsAdmin, &created, &last); err != nil {
			return nil, err
		}
		u.CreatedAt, u.LastSeenAt = parseTime(created.String), parseTime(last.String)
		if u.Role == "" {
			u.Role = service.RoleMember
			if u.IsAdmin {
				u.Role = service.RoleAdmin
			}
		}
		u.IsAdmin = u.Role == service.RoleAdmin || u.IsAdmin
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *Store) SetUserRole(username, role string) error {
	res, err := s.db.Exec("UPDATE users SET role = ?, is_admin = ? WHERE username = ? COLLATE NOCASE", role, role == service.RoleAdmin, username)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err == nil && n == 0 {
		return service.ErrUserNotFound
	}
	return err
}

func (s *Store) SetUserStatus(username, status string) error {
	res, err := s.db.Exec("UPDATE users SET status = ? WHERE username = ? COLLATE NOCASE", status, username)
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
