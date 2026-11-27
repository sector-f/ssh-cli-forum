package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/sector-f/ssh-cli-forum/internal/service"
)

func (s *Store) CreateThreadWithBody(ctx context.Context, title, body string, boardID, authorID int) (int, error) {
	res, err := s.db.ExecContext(ctx, `INSERT INTO threads (title, board_id, author_id, body, created_at, last_activity_at)
		VALUES (?, ?, ?, ?, ?, ?)`, title, boardID, authorID, body, nowText(), nowText())
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	return int(id), err
}

const threadSelect = `SELECT t.id, t.title, t.board_id, t.body,
	t.created_at, t.last_activity_at,
	u.username, u.display_name, u.role, u.status, u.id, u.created_at, u.last_seen_at,
	(SELECT COUNT(*) FROM comments c WHERE c.thread_id = t.id AND c.deleted_at IS NULL)
	FROM threads t JOIN users u ON u.id = t.author_id`

func scanThreadListing(row interface{ Scan(...interface{}) error }) (service.ThreadListing, error) {
	var t service.ThreadListing
	var boardID int
	var created, activity, userCreated, userLast sql.NullString
	var role, userStatus, display string
	var authorID int
	err := row.Scan(&t.ID, &t.Title, &boardID, &t.Body,
		&created, &activity, &t.Author.Username, &display, &role, &userStatus,
		&authorID, &userCreated, &userLast, &t.ReplyCount)
	if err != nil {
		return service.ThreadListing{}, err
	}
	t.BoardID = boardID
	t.CreatedAt, t.LastActivityAt = parseTime(created.String), parseTime(activity.String)
	t.Author.ID, t.Author.DisplayName, t.Author.Role, t.Author.Status = authorID, display, role, userStatus
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
		ID: listing.ID, BoardID: listing.BoardID, Title: listing.Title,
		Author: listing.Author, Body: listing.Body,
		CreatedAt: listing.CreatedAt, LastActivityAt: listing.LastActivityAt,
	}, nil
}

func (s *Store) ListThreadsPage(ctx context.Context, boardID, page, pageSize int) ([]service.ThreadListing, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	where := " WHERE t.deleted_at IS NULL"
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
	rows, err := s.db.QueryContext(ctx, threadSelect+where+" ORDER BY t.last_activity_at DESC, t.id DESC LIMIT ? OFFSET ?", args...)
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
	rows, err := s.db.QueryContext(ctx, threadSelect+` WHERE t.deleted_at IS NULL
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

func (s *Store) DeleteThread(ctx context.Context, id int) error {
	result, err := s.db.ExecContext(ctx, "UPDATE threads SET deleted_at = ? WHERE id = ? AND deleted_at IS NULL", nowText(), id)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return service.ErrThreadNotFound
	}
	return nil
}
