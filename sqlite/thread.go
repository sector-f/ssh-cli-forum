package sqlite

import "github.com/sector-f/ssh-cli-forum/forum"

func (s *Service) CreateThread(title string, boardID int, authorID int) (*forum.Thread, error) {
	res, err := s.db.Exec(`
		INSERT INTO threads (title, board_id, author_id)
		VALUES (?, ?, ?)
	`, title, boardID, authorID)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &forum.Thread{ID: int(id), Title: title}, nil
}

func (s *Service) GetThreadByID(id int) (*forum.Thread, error) {
	row := s.db.QueryRow(`SELECT id, title, board_id, author_id FROM threads WHERE id = ?`, id)
	var t forum.Thread
	if err := row.Scan(&t.ID, &t.Title, &t.Board.ID, &t.Author.ID); err != nil {
		return nil, err
	}

	comments, err := s.ListComments(t.ID)
	if err != nil {
		return nil, err
	}
	t.Comments = comments

	return &t, nil
}

func (s *Service) ListThreads(boardID int) ([]*forum.Thread, error) {
	rows, err := s.db.Query(`SELECT id, title, board_id, author_id FROM threads WHERE board_id = ?`, boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var threads []*forum.Thread
	for rows.Next() {
		var t forum.Thread
		if err := rows.Scan(&t.ID, &t.Title, &t.Board.ID, &t.Author.ID); err != nil {
			return nil, err
		}
		threads = append(threads, &t)
	}
	return threads, nil
}
