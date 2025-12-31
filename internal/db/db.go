package db

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

// DB wraps the sql.DB connection.
type DB struct {
	*sql.DB
}

// New opens a connection to the database.
func New(path string) (*DB, error) {
	db, err := sql.Open("sqlite3", path+"?_foreign_keys=on&_busy_timeout=5000&_journal_mode=WAL")
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(1)

	if err = db.Ping(); err != nil {
		return nil, err
	}

	return &DB{db}, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.DB.Close()
}
