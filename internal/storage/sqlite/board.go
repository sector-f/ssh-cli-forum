package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/sector-f/ssh-cli-forum/internal/service"
)

func (s *Store) CreateBoard(ctx context.Context, name string) error {
	slug := slugify(name)
	if slug == "" {
		return errors.New("board name must contain letters or numbers")
	}
	return s.CreateBoardWithDetails(slug, name, "", 0, 0)
}

func (s *Store) CreateBoardWithDetails(ctx context.Context, slug, name, description string, sortOrder, createdBy int) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO boards (slug, name, description, sort_order, status, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, 'active', NULLIF(?, 0), ?, ?)`, slug, name, description, sortOrder, createdBy, nowText(), nowText())
	return err
}

func (s *Store) ListBoards(ctx context.Context) ([]service.BoardListing, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, slug, name, description, sort_order, status FROM boards ORDER BY sort_order, name COLLATE NOCASE")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var boards []service.BoardListing
	for rows.Next() {
		var b service.BoardListing
		if err := rows.Scan(&b.ID, &b.Slug, &b.Name, &b.Description, &b.SortOrder, &b.Status); err != nil {
			return nil, err
		}
		boards = append(boards, b)
	}
	return boards, rows.Err()
}

func (s *Store) GetBoardByID(ctx context.Context, id int) (service.Board, error) {
	return s.getBoard(ctx, "id", id)
}

func (s *Store) GetBoardBySlug(ctx context.Context, slug string) (service.Board, error) {
	return s.getBoard(ctx, "slug", slug)
}

func (s *Store) getBoard(ctx context.Context, column string, value interface{}) (service.Board, error) {
	if column != "id" && column != "slug" {
		return service.Board{}, errors.New("invalid board lookup")
	}
	var b service.Board
	err := s.db.QueryRowContext(ctx, fmt.Sprintf("SELECT id, slug, name, description, status FROM boards WHERE %s = ?", column), value).
		Scan(&b.ID, &b.Slug, &b.Name, &b.Description, &b.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return service.Board{}, service.ErrBoardNotFound
	}
	if err != nil {
		return service.Board{}, err
	}
	return b, nil
}

func (s *Store) UpdateBoard(ctx context.Context, slug, name, description string) error {
	res, err := s.db.ExecContext(ctx, "UPDATE boards SET name = ?, description = ?, updated_at = ? WHERE slug = ?", name, description, nowText(), slug)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err == nil && n == 0 {
		return service.ErrBoardNotFound
	}
	return err
}

func (s *Store) SetBoardStatus(ctx context.Context, slug, status string) error {
	res, err := s.db.ExecContext(ctx, "UPDATE boards SET status = ?, updated_at = ? WHERE slug = ?", status, nowText(), slug)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err == nil && n == 0 {
		return service.ErrBoardNotFound
	}
	return err
}
