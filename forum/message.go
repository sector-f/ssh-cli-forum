package forum

import (
	"fmt"
	"time"
)

type Message interface {
	Time() time.Time
	Body() string
}

type SystemMsg struct {
	Timestamp time.Time
	Text      string
}

func (msg SystemMsg) Body() string {
	return msg.Text
}

func (msg SystemMsg) Time() time.Time {
	return msg.Timestamp
}

type ChatMsg struct {
	Timestamp time.Time
	Username  string
	Text      string
}

func (msg ChatMsg) Body() string {
	return fmt.Sprintf("%s: %s", msg.Username, msg.Text)
}

func (msg ChatMsg) Time() time.Time {
	return msg.Timestamp
}
