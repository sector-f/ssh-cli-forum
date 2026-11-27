package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/sector-f/ssh-cli-forum/internal/service"
)

func (s *Service) CreateBoard(name string) error {
	slug := slugify(name)
	if slug == "" {
		return errors.New("board name must contain letters or numbers")
	}
	return s.CreateBoardWithDetails(slug, name, "", 0, 0)
}

func (s *Service) CreateBoardWithDetails(slug, name, description string, sortOrder, createdBy int) error {
	slug = slugify(slug)
	if slug == "" {
		return errors.New("invalid board slug")
	}
	if strings.TrimSpace(name) == "" {
		return errors.New("board name cannot be empty")
	}
	_, err := s.db.Exec(`INSERT INTO boards (slug, name, description, sort_order, status, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, 'active', NULLIF(?, 0), ?, ?)`, slug, strings.TrimSpace(name), description, sortOrder, createdBy, nowText(), nowText())
	return err
}

func (s *Service) ListBoards() ([]service.BoardListing, error) {
	rows, err := s.db.Query("SELECT id, slug, name, description, sort_order, status FROM boards ORDER BY sort_order, name COLLATE NOCASE")
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

func (s *Service) GetBoardByID(id int) (service.Board, error) {
	return s.getBoard("id", id)
}

func (s *Service) GetBoardBySlug(slug string) (service.Board, error) {
	return s.getBoard("slug", slug)
}

func (s *Service) getBoard(column string, value interface{}) (service.Board, error) {
	if column != "id" && column != "slug" {
		return service.Board{}, errors.New("invalid board lookup")
	}
	var b service.Board
	err := s.db.QueryRow(fmt.Sprintf("SELECT id, slug, name, description, status FROM boards WHERE %s = ?", column), value).
		Scan(&b.ID, &b.Slug, &b.Name, &b.Description, &b.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return service.Board{}, service.ErrBoardNotFound
	}
	if err != nil {
		return service.Board{}, err
	}
	return b, nil
}

func (s *Service) UpdateBoard(slug, name, description string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("board name cannot be empty")
	}
	res, err := s.db.Exec("UPDATE boards SET name = ?, description = ?, updated_at = ? WHERE slug = ?", strings.TrimSpace(name), description, nowText(), slug)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err == nil && n == 0 {
		return service.ErrBoardNotFound
	}
	return err
}

func (s *Service) SetBoardStatus(slug, status string) error {
	if status != service.BoardActive && status != service.BoardReadOnly && status != service.BoardArchived {
		return errors.New("board status must be active, read_only, or archived")
	}
	res, err := s.db.Exec("UPDATE boards SET status = ?, updated_at = ? WHERE slug = ?", status, nowText(), slug)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err == nil && n == 0 {
		return service.ErrBoardNotFound
	}
	return err
}
