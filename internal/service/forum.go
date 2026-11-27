package service

import "time"

const (
	RoleMember    = "member"
	RoleModerator = "moderator"
	RoleAdmin     = "admin"

	StatusActive    = "active"
	StatusSuspended = "suspended"

	BoardActive   = "active"
	BoardReadOnly = "read_only"
	BoardArchived = "archived"

	ThreadOpen   = "open"
	ThreadLocked = "locked"
	ThreadHidden = "hidden"
)

type Invite struct {
	Code       string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	IsAdmin    bool
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
	IsPinned       bool
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	LastActivityAt time.Time
}

type Thread struct {
	ID             int
	BoardID        int
	Title          string
	Author         User
	Body           string
	IsPinned       bool
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	LastActivityAt time.Time
}

type Comment struct {
	ID        int
	ThreadID  int
	Author    User
	Body      string
	PostedAt  time.Time
	UpdatedAt time.Time
}

type ModerationEntry struct {
	ID         int
	Actor      User
	Action     string
	TargetType string
	TargetID   int
	Reason     string
	CreatedAt  time.Time
}

type Message interface {
	Time() time.Time
	Body() string
}
