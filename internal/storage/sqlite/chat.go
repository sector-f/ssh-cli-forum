package sqlite

import (
	"context"
	"database/sql"
	"strings"
	"unicode"

	"github.com/sector-f/ssh-cli-forum/internal/service"
)

const maxChatHistoryLimit = 1000

// RecentMessages reads durable history and returns messages in chronological order.
func (s *Service) RecentMessages(channelSlug string, limit int) ([]service.ChatMessage, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > maxChatHistoryLimit {
		limit = maxChatHistoryLimit
	}

	rows, err := s.db.Query("SELECT m.id, c.slug, u.id, u.username, m.kind, m.body, m.created_at "+
		"FROM chat_messages m JOIN chat_channels c ON c.id = m.channel_id "+
		"JOIN users u ON u.id = m.user_id WHERE c.slug = ? ORDER BY m.id DESC LIMIT ?",
		channelSlug, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := make([]service.ChatMessage, 0, limit)
	for rows.Next() {
		var message service.ChatMessage
		var createdAt string
		if err := rows.Scan(&message.ID, &message.ChannelSlug, &message.UserID,
			&message.Username, &message.Kind, &message.Text, &createdAt); err != nil {
			return nil, err
		}
		message.Timestamp = parseTime(createdAt)
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
		messages[left], messages[right] = messages[right], messages[left]
	}
	return messages, nil
}

// PostMessage persists a message before publishing it to live subscribers.
// SQLite remains the source of truth; the in-memory hub is only an event path.
func (s *Service) PostMessage(channelSlug string, userID int, kind service.ChatMessageKind, body string) (service.ChatMessage, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return service.ChatMessage{}, service.ErrChatMessageEmpty
	}
	if len([]rune(body)) > service.ChatMessageMaxLength {
		return service.ChatMessage{}, service.ErrChatMessageTooLong
	}
	for _, r := range body {
		if unicode.IsControl(r) {
			return service.ChatMessage{}, service.ErrChatMessageInvalid
		}
	}
	if kind != service.ChatMessageText && kind != service.ChatMessageAction {
		return service.ChatMessage{}, service.ErrChatMessageInvalid
	}

	tx, err := s.db.Begin()
	if err != nil {
		return service.ChatMessage{}, err
	}
	defer tx.Rollback()

	var channelID int64
	var requiresMembership bool
	if err := tx.QueryRow("SELECT id, requires_membership FROM chat_channels WHERE slug = ?", channelSlug).
		Scan(&channelID, &requiresMembership); err != nil {
		if err == sql.ErrNoRows {
			return service.ChatMessage{}, service.ErrChatChannelNotFound
		}
		return service.ChatMessage{}, err
	}

	var userStatus string
	if err := tx.QueryRow("SELECT status FROM users WHERE id = ?", userID).Scan(&userStatus); err != nil {
		if err == sql.ErrNoRows {
			return service.ChatMessage{}, service.ErrUserNotFound
		}
		return service.ChatMessage{}, err
	}
	if userStatus == service.StatusSuspended {
		return service.ChatMessage{}, service.ErrUserSuspended
	}

	if requiresMembership {
		var isMember bool
		if err := tx.QueryRow("SELECT EXISTS(SELECT 1 FROM chat_channel_memberships "+
			"WHERE channel_id = ? AND user_id = ? AND left_at IS NULL)", channelID, userID).Scan(&isMember); err != nil {
			return service.ChatMessage{}, err
		}
		if !isMember {
			return service.ChatMessage{}, service.ErrPermissionDenied
		}
	}

	createdAt := nowText()
	result, err := tx.Exec("INSERT INTO chat_messages(channel_id, user_id, kind, body, created_at) VALUES (?, ?, ?, ?, ?)",
		channelID, userID, string(kind), body, createdAt)
	if err != nil {
		return service.ChatMessage{}, err
	}
	messageID, err := result.LastInsertId()
	if err != nil {
		return service.ChatMessage{}, err
	}

	var message service.ChatMessage
	var storedAt string
	err = tx.QueryRow("SELECT m.id, c.slug, u.id, u.username, m.kind, m.body, m.created_at "+
		"FROM chat_messages m JOIN chat_channels c ON c.id = m.channel_id "+
		"JOIN users u ON u.id = m.user_id WHERE m.id = ?", messageID).Scan(
		&message.ID, &message.ChannelSlug, &message.UserID,
		&message.Username, &message.Kind, &message.Text, &storedAt)
	if err != nil {
		return service.ChatMessage{}, err
	}
	message.Timestamp = parseTime(storedAt)

	if err := tx.Commit(); err != nil {
		return service.ChatMessage{}, err
	}
	s.chatHub.Publish(message)
	return message, nil
}

// Subscribe registers a channel listener. The SSH session context ensures that
// subscribers are released automatically when their connection ends.
func (s *Service) Subscribe(ctx context.Context, channelSlug string) (<-chan service.ChatMessage, func()) {
	return s.chatHub.Subscribe(ctx, channelSlug)
}
