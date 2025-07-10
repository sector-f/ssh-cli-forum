package sqlite

import "github.com/sector-f/ssh-cli-forum/forum"

func (s *Service) CreateBoard(name string) (*forum.Board, error) {
	res, err := s.db.Exec(`INSERT INTO boards (name) VALUES (?)`, name)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &forum.Board{ID: int(id), Name: name}, nil
}

func (s *Service) ListBoards() ([]*forum.Board, error) {
	rows, err := s.db.Query(`SELECT id, name FROM boards`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var boards []*forum.Board
	for rows.Next() {
		var b forum.Board
		if err := rows.Scan(&b.ID, &b.Name); err != nil {
			return nil, err
		}
		boards = append(boards, &b)
	}
	return boards, nil
}

func (s *Service) GetBoardByID(id int) (*forum.Board, error) {
	row := s.db.QueryRow(`SELECT id, name FROM boards WHERE id = ?`, id)
	var b forum.Board
	if err := row.Scan(&b.ID, &b.Name); err != nil {
		return nil, err
	}
	return &b, nil
}
