package sqlite

import (
	"context"
	"database/sql"

	"github.com/sector-f/ssh-cli-forum/internal/service"
)

const maxChatHistoryLimit = 1000

// RecentMessages reads durable history and returns messages in chronological order.
func (s *Store) RecentMessages(ctx context.Context, channelSlug string, limit int) ([]service.ChatMessage, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > maxChatHistoryLimit {
		limit = maxChatHistoryLimit
	}

	rows, err := s.db.QueryContext(ctx, "SELECT m.id, c.slug, u.id, u.username, m.body, m.created_at "+
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
			&message.Username, &message.Text, &createdAt); err != nil {
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

// PostMessage persists a message and returns the stored representation.
func (s *Store) PostMessage(ctx context.Context, channelSlug string, userID int, body string) (service.ChatMessage, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return service.ChatMessage{}, err
	}
	defer tx.Rollback()

	var channelID int64
	if err := tx.QueryRowContext(ctx, "SELECT id FROM chat_channels WHERE slug = ?", channelSlug).
		Scan(&channelID); err != nil {
		if err == sql.ErrNoRows {
			return service.ChatMessage{}, service.ErrChatChannelNotFound
		}
		return service.ChatMessage{}, err
	}

	createdAt := nowText()
	result, err := tx.ExecContext(ctx, "INSERT INTO chat_messages(channel_id, user_id, body, created_at) VALUES (?, ?, ?, ?)",
		channelID, userID, body, createdAt)
	if err != nil {
		return service.ChatMessage{}, err
	}
	messageID, err := result.LastInsertId()
	if err != nil {
		return service.ChatMessage{}, err
	}

	var message service.ChatMessage
	var storedAt string
	err = tx.QueryRowContext(ctx, "SELECT m.id, c.slug, u.id, u.username, m.body, m.created_at "+
		"FROM chat_messages m JOIN chat_channels c ON c.id = m.channel_id "+
		"JOIN users u ON u.id = m.user_id WHERE m.id = ?", messageID).Scan(
		&message.ID, &message.ChannelSlug, &message.UserID,
		&message.Username, &message.Text, &storedAt)
	if err != nil {
		return service.ChatMessage{}, err
	}
	message.Timestamp = parseTime(storedAt)

	if err := tx.Commit(); err != nil {
		return service.ChatMessage{}, err
	}
	return message, nil
}
