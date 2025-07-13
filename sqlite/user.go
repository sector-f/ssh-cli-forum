package sqlite

import (
	"database/sql"
	"errors"
	"time"

	"github.com/charmbracelet/ssh"
	"github.com/sector-f/ssh-cli-forum/forum"
	"go.uber.org/zap"
)

func (s *Service) CreateUser(username string, pubKey ssh.PublicKey, inviteCode string) error {
	tx, err := s.db.Begin()
	if err != nil {
		s.logger.Error("Error starting transaction", zap.Error(err))
		return forum.ErrUnknown
	}
	defer tx.Rollback()

	var (
		dbUser   string
		dbInvite Invite
		isAdmin  bool
	)

	// Verify that invite code exists in database
	row := tx.QueryRow("SELECT code, created_at, expires_at, is_admin FROM invites WHERE code = ?", inviteCode)
	err = row.Scan(&dbInvite.Code, &dbInvite.CreatedAt, &dbInvite.ExpiresAt, &isAdmin)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return forum.ErrInviteInvalid
	case err != nil:
		s.logger.Error("Error checking invite code", zap.Error(err))
		return forum.ErrUnknown
	}

	// Verify that invite code has not expired
	if time.Now().After(dbInvite.ExpiresAt) {
		return forum.ErrInviteExpired
	}

	// Verify that invite code hasn't been used
	row = tx.QueryRow("SELECT username FROM users WHERE invite_code = ?", inviteCode)
	err = row.Scan(&dbUser)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		// Invite code hasn't been used, continue
	case err != nil:
		s.logger.Error("Error checking invite code usage", zap.Error(err))
		return forum.ErrUnknown
	}

	// Verify that username isn't taken
	row = tx.QueryRow("SELECT username FROM users WHERE username = ?", username)
	err = row.Scan(&dbUser)
	if !errors.Is(err, sql.ErrNoRows) {
		return forum.ErrUserExists
	}

	// Create the user and associate them with the invite code
	_, err = tx.Exec(`
		INSERT INTO users (username, pubkey, invite_code, is_admin)
		VALUES (?, ?, ?, ?)
	`, username, pubKey.Marshal(), inviteCode, isAdmin)

	if err != nil {
		s.logger.Error("Error creating user", zap.Error(err))
		return forum.ErrUnknown
	}

	err = tx.Commit()
	if err != nil {
		s.logger.Error("Error committing transaction", zap.Error(err))
		return forum.ErrUnknown
	}

	s.logger.Info("Created user", zap.String("username", username), zap.Bool("is_admin", isAdmin))

	return nil
}

func (s *Service) GetUserByID(id int) (forum.User, error) {
	row := s.db.QueryRow(`SELECT id, username, pubkey, invite_code, is_admin FROM users WHERE id = ?`, id)
	var u User
	var pubKey []byte
	if err := row.Scan(&u.ID, &u.Username, pubKey, &u.InviteCode, &u.IsAdmin); err != nil {
		return forum.User{}, err
	}
	key, err := ssh.ParsePublicKey(pubKey)
	if err != nil {
		return forum.User{}, err
	}
	u.PubKey = key

	forumUser := forum.User{
		ID:       u.ID,
		Username: u.Username,
		PubKey:   u.PubKey,
		IsAdmin:  u.IsAdmin,
	}

	return forumUser, nil
}

func (s *Service) GetUserByUsername(username string) (forum.User, error) {
	s.logger.Debug("Getting user by username", zap.String("username", username))

	row := s.db.QueryRow(`SELECT id, username, pubkey, invite_code, is_admin FROM users WHERE username = ?`, username)
	var u User
	var pubKey []byte
	if err := row.Scan(&u.ID, &u.Username, &pubKey, &u.InviteCode, &u.IsAdmin); err != nil {
		return forum.User{}, err
	}
	key, err := ssh.ParsePublicKey(pubKey)
	if err != nil {
		return forum.User{}, err
	}
	u.PubKey = key

	forumUser := forum.User{
		ID:       u.ID,
		Username: u.Username,
		PubKey:   u.PubKey,
		IsAdmin:  u.IsAdmin,
	}

	return forumUser, nil
}

func (s *Service) ListUsers() ([]forum.User, error) {
	rows, err := s.db.Query(`SELECT id, username, pubkey, invite_code, is_admin FROM users`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []forum.User
	for rows.Next() {
		var u User
		var pubKeyStr string
		if err := rows.Scan(&u.ID, &u.Username, &pubKeyStr, &u.InviteCode, &u.IsAdmin); err != nil {
			return nil, err
		}
		key, _, _, _, err := ssh.ParseAuthorizedKey([]byte(pubKeyStr))
		if err != nil {
			return nil, err
		}
		u.PubKey = key

		forumUser := forum.User{
			ID:       u.ID,
			Username: u.Username,
			PubKey:   u.PubKey,
			IsAdmin:  u.IsAdmin,
		}

		users = append(users, forumUser)
	}
	return users, nil
}
