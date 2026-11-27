package sqlite

import (
	"database/sql"
	"time"
)

func parseTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	layouts := []string{
		time.RFC3339Nano, time.RFC3339,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, value); err == nil {
			return t
		}
	}
	return time.Time{}
}

func nullableText(value sql.NullString) string {
	if value.Valid {
		return value.String
	}
	return ""
}
