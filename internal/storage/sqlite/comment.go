package sqlite

import (
	"database/sql"
	"errors"
	"time"

	"github.com/sector-f/ssh-cli-forum/internal/service"
)

func (s *Store) CreateComment(threadID, authorID int, body string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := nowText()
	if _, err := tx.Exec("INSERT INTO comments (thread_id, author_id, body, posted_at) VALUES (?, ?, ?, ?)", threadID, authorID, body, now); err != nil {
		return err
	}
	if _, err := tx.Exec("UPDATE threads SET last_activity_at = ?, updated_at = ? WHERE id = ?", now, now, threadID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ListComments(threadID int) ([]service.Comment, error) {
	var all []service.Comment
	for page := 1; ; page++ {
		comments, total, err := s.ListCommentsPage(threadID, page, 100)
		if err != nil {
			return nil, err
		}
		all = append(all, comments...)
		if len(all) >= total || len(comments) == 0 {
			return all, nil
		}
	}
}

func (s *Store) ListCommentsPage(threadID, page, pageSize int) ([]service.Comment, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}
	var total int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM comments WHERE thread_id = ? AND deleted_at IS NULL", threadID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.Query(`SELECT c.id, c.thread_id, c.author_id, c.body, c.posted_at,
		u.username, u.display_name, u.role, u.status, u.is_admin, u.created_at, u.last_seen_at
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
		var isAdmin bool
		if err := rows.Scan(&c.ID, &c.ThreadID, &authorID, &c.Body, &posted,
			&c.Author.Username, &display, &role, &status, &isAdmin, &created, &last); err != nil {
			return nil, 0, err
		}
		c.PostedAt = parseTime(posted.String)
		c.Author.ID, c.Author.DisplayName, c.Author.Role, c.Author.Status = authorID, display, role, status
		c.Author.IsAdmin = isAdmin || role == service.RoleAdmin
		c.Author.CreatedAt, c.Author.LastSeenAt = parseTime(created.String), parseTime(last.String)
		comments = append(comments, c)
	}
	return comments, total, rows.Err()
}

func (s *Store) UpdateComment(id, actorID int, body string, _ bool) error {
	_, err = s.db.Exec("UPDATE comments SET body = ?, updated_at = ? WHERE id = ?", body, nowText(), id)
	return err
}

func (s *Store) DeleteComment(id, actorID int, moderator bool, reason string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var threadID int
	err = tx.QueryRow("SELECT thread_id FROM comments WHERE id = ? AND deleted_at IS NULL", id).Scan(&threadID)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrCommentNotFound
	}
	if err != nil {
		return err
	}
	if _, err := tx.Exec("UPDATE comments SET deleted_at = ? WHERE id = ?", nowText(), id); err != nil {
		return err
	}
	if moderator {
		if err := addModlog(tx, actorID, "reply_deleted", "comment", id, reason); err != nil {
			return err
		}
	}
	if _, err := tx.Exec("UPDATE threads SET last_activity_at = ? WHERE id = ?", nowText(), threadID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) GetCommentByID(id int) (service.Comment, error) {
	var c service.Comment
	var authorID int
	var posted, updated, created, last sql.NullString
	var display, role, status string
	var isAdmin bool
	err := s.db.QueryRow(`SELECT c.id, c.thread_id, c.author_id, c.body, c.posted_at, c.updated_at,
		u.username, u.display_name, u.role, u.status, u.is_admin, u.created_at, u.last_seen_at
		FROM comments c JOIN users u ON u.id = c.author_id
		WHERE c.id = ? AND c.deleted_at IS NULL`, id).Scan(
		&c.ID, &c.ThreadID, &authorID, &c.Body, &posted, &updated,
		&c.Author.Username, &display, &role, &status, &isAdmin, &created, &last)
	if errors.Is(err, sql.ErrNoRows) {
		return service.Comment{}, service.ErrCommentNotFound
	}
	if err != nil {
		return service.Comment{}, err
	}
	c.PostedAt, c.UpdatedAt = parseTime(posted.String), parseTime(updated.String)
	c.Author.ID, c.Author.DisplayName, c.Author.Role, c.Author.Status = authorID, display, role, status
	c.Author.IsAdmin = isAdmin || role == service.RoleAdmin
	c.Author.CreatedAt, c.Author.LastSeenAt = parseTime(created.String), parseTime(last.String)
	return c, nil
}

func (s *Store) ListModerationLog(limit int) ([]service.ModerationEntry, error) {
	if limit < 1 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.Query(`SELECT m.id, m.actor_id, u.username, m.action, m.target_type, m.target_id, m.reason, m.created_at
		FROM moderation_log m JOIN users u ON u.id = m.actor_id ORDER BY m.id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []service.ModerationEntry
	for rows.Next() {
		var e service.ModerationEntry
		var created string
		if err := rows.Scan(&e.ID, &e.Actor.ID, &e.Actor.Username, &e.Action, &e.TargetType, &e.TargetID, &e.Reason, &created); err != nil {
			return nil, err
		}
		e.CreatedAt = parseTime(created)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func addModlog(tx *sql.Tx, actorID int, action, targetType string, targetID int, reason string) error {
	_, err := tx.Exec("INSERT INTO moderation_log (actor_id, action, target_type, target_id, reason, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		actorID, action, targetType, targetID, reason, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}
