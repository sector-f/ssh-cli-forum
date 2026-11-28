package service

import (
	"errors"
	"time"
)

const (
	GlobalChatChannel    = "global"
	ChatMessageMaxLength = 1000
)

var (
	ErrChatChannelNotFound = errors.New("chat channel not found")
	ErrChatMessageEmpty    = errors.New("chat message cannot be empty")
	ErrChatMessageTooLong  = errors.New("chat message is too long")
	ErrChatMessageInvalid  = errors.New("chat message must be a single line")
)

// ChatMessage is a durable message in a channel. The message ID is globally
// unique within the database and can be used for pagination and read markers.
type ChatMessage struct {
	ID          int64
	ChannelSlug string
	UserID      int
	Username    string
	Text        string
	Timestamp   time.Time
}

func (m ChatMessage) Time() time.Time { return m.Timestamp }

func (m ChatMessage) DisplayText() string {
	return m.Username + ": " + m.Text
}

func (m ChatMessage) Body() string { return m.DisplayText() }
