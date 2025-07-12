package forum

import (
	"time"

	"github.com/charmbracelet/ssh"
)

type Invite struct {
	Code      string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type User struct {
	ID       int
	Username string
	PubKey   ssh.PublicKey
	IsAdmin  bool
	Invite   *Invite
}

type Board struct {
	ID      int
	Name    string
	Threads []*Thread
}

type Thread struct {
	ID       int
	Title    string
	Author   *User
	Board    *Board
	Comments []*Comment
}

type Comment struct {
	ID       int
	Author   *User
	Body     string
	PostedAt time.Time
	Thread   *Thread
}
