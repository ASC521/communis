package userstore

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ASC521/communis/chanutil"
	"github.com/ASC521/communis/config"
	datastore "github.com/ASC521/communis/data-store"
	"github.com/ASC521/communis/dbx/migrations"
	"github.com/ASC521/communis/dbx/sqlitex"
	"github.com/mitchellh/go-homedir"
)

type commandTag int

const (
	cmdGetConn commandTag = iota
	cmdRemoveConn
	cmdCreateDB
	cmdDeleteDB
	cmdCheckMigrations
	cmdGetState
	cmdRemoveExpired
)

type CacheState struct {
	Connections     map[int64]time.Time
	TimeToLive      time.Duration
	DBDirectory     string
	IndexDBFileName string
}

type SQLiteConnManagerConfig struct {
	DBDirectory       string
	IndexDBFileName   string
	NotesDBMigrations []migrations.Migration
	IndexDBMigrations []migrations.Migration
	SQLiteOptions     []sqlitex.SQLiteOption
}

func ConfigToSQLiteConnManagerConfig(conf *config.Config) (SQLiteConnManagerConfig, error) {

	var dbd string
	if conf.DataDirectory == "" {
		return SQLiteConnManagerConfig{}, errors.New("data directory cannot be empty")
	}

	dbd, err := homedir.Expand(conf.DataDirectory)
	if err != nil {
		return SQLiteConnManagerConfig{}, err
	}

	opts := []sqlitex.SQLiteOption{}
	if conf.SQLite.JournalMode != "" {
		_, err = sqlitex.JournalModeFromString(conf.SQLite.JournalMode)
		if err != nil {
			return SQLiteConnManagerConfig{}, err
		}
		opts = append(opts, sqlitex.WithJournalMode(conf.SQLite.JournalMode))
	}

	if conf.SQLite.Synchronous != "" {
		_, err = sqlitex.SynchronousFromString(conf.SQLite.Synchronous)
		if err != nil {
			return SQLiteConnManagerConfig{}, err
		}
		opts = append(opts, sqlitex.WithSynchronous(conf.SQLite.Synchronous))
	}

	if conf.SQLite.TempStore != "" {
		_, err = sqlitex.TempStoreFromString(conf.SQLite.TempStore)
		if err != nil {
			return SQLiteConnManagerConfig{}, err
		}
		opts = append(opts, sqlitex.WithTempStore(conf.SQLite.TempStore))
	}

	opts = append(opts,
		sqlitex.WithBusyTimeout(conf.SQLite.BusyTimeout),
		sqlitex.WithCacheSize(conf.SQLite.CacheSize),
		sqlitex.WithForeignKeys(conf.SQLite.ForeignKeys))

	if conf.SQLite.NotesDBMigrations == "" {
		return SQLiteConnManagerConfig{}, errors.New("NotesDBMigrations cannot be empty")
	}

	noteDBMigrations, err := migrations.Load(conf.SQLite.NotesDBMigrations)
	if err != nil {
		return SQLiteConnManagerConfig{}, err
	}

	if conf.SQLite.IndexDBMigrations == "" {
		return SQLiteConnManagerConfig{}, errors.New("IndexDBMigrations cannot be empty")
	}

	indexDBMigrations, err := migrations.Load(conf.SQLite.IndexDBMigrations)
	if err != nil {
		return SQLiteConnManagerConfig{}, err
	}

	if conf.SQLite.IndexDBFileName == "" {
		return SQLiteConnManagerConfig{}, errors.New("IndexDBFileName cannot be empty")
	}

	c := SQLiteConnManagerConfig{
		DBDirectory:       dbd,
		SQLiteOptions:     opts,
		NotesDBMigrations: noteDBMigrations,
		IndexDBFileName:   conf.SQLite.IndexDBFileName,
		IndexDBMigrations: indexDBMigrations,
	}

	return c, nil
}

type cachedConn struct {
	conn   *sqlitex.SQLiteDB
	expiry time.Time
}

type sqliteConnCmd struct {
	tag    commandTag
	key    int64
	ctx    context.Context
	result chan any
}

func (c sqliteConnCmd) WithResult(ch chan any) sqliteConnCmd {
	c.result = ch
	return c
}

func runPoller(ctx context.Context, interval time.Duration, cmdCh chan sqliteConnCmd, logger *slog.Logger) {

	timer := time.NewTimer(interval)
	defer timer.Stop()
	logger.Debug("starting cache poller")

	for {
		select {
		case <-ctx.Done():

			logger.Debug("stopped connection cache expiry timer")
			return

		case <-timer.C:

			logger.Debug("sending message to clean cache")
			err := chanutil.SendReceiveError[sqliteConnCmd](cmdCh, sqliteConnCmd{tag: cmdRemoveExpired})
			if err != nil {
				logger.Warn("error returned from removing expiried connects", "errMsg", err.Error())
			}
			timer.Reset(interval)
			logger.Debug("reset timer for cache poller")

		}
	}

}

