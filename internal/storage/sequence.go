package storage

import (
	"context"
	"fmt"
)

func (t *Tx) NextPermitSerial(ctx context.Context, year int) (string, error) {
	var n int
	e := t.tx.QueryRowContext(ctx, `SELECT COUNT(*)+1 FROM permits WHERE serial_number LIKE ?`, fmt.Sprintf("FC-%d-%%", year)).Scan(&n)
	if e != nil {
		return "", e
	}
	return fmt.Sprintf("FC-%d-%06d", year, n), nil
}
