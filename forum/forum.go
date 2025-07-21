package forum

import (
	"time"
)

type Invite struct {
	Code      string
	CreatedAt time.Time
	ExpiresAt time.Time
	IsAdmin   bool
}

type User struct {
	ID       int
	Username string
	IsAdmin  bool
}

type BoardListing struct {
	ID   int
	Name string
}

// TODO: add description
type Board struct {
	ID      int
	Name    string
	Threads []ThreadListing
}

type ThreadListing struct {
	ID     int
	Title  string
	Author User
	Body   string
}

type Thread struct {
	ID       int
	Title    string
	Author   User
	Body     string
	Comments []Comment
}

type Comment struct {
	ID       int
	Author   User
	Body     string
	PostedAt time.Time
}
