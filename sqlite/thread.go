package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/sector-f/ssh-cli-forum/forum"
)

func (s *Service) CreateThread(title string, boardID, authorID int) error {
	_, err := s.CreateThreadWithBody(title, "", boardID, authorID)
	return err
}

func (s *Service) CreateThreadWithBody(title, body string, boardID, authorID int) (int, error) {
	title = strings.TrimSpace(title)
	if title == "" { return 0, errors.New("topic title cannot be empty") }
		var boardStatus string
	if err := s.db.QueryRow("SELECT status FROM boards WHERE id = ?", boardID).Scan(&boardStatus); errors.Is(err, sql.ErrNoRows) {
		return 0, forum.ErrBoardNotFound
	} else if err != nil { return 0, err }
	if boardStatus != forum.BoardActive { return 0, forum.ErrReadOnly }
	res, err := s.db.Exec(`INSERT INTO threads (title, board_id, author_id, body, created_at, updated_at, last_activity_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, title, boardID, authorID, body, nowText(), nowText(), nowText())
	if err != nil { return 0, err }
	id, err := res.LastInsertId()
	return int(id), err
}

const threadSelect = `SELECT t.id, t.title, t.board_id, t.author_id, t.body, t.is_pinned, t.status,
	t.created_at, t.updated_at, t.last_activity_at,
	u.username, u.display_name, u.role, u.status, u.is_admin, u.id, u.created_at, u.last_seen_at,
	(SELECT COUNT(*) FROM comments c WHERE c.thread_id = t.id AND c.deleted_at IS NULL)
	FROM threads t JOIN users u ON u.id = t.author_id`

func scanThreadListing(row interface{ Scan(...interface{}) error }) (forum.ThreadListing, error) {
	var t forum.ThreadListing
	var boardID, authorID int
	var created, updated, activity, userCreated, userLast sql.NullString
	var role, userStatus, display string
	var isAdmin, pinned bool
	var authorIDDB int
	err := row.Scan(&t.ID, &t.Title, &boardID, &authorID, &t.Body, &pinned, &t.Status,
		&created, &updated, &activity, &t.Author.Username, &display, &role, &userStatus, &isAdmin,
		&authorIDDB, &userCreated, &userLast, &t.ReplyCount)
	if err != nil { return forum.ThreadListing{}, err }
	t.IsPinned = pinned
	t.CreatedAt, t.LastActivityAt = parseTime(created.String), parseTime(activity.String)
	t.Author.ID, t.Author.DisplayName, t.Author.Role, t.Author.Status = authorIDDB, display, role, userStatus
	t.Author.IsAdmin = isAdmin || role == forum.RoleAdmin
	t.Author.CreatedAt, t.Author.LastSeenAt = parseTime(userCreated.String), parseTime(userLast.String)
	_ = authorID
	_ = updated
	return t, nil
}

func (s *Service) GetThreadByID(id int) (forum.Thread, error) {
	row := s.db.QueryRow(threadSelect+` WHERE t.id = ? AND t.deleted_at IS NULL`, id)
	listing, err := scanThreadListing(row)
	if errors.Is(err, sql.ErrNoRows) { return forum.Thread{}, forum.ErrThreadNotFound }
	if err != nil { return forum.Thread{}, err }
	var boardID int
	var updated sql.NullString
	if err := s.db.QueryRow("SELECT board_id, updated_at FROM threads WHERE id = ?", id).Scan(&boardID, &updated); err != nil { return forum.Thread{}, err }
	comments, err := s.ListComments(id)
	if err != nil { return forum.Thread{}, err }
	return forum.Thread{
		ID: listing.ID, BoardID: boardID, Title: listing.Title, Author: listing.Author,
		Body: listing.Body, IsPinned: listing.IsPinned, Status: listing.Status,
		CreatedAt: listing.CreatedAt, UpdatedAt: parseTime(updated.String),
		LastActivityAt: listing.LastActivityAt, Comments: comments,
	}, nil
}

func (s *Service) ListThreads(boardID int) ([]forum.ThreadListing, error) {
	var all []forum.ThreadListing
	for page := 1; ; page++ {
		threads, total, err := s.ListThreadsPage(boardID, page, 100)
		if err != nil { return nil, err }
		all = append(all, threads...)
		if len(all) >= total || len(threads) == 0 { return all, nil }
	}
}

func (s *Service) ListThreadsPage(boardID, page, pageSize int) ([]forum.ThreadListing, int, error) {
	if page < 1 { page = 1 }
	if pageSize < 1 || pageSize > 100 { pageSize = 20 }
	where := " WHERE t.deleted_at IS NULL AND t.status <> 'hidden' "
	args := []interface{}{}
	if boardID > 0 { where += " AND t.board_id = ?"; args = append(args, boardID) }
	var total int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM threads t"+where, args...).Scan(&total); err != nil { return nil, 0, err }
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := s.db.Query(threadSelect+where+" ORDER BY t.is_pinned DESC, t.last_activity_at DESC, t.id DESC LIMIT ? OFFSET ?", args...)
	if err != nil { return nil, 0, err }
	defer rows.Close()
	var threads []forum.ThreadListing
	for rows.Next() {
		t, err := scanThreadListing(rows)
		if err != nil { return nil, 0, err }
		threads = append(threads, t)
	}
	return threads, total, rows.Err()
}

func (s *Service) RecentThreads(limit int) ([]forum.ThreadListing, error) {
	if limit < 1 || limit > 100 { limit = 20 }
	rows, err := s.db.Query(threadSelect+` WHERE t.deleted_at IS NULL AND t.status <> 'hidden'
		ORDER BY t.last_activity_at DESC, t.id DESC LIMIT ?`, limit)
	if err != nil { return nil, err }
	defer rows.Close()
	var threads []forum.ThreadListing
	for rows.Next() {
		t, err := scanThreadListing(rows)
		if err != nil { return nil, err }
		threads = append(threads, t)
	}
	return threads, rows.Err()
}

func (s *Service) UpdateThread(id, actorID int, title, body string, moderator bool) error {
	var owner int
	if err := s.db.QueryRow("SELECT author_id FROM threads WHERE id = ? AND deleted_at IS NULL", id).Scan(&owner); errors.Is(err, sql.ErrNoRows) { return forum.ErrThreadNotFound } else if err != nil { return err }
	if owner != actorID && !moderator { return forum.ErrPermissionDenied }
	title = strings.TrimSpace(title)
	if title == "" { return errors.New("topic title cannot be empty") }
	_, err := s.db.Exec("UPDATE threads SET title = ?, body = ?, updated_at = ? WHERE id = ?", title, body, nowText(), id)
	return err
}

func (s *Service) DeleteThread(id, actorID int, moderator bool, reason string) error {
	tx, err := s.db.Begin()
	if err != nil { return err }
	defer tx.Rollback()
	var owner int
	if err := tx.QueryRow("SELECT author_id FROM threads WHERE id = ? AND deleted_at IS NULL", id).Scan(&owner); errors.Is(err, sql.ErrNoRows) { return forum.ErrThreadNotFound } else if err != nil { return err }
	if owner != actorID && !moderator { return forum.ErrPermissionDenied }
	if _, err := tx.Exec("UPDATE threads SET deleted_at = ?, status = 'hidden', updated_at = ? WHERE id = ?", nowText(), nowText(), id); err != nil { return err }
	if moderator {
		if err := addModlog(tx, actorID, "thread_deleted", "thread", id, reason); err != nil { return err }
	}
	return tx.Commit()
}

func (s *Service) SetThreadPinned(id, actorID int, pinned bool, reason string) error {
	tx, err := s.db.Begin()
	if err != nil { return err }
	defer tx.Rollback()
	res, err := tx.Exec("UPDATE threads SET is_pinned = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL", pinned, nowText(), id)
	if err != nil { return err }
	n, err := res.RowsAffected()
	if err != nil { return err }
	if n == 0 { return forum.ErrThreadNotFound }
	action := "thread_unpinned"
	if pinned { action = "thread_pinned" }
	if err := addModlog(tx, actorID, action, "thread", id, reason); err != nil { return err }
	return tx.Commit()
}

func (s *Service) SetThreadStatus(id, actorID int, status, reason string) error {
	if status != forum.ThreadOpen && status != forum.ThreadLocked && status != forum.ThreadHidden { return fmt.Errorf("invalid topic status %q", status) }
	tx, err := s.db.Begin()
	if err != nil { return err }
	defer tx.Rollback()
	res, err := tx.Exec("UPDATE threads SET status = ?, deleted_at = CASE WHEN ? = 'open' THEN NULL ELSE deleted_at END, updated_at = ? WHERE id = ? AND (deleted_at IS NULL OR ? = 'open')", status, status, nowText(), id, status)
	if err != nil { return err }
	n, err := res.RowsAffected()
	if err != nil { return err }
	if n == 0 { return forum.ErrThreadNotFound }
	if err := addModlog(tx, actorID, "thread_"+status, "thread", id, reason); err != nil { return err }
	return tx.Commit()
}
