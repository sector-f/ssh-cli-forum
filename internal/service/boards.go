package service

import (
	"context"
	"strings"
)

// BoardManager provides board-administration use cases over the persistence
// contract. It keeps normalization and validation out of the SQLite store.
type BoardManager struct {
	boards BoardService
}

func NewBoardManager(boards BoardService) *BoardManager {
	return &BoardManager{boards: boards}
}

func (m *BoardManager) CreateBoard(ctx context.Context, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrEmptyBoardName
	}
	return m.CreateBoardWithDetails(ctx, Slugify(name), name, "", 0, 0)
}

func (m *BoardManager) CreateBoardWithDetails(ctx context.Context, slug, name, description string, sortOrder, createdBy int) error {
	slug = Slugify(slug)
	name = strings.TrimSpace(name)
	if slug == "" {
		return ErrInvalidBoardSlug
	}
	if name == "" {
		return ErrEmptyBoardName
	}
	return m.boards.CreateBoardWithDetails(ctx, slug, name, description, sortOrder, createdBy)
}

func (m *BoardManager) UpdateBoard(ctx context.Context, slug, name, description string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrEmptyBoardName
	}
	return m.boards.UpdateBoard(ctx, slug, name, description)
}

func (m *BoardManager) SetBoardStatus(ctx context.Context, slug, status string) error {
	if status != BoardActive && status != BoardReadOnly && status != BoardArchived {
		return ErrInvalidBoardStatus
	}
	return m.boards.SetBoardStatus(ctx, slug, status)
}

// Slugify normalizes a human-readable board identifier to a URL-safe component.
func Slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	dash := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		case r == '-' || r == '_' || r == ' ':
			if b.Len() > 0 && !dash {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}
