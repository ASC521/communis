package sqlitex

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"modernc.org/sqlite"
)

// SELECT name FROM sqlite_master WHERE type='table' AND name='your_table_name';

type journalMode string
type synchronous string
type tempStore string

const (
	JournalModeDelete   journalMode = "DELETE"
	JournalModeTruncate journalMode = "TRUNCATE"
	JournalModePersist  journalMode = "PERSIST"
	JournalModeMemory   journalMode = "MEMORY"
	JournalModeWAL      journalMode = "WAL"
	SynchronousOff      synchronous = "OFF"
	SynchronousNormal   synchronous = "NORMAL"
	SynchronousFull     synchronous = "FULL"
	SynchronousExtra    synchronous = "EXTRA"
	TempStoreDefault    tempStore   = "DEFAULT"
	TempStoreFile       tempStore   = "FILE"
	TempStoreMemory     tempStore   = "MEMORY"
)

func SQLiteJournalModeFromString(m string) (journalMode, error) {
	switch strings.ToUpper(m) {
	case "DELETE":
		return JournalModeDelete, nil
	case "TRUNCATE":
		return JournalModeTruncate, nil
	case "PERSIST":
		return JournalModePersist, nil
	case "MEMORY":
		return JournalModeMemory, nil
	case "WAL":
		return JournalModeWAL, nil
	default:
		return "", fmt.Errorf("%s is not a valid sqlite journal mode", m)
	}
}

func SQLiteSynchronousFromString(s string) (synchronous, error) {
	switch strings.ToUpper(s) {
	case "OFF":
		return SynchronousOff, nil
	case "NORMAL":
		return SynchronousNormal, nil
	case "FULL":
		return SynchronousFull, nil
	case "EXTRA":
		return SynchronousExtra, nil
	default:
		return "", fmt.Errorf("%s is not a valid synchronous setting", s)
	}
}

func SQLiteTempStoreFromString(t string) (tempStore, error) {
	switch strings.ToUpper(t) {
	case "DEFAULT":
		return TempStoreDefault, nil
	case "FILE":
		return TempStoreFile, nil
	case "MEMORY":
		return TempStoreMemory, nil
	default:
		return "", fmt.Errorf("%s is not a valid temp store setting", t)
	}
}

type sqliteOptions struct {
	JournalMode    journalMode
	Synchronous    synchronous
	TempStore      tempStore
	BusyTimeout    int
	CacheSize      int
	ForeignKeys    bool
	MaxReaderConns int
	QueryTimeout   time.Duration
}

type SQLiteOption func(so *sqliteOptions) error

func WithJournalMode(js string) SQLiteOption {
	return func(o *sqliteOptions) error {
		j, err := SQLiteJournalModeFromString(js)
		if err != nil {
			return err
		}
		o.JournalMode = j
		return nil
	}
}

func WithSynchronous(ss string) SQLiteOption {
	return func(o *sqliteOptions) error {
		s, err := SQLiteSynchronousFromString(ss)
		if err != nil {
			return err
		}
		o.Synchronous = s
		return nil
	}
}

func WithTempStore(tss string) SQLiteOption {
	return func(o *sqliteOptions) error {
		ts, err := SQLiteTempStoreFromString(tss)
		if err != nil {
			return err
		}
		o.TempStore = ts
		return nil
	}
}

func WithBusyTimeout(bt int) SQLiteOption {
	return func(o *sqliteOptions) error {
		o.BusyTimeout = bt
		return nil
	}
}

func WithCacheSize(cs int) SQLiteOption {
	return func(o *sqliteOptions) error {
		o.CacheSize = cs
		return nil
	}
}

func WithForeignKeys(fk bool) SQLiteOption {
	return func(o *sqliteOptions) error {
		o.ForeignKeys = fk
		return nil
	}
}

func WithMaxReaderConns(c int) SQLiteOption {
	return func(o *sqliteOptions) error {
		o.MaxReaderConns = c
		return nil
	}
}

func (o sqliteOptions) pragmaStatements() []string {
	return []string{
		fmt.Sprintf("PRAGMA journal_mode = %s;", o.JournalMode),
		fmt.Sprintf("PRAGMA synchronous = %s;", o.Synchronous),
		fmt.Sprintf("PRAGMA temp_store = %s;", o.TempStore),
		fmt.Sprintf("PRAGMA busy_timeout = %d;", o.BusyTimeout),
		fmt.Sprintf("PRAGMA cache_size = %d;", o.CacheSize),
		fmt.Sprintf("PRAGMA foreign_keys = %t;", o.ForeignKeys),
	}
}

type SQLiteDB struct {
	dbPath string
	read   *sql.DB
	write  *sql.DB
	opts   sqliteOptions
}

