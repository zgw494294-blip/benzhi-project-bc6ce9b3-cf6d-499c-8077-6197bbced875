package storage

import (
	"database/sql"
	"time"
)

func timeText(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }
func nullableTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return timeText(*t)
}
func parseTime(s string) (time.Time, error) { return time.Parse(time.RFC3339Nano, s) }
func scanNullTime(n sql.NullString) (*time.Time, error) {
	if !n.Valid {
		return nil, nil
	}
	t, e := parseTime(n.String)
	return &t, e
}
