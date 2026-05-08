package services

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
	"github.com/ASC521/communis/dbx/migrations"
	"github.com/ASC521/communis/dbx/sqlitex"
	"github.com/ASC521/communis/models"
	"github.com/ASC521/communis/repository/sqlite"
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
)

type CacheState struct {
	Connections     map[int64]time.Time
	TimeToLive      time.Duration
	DBDirectory     string
	IndexDBFileName string
}

type SQLiteDataStoreConfig struct {
	DBDirectory       string
	IndexDBFileName   string
	NotesDBMigrations []migrations.Migration
	IndexDBMigrations []migrations.Migration
	SQLiteOptions     []sqlitex.SQLiteOption
}

func ConfigToSQLiteDataStoreConfig(conf config.Config) (SQLiteDataStoreConfig, error) {

	var dbd string
	if conf.DataDirectory == "" {
		return SQLiteDataStoreConfig{}, errors.New("data directory cannot be empty")
	}

	dbd, err := homedir.Expand(conf.DataDirectory)
	if err != nil {
		return SQLiteDataStoreConfig{}, err
	}

	opts := []sqlitex.SQLiteOption{}
	if conf.SQLite.JournalMode != "" {
		_, err = sqlitex.JournalModeFromString(conf.SQLite.JournalMode)
		if err != nil {
			return SQLiteDataStoreConfig{}, err
		}
		opts = append(opts, sqlitex.WithJournalMode(conf.SQLite.JournalMode))
	}

	if conf.SQLite.Synchronous != "" {
		_, err = sqlitex.SynchronousFromString(conf.SQLite.Synchronous)
		if err != nil {
			return SQLiteDataStoreConfig{}, err
		}
		opts = append(opts, sqlitex.WithSynchronous(conf.SQLite.Synchronous))
	}

	if conf.SQLite.TempStore != "" {
		_, err = sqlitex.TempStoreFromString(conf.SQLite.TempStore)
		if err != nil {
			return SQLiteDataStoreConfig{}, err
		}
		opts = append(opts, sqlitex.WithTempStore(conf.SQLite.TempStore))
	}

	opts = append(opts,
		sqlitex.WithBusyTimeout(conf.SQLite.BusyTimeout),
		sqlitex.WithCacheSize(conf.SQLite.CacheSize),
		sqlitex.WithForeignKeys(conf.SQLite.ForeignKeys))

	if conf.SQLite.NotesDBMigrations == "" {
		return SQLiteDataStoreConfig{}, errors.New("NotesDBMigrations cannot be empty")
	}

	noteDBMigrations, err := migrations.Load(conf.SQLite.NotesDBMigrations)
	if err != nil {
		return SQLiteDataStoreConfig{}, err
	}

	if conf.SQLite.IndexDBMigrations == "" {
		return SQLiteDataStoreConfig{}, errors.New("IndexDBMigrations cannot be empty")
	}

	indexDBMigrations, err := migrations.Load(conf.SQLite.IndexDBMigrations)
	if err != nil {
		return SQLiteDataStoreConfig{}, err
	}

	if conf.SQLite.IndexDBFileName == "" {
		return SQLiteDataStoreConfig{}, errors.New("IndexDBFileName cannot be empty")
	}

	c := SQLiteDataStoreConfig{
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

// SQLiteDataStoreActor manages the creation of connections to individual notes sqlite databases.
type SQLiteDataStoreActor struct {
	connections map[int64]*cachedConn
	ttl         time.Duration
	wg          *sync.WaitGroup
	indexDB     *sqlitex.SQLiteDB
	commands    chan sqliteConnCmd
	conf        SQLiteDataStoreConfig
	logger      *slog.Logger
}

func NewSQLiteDataStoreActor(wg *sync.WaitGroup, ttl time.Duration, conf SQLiteDataStoreConfig, logger *slog.Logger) (*SQLiteDataStoreActor, error) {

	indexDB, err := sqlitex.NewSQLiteDB(filepath.Join(conf.DBDirectory, conf.IndexDBFileName), conf.SQLiteOptions...)
	if err != nil {
		return nil, err
	}

	svc := &SQLiteDataStoreActor{
		connections: map[int64]*cachedConn{},
		ttl:         ttl,
		wg:          wg,
		commands:    make(chan sqliteConnCmd),
		conf:        conf,
		indexDB:     indexDB,
		logger:      logger,
	}

	go svc.run()

	return svc, nil
}

func (s *SQLiteDataStoreActor) run() {

	s.wg.Add(1)
	defer s.wg.Done()

	expiryCheckTimer := time.NewTicker(60 * time.Second)
	defer expiryCheckTimer.Stop()

	for {
		select {
		case msg, ok := <-s.commands:

			if !ok {
				return // channel closed
			}

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

			}
			close(msg.result)
		case <-expiryCheckTimer.C:
			s.removeExpired()
		}
	}

}

func (s *SQLiteDataStoreActor) createNewConnection(ctx context.Context, key int64) (*sqlitex.SQLiteDB, error) {

	us := sqlite.NewIndexDBRepository(s.indexDB)
	userDB, err := us.GetUserDB(ctx, key)
	if err != nil {
		return nil, err
	}

	return sqlitex.NewSQLiteDB(filepath.Join(s.conf.DBDirectory, userDB.Path), s.conf.SQLiteOptions...)

}

func (s *SQLiteDataStoreActor) GetNotesStore(ctx context.Context, key int64) (models.NotesRepository, error) {
	cmd := sqliteConnCmd{
		tag: cmdGetConn,
		ctx: ctx,
		key: key,
	}

	return chanutil.SendReceive[sqliteConnCmd, models.NotesRepository](s.commands, cmd)
}

func (s *SQLiteDataStoreActor) getNotesStore(ctx context.Context, key int64) (models.NotesRepository, error) {
	cc, ok := s.connections[key]
	if !ok {
		conn, err := s.createNewConnection(ctx, key)
		if err != nil {
			return nil, err
		}

		cc = &cachedConn{conn: conn}
		s.connections[key] = cc
	}
	cc.expiry = time.Now().Add(s.ttl)
	return sqlite.NewNotesRepository(cc.conn), nil
}

func (s *SQLiteDataStoreActor) Remove(key int64) error {
	cmd := sqliteConnCmd{
		tag: cmdRemoveConn,
		key: key,
	}

	return chanutil.SendReceiveError[sqliteConnCmd](s.commands, cmd)
}

func (s *SQLiteDataStoreActor) removeConnection(key int64) error {
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

func (s *SQLiteDataStoreActor) CreateDB(ctx context.Context, key int64) error {

	cmd := sqliteConnCmd{
		tag: cmdCreateDB,
		ctx: ctx,
		key: key,
	}

	return chanutil.SendReceiveError[sqliteConnCmd](s.commands, cmd)
}

func (s *SQLiteDataStoreActor) createDatabase(ctx context.Context, key int64) error {
	conn, err := s.createNewConnection(ctx, key)
	if err != nil {
		return err
	}

	migrationDriver := sqlitex.NewMigrationDriver(conn)
	ver, err := migrations.Bootstrap(ctx, s.conf.NotesDBMigrations, migrationDriver)
	if err != nil {
		return err
	}

	indexDB := sqlite.NewIndexDBRepository(s.indexDB)
	return indexDB.UpdateDBVersion(ctx, key, ver)
}

func (s *SQLiteDataStoreActor) DeleteDB(ctx context.Context, key int64) error {

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

func (s *SQLiteDataStoreActor) deleteDatabase(ctx context.Context, key int64) error {
	indexDB := sqlite.NewIndexDBRepository(s.indexDB)
	userDB, err := indexDB.GetUserDB(ctx, key)
	if err != nil {
		return err
	}

	return os.Remove(filepath.Join(s.conf.DBDirectory, userDB.Path))

}

func (s *SQLiteDataStoreActor) GetUserStore() models.IndexRepository {
	return sqlite.NewIndexDBRepository(s.indexDB)
}

func (s *SQLiteDataStoreActor) GetIndexDatabase() *sqlitex.SQLiteDB {
	return s.indexDB
}

func (s *SQLiteDataStoreActor) RunMigrations(ctx context.Context) error {
	return chanutil.SendReceiveError[sqliteConnCmd](s.commands, sqliteConnCmd{tag: cmdCheckMigrations, ctx: ctx})
}

func (s *SQLiteDataStoreActor) runMigrations(ctx context.Context) error {
	indexDriver := sqlitex.NewMigrationDriver(s.indexDB)

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

	us := sqlite.NewIndexDBRepository(s.indexDB)
	dbsToUpgrade, err := us.DBVersionBefore(ctx, int(latestNotesVer.Version))
	if err != nil {
		return err
	}

	userStore := sqlite.NewIndexDBRepository(s.indexDB)
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

		err = userStore.UpdateDBVersion(ctx, userDB.UserID, ver)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLiteDataStoreActor) Stop() error {
	err := s.indexDB.Close()
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
	return nil
}

func (s *SQLiteDataStoreActor) removeExpired() {
	for key, cc := range s.connections {
		if !time.Now().After(cc.expiry) {
			continue
		}
		err := cc.conn.Close()
		s.logger.Warn(fmt.Sprintf("failed to close database connection to %s", cc.conn.DBPath), "errMsg", err.Error())
		delete(s.connections, key)
	}
}

func (s *SQLiteDataStoreActor) GetState() CacheState {
	cmd := sqliteConnCmd{
		tag: cmdGetState,
	}
	state, _ := chanutil.SendReceive[sqliteConnCmd, CacheState](s.commands, cmd)
	return state
}

func (s *SQLiteDataStoreActor) getState() CacheState {

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
