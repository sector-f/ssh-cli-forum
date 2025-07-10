package sqlite

import (
	"time"

	"github.com/sector-f/ssh-cli-forum/forum"
)

func (s *Service) CreateComment(threadID int, authorID int, body string) (*forum.Comment, error) {
	postedAt := time.Now()
	res, err := s.db.Exec(`
		INSERT INTO comments (thread_id, author_id, body, posted_at)
		VALUES (?, ?, ?, ?)
	`, threadID, authorID, body, postedAt)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &forum.Comment{
		ID:       int(id),
		Thread:   &forum.Thread{ID: threadID},
		Author:   &forum.User{ID: authorID},
		Body:     body,
		PostedAt: postedAt,
	}, nil
}

func (s *Service) ListComments(threadID int) ([]*forum.Comment, error) {
	rows, err := s.db.Query(`
		SELECT id, author_id, body, posted_at FROM comments
		WHERE thread_id = ? ORDER BY posted_at ASC
	`, threadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []*forum.Comment
	for rows.Next() {
		var c forum.Comment
		var postedAt time.Time
		if err := rows.Scan(&c.ID, &c.Author.ID, &c.Body, &postedAt); err != nil {
			return nil, err
		}
		c.Thread = &forum.Thread{ID: threadID}
		c.PostedAt = postedAt
		comments = append(comments, &c)
	}
	return comments, nil
}

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
