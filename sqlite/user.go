package sqlite

import (
	"database/sql"

	"github.com/sector-f/ssh-cli-forum/forum"
)

func (s *Service) CreateUser(username, pubKey, inviteCode string, isAdmin bool) (*forum.User, error) {
	var nullableInviteCode sql.NullString
	if inviteCode != "" {
		nullableInviteCode.String = inviteCode
		nullableInviteCode.Valid = true
	}

	res, err := s.db.Exec(`
		INSERT INTO users (username, pubkey, invite_code, is_admin)
		VALUES (?, ?, ?, ?)
	`, username, pubKey, nullableInviteCode, isAdmin)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()

	user := &forum.User{
		ID:       int(id),
		Username: username,
		PubKey:   pubKey,
		IsAdmin:  isAdmin,
	}
	if nullableInviteCode.Valid {
		user.Invite = &forum.Invite{Code: inviteCode}
	}
	return user, nil
}

func (s *Service) GetUserByID(id int) (*forum.User, error) {
	row := s.db.QueryRow(`SELECT id, username, pubkey, invite_code, is_admin FROM users WHERE id = ?`, id)
	var u forum.User
	var inviteCode sql.NullString
	if err := row.Scan(&u.ID, &u.Username, &u.PubKey, &inviteCode, &u.IsAdmin); err != nil {
		return nil, err
	}
	if inviteCode.Valid {
		u.Invite = &forum.Invite{Code: inviteCode.String}
	}
	return &u, nil
}

func (s *Service) GetUserByUsername(username string) (*forum.User, error) {
	row := s.db.QueryRow(`SELECT id, username, pubkey, invite_code, is_admin FROM users WHERE username = ?`, username)
	var u forum.User
	var inviteCode sql.NullString
	if err := row.Scan(&u.ID, &u.Username, &u.PubKey, &inviteCode, &u.IsAdmin); err != nil {
		return nil, err
	}
	if inviteCode.Valid {
		u.Invite = &forum.Invite{Code: inviteCode.String}
	}
	return &u, nil
}

func (s *Service) ListUsers() ([]*forum.User, error) {
	rows, err := s.db.Query(`SELECT id, username, pubkey, invite_code, is_admin FROM users`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*forum.User
	for rows.Next() {
		var u forum.User
		var inviteCode sql.NullString
		if err := rows.Scan(&u.ID, &u.Username, &u.PubKey, &inviteCode, &u.IsAdmin); err != nil {
			return nil, err
		}
		if inviteCode.Valid {
			u.Invite = &forum.Invite{Code: inviteCode.String}
		}
		users = append(users, &u)
	}
	return users, nil
}
