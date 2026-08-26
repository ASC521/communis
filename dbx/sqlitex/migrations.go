package sqlitex

import (
	"context"
	"database/sql"
	"fmt"
)

type SQLiteMigrationDriver struct {
	db *SQLiteDB
}

func NewMigrationDriver(db *SQLiteDB) *SQLiteMigrationDriver {
	return &SQLiteMigrationDriver{db: db}
}

func (s *SQLiteMigrationDriver) IsEmpty(ctx context.Context) (bool, error) {
	var count int
	q := "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%';"
	ctxWTO, cancel := context.WithTimeout(ctx, s.db.QueryTimeout)
	defer cancel()
	err := s.db.Read.QueryRowContext(ctxWTO, q).Scan(&count)
	if err != nil {
		return false, err
	}
	return count == 0, nil
}

func (s *SQLiteMigrationDriver) AddVersionTable(ctx context.Context) error {
	// SQLite has a built in pragma 'user_version' that can be used to track versioning of a database.
	// That pragma will be leveraged so there is no need to a new table to the database so this is a no op.
	return nil
}

func (s *SQLiteMigrationDriver) RunMigration(ctx context.Context, sqlMig string, version uint) error {
	_, err := WithTransaction(s.db.Write, ctx, func(ctx context.Context, tx *sql.Tx) (any, error) {
		_, err := tx.Exec(sqlMig)
		if err != nil {
			return nil, err
		}

		_, err = tx.Exec(fmt.Sprintf("PRAGMA user_version = %d;", version))
		if err != nil {
			return nil, err
		}
		return nil, nil
	})

	return err
}

func (s *SQLiteMigrationDriver) Version(ctx context.Context) (uint, error) {
	sql := "PRAGMA user_version;"

	ctxWTO, cancel := context.WithTimeout(ctx, s.db.QueryTimeout)
	defer cancel()

	var ver uint
	err := s.db.Read.QueryRowContext(ctxWTO, sql).Scan(&ver)
	if err != nil {
		return 0, err
	}

	return ver, err
}
