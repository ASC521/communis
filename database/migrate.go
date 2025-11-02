package database

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"time"

	_ "modernc.org/sqlite"
)

var migrationRegex = regexp.MustCompile(`^(?<number>\d+)_(?<name>.*)\.(up|down)\.sql$`)

type SQLiteURI struct {
	fp   string
	mode string
	wal  bool
}

func (u SQLiteURI) String() string {
	uri := fmt.Sprintf("file:%s?mode=%s", u.fp, u.mode)
	if u.wal {
		return uri + "&_journal_mode=WAL"
	}
	return uri
}

type migration struct {
	version int
	up      bool
	query   string
	name    string
}

type Migrator struct {
	db         *sql.DB
	sqlFP      string
	migrations []migration
}

func NewSQLiteMigrator(uri SQLiteURI) (*Migrator, error) {
	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err = db.PingContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	return &Migrator{db: db}, nil
}

func (m *Migrator) loadMigrations() {

}

func (m *Migrator) Up() error {

	return nil
}
