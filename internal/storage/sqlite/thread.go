package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/sector-f/ssh-cli-forum/internal/service"
)

func (s *Store) CreateThreadWithBody(ctx context.Context, title, body string, boardID, authorID int) (int, error) {
	res, err := s.db.ExecContext(ctx, `INSERT INTO threads (title, board_id, author_id, body, created_at, updated_at, last_activity_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, title, boardID, authorID, body, nowText(), nowText(), nowText())
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	return int(id), err
}

const threadSelect = `SELECT t.id, t.title, t.board_id, t.body, t.is_pinned, t.status,
	t.created_at, t.updated_at, t.last_activity_at,
	u.username, u.display_name, u.role, u.status, u.id, u.created_at, u.last_seen_at,
	(SELECT COUNT(*) FROM comments c WHERE c.thread_id = t.id AND c.deleted_at IS NULL)
	FROM threads t JOIN users u ON u.id = t.author_id`

func scanThreadListing(row interface{ Scan(...interface{}) error }) (service.ThreadListing, error) {
	var t service.ThreadListing
	var boardID int
	var created, updated, activity, userCreated, userLast sql.NullString
	var role, userStatus, display string
	var pinned bool
	var authorIDDB int
	err := row.Scan(&t.ID, &t.Title, &boardID, &t.Body, &pinned, &t.Status,
		&created, &updated, &activity, &t.Author.Username, &display, &role, &userStatus,
		&authorIDDB, &userCreated, &userLast, &t.ReplyCount)
	if err != nil {
		return service.ThreadListing{}, err
	}
	t.BoardID = boardID
	t.IsPinned = pinned
	t.CreatedAt, t.UpdatedAt, t.LastActivityAt = parseTime(created.String), parseTime(updated.String), parseTime(activity.String)
	t.Author.ID, t.Author.DisplayName, t.Author.Role, t.Author.Status = authorIDDB, display, role, userStatus
	t.Author.CreatedAt, t.Author.LastSeenAt = parseTime(userCreated.String), parseTime(userLast.String)
	return t, nil
}

func (s *Store) GetThreadByID(ctx context.Context, id int) (service.Thread, error) {
	row := s.db.QueryRowContext(ctx, threadSelect+` WHERE t.id = ? AND t.deleted_at IS NULL`, id)
	listing, err := scanThreadListing(row)
	if errors.Is(err, sql.ErrNoRows) {
		return service.Thread{}, service.ErrThreadNotFound
	}
	if err != nil {
		return service.Thread{}, err
	}
	return service.Thread{
		ID: listing.ID, BoardID: listing.BoardID, Title: listing.Title, Author: listing.Author,
		Body: listing.Body, IsPinned: listing.IsPinned, Status: listing.Status,
		CreatedAt: listing.CreatedAt, UpdatedAt: listing.UpdatedAt,
		LastActivityAt: listing.LastActivityAt,
	}, nil
}

func (s *Store) ListThreadsPage(ctx context.Context, boardID, page, pageSize int) ([]service.ThreadListing, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	where := " WHERE t.deleted_at IS NULL AND t.status <> 'hidden' "
	args := []interface{}{}
	if boardID > 0 {
		where += " AND t.board_id = ?"
		args = append(args, boardID)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM threads t"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := s.db.QueryContext(ctx, threadSelect+where+" ORDER BY t.is_pinned DESC, t.last_activity_at DESC, t.id DESC LIMIT ? OFFSET ?", args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var threads []service.ThreadListing
	for rows.Next() {
		t, err := scanThreadListing(rows)
		if err != nil {
			return nil, 0, err
		}
		threads = append(threads, t)
	}
	return threads, total, rows.Err()
}

func (s *Store) RecentThreads(ctx context.Context, limit int) ([]service.ThreadListing, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, threadSelect+` WHERE t.deleted_at IS NULL AND t.status <> 'hidden'
		ORDER BY t.last_activity_at DESC, t.id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var threads []service.ThreadListing
	for rows.Next() {
		t, err := scanThreadListing(rows)
		if err != nil {
			return nil, err
		}
		threads = append(threads, t)
	}
	return threads, rows.Err()
}

func (s *Store) UpdateThread(ctx context.Context, id int, title, body string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE threads SET title = ?, body = ?, updated_at = ? WHERE id = ?", title, body, nowText(), id)
	return err
}

func (s *Store) DeleteThread(ctx context.Context, id, actorID int, reason string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "UPDATE threads SET deleted_at = ?, status = 'hidden', updated_at = ? WHERE id = ?", nowText(), nowText(), id); err != nil {
		return err
	}
	if err := addModlog(tx, actorID, "thread_deleted", "thread", id, reason); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) SetThreadPinned(ctx context.Context, id, actorID int, pinned bool, reason string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, "UPDATE threads SET is_pinned = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL", pinned, nowText(), id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return service.ErrThreadNotFound
	}
	action := "thread_unpinned"
	if pinned {
		action = "thread_pinned"
	}
	if err := addModlog(tx, actorID, action, "thread", id, reason); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) SetThreadStatus(ctx context.Context, id, actorID int, status, reason string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, "UPDATE threads SET status = ?, deleted_at = CASE WHEN ? = 'open' THEN NULL ELSE deleted_at END, updated_at = ? WHERE id = ? AND (deleted_at IS NULL OR ? = 'open')", status, status, nowText(), id, status)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return service.ErrThreadNotFound
	}
	if err := addModlog(tx, actorID, "thread_"+status, "thread", id, reason); err != nil {
		return err
	}
	return tx.Commit()
}
