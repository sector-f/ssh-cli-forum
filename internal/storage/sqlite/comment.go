package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/sector-f/ssh-cli-forum/internal/service"
)

func (s *Store) CreateComment(ctx context.Context, threadID, authorID int, body string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := nowText()
	if _, err := tx.ExecContext(ctx, "INSERT INTO comments (thread_id, author_id, body, posted_at) VALUES (?, ?, ?, ?)", threadID, authorID, body, now); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE threads SET last_activity_at = ? WHERE id = ?", now, threadID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ListCommentsPage(ctx context.Context, threadID, page, pageSize int) ([]service.Comment, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM comments WHERE thread_id = ? AND deleted_at IS NULL", threadID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT c.id, c.thread_id, c.author_id, c.body, c.posted_at,
		u.username, u.display_name, u.role, u.status, u.created_at, u.last_seen_at
		FROM comments c JOIN users u ON u.id = c.author_id
		WHERE c.thread_id = ? AND c.deleted_at IS NULL
		ORDER BY c.posted_at ASC, c.id ASC LIMIT ? OFFSET ?`, threadID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var comments []service.Comment
	for rows.Next() {
		var c service.Comment
		var authorID int
		var posted, created, last sql.NullString
		var display, role, status string
		if err := rows.Scan(&c.ID, &c.ThreadID, &authorID, &c.Body, &posted,
			&c.Author.Username, &display, &role, &status, &created, &last); err != nil {
			return nil, 0, err
		}
		c.PostedAt = parseTime(posted.String)
		c.Author.ID, c.Author.DisplayName, c.Author.Role, c.Author.Status = authorID, display, role, status
		c.Author.CreatedAt, c.Author.LastSeenAt = parseTime(created.String), parseTime(last.String)
		comments = append(comments, c)
	}
	return comments, total, rows.Err()
}

func (s *Store) DeleteComment(ctx context.Context, id int) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var threadID int
	err = tx.QueryRowContext(ctx, "SELECT thread_id FROM comments WHERE id = ? AND deleted_at IS NULL", id).Scan(&threadID)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrCommentNotFound
	}
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE comments SET deleted_at = ? WHERE id = ?", nowText(), id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE threads SET last_activity_at = ? WHERE id = ?", nowText(), threadID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) GetCommentByID(ctx context.Context, id int) (service.Comment, error) {
	var c service.Comment
	var authorID int
	var posted, created, last sql.NullString
	var display, role, status string
	err := s.db.QueryRowContext(ctx, `SELECT c.id, c.thread_id, c.author_id, c.body, c.posted_at,
		u.username, u.display_name, u.role, u.status, u.created_at, u.last_seen_at
		FROM comments c JOIN users u ON u.id = c.author_id
		WHERE c.id = ? AND c.deleted_at IS NULL`, id).Scan(
		&c.ID, &c.ThreadID, &authorID, &c.Body, &posted,
		&c.Author.Username, &display, &role, &status, &created, &last)
	if errors.Is(err, sql.ErrNoRows) {
		return service.Comment{}, service.ErrCommentNotFound
	}
	if err != nil {
		return service.Comment{}, err
	}
	c.PostedAt = parseTime(posted.String)
	c.Author.ID, c.Author.DisplayName, c.Author.Role, c.Author.Status = authorID, display, role, status
	c.Author.CreatedAt, c.Author.LastSeenAt = parseTime(created.String), parseTime(last.String)
	return c, nil
}
