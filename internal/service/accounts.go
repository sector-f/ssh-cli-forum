package service

import (
	"context"
	"errors"
	"regexp"
	"time"
)

const (
	minUsernameLength = 3
	maxUsernameLength = 20
)

var validUsername = regexp.MustCompile("^[A-Za-z0-9_-]+$")

// AccountService implements account and invitation use cases. UserService is
// deliberately a persistence contract; policy belongs here.
type AccountService struct {
	users AccountStore
}

func NewAccountService(users AccountStore) *AccountService {
	return &AccountService{users: users}
}

func ValidateUsername(username string) error {
	if username == SignupUsername || !validUsername.MatchString(username) {
		return ErrUsernameInvalid
	}
	if len(username) < minUsernameLength {
		return ErrUsernameTooShort
	}
	if len(username) > maxUsernameLength {
		return ErrUsernameTooLong
	}
	return nil
}

func (s *AccountService) RegisterUser(ctx context.Context, username string, pubKey []byte, inviteCode string) error {
	if err := ValidateUsername(username); err != nil {
		return err
	}
	if len(pubKey) == 0 {
		return ErrUsernameInvalid
	}
	invite, err := s.users.GetInvite(ctx, inviteCode)
	if err != nil {
		return err
	}
	if !invite.RevokedAt.IsZero() {
		return ErrInviteRevoked
	}
	if time.Now().After(invite.ExpiresAt) {
		return ErrInviteExpired
	}
	if invite.RedeemedBy != "" {
		return ErrInviteUsed
	}
	role := RoleMember
	if invite.IsAdmin {
		role = RoleAdmin
	}
	return s.users.CreateUser(ctx, username, pubKey, inviteCode, role)
}

func (s *AccountService) BootstrapAdmin(ctx context.Context, username string, pubKey []byte) error {
	if err := ValidateUsername(username); err != nil {
		return err
	}
	if len(pubKey) == 0 {
		return ErrUsernameInvalid
	}
	users, err := s.users.ListUsers(ctx)
	if err != nil {
		return err
	}
	for _, user := range users {
		if user.Role == RoleAdmin {
			return ErrAdminExists
		}
	}
	return s.users.BootstrapAdmin(ctx, username, pubKey)
}

func (s *AccountService) CreateInvite(ctx context.Context, code string, expiresAt time.Time, isAdmin bool) error {
	if code == "" {
		return errors.New("invite code cannot be empty")
	}
	if !expiresAt.After(time.Now()) {
		return errors.New("invite expiration must be in the future")
	}
	return s.users.CreateInvite(ctx, code, expiresAt, isAdmin)
}

func (s *AccountService) SetUserRole(ctx context.Context, username, role string) error {
	if role != RoleMember && role != RoleModerator && role != RoleAdmin {
		return ErrInvalidRole
	}
	target, err := s.users.GetUserByUsername(ctx, username)
	if err != nil {
		return err
	}
	if (target.Role == RoleAdmin) && role != RoleAdmin {
		users, err := s.users.ListUsers(ctx)
		if err != nil {
			return err
		}
		admins := 0
		for _, user := range users {
			if user.Role == RoleAdmin {
				admins++
			}
		}
		if admins <= 1 {
			return ErrLastAdmin
		}
	}
	return s.users.SetUserRole(ctx, username, role)
}

func (s *AccountService) SetUserStatus(ctx context.Context, username, status string) error {
	if status != StatusActive && status != StatusSuspended {
		return ErrInvalidStatus
	}
	target, err := s.users.GetUserByUsername(ctx, username)
	if err != nil {
		return err
	}
	if (target.Role == RoleAdmin) && status == StatusSuspended {
		users, err := s.users.ListUsers(ctx)
		if err != nil {
			return err
		}
		activeAdmins := 0
		for _, user := range users {
			if (user.Role == RoleAdmin) && user.Status != StatusSuspended {
				activeAdmins++
			}
		}
		if activeAdmins <= 1 {
			return ErrLastAdmin
		}
	}
	return s.users.SetUserStatus(ctx, username, status)
}
