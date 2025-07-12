package forum

import (
	"time"

	"github.com/charmbracelet/ssh"
)

type Service interface {
	// Invites
	CreateInvite(code string, expiresAt time.Time) (*Invite, error)
	ListInvites() ([]*Invite, error)
	GetInvite(code string) (*Invite, error)

	// Users
	CreateUser(username string, pubKey ssh.PublicKey, inviteCode string, isAdmin bool) (*User, error)
	GetUserByID(id int) (*User, error)
	GetUserByUsername(username string) (*User, error)
	ListUsers() ([]*User, error)

	// Boards
	CreateBoard(name string) (*Board, error)
	ListBoards() ([]*Board, error)
	GetBoardByID(id int) (*Board, error)

	// Threads
	CreateThread(title string, boardID int, authorID int) (*Thread, error)
	ListThreads(boardID int) ([]*Thread, error)
	GetThreadByID(id int) (*Thread, error)

	// Comments
	CreateComment(threadID int, authorID int, body string) (*Comment, error)
	ListComments(threadID int) ([]*Comment, error)
	GetCommentByID(id int) (*Comment, error)
}
