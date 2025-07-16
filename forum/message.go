package forum

import "time"

type Message struct {
	Timestamp time.Time
	Username  string
	Text      string
}
