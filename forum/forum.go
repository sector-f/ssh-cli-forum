package forum

import (
	"time"

	"github.com/charmbracelet/ssh"
)

type Invite struct {
	Code      string
	CreatedAt time.Time
	ExpiresAt time.Time
	IsAdmin   bool
}

type User struct {
	Username string
	PubKey   ssh.PublicKey
	IsAdmin  bool
}

type Board struct {
	Name    string
	Threads []Thread
}

type Thread struct {
	Title  string
	Author User
	// TODO: add body here
	Comments []Comment
}

type Comment struct {
	Author   User
	Body     string
	PostedAt time.Time
}
