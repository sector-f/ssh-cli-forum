package service

import (
	"context"
	"errors"
	"time"

)

const SignupUsername = "signup"

var (
	ErrUserExists       = errors.New("username already exists")
	ErrUsernameInvalid  = errors.New("username is not valid")
	ErrUsernameTooShort = errors.New("username is too short")
	ErrUsernameTooLong  = errors.New("username is too long")
	ErrUserNotFound     = errors.New("user not found")
	ErrUserSuspended    = errors.New("user account is suspended")

	ErrInviteUsed    = errors.New("invite code has already been used")
	ErrInviteExpired = errors.New("invite code has expired")
	ErrInviteInvalid = errors.New("invalid invite code")
	ErrInviteRevoked = errors.New("invite code has been revoked")

	ErrBoardNotFound    = errors.New("board not found")
	ErrThreadNotFound   = errors.New("topic not found")
	ErrCommentNotFound  = errors.New("reply not found")
	ErrPermissionDenied = errors.New("permission denied")
	ErrReadOnly         = errors.New("this board is read-only")
	ErrThreadLocked     = errors.New("this topic is locked")
	ErrAdminExists      = errors.New("an administrator account already exists")
	ErrLastAdmin        = errors.New("cannot remove the last administrator")
	ErrInvalidRole      = errors.New("role must be member, moderator, or admin")
	ErrInvalidStatus       = errors.New("status must be active or suspended")
	ErrInvalidBoardSlug   = errors.New("invalid board slug")
	ErrEmptyBoardName     = errors.New("board name cannot be empty")
	ErrInvalidBoardStatus = errors.New("board status must be active, read_only, or archived")
	ErrInvalidThreadStatus = errors.New("invalid topic status")
	ErrEmptyThreadTitle   = errors.New("topic title cannot be empty")
	ErrEmptyCommentBody   = errors.New("reply cannot be empty")

	ErrUnknown = errors.New("unknown error")
)

type UserService interface {
	CreateInvite(code string, expiresAt time.Time, isAdmin bool) error
	ListInvites() ([]Invite, error)
	GetInvite(code string) (Invite, error)
	RevokeInvite(code string) error

	CreateUser(username string, pubKey []byte, inviteCode, role string) error
	BootstrapAdmin(username string, pubKey []byte) error
	GetUserByID(id int) (User, error)
	GetUserByUsername(username string) (User, error)
	AuthenticateUser(username string, givenKey []byte) (bool, error)
	ListUsers() ([]User, error)
	SetUserRole(username, role string) error
	SetUserStatus(username, status string) error
}

type BoardService interface {
	// Compatibility methods retained for callers from the original prototype.
	CreateBoard(name string) error
	ListBoards() ([]BoardListing, error)
	GetBoardByID(id int) (Board, error)
	CreateThread(title string, boardID, authorID int) error
	ListThreads(boardID int) ([]ThreadListing, error)
	GetThreadByID(id int) (Thread, error)
	CreateComment(threadID, authorID int, body string) error
	ListComments(threadID int) ([]Comment, error)

	CreateBoardWithDetails(slug, name, description string, sortOrder, createdBy int) error
	GetBoardBySlug(slug string) (Board, error)
	UpdateBoard(slug, name, description string) error
	SetBoardStatus(slug, status string) error

	CreateThreadWithBody(title, body string, boardID, authorID int) (int, error)
	ListThreadsPage(boardID, page, pageSize int) ([]ThreadListing, int, error)
	RecentThreads(limit int) ([]ThreadListing, error)
	UpdateThread(id, actorID int, title, body string, moderator bool) error
	DeleteThread(id, actorID int, moderator bool, reason string) error
	SetThreadPinned(id, actorID int, pinned bool, reason string) error
	SetThreadStatus(id, actorID int, status, reason string) error

	ListCommentsPage(threadID, page, pageSize int) ([]Comment, int, error)
	GetCommentByID(id int) (Comment, error)
	UpdateComment(id, actorID int, body string, moderator bool) error
	DeleteComment(id, actorID int, moderator bool, reason string) error
	ListModerationLog(limit int) ([]ModerationEntry, error)
}

// ChatService provides persistent channel history and independent live-event
// subscriptions. Consumers may subscribe independently of their current view.
type ChatStore interface {
	RecentMessages(channelSlug string, limit int) ([]ChatMessage, error)
	PostMessage(channelSlug string, userID int, kind ChatMessageKind, body string) (ChatMessage, error)
}

type ChatService interface {
	ChatStore
	Subscribe(ctx context.Context, channelSlug string) (<-chan ChatMessage, func())
}
