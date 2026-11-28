package service

import "time"

const (
	RoleMember = "member"
	RoleAdmin  = "admin"

	StatusActive    = "active"
	StatusSuspended = "suspended"

	BoardActive   = "active"
	BoardReadOnly = "read_only"
	BoardArchived = "archived"
)

type Invite struct {
	Code       string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	RedeemedBy string
	RedeemedAt time.Time
	RevokedAt  time.Time
}

type User struct {
	ID          int
	Username    string
	DisplayName string
	Role        string
	Status      string
	CreatedAt   time.Time
	LastSeenAt  time.Time
}

type BoardListing struct {
	ID          int
	Slug        string
	Name        string
	Description string
	SortOrder   int
	Status      string
}

type Board struct {
	ID          int
	Slug        string
	Name        string
	Description string
	Status      string
}

type ThreadListing struct {
	ID             int
	BoardID        int
	Title          string
	Author         User
	Body           string
	ReplyCount     int
	CreatedAt      time.Time
	LastActivityAt time.Time
}

type Thread struct {
	ID             int
	BoardID        int
	Title          string
	Author         User
	Body           string
	CreatedAt      time.Time
	LastActivityAt time.Time
}

type Comment struct {
	ID       int
	ThreadID int
	Author   User
	Body     string
	PostedAt time.Time
}
