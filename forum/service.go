package forum

import (
	"errors"
	"time"

	"github.com/charmbracelet/ssh"
)

var (
	ErrUserExists    = errors.New("username already exists")
	ErrInviteUsed    = errors.New("invite code has already been used")
	ErrInviteExpired = errors.New("invite code has expired")
	ErrInviteInvalid = errors.New("invalid invite code")
	ErrUnknown       = errors.New("unknown error")
)

type Service interface {
	// Invites
	CreateInvite(code string, expiresAt time.Time, isAdmin bool) error
	ListInvites() ([]Invite, error)
	GetInvite(code string) (Invite, error)

	// Users
	CreateUser(username string, pubKey ssh.PublicKey, inviteCode string) error
	GetUserByID(id int) (User, error)
	GetUserByUsername(username string) (User, error)
	ListUsers() ([]User, error)

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
