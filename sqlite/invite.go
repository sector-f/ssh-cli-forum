package sqlite

import (
	"time"

	"github.com/sector-f/ssh-cli-forum/forum"
)

func (s *Service) CreateInvite(code string, expiresAt time.Time) (*forum.Invite, error) {
	createdAt := time.Now()
	_, err := s.db.Exec(`INSERT INTO invites (code, created_at, expires_at) VALUES (?, ?, ?)`,
		code, createdAt, expiresAt)
	if err != nil {
		return nil, err
	}
	return &forum.Invite{
		Code:      code,
		CreatedAt: createdAt,
		ExpiresAt: expiresAt,
	}, nil
}

func (s *Service) ListInvites() ([]*forum.Invite, error) {
	rows, err := s.db.Query(`SELECT code, created_at, expires_at FROM invites`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var invites []*forum.Invite
	for rows.Next() {
		var i forum.Invite
		if err := rows.Scan(&i.Code, &i.CreatedAt, &i.ExpiresAt); err != nil {
			return nil, err
		}
		invites = append(invites, &i)
	}
	return invites, nil
}

func (s *Service) GetInvite(code string) (*forum.Invite, error) {
	row := s.db.QueryRow(`SELECT code, created_at, expires_at FROM invites WHERE code = ?`, code)
	var i forum.Invite
	if err := row.Scan(&i.Code, &i.CreatedAt, &i.ExpiresAt); err != nil {
		return nil, err
	}
	return &i, nil
}
