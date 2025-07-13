package sqlite

import (
	"time"

	"github.com/sector-f/ssh-cli-forum/forum"
)

func (s *Service) CreateComment(threadID int, authorID int, body string) error {
	postedAt := time.Now()
	_, err := s.db.Exec(`
		INSERT INTO comments (thread_id, author_id, body, posted_at)
		VALUES (?, ?, ?, ?)
	`, threadID, authorID, body, postedAt)

	return err
}

func (s *Service) ListComments(threadID int) ([]forum.Comment, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := tx.Query(`
		SELECT id, author_id, body, posted_at FROM comments
		WHERE thread_id = ? ORDER BY posted_at ASC
	`, threadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []forum.Comment
	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.ID, &c.AuthorID, &c.Body, &c.PostedAt); err != nil {
			return nil, err
		}

		user, err := s.GetUserByID(c.AuthorID)
		if err != nil {
			return nil, err
		}

		comment := forum.Comment{
			Author:   user,
			Body:     c.Body,
			PostedAt: c.PostedAt,
		}

		comments = append(comments, comment)
	}
	return comments, nil
}

/*
func (s *Service) GetCommentByID(id int) (*forum.Comment, error) {
	row := s.db.QueryRow(`SELECT thread_id, author_id, body, posted_at FROM comments WHERE id = ?`, id)
	var c forum.Comment
	var postedAt time.Time
	if err := row.Scan(&c.Thread.ID, &c.Author.ID, &c.Body, &postedAt); err != nil {
		return nil, err
	}
	c.ID = id
	c.PostedAt = postedAt
	return &c, nil
}
*/
