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

type BoardListing struct {
	ID   int
	Name string
}

type Board struct {
	ID      int
	Name    string
	Threads []ThreadListing
}

type ThreadListing struct {
	ID     int
	Title  string
	Author User
}

type Thread struct {
	ID     int
	Title  string
	Author User
	// TODO: add body here
	Comments []Comment
}

type Comment struct {
	ID       int
	Author   User
	Body     string
	PostedAt time.Time
}
