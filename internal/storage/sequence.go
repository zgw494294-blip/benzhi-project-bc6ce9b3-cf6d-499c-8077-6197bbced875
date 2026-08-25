package storage

import (
	"context"
	"fmt"
)

func (t *Tx) NextPermitSerial(ctx context.Context, year int) (string, error) {
	t.store.sequenceMu.Lock()
	defer t.store.sequenceMu.Unlock()
	if current, ok := t.store.nextPermitByYear[year]; ok {
		current++
		t.store.nextPermitByYear[year] = current
		return fmt.Sprintf("FC-%d-%06d", year, current), nil
	}
	var n int
	e := t.tx.QueryRowContext(ctx, `SELECT COUNT(*)+1 FROM permits WHERE serial_number LIKE ?`, fmt.Sprintf("FC-%d-%%", year)).Scan(&n)
	if e != nil {
		return "", e
	}
	t.store.nextPermitByYear[year] = n
	return fmt.Sprintf("FC-%d-%06d", year, n), nil
}
