package sqlite

import "github.com/sector-f/ssh-cli-forum/forum"

func (s *Service) CreateThread(title string, boardID int, authorID int) error {
	_, err := s.db.Exec(`
		INSERT INTO threads (title, board_id, author_id)
		VALUES (?, ?, ?)
	`, title, boardID, authorID)
	return err
}

func (s *Service) GetThreadByID(id int) (forum.Thread, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return forum.Thread{}, err
	}
	defer tx.Rollback()

	var t Thread
	row := tx.QueryRow(`SELECT id, title, board_id, author_id FROM threads WHERE id = ?`, id)
	if err := row.Scan(&t.ID, &t.Title, &t.BoardID, &t.AuthorID); err != nil {
		return forum.Thread{}, err
	}

	author, err := s.GetUserByID(t.AuthorID)
	if err != nil {
		return forum.Thread{}, err
	}

	comments, err := s.ListComments(t.ID)
	if err != nil {
		return forum.Thread{}, err
	}

	thread := forum.Thread{
		ID:       t.ID,
		Title:    t.Title,
		Author:   author,
		Comments: comments,
	}

	return thread, nil
}

func (s *Service) ListThreads(boardID int) ([]forum.ThreadListing, error) {
	rows, err := s.db.Query(`SELECT id, title, board_id, author_id FROM threads WHERE board_id = ?`, boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var threads []forum.ThreadListing
	for rows.Next() {
		var t Thread
		if err := rows.Scan(&t.ID, &t.Title, &t.BoardID, &t.AuthorID); err != nil {
			return nil, err
		}

		author, err := s.GetUserByID(t.AuthorID)
		if err != nil {
			return nil, err
		}

		thread := forum.ThreadListing{
			ID:     t.ID,
			Title:  t.Title,
			Author: author,
		}

		threads = append(threads, thread)
	}
	return threads, nil
}
