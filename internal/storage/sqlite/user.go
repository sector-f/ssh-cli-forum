package sqlite

import (
	"database/sql"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/ssh"
	"github.com/sector-f/ssh-cli-forum/internal/service"
)

var (
	minUsernameLength = 3
	maxUsernameLength = 20
	validUsername     = regexp.MustCompile("^[A-Za-z0-9_-]+$")
)

func (s *Service) CreateUser(username string, pubKey ssh.PublicKey, inviteCode string) error {
	if err := isValidUsername(username); err != nil {
		return err
	}
	if pubKey == nil {
		return service.ErrUsernameInvalid
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var expires time.Time
	var isAdmin bool
	var revoked sql.NullString
	err = tx.QueryRow("SELECT expires_at, is_admin, revoked_at FROM invites WHERE code = ?", inviteCode).Scan(&expires, &isAdmin, &revoked)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrInviteInvalid
	}
	if err != nil {
		return err
	}
	if revoked.Valid {
		return service.ErrInviteRevoked
	}
	if time.Now().After(expires) {
		return service.ErrInviteExpired
	}

	var used int
	if err := tx.QueryRow("SELECT COUNT(*) FROM users WHERE invite_code = ?", inviteCode).Scan(&used); err != nil {
		return err
	}
	if used > 0 {
		return service.ErrInviteUsed
	}
	_, err = tx.Exec(`INSERT INTO users (username, pubkey, invite_code, is_admin, role, status, created_at)
		VALUES (?, ?, ?, ?, ?, 'active', ?)`, username, pubKey.Marshal(), inviteCode, isAdmin, roleForAdmin(isAdmin), nowText())
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return service.ErrUserExists
		}
		return err
	}
	return tx.Commit()
}

func (s *Service) BootstrapAdmin(username string, pubKey ssh.PublicKey) error {
	if err := isValidUsername(username); err != nil {
		return err
	}
	if pubKey == nil {
		return service.ErrUsernameInvalid
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var count int
	if err := tx.QueryRow("SELECT COUNT(*) FROM users WHERE role = 'admin' OR is_admin = 1").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return service.ErrAdminExists
	}
	code, err := randomToken(24)
	if err != nil {
		return err
	}
	if _, err := tx.Exec("INSERT INTO invites (code, created_at, expires_at, is_admin) VALUES (?, ?, ?, 1)", code, nowText(), "9999-12-31T23:59:59Z"); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO users (username, pubkey, invite_code, is_admin, role, status, created_at)
		VALUES (?, ?, ?, 1, 'admin', 'active', ?)`, username, pubKey.Marshal(), code, nowText()); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return service.ErrUserExists
		}
		return err
	}
	return tx.Commit()
}

func (s *Service) GetUserByID(id int) (service.User, error) {
	return scanUser(s.db.QueryRow(userSelect+" WHERE id = ?", id))
}

func (s *Service) GetUserByUsername(username string) (service.User, error) {
	return scanUser(s.db.QueryRow(userSelect+" WHERE username = ? COLLATE NOCASE", username))
}

const userSelect = `SELECT id, username, display_name, role, status, is_admin, created_at, last_seen_at FROM users`

func scanUser(row *sql.Row) (service.User, error) {
	var u service.User
	var created, last sql.NullString
	if err := row.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Role, &u.Status, &u.IsAdmin, &created, &last); err != nil {
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

func (s *Service) AuthenticateUser(username string, givenKey ssh.PublicKey) bool {
	if givenKey == nil {
		return false
	}
	var keyBytes []byte
	var status string
	err := s.db.QueryRow("SELECT pubkey, status FROM users WHERE username = ? COLLATE NOCASE", username).Scan(&keyBytes, &status)
	if err != nil || status == service.StatusSuspended {
		return false
	}
	dbKey, err := ssh.ParsePublicKey(keyBytes)
	if err != nil || !ssh.KeysEqual(dbKey, givenKey) {
		return false
	}
	_, _ = s.db.Exec("UPDATE users SET last_seen_at = ? WHERE username = ? COLLATE NOCASE", nowText(), username)
	return true
}

func (s *Service) ListUsers() ([]service.User, error) {
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

func (s *Service) SetUserRole(username, role string) error {
	if role != service.RoleMember && role != service.RoleModerator && role != service.RoleAdmin {
		return errors.New("role must be member, moderator, or admin")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var current string
	if err := tx.QueryRow("SELECT role FROM users WHERE username = ? COLLATE NOCASE", username).Scan(&current); errors.Is(err, sql.ErrNoRows) {
		return service.ErrUserNotFound
	} else if err != nil {
		return err
	}
	if current == service.RoleAdmin && role != service.RoleAdmin {
		var count int
		if err := tx.QueryRow("SELECT COUNT(*) FROM users WHERE role = 'admin' OR is_admin = 1").Scan(&count); err != nil {
			return err
		}
		if count <= 1 {
			return errors.New("cannot demote the last administrator")
		}
	}
	if _, err := tx.Exec("UPDATE users SET role = ?, is_admin = ? WHERE username = ? COLLATE NOCASE", role, role == service.RoleAdmin, username); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) SetUserStatus(username, status string) error {
	if status != service.StatusActive && status != service.StatusSuspended {
		return errors.New("status must be active or suspended")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var role string
	if err := tx.QueryRow("SELECT role FROM users WHERE username = ? COLLATE NOCASE", username).Scan(&role); errors.Is(err, sql.ErrNoRows) {
		return service.ErrUserNotFound
	} else if err != nil {
		return err
	}
	if status == service.StatusSuspended && role == service.RoleAdmin {
		var activeAdmins int
		if err := tx.QueryRow("SELECT COUNT(*) FROM users WHERE role = 'admin' AND status = 'active'").Scan(&activeAdmins); err != nil {
			return err
		}
		if activeAdmins <= 1 {
			return errors.New("cannot suspend the last active administrator")
		}
	}
	if _, err := tx.Exec("UPDATE users SET status = ? WHERE username = ? COLLATE NOCASE", status, username); err != nil {
		return err
	}
	return tx.Commit()
}

func roleForAdmin(admin bool) string {
	if admin {
		return service.RoleAdmin
	}
	return service.RoleMember
}

func isValidUsername(value string) error {
	if value == service.SignupUsername || !validUsername.MatchString(value) {
		return service.ErrUsernameInvalid
	}
	if len(value) < minUsernameLength {
		return service.ErrUsernameTooShort
	}
	if len(value) > maxUsernameLength {
		return service.ErrUsernameTooLong
	}
	return nil
}