func NewSQLiteDB(ctx context.Context, dbPath string, opts ...SQLiteOption) (*SQLiteDB, error) {

	sopts := sqliteOptions{
		JournalMode:    JournalModeWAL,
		Synchronous:    SynchronousNormal,
		TempStore:      TempStoreMemory,
		BusyTimeout:    5000,
		CacheSize:      2000,
		ForeignKeys:    true,
		MaxReaderConns: 100,
		QueryTimeout:   5 * time.Second,
	}

	for _, opt := range opts {
		err := opt(&sopts)
		if err != nil {
			return nil, err
		}
	}

	db := &SQLiteDB{dbPath: dbPath, opts: sopts}

	sqlite.RegisterConnectionHook(func(conn sqlite.ExecQuerierContext, _ string) error {
		for _, p := range sopts.pragmaStatements() {
			_, err := conn.ExecContext(ctx, p, nil)
			if err != nil {
				return err
			}
		}
		return nil
	})

	write, err := sql.Open("sqlite", "file:"+db.dbPath)
	if err != nil {
		return nil, err
	}

	write.SetMaxOpenConns(1)
	write.SetConnMaxIdleTime(time.Minute)

	read, err := sql.Open("sqlite", "file:"+db.dbPath)
	if err != nil {
		return nil, err
	}
	read.SetMaxOpenConns(sopts.MaxReaderConns)
	read.SetConnMaxIdleTime(time.Minute)

	db.read = read
	db.write = write
	return db, nil
}

func (d *SQLiteDB) Close() error {
	readErr := d.read.Close()
	writeErr := d.write.Close()
	if readErr != nil && writeErr != nil {
		return fmt.Errorf("error closing read and write connections:  read error - %w   write error - %w", readErr, writeErr)
	} else if readErr != nil {
		return readErr
	} else if writeErr != nil {
		return writeErr
	} else {
		return nil
	}
}

func WithTransaction[R any](db *SQLiteDB, ctx context.Context, txIn func(context.Context, *sql.Tx) (result R, err error)) (result R, err error) {
	ctxWTO, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	tx, err := db.write.BeginTx(ctxWTO, nil)
	if err != nil {
		return result, err
	}

	wrappedTx := func() (result R, err error) {
		defer func() {
			r := recover()
			if r != nil {
				err = fmt.Errorf("error wrapped: %v", r)
			}
		}()
		return txIn(ctx, tx)
	}

	result, err = wrappedTx()
	if err != nil {
		rbErr := tx.Rollback()
		if rbErr != nil {
			return result, fmt.Errorf("transaction failed, rollback failed: %w - %w", err, rbErr)
		}
		return result, err
	}

	err = tx.Commit()
	if err != nil {
		rbErr := tx.Rollback()
		if rbErr != nil {
			return result, fmt.Errorf("failed to commit, failed to rollback: %w - %w", err, rbErr)
		}
		return result, fmt.Errorf("failed to commit: %w", err)
	}

	return result, nil

}

func Query(db *SQLiteDB, ctx context.Context, q string) (*sql.Rows, error) {
	ctxWTO, cancel := context.WithTimeout(ctx, db.opts.QueryTimeout)
	defer cancel()

	return db.read.QueryContext(ctxWTO, q)
}

func QueryRow(db *SQLiteDB, ctx context.Context, q string) *sql.Row {
	ctxWTO, cancel := context.WithTimeout(ctx, db.opts.QueryTimeout)
	defer cancel()

	return db.read.QueryRowContext(ctxWTO, q)
}

func Exec(db *SQLiteDB, ctx context.Context, sql string) (sql.Result, error) {
	ctxWTO, cancel := context.WithTimeout(ctx, db.opts.QueryTimeout)
	defer cancel()

	return db.write.ExecContext(ctxWTO, sql)
}

type SQLiteMigrationDriver struct {
	db  *SQLiteDB
	ctx context.Context
}

func NewSQLiteMigrationDriver(db *SQLiteDB, ctx context.Context) (*SQLiteMigrationDriver, error) {
	return &SQLiteMigrationDriver{db: db, ctx: ctx}, nil
}

func (s *SQLiteMigrationDriver) AddVersionTable() error {
	_, err := WithTransaction[any](s.db, s.ctx, func(ctx context.Context, tx *sql.Tx) (any, error) {
		_, err := tx.Exec("CREATE TABLE IF NOT EXISTS schema_version (id INTEGER PRIMARY KEY, version INTEGER, dirty TEXT) strict;")
		if err != nil {
			return nil, err
		}
		_, err = tx.Exec("INSERT INTO schema_version (version, dirty) VALUES (0, 'N');")
		if err != nil {
			return nil, err
		}
		return nil, nil
	})

	return err
}

func (s *SQLiteMigrationDriver) RunMigration(sqlMig string, version uint) error {
	_, err := WithTransaction[any](s.db, s.ctx, func(ctx context.Context, tx *sql.Tx) (any, error) {
		_, err := tx.Exec(sqlMig)
		if err != nil {
			return nil, err
		}

		var verID int
		err = tx.QueryRow("SELECT id from schema_version;").Scan(&verID)
		if err != nil {
			return nil, err
		}
		_, err = tx.Exec("UPDATE schema_version SET version = ? WHERE id = ?;", version, verID)

		if err != nil {
			return nil, err
		}
		return nil, nil
	})

	return err

}

func (s *SQLiteMigrationDriver) Version() (uint, error) {
	sql := "SELECT version from schema_version;"
	row := QueryRow(s.db, s.ctx, sql)

	var ver uint
	err := row.Scan(&ver)
	if err != nil {
		return 0, err
	}

	return ver, err

}
