package sqlitex

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mitchellh/go-homedir"
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

func JournalModeFromString(m string) (journalMode, error) {
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

func SynchronousFromString(s string) (synchronous, error) {
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

func TempStoreFromString(t string) (tempStore, error) {
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
	journalMode    journalMode
	synchronous    synchronous
	tempStore      tempStore
	busyTimeout    int
	cacheSize      int
	foreignKeys    bool
	maxReaderConns int
	queryTimeout   time.Duration
}

type SQLiteOption func(so *sqliteOptions) error

func WithJournalMode(js string) SQLiteOption {
	return func(o *sqliteOptions) error {
		j, err := JournalModeFromString(js)
		if err != nil {
			return err
		}
		o.journalMode = j
		return nil
	}
}

func WithSynchronous(ss string) SQLiteOption {
	return func(o *sqliteOptions) error {
		s, err := SynchronousFromString(ss)
		if err != nil {
			return err
		}
		o.synchronous = s
		return nil
	}
}

func WithTempStore(tss string) SQLiteOption {
	return func(o *sqliteOptions) error {
		ts, err := TempStoreFromString(tss)
		if err != nil {
			return err
		}
		o.tempStore = ts
		return nil
	}
}

func WithBusyTimeout(bt int) SQLiteOption {
	return func(o *sqliteOptions) error {
		o.busyTimeout = bt
		return nil
	}
}

func WithCacheSize(cs int) SQLiteOption {
	return func(o *sqliteOptions) error {
		o.cacheSize = cs
		return nil
	}
}

func WithForeignKeys(fk bool) SQLiteOption {
	return func(o *sqliteOptions) error {
		o.foreignKeys = fk
		return nil
	}
}

func WithMaxReaderConns(c int) SQLiteOption {
	return func(o *sqliteOptions) error {
		o.maxReaderConns = c
		return nil
	}
}

func (o sqliteOptions) pragmaStatements() []string {
	return []string{
		fmt.Sprintf("PRAGMA journal_mode = %s;", o.journalMode),
		fmt.Sprintf("PRAGMA synchronous = %s;", o.synchronous),
		fmt.Sprintf("PRAGMA temp_store = %s;", o.tempStore),
		fmt.Sprintf("PRAGMA busy_timeout = %d;", o.busyTimeout),
		fmt.Sprintf("PRAGMA cache_size = %d;", o.cacheSize),
		fmt.Sprintf("PRAGMA foreign_keys = %t;", o.foreignKeys),
	}
}

func (o sqliteOptions) String() string {
	s := []string{
		fmt.Sprintf("journal_mode = %s", o.journalMode),
		fmt.Sprintf("synchronous = %s", o.synchronous),
		fmt.Sprintf("temp_store = %s", o.tempStore),
		fmt.Sprintf("busy_timeout = %d", o.busyTimeout),
		fmt.Sprintf("cache_size = %d", o.cacheSize),
		fmt.Sprintf("foreign_keys = %t", o.foreignKeys),
		fmt.Sprintf("max-reader-conns = %v", o.maxReaderConns),
		fmt.Sprintf("query-timeout = %v", o.queryTimeout),
	}
	return strings.Join(s, "\n")
}

type SQLiteDB struct {
	dbPath       string
	Read         *sql.DB
	Write        *sql.DB
	QueryTimeout time.Duration
	opts         *sqliteOptions
}

func NewSQLiteDB(ctx context.Context, dbPath string, opts ...SQLiteOption) (*SQLiteDB, error) {

	sopts := sqliteOptions{
		journalMode:    JournalModeWAL,
		synchronous:    SynchronousNormal,
		tempStore:      TempStoreMemory,
		busyTimeout:    5000,
		cacheSize:      2000,
		foreignKeys:    true,
		maxReaderConns: 100,
		queryTimeout:   10 * time.Second,
	}

	for _, opt := range opts {
		err := opt(&sopts)
		if err != nil {
			return nil, err
		}
	}

	dbp, err := homedir.Expand(dbPath)
	if err != nil {
		return nil, err
	}
	fi, err := os.Stat(dbp)
	if errors.Is(err, os.ErrNotExist) {
		dir := filepath.Dir(dbp)
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			return nil, fmt.Errorf("failed to create directory to create sqlite database file: %w", err)
		}
	} else if err != nil {
		return nil, err
	} else if fi.IsDir() {
		return nil, fmt.Errorf("%s references to a directory not a database file", dbPath)
	}

	db := &SQLiteDB{dbPath: dbp, QueryTimeout: sopts.queryTimeout, opts: &sopts}

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
	read.SetMaxOpenConns(sopts.maxReaderConns)
	read.SetConnMaxIdleTime(time.Minute)

	db.Read = read
	db.Write = write
	return db, nil
}

func (d *SQLiteDB) PrintConfig() string {
	return d.opts.String()
}

func (d *SQLiteDB) Close() error {
	readErr := d.Read.Close()
	writeErr := d.Write.Close()
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

	tx, err := db.Write.BeginTx(ctxWTO, nil)
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

type SQLiteMigrationDriver struct {
	db  *SQLiteDB
	ctx context.Context
}

func NewSQLiteMigrationDriver(db *SQLiteDB, ctx context.Context) (*SQLiteMigrationDriver, error) {
	return &SQLiteMigrationDriver{db: db, ctx: ctx}, nil
}

func (s *SQLiteMigrationDriver) IsEmpty() (bool, error) {
	var count int
	q := "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%';"
	ctxWTO, cancel := context.WithTimeout(s.ctx, s.db.QueryTimeout)
	defer cancel()
	err := s.db.Read.QueryRowContext(ctxWTO, q).Scan(&count)
	if err != nil {
		return false, err
	}
	return count == 0, nil
}

func (s *SQLiteMigrationDriver) AddVersionTable() error {
	_, err := WithTransaction(s.db, s.ctx, func(ctx context.Context, tx *sql.Tx) (any, error) {
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
	_, err := WithTransaction(s.db, s.ctx, func(ctx context.Context, tx *sql.Tx) (any, error) {
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

	ctxWTO, cancel := context.WithTimeout(s.ctx, s.db.QueryTimeout)
	defer cancel()

	var ver uint
	err := s.db.Read.QueryRowContext(ctxWTO, sql).Scan(&ver)

	if err != nil {
		return 0, err
	}

	return ver, err

}
