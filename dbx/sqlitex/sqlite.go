package sqlitex

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mitchellh/go-homedir"
	"modernc.org/sqlite"
)

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

type key string

var dbKey key

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

func (d *SQLiteDB) LogDBConfig() slog.Value {
	return slog.GroupValue(
		slog.String("file-location", d.dbPath),
		slog.String("journal_mode", string(d.opts.journalMode)),
		slog.String("synchronous", string(d.opts.synchronous)),
		slog.String("temp_store", string(d.opts.tempStore)),
		slog.Int("busy_timeout", d.opts.busyTimeout),
		slog.Int("cache_size", d.opts.cacheSize),
		slog.Bool("foreign_keys", d.opts.foreignKeys),
		slog.Int("max-reader-conns", d.opts.maxReaderConns),
		slog.String("query-timeout", d.QueryTimeout.String()),
	)
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

// NewContext returns a new context that carries the db value.
func NewContext(ctx context.Context, db *SQLiteDB) context.Context {
	return context.WithValue(ctx, dbKey, db)
}

// FromContext returns the DB value stored in ctx, if any.
func FromContext(ctx context.Context) (*SQLiteDB, bool) {
	db, ok := ctx.Value(dbKey).(*SQLiteDB)
	return db, ok
}

func WithTransaction[R any](db *sql.DB, ctx context.Context, txIn func(context.Context, *sql.Tx) (result R, err error)) (result R, err error) {
	ctxWTO, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	tx, err := db.BeginTx(ctxWTO, nil)
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

func NewMigrationDriver(db *SQLiteDB, ctx context.Context) *SQLiteMigrationDriver {
	return &SQLiteMigrationDriver{db: db, ctx: ctx}
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
	// SQLite has a built in pragma 'user_version' that can be used to track versioning of a database.
	// That pragma will be leveraged so there is no need to a new table to the database so this is a no op.
	return nil
}

func (s *SQLiteMigrationDriver) RunMigration(sqlMig string, version uint) error {
	_, err := WithTransaction(s.db.Write, s.ctx, func(ctx context.Context, tx *sql.Tx) (any, error) {
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

func (s *SQLiteMigrationDriver) Version() (uint, error) {
	sql := "PRAGMA user_version;"

	ctxWTO, cancel := context.WithTimeout(s.ctx, s.db.QueryTimeout)
	defer cancel()

	var ver uint
	err := s.db.Read.QueryRowContext(ctxWTO, sql).Scan(&ver)

	if err != nil {
		return 0, err
	}

	return ver, err

}
