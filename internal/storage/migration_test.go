package storage

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestLegacyIdempotencyRowsRemainReadable(t *testing.T) {
	ctx := context.Background()
	dsn := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(ctx, `CREATE TABLE idempotency(key TEXT NOT NULL, operation TEXT NOT NULL, case_id TEXT NOT NULL, status_code INTEGER NOT NULL, response_json TEXT NOT NULL, created_at TEXT NOT NULL, PRIMARY KEY(key,operation))`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(ctx, `INSERT INTO idempotency VALUES('legacy-key','update-case:case-1','case-1',200,'{}','2026-08-25T00:00:00Z')`)
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	store, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	err = store.Write(ctx, func(tx *Tx) error {
		record, readErr := tx.GetIdempotency(ctx, "legacy-key")
		if readErr != nil {
			return readErr
		}
		if record == nil || record.RequestDigest != "" || record.Actor != "" || record.ResponseJSON != "{}" {
			t.Fatalf("legacy row changed: %+v", record)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
