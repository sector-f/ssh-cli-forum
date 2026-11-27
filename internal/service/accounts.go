package service

import (
	"errors"
	"regexp"
	"time"

	"github.com/charmbracelet/ssh"
)

const (
	minUsernameLength = 3
	maxUsernameLength = 20
)

var validUsername = regexp.MustCompile("^[A-Za-z0-9_-]+$")

// AccountService implements account and invitation use cases. UserService is
// deliberately a persistence contract; policy belongs here.
type AccountService struct {
	users UserService
}

func NewAccountService(users UserService) *AccountService {
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

func (s *AccountService) RegisterUser(username string, pubKey ssh.PublicKey, inviteCode string) error {
	if err := ValidateUsername(username); err != nil {
		return err
	}
	if pubKey == nil {
		return ErrUsernameInvalid
	}
	invite, err := s.users.GetInvite(inviteCode)
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
	return s.users.CreateUser(username, pubKey, inviteCode, role)
}

func (s *AccountService) BootstrapAdmin(username string, pubKey ssh.PublicKey) error {
	if err := ValidateUsername(username); err != nil {
		return err
	}
	if pubKey == nil {
		return ErrUsernameInvalid
	}
	users, err := s.users.ListUsers()
	if err != nil {
		return err
	}
	for _, user := range users {
		if user.Role == RoleAdmin || user.IsAdmin {
			return ErrAdminExists
		}
	}
	return s.users.BootstrapAdmin(username, pubKey)
}

func (s *AccountService) CreateInvite(code string, expiresAt time.Time, isAdmin bool) error {
	if code == "" {
		return errors.New("invite code cannot be empty")
	}
	if !expiresAt.After(time.Now()) {
		return errors.New("invite expiration must be in the future")
	}
	return s.users.CreateInvite(code, expiresAt, isAdmin)
}

func (s *AccountService) SetUserRole(username, role string) error {
	if role != RoleMember && role != RoleModerator && role != RoleAdmin {
		return ErrInvalidRole
	}
	target, err := s.users.GetUserByUsername(username)
	if err != nil {
		return err
	}
	if (target.Role == RoleAdmin || target.IsAdmin) && role != RoleAdmin {
		users, err := s.users.ListUsers()
		if err != nil {
			return err
		}
		admins := 0
		for _, user := range users {
			if user.Role == RoleAdmin || user.IsAdmin {
				admins++
			}
		}
		if admins <= 1 {
			return ErrLastAdmin
		}
	}
	return s.users.SetUserRole(username, role)
}

func (s *AccountService) SetUserStatus(username, status string) error {
	if status != StatusActive && status != StatusSuspended {
		return ErrInvalidStatus
	}
	target, err := s.users.GetUserByUsername(username)
	if err != nil {
		return err
	}
	if (target.Role == RoleAdmin || target.IsAdmin) && status == StatusSuspended {
		users, err := s.users.ListUsers()
		if err != nil {
			return err
		}
		activeAdmins := 0
		for _, user := range users {
			if (user.Role == RoleAdmin || user.IsAdmin) && user.Status != StatusSuspended {
				activeAdmins++
			}
		}
		if activeAdmins <= 1 {
			return ErrLastAdmin
		}
	}
	return s.users.SetUserStatus(username, status)
}
