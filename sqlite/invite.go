package sqlite

import (
	"database/sql"
	"time"

	"github.com/sector-f/ssh-cli-forum/forum"
	"go.uber.org/zap"
)

func (s *Service) CreateInvite(code string, expiresAt time.Time, isAdmin bool) error {
	createdAt := time.Now()
	_, err := s.db.Exec(`INSERT INTO invites (code, created_at, expires_at, is_admin) VALUES (?, ?, ?, ?)`, code, createdAt, expiresAt, isAdmin)
	return err
}

func (s *Service) ListInvites() ([]forum.Invite, error) {
	rows, err := s.db.Query(`SELECT code, created_at, expires_at, is_admin FROM invites`)
	if err != nil {
		s.logger.Error("Error listing invites", zap.Error(err))
		return nil, err
	}
	defer rows.Close()
	var invites []forum.Invite
	for rows.Next() {
		var i Invite // Create a new invite for each row
		if err := rows.Scan(&i.Code, &i.CreatedAt, &i.ExpiresAt, &i.IsAdmin); err != nil {
			return nil, err
		}

		invite := forum.Invite{
			Code:      i.Code,
			CreatedAt: i.CreatedAt,
			ExpiresAt: i.ExpiresAt,
			IsAdmin:   i.IsAdmin,
		}

		invites = append(invites, invite)
	}

	return invites, nil
}

func (s *Service) GetInvite(code string) (forum.Invite, error) {
	row := s.db.QueryRow(`SELECT code, created_at, expires_at, is_admin FROM invites WHERE code = ?`, code)
	var i Invite
	if err := row.Scan(&i.Code, &i.CreatedAt, &i.ExpiresAt, &i.IsAdmin); err != nil {
		if err == sql.ErrNoRows {
			return forum.Invite{}, forum.ErrInviteInvalid
		}
		s.logger.Error("Error getting invite", zap.Error(err), zap.String("code", code))
		return forum.Invite{}, forum.ErrUnknown
	}

	invite := forum.Invite{
		Code:      i.Code,
		CreatedAt: i.CreatedAt,
		ExpiresAt: i.ExpiresAt,
		IsAdmin:   i.IsAdmin,
	}

	return invite, nil
}
