package store

import (
	"context"
	"database/sql"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// DB wraps sql.DB for Postgres using pgx.
type DB struct {
	Client *sql.DB
}

// NewDB creates a Postgres connection with sane defaults.
func NewDB(connString string) (*DB, error) {
	db, err := sql.Open("pgx", connString)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)
	return &DB{Client: db}, db.PingContext(context.Background())
}

// Close closes the underlying connection.
func (d *DB) Close() error {
	if d == nil || d.Client == nil {
		return nil
	}
	return d.Client.Close()
}