// SQLiteConnManager manages the creation of connections to individual notes sqlite databases.
type SQLiteConnManager struct {
	connections map[int64]*cachedConn
	UserStore   *SQLite
	commands    chan sqliteConnCmd
	conf        SQLiteConnManagerConfig
	logger      *slog.Logger
	ttl         time.Duration
}

func NewSQLiteConnManager(conf SQLiteConnManagerConfig, logger *slog.Logger) (*SQLiteConnManager, error) {

	indexDB, err := sqlitex.NewSQLiteDB(filepath.Join(conf.DBDirectory, conf.IndexDBFileName), conf.SQLiteOptions...)
	if err != nil {
		return nil, err
	}

	al := logger.WithGroup("DATA-STORE-CACHE")
	svc := &SQLiteConnManager{
		connections: map[int64]*cachedConn{},
		ttl:         8 * time.Hour,
		commands:    make(chan sqliteConnCmd),
		conf:        conf,
		UserStore:   NewSQLite(indexDB),
		logger:      al,
	}

	return svc, nil
}

func (s *SQLiteConnManager) Start(ctx context.Context, wg *sync.WaitGroup) {

	wg.Go(func() {
		timerCtx, cancelTimer := context.WithCancel(ctx)
		defer cancelTimer()
		wg.Go(func() {
			runPoller(timerCtx, 10*time.Minute, s.commands, s.logger)
		})

		for msg := range s.commands {

			switch msg.tag {
			case cmdGetConn:
				nr, err := s.getNotesStore(msg.ctx, msg.key)
				if err != nil {
					msg.result <- err
				} else {
					msg.result <- nr
				}

			case cmdRemoveConn:
				msg.result <- s.removeConnection(msg.key)

			case cmdCreateDB:
				msg.result <- s.createDatabase(msg.ctx, msg.key)

			case cmdDeleteDB:
				msg.result <- s.deleteDatabase(msg.ctx, msg.key)

			case cmdCheckMigrations:
				msg.result <- s.runMigrations(msg.ctx)

			case cmdGetState:
				msg.result <- s.getState()

			case cmdRemoveExpired:
				msg.result <- s.removeExpired()
			}
			close(msg.result)
		}
	})
}

func (s *SQLiteConnManager) createNewConnection(ctx context.Context, key int64) (*sqlitex.SQLiteDB, error) {

	userDB, err := s.UserStore.GetUserDB(ctx, key)
	if err != nil {
		return nil, err
	}

	return sqlitex.NewSQLiteDB(filepath.Join(s.conf.DBDirectory, userDB.Path), s.conf.SQLiteOptions...)

}

func (s *SQLiteConnManager) GetNotesStore(ctx context.Context, key int64) (*datastore.SQLite, error) {
	cmd := sqliteConnCmd{
		tag: cmdGetConn,
		ctx: ctx,
		key: key,
	}

	return chanutil.SendReceive[sqliteConnCmd, *datastore.SQLite](s.commands, cmd)
}

func (s *SQLiteConnManager) getNotesStore(ctx context.Context, key int64) (*datastore.SQLite, error) {
	cc, ok := s.connections[key]
	if !ok {
		s.logger.Debug("cache miss -- create new connection")
		conn, err := s.createNewConnection(ctx, key)
		if err != nil {
			return nil, err
		}

		cc = &cachedConn{conn: conn}
		s.connections[key] = cc
	}
	cc.expiry = time.Now().Add(s.ttl)
	return datastore.NewSQLite(cc.conn), nil
}

func (s *SQLiteConnManager) Remove(key int64) error {
	cmd := sqliteConnCmd{
		tag: cmdRemoveConn,
		key: key,
	}

	return chanutil.SendReceiveError[sqliteConnCmd](s.commands, cmd)
}

func (s *SQLiteConnManager) removeConnection(key int64) error {
	cc, ok := s.connections[key]
	if !ok {
		return nil
	}
	err := cc.conn.Close()
	if err != nil {
		return err
	}
	delete(s.connections, key)
	return nil
}

func (s *SQLiteConnManager) CreateDB(ctx context.Context, key int64) error {

	cmd := sqliteConnCmd{
		tag: cmdCreateDB,
		ctx: ctx,
		key: key,
	}

	return chanutil.SendReceiveError[sqliteConnCmd](s.commands, cmd)
}

