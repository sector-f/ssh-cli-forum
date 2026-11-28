package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/sector-f/ssh-cli-forum/internal/service"
)

func (s *Store) CreateInvite(ctx context.Context, code string, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx, "INSERT INTO invites (code, created_at, expires_at) VALUES (?, ?, ?)", code, nowText(), expiresAt.UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Store) ListInvites(ctx context.Context) ([]service.Invite, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT i.code, i.created_at, i.expires_at, COALESCE(u.username, ''),
		COALESCE(u.created_at, ''), COALESCE(i.revoked_at, '')
		FROM invites i LEFT JOIN users u ON u.invite_code = i.code ORDER BY i.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var invites []service.Invite
	for rows.Next() {
		var i service.Invite
		var created, expires, redeemed, revoked string
		if err := rows.Scan(&i.Code, &created, &expires, &i.RedeemedBy, &redeemed, &revoked); err != nil {
			return nil, err
		}
		i.CreatedAt, i.ExpiresAt = parseTime(created), parseTime(expires)
		i.RedeemedAt, i.RevokedAt = parseTime(redeemed), parseTime(revoked)
		invites = append(invites, i)
	}
	return invites, rows.Err()
}

func (s *Store) GetInvite(ctx context.Context, code string) (service.Invite, error) {
	var i service.Invite
	var created, expires, revoked sql.NullString
	err := s.db.QueryRowContext(ctx, "SELECT code, created_at, expires_at, revoked_at FROM invites WHERE code = ?", code).
		Scan(&i.Code, &created, &expires, &revoked)
	if errors.Is(err, sql.ErrNoRows) {
		return service.Invite{}, service.ErrInviteInvalid
	}
	if err != nil {
		return service.Invite{}, err
	}
	i.CreatedAt, i.ExpiresAt, i.RevokedAt = parseTime(created.String), parseTime(expires.String), parseTime(revoked.String)
	var redeemed sql.NullString
	err = s.db.QueryRowContext(ctx, "SELECT username, created_at FROM users WHERE invite_code = ?", code).Scan(&i.RedeemedBy, &redeemed)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return service.Invite{}, fmt.Errorf("load invitation redemption: %w", err)
	}
	if err == nil {
		i.RedeemedAt = parseTime(redeemed.String)
	}
	return i, nil
}

func (s *Store) RevokeInvite(ctx context.Context, code string) error {
	res, err := s.db.ExecContext(ctx, "UPDATE invites SET revoked_at = ? WHERE code = ? AND revoked_at IS NULL", nowText(), code)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err == nil && n == 0 {
		var exists int
		if e := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM invites WHERE code = ?", code).Scan(&exists); e != nil {
			return e
		}
		if exists == 0 {
			return service.ErrInviteInvalid
		}
	}
	return err
}
