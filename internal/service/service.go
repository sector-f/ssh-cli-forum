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
	ErrAdminExists      = errors.New("an administrator account already exists")
	ErrLastAdmin        = errors.New("cannot remove the last administrator")
	ErrInvalidStatus       = errors.New("status must be active or suspended")
	ErrInvalidBoardSlug   = errors.New("invalid board slug")
	ErrEmptyBoardName     = errors.New("board name cannot be empty")
	ErrInvalidBoardStatus = errors.New("board status must be active, read_only, or archived")
	ErrEmptyThreadTitle   = errors.New("topic title cannot be empty")
	ErrEmptyCommentBody   = errors.New("reply cannot be empty")

	ErrUnknown = errors.New("unknown error")
)

type AccountStore interface {
	CreateInvite(ctx context.Context, code string, expiresAt time.Time) error
	GetInvite(ctx context.Context, code string) (Invite, error)
	CreateUser(ctx context.Context, username string, pubKey []byte, inviteCode, role string) error
	BootstrapAdmin(ctx context.Context, username string, pubKey []byte) error
	GetUserByUsername(ctx context.Context, username string) (User, error)
	ListUsers(ctx context.Context) ([]User, error)
	SetUserStatus(ctx context.Context, username, status string) error
}

type UserService interface {
	AccountStore
	ListInvites(ctx context.Context) ([]Invite, error)
	RevokeInvite(ctx context.Context, code string) error
	GetUserByID(ctx context.Context, id int) (User, error)
	AuthenticateUser(ctx context.Context, username string, givenKey []byte) (bool, error)
}
type BoardAdminStore interface {
	CreateBoardWithDetails(ctx context.Context, slug, name, description string, sortOrder, createdBy int) error
	SetBoardStatus(ctx context.Context, slug, status string) error
}

type BoardService interface {
	BoardAdminStore
	ListBoards(ctx context.Context) ([]BoardListing, error)
	GetBoardByID(ctx context.Context, id int) (Board, error)
	GetBoardBySlug(ctx context.Context, slug string) (Board, error)
	GetThreadByID(ctx context.Context, id int) (Thread, error)
	CreateComment(ctx context.Context, threadID, authorID int, body string) error
	CreateThreadWithBody(ctx context.Context, title, body string, boardID, authorID int) (int, error)
	ListThreadsPage(ctx context.Context, boardID, page, pageSize int) ([]ThreadListing, int, error)
	RecentThreads(ctx context.Context, limit int) ([]ThreadListing, error)
	DeleteThread(ctx context.Context, id int) error
	ListCommentsPage(ctx context.Context, threadID, page, pageSize int) ([]Comment, int, error)
	GetCommentByID(ctx context.Context, id int) (Comment, error)
	DeleteComment(ctx context.Context, id int) error
}
// ChatService provides persistent channel history and independent live-event
// subscriptions. Consumers may subscribe independently of their current view.
type UserSession interface {
	GetUserByID(ctx context.Context, id int) (User, error)
	GetUserByUsername(ctx context.Context, username string) (User, error)
	ListInvites(ctx context.Context) ([]Invite, error)
	RevokeInvite(ctx context.Context, code string) error
	ListUsers(ctx context.Context) ([]User, error)
	CreateInvite(ctx context.Context, code string, expiresAt time.Time, isAdmin bool) error
	SetUserRole(ctx context.Context, username, role string) error
	SetUserStatus(ctx context.Context, username, status string) error
}

// BoardSession is the application-facing forum contract. Session implements
// its mutations with actor authorization rather than exposing the store.
type BoardSession interface {
	BoardService
}

type ChatStore interface {
	RecentMessages(ctx context.Context, channelSlug string, limit int) ([]ChatMessage, error)
	PostMessage(ctx context.Context, channelSlug string, userID int, kind ChatMessageKind, body string) (ChatMessage, error)
}

type ChatService interface {
	ChatStore
	Subscribe(ctx context.Context, channelSlug string) (<-chan ChatMessage, func())
}