func (s *SQLiteConnManager) createDatabase(ctx context.Context, key int64) error {
	conn, err := s.createNewConnection(ctx, key)
	if err != nil {
		return err
	}

	migrationDriver := sqlitex.NewMigrationDriver(conn)
	ver, err := migrations.Bootstrap(ctx, s.conf.NotesDBMigrations, migrationDriver)
	if err != nil {
		return err
	}

	return s.UserStore.UpdateDBVersion(ctx, key, ver)
}

func (s *SQLiteConnManager) DeleteDB(ctx context.Context, key int64) error {

	err := s.Remove(key)
	if err != nil {
		return err
	}

	cmd := sqliteConnCmd{
		tag: cmdDeleteDB,
		ctx: ctx,
		key: key,
	}

	return chanutil.SendReceiveError[sqliteConnCmd](s.commands, cmd)
}

func (s *SQLiteConnManager) deleteDatabase(ctx context.Context, key int64) error {
	userDB, err := s.UserStore.GetUserDB(ctx, key)
	if err != nil {
		return err
	}

	return os.Remove(filepath.Join(s.conf.DBDirectory, userDB.Path))

}

func (s *SQLiteConnManager) RunMigrations(ctx context.Context) error {
	return chanutil.SendReceiveError[sqliteConnCmd](s.commands, sqliteConnCmd{tag: cmdCheckMigrations, ctx: ctx})
}

func (s *SQLiteConnManager) runMigrations(ctx context.Context) error {
	indexDriver := sqlitex.NewMigrationDriver(s.UserStore.DB)

	isEmpty, err := indexDriver.IsEmpty(ctx)
	if err != nil {
		return err
	}

	if isEmpty {
		s.logger.Info("Index database is empty - bootstrapping to latest version")
		_, err = migrations.Bootstrap(ctx, s.conf.IndexDBMigrations, indexDriver)
		if err != nil {
			return err
		}
	} else {
		isLatest, err := migrations.IsLatest(ctx, s.conf.IndexDBMigrations, indexDriver)
		if err != nil {
			return err
		}

		if !isLatest {
			s.logger.Info("Index database pending migration found - running up migration")
			_, err = migrations.Up(ctx, s.conf.IndexDBMigrations, indexDriver)
			if err != nil {
				return err
			}
		}
	}

	latestNotesVer, err := migrations.Latest(s.conf.NotesDBMigrations)
	if err != nil {
		return err
	}

	dbsToUpgrade, err := s.UserStore.DBVersionBefore(ctx, int(latestNotesVer.Version))
	if err != nil {
		return err
	}

	for _, userDB := range dbsToUpgrade {
		s.logger.Info(fmt.Sprintf("Notes database migration found for user %v - running up migration", userDB.UserID))
		conn, err := sqlitex.NewSQLiteDB(filepath.Join(s.conf.DBDirectory, userDB.Path), s.conf.SQLiteOptions...)
		if err != nil {
			return err
		}
		driver := sqlitex.NewMigrationDriver(conn)
		ver, err := migrations.Up(ctx, s.conf.NotesDBMigrations, driver)
		if err != nil {
			return err
		}

		err = s.UserStore.UpdateDBVersion(ctx, userDB.UserID, ver)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLiteConnManager) Stop() error {
	err := s.UserStore.DB.Close()
	if err != nil {
		return err
	}

	for _, cc := range s.connections {
		err = cc.conn.Close()
		if err != nil {
			return err
		}
	}
	close(s.commands)
	s.logger.Debug("stopped sqlite connection cache")
	return nil
}

func (s *SQLiteConnManager) removeExpired() error {
	s.logger.Debug("cleaning cache of expired connections")
	for key, cc := range s.connections {
		if !time.Now().After(cc.expiry) {
			continue
		}
		err := s.removeConnection(key)
		if err != nil {
			return err
		}
		s.logger.Debug(fmt.Sprintf("connection removed for user %v", key))
	}
	return nil
}

func (s *SQLiteConnManager) GetState() CacheState {
	cmd := sqliteConnCmd{
		tag: cmdGetState,
	}
	state, _ := chanutil.SendReceive[sqliteConnCmd, CacheState](s.commands, cmd)
	return state
}

func (s *SQLiteConnManager) getState() CacheState {

	pubConns := make(map[int64]time.Time, len(s.connections))
	for key, cc := range s.connections {
		pubConns[key] = cc.expiry
	}

	return CacheState{
		Connections:     pubConns,
		TimeToLive:      s.ttl,
		IndexDBFileName: s.conf.IndexDBFileName,
		DBDirectory:     s.conf.DBDirectory,
	}
}
