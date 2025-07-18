package recorder

import (
	"database/sql"
	"sync"

	"github.com/sector-f/ssh-cli-forum/forum"
	"go.uber.org/zap"
)

var ServiceID string = "chatroom_service"

type Recorder struct {
	db     *sql.DB
	logger *zap.Logger
	mu     sync.Mutex
}

func Open(logger *zap.Logger, path string) (func(msg forum.Message), error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	if err := createTables(db); err != nil {
		return nil, err
	}

	r := &Recorder{logger: logger, db: db}
	callback := func(msg forum.Message) {
		err := r.record(msg)
		if err != nil {
			logger.Error("Failed to record message", zap.Error(err))
		}
	}

	return callback, nil
}

func (r *Recorder) record(msg forum.Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, err := r.db.Exec(`INSERT INTO messages (timestamp, body) VALUES (?, ?)`, msg.Time(), msg.Body())
	return err
}

func createTables(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp DATETIME NOT NULL,
			body TEXT NOT NULL
		);`,
	}

	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}

	return nil
}
