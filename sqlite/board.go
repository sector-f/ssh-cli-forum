package sqlite

import "github.com/sector-f/ssh-cli-forum/forum"

func (s *Service) CreateBoard(name string) error {
	_, err := s.db.Exec(`INSERT INTO boards (name) VALUES (?)`, name)
	return err
}

func (s *Service) ListBoards() ([]forum.BoardListing, error) {
	rows, err := s.db.Query(`SELECT id, name FROM boards`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var boards []forum.BoardListing
	for rows.Next() {
		var b Board
		if err := rows.Scan(&b.ID, &b.Name); err != nil {
			return nil, err
		}

		board := forum.BoardListing{
			ID:   b.ID,
			Name: b.Name,
		}

		boards = append(boards, board)
	}
	return boards, nil
}

func (s *Service) GetBoardByID(id int) (forum.Board, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return forum.Board{}, err
	}
	defer tx.Rollback()

	row := tx.QueryRow(`SELECT id, name FROM boards WHERE id = ?`, id)
	var b Board
	if err := row.Scan(&b.ID, &b.Name); err != nil {
		return forum.Board{}, err
	}

	threads, err := s.ListThreads(b.ID)
	if err != nil {
		return forum.Board{}, err
	}

	board := forum.Board{
		ID:      b.ID,
		Name:    b.Name,
		Threads: threads,
	}
	return board, nil
}
