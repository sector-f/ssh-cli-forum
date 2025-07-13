package sqlite

import (
	"database/sql"
	"time"

	"github.com/sector-f/ssh-cli-forum/forum"
	"go.uber.org/zap"
)

func (s *Service) CreateInvite(code string, expiresAt time.Time, isAdmin bool) (*forum.Invite, error) {
	createdAt := time.Now()
	_, err := s.db.Exec(`INSERT INTO invites (code, created_at, expires_at, is_admin) VALUES (?, ?, ?, ?)`, code, createdAt, expiresAt, isAdmin)
	if err != nil {
		s.logger.Error("Error inserting invite", zap.Error(err), zap.String("code", code))
		return nil, err
	}
	return &forum.Invite{
		Code:      code,
		CreatedAt: createdAt,
		ExpiresAt: expiresAt,
	}, nil
}
func (s *Service) ListInvites() ([]*forum.Invite, error) {
	rows, err := s.db.Query(`SELECT code, created_at, expires_at, is_admin FROM invites`)
	if err != nil {
		s.logger.Error("Error listing invites", zap.Error(err))
		return nil, err
	}
	defer rows.Close()
	var invites []*forum.Invite
	for rows.Next() {
		var i forum.Invite // Create a new invite for each row
		if err := rows.Scan(&i.Code, &i.CreatedAt, &i.ExpiresAt, &i.IsAdmin); err != nil {
			return nil, err
		}
		invites = append(invites, &i)
	}
	return invites, nil
}

func (s *Service) GetInvite(code string) (*forum.Invite, error) {
	row := s.db.QueryRow(`SELECT code, created_at, expires_at, is_admin FROM invites WHERE code = ?`, code)
	var i forum.Invite
	if err := row.Scan(&i.Code, &i.CreatedAt, &i.ExpiresAt, &i.IsAdmin); err != nil {
		if err == sql.ErrNoRows {
			return nil, forum.ErrInviteInvalid
		}
		s.logger.Error("Error getting invite", zap.Error(err), zap.String("code", code))
		return nil, forum.ErrUnknown
	}
	return &i, nil
}
