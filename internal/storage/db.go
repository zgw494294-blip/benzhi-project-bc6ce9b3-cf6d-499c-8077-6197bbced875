package storage

import (
	"context"
	"database/sql"
	"fmt"
	_ "modernc.org/sqlite"
)

type Store struct{ db *sql.DB }

func Open(ctx context.Context, dsn string) (*Store, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err = db.ExecContext(ctx, schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("初始化数据库: %w", err)
	}
	if err = migrateIdempotency(ctx, db); err != nil {
		db.Close()
		return nil, fmt.Errorf("迁移幂等记录: %w", err)
	}
	return &Store{db: db}, nil
}

func migrateIdempotency(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, `PRAGMA table_info(idempotency)`)
	if err != nil {
		return err
	}
	columns := map[string]bool{}
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, kind string
		var defaultValue any
		if err = rows.Scan(&cid, &name, &kind, &notNull, &defaultValue, &primaryKey); err != nil {
			rows.Close()
			return err
		}
		columns[name] = true
	}
	if err = rows.Close(); err != nil {
		return err
	}
	for name, statement := range map[string]string{
		"actor":          `ALTER TABLE idempotency ADD COLUMN actor TEXT NOT NULL DEFAULT ''`,
		"request_digest": `ALTER TABLE idempotency ADD COLUMN request_digest TEXT NOT NULL DEFAULT ''`,
	} {
		if columns[name] {
			continue
		}
		if _, err = db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	_, err = db.ExecContext(ctx, `INSERT OR IGNORE INTO schema_version(version,applied_at) VALUES(2,CURRENT_TIMESTAMP)`)
	return err
}
func (s *Store) Close() error                   { return s.db.Close() }
func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }
func (s *Store) Write(ctx context.Context, fn func(*Tx) error) error {
	tx, err := s.db.BeginTx(context.WithoutCancel(ctx), &sql.TxOptions{})
	if err != nil {
		return err
	}
	w := &Tx{tx: tx}
	if err = fn(w); err != nil {
		_ = tx.Rollback()
		return err
	}
	return finishWrite(ctx, tx)
}

func finishWrite(ctx context.Context, tx *sql.Tx) error {
	if err := ctx.Err(); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("提交事务前请求已取消: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交事务: %w", err)
	}
	return nil
}

type Tx struct{ tx *sql.Tx }
