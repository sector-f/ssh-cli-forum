package sqlite

import (
	"crypto/rand"
	"encoding/hex"
	"database/sql"
	"errors"
	"time"

	"github.com/sector-f/ssh-cli-forum/forum"
)

func (s *Service) CreateInvite(code string, expiresAt time.Time, isAdmin bool) error {
	if code == "" { return errors.New("invite code cannot be empty") }
	_, err := s.db.Exec("INSERT INTO invites (code, created_at, expires_at, is_admin) VALUES (?, ?, ?, ?)", code, nowText(), expiresAt.UTC().Format(time.RFC3339Nano), isAdmin)
	return err
}

func (s *Service) ListInvites() ([]forum.Invite, error) {
	rows, err := s.db.Query(`SELECT i.code, i.created_at, i.expires_at, i.is_admin, COALESCE(u.username, ''),
		COALESCE(u.created_at, ''), COALESCE(i.revoked_at, '')
		FROM invites i LEFT JOIN users u ON u.invite_code = i.code ORDER BY i.created_at DESC`)
	if err != nil { return nil, err }
	defer rows.Close()
	var invites []forum.Invite
	for rows.Next() {
		var i forum.Invite
		var created, expires, redeemed, revoked string
		if err := rows.Scan(&i.Code, &created, &expires, &i.IsAdmin, &i.RedeemedBy, &redeemed, &revoked); err != nil { return nil, err }
		i.CreatedAt, i.ExpiresAt = parseTime(created), parseTime(expires)
		i.RedeemedAt, i.RevokedAt = parseTime(redeemed), parseTime(revoked)
		invites = append(invites, i)
	}
	return invites, rows.Err()
}

func (s *Service) GetInvite(code string) (forum.Invite, error) {
	var i forum.Invite
	var created, expires, revoked sql.NullString
	err := s.db.QueryRow("SELECT code, created_at, expires_at, is_admin, revoked_at FROM invites WHERE code = ?", code).
		Scan(&i.Code, &created, &expires, &i.IsAdmin, &revoked)
	if errors.Is(err, sql.ErrNoRows) { return forum.Invite{}, forum.ErrInviteInvalid }
	if err != nil { return forum.Invite{}, err }
	i.CreatedAt, i.ExpiresAt, i.RevokedAt = parseTime(created.String), parseTime(expires.String), parseTime(revoked.String)
	var redeemed sql.NullString
	_ = s.db.QueryRow("SELECT username, created_at FROM users WHERE invite_code = ?", code).Scan(&i.RedeemedBy, &redeemed)
	i.RedeemedAt = parseTime(redeemed.String)
	return i, nil
}

func (s *Service) RevokeInvite(code string) error {
	res, err := s.db.Exec("UPDATE invites SET revoked_at = ? WHERE code = ? AND revoked_at IS NULL", nowText(), code)
	if err != nil { return err }
	n, err := res.RowsAffected()
	if err == nil && n == 0 {
		var exists int
		if e := s.db.QueryRow("SELECT COUNT(*) FROM invites WHERE code = ?", code).Scan(&exists); e != nil { return e }
		if exists == 0 { return forum.ErrInviteInvalid }
	}
	return err
}

func randomToken(bytes int) (string, error) {
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil { return "", err }
	return hex.EncodeToString(buf), nil
}
