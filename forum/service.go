package forum

import (
	"errors"
	"time"

	"github.com/charmbracelet/ssh"
)

var (
	SignupUsername = "signup"

	ErrUserExists       = errors.New("username already exists")
	ErrUsernameInvalid  = errors.New("username is not valid")
	ErrUsernameTooShort = errors.New("username is too short")
	ErrUsernameTooLong  = errors.New("username is too long")

	ErrInviteUsed    = errors.New("invite code has already been used")
	ErrInviteExpired = errors.New("invite code has expired")
	ErrInviteInvalid = errors.New("invalid invite code")

	ErrUnknown = errors.New("unknown error")
)

type UserService interface {
	// Invites
	CreateInvite(code string, expiresAt time.Time, isAdmin bool) error
	ListInvites() ([]Invite, error)
	GetInvite(code string) (Invite, error)

	// Users
	CreateUser(username string, pubKey ssh.PublicKey, inviteCode string) error
	GetUserByID(id int) (User, error)
	GetUserByUsername(username string) (User, error)
	AuthenticateUser(username string, givenKey ssh.PublicKey) bool
	ListUsers() ([]User, error)
}

type BoardService interface {
	// Boards
	CreateBoard(name string) error
	ListBoards() ([]BoardListing, error)
	GetBoardByID(id int) (Board, error)

	// Threads
	CreateThread(title string, boardID int, authorID int) error
	ListThreads(boardID int) ([]ThreadListing, error)
	GetThreadByID(id int) (Thread, error)

	// Comments
	CreateComment(threadID int, authorID int, body string) error
	ListComments(threadID int) ([]Comment, error)
}

type ChatroomService interface {
	ConnectUser(session ssh.Session, callback func(msg Message)) []Message
	ConnectService(id string, callback func(msg Message)) []Message
	DisconnectUser(id string)
	DisconnectService(id string)
	SendMsg(msg ChatMsg)
}
