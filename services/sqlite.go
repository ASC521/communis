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
)

type SQLiteDataStoreConfig struct {
	DBDirectory       string
	IndexDBFileName   string
	NotesDBMigrations []migrations.Migration
	IndexDBMigrations []migrations.Migration
	SQLiteOptions     []sqlitex.SQLiteOption
}

func ConfigToSQLiteDataStoreConfig(conf config.Config) (SQLiteDataStoreConfig, error) {

	if conf.SQLite.DBDirectory == "" {
		return SQLiteDataStoreConfig{}, errors.New("DBDirectory cannot be empty")
	}

	dbd, err := homedir.Expand(conf.SQLite.DBDirectory)
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

// SQLiteDataStoreService manages the creation of connections to individual notes sqlite databases.
type SQLiteDataStoreService struct {
	connections map[int64]cachedConn
	ttl         time.Duration
	wg          *sync.WaitGroup
	indexDB     *sqlitex.SQLiteDB
	commands    chan sqliteConnCmd
	conf        SQLiteDataStoreConfig
	logger      *slog.Logger
}

func NewSQLiteDataStoreService(wg *sync.WaitGroup, ttl time.Duration, conf SQLiteDataStoreConfig) (*SQLiteDataStoreService, error) {

	indexDB, err := sqlitex.NewSQLiteDB(filepath.Join(conf.DBDirectory, conf.IndexDBFileName), conf.SQLiteOptions...)
	if err != nil {
		return nil, err
	}

	svc := &SQLiteDataStoreService{
		connections: map[int64]cachedConn{},
		ttl:         ttl,
		wg:          wg,
		commands:    make(chan sqliteConnCmd),
		conf:        conf,
		indexDB:     indexDB,
	}

	go svc.run()

	return svc, nil
}

func (s *SQLiteDataStoreService) run() {

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
				if msg.ctx == nil {
					msg.result <- errors.New("a context is required to retrieve a database connection")
					continue
				}

				cc, ok := s.connections[msg.key]
				if !ok {
					conn, err := s.newConn(msg.ctx, msg.key)
					if err != nil {
						msg.result <- err
						continue
					}
					cc = cachedConn{conn: conn, expiry: time.Now().Add(s.ttl)}
					s.connections[msg.key] = cc
				}
				msg.result <- cc.conn

			case cmdRemoveConn:
				cc, ok := s.connections[msg.key]
				if !ok {
					msg.result <- error(nil)
					continue
				}
				err := cc.conn.Close()
				if err != nil {
					msg.result <- err
					continue
				}
				delete(s.connections, msg.key)
				msg.result <- error(nil)

			case cmdCreateDB:

				if msg.ctx == nil {
					msg.result <- errors.New("a context is required to create a database")
					continue
				}

				dbConn, err := s.newConn(msg.ctx, msg.key)
				if err != nil {
					msg.result <- err
					continue
				}

				migDriver := sqlitex.NewMigrationDriver(dbConn)
				ver, err := migrations.Bootstrap(msg.ctx, s.conf.NotesDBMigrations, migDriver)
				if err != nil {
					msg.result <- err
				}
				us := sqlite.NewIndexDBRepository(s.indexDB)
				err = us.UpdateDBVersion(msg.ctx, msg.key, ver)
				if err != nil {
					msg.result <- err
				} else {
					msg.result <- error(nil)
				}
			case cmdDeleteDB:

				if msg.ctx == nil {
					msg.result <- errors.New("a context is required to create a database")
					continue
				}

				userStore := sqlite.NewIndexDBRepository(s.indexDB)
				userDB, err := userStore.GetUserDB(msg.ctx, msg.key)
				if err != nil {
					msg.result <- err
					continue
				}

				err = userStore.DeleteUser(msg.ctx, msg.key)
				if err != nil {
					msg.result <- err
					continue
				}

				err = os.Remove(filepath.Join(s.conf.DBDirectory, userDB.Path))
				if err != nil {
					msg.result <- fmt.Errorf("failed to delete user datbase: %s", err.Error())
				} else {
					msg.result <- error(nil)
				}

			case cmdCheckMigrations:
				if msg.ctx == nil {
					msg.result <- errors.New("a context is required to check database migrations")
					continue
				}

				indexDriver := sqlitex.NewMigrationDriver(s.indexDB)

				isEmpty, err := indexDriver.IsEmpty(msg.ctx)
				if err != nil {
					msg.result <- err
					continue
				}

				if isEmpty {
					s.logger.Info("Index database is empty - bootstrapping to latest version")
					_, err = migrations.Bootstrap(msg.ctx, s.conf.IndexDBMigrations, indexDriver)
					if err != nil {
						msg.result <- err
						continue
					}
				} else {
					isLatest, err := migrations.IsLatest(msg.ctx, s.conf.IndexDBMigrations, indexDriver)
					if err != nil {
						msg.result <- err
						continue
					}

					if !isLatest {
						s.logger.Info("Index database pending migration found - running up migration")
						_, err = migrations.Up(msg.ctx, s.conf.IndexDBMigrations, indexDriver)
						if err != nil {
							msg.result <- err
							continue
						}
					}
				}

				latestNotesVer, err := migrations.Latest(s.conf.NotesDBMigrations)
				if err != nil {
					msg.result <- err
					continue
				}

				us := sqlite.NewIndexDBRepository(s.indexDB)
				dbsToUpgrade, err := us.DBVersionBefore(msg.ctx, int(latestNotesVer.Version))
				if err != nil {
					msg.result <- err
					continue
				}

				userStore := sqlite.NewIndexDBRepository(s.indexDB)
				for _, userDB := range dbsToUpgrade {
					s.logger.Info(fmt.Sprintf("Notes database migration found for user %v - running up migration", userDB.UserId))
					conn, err := sqlitex.NewSQLiteDB(filepath.Join(s.conf.DBDirectory, userDB.Path), s.conf.SQLiteOptions...)
					if err != nil {
						msg.result <- err
						break
					}
					driver := sqlitex.NewMigrationDriver(conn)
					ver, err := migrations.Up(msg.ctx, s.conf.NotesDBMigrations, driver)
					if err != nil {
						msg.result <- err
						break
					}

					err = userStore.UpdateDBVersion(msg.ctx, userDB.UserId, ver)
					if err != nil {
						msg.result <- err
					} else {
						msg.result <- error(nil)
					}
				}
				msg.result <- error(nil)

			}
			close(msg.result)
		case <-expiryCheckTimer.C:
			s.removeExpired()
		}
	}

}

func (s *SQLiteDataStoreService) newConn(ctx context.Context, key int64) (*sqlitex.SQLiteDB, error) {

	us := sqlite.NewIndexDBRepository(s.indexDB)
	userDB, err := us.GetUserDB(ctx, key)
	if err != nil {
		return nil, err
	}

	return sqlitex.NewSQLiteDB(filepath.Join(s.conf.DBDirectory, userDB.Path), s.conf.SQLiteOptions...)

}

func (s *SQLiteDataStoreService) GetNotesStore(ctx context.Context, key int64) (models.NotesRepository, error) {
	cmd := sqliteConnCmd{
		tag: cmdGetConn,
		ctx: ctx,
		key: key,
	}

	db, err := chanutil.SendReceive[sqliteConnCmd, *sqlitex.SQLiteDB](s.commands, cmd)
	if err != nil {
		return nil, err
	}

	return sqlite.NewNotesRepository(db), nil
}

func (s *SQLiteDataStoreService) Remove(ctx context.Context, key int64) error {
	cmd := sqliteConnCmd{
		tag: cmdRemoveConn,
		ctx: ctx,
		key: key,
	}

	return chanutil.SendReceiveError[sqliteConnCmd](s.commands, cmd)
}

func (s *SQLiteDataStoreService) CreateDB(ctx context.Context, key int64) error {

	cmd := sqliteConnCmd{
		tag: cmdCreateDB,
		ctx: ctx,
		key: key,
	}

	return chanutil.SendReceiveError[sqliteConnCmd](s.commands, cmd)
}

func (s *SQLiteDataStoreService) DeleteDB(ctx context.Context, key int64) error {

	err := s.Remove(ctx, key)
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

func (s *SQLiteDataStoreService) GetUserStore() models.IndexRepository {
	return sqlite.NewIndexDBRepository(s.indexDB)
}

func (s *SQLiteDataStoreService) GetIndexDatabase() *sqlitex.SQLiteDB {
	return s.indexDB
}

func (s *SQLiteDataStoreService) RunMigrations(ctx context.Context) error {
	return chanutil.SendReceiveError[sqliteConnCmd](s.commands, sqliteConnCmd{tag: cmdCheckMigrations, ctx: ctx})
}

func (s *SQLiteDataStoreService) Stop() {
	err := s.indexDB.Close()
	if err != nil {
		s.logger.Warn(fmt.Sprintf("failed to close database connection to %s", s.indexDB.DBPath), "errMsg", err.Error())
	}

	for _, cc := range s.connections {
		err = cc.conn.Close()
		s.logger.Warn(fmt.Sprintf("failed to close database connection to %s", cc.conn.DBPath), "errMsg", err.Error())
	}
	close(s.commands)
}

func (s *SQLiteDataStoreService) removeExpired() {
	for key, cc := range s.connections {
		if !time.Now().After(cc.expiry) {
			continue
		}
		err := cc.conn.Close()
		s.logger.Warn(fmt.Sprintf("failed to close database connection to %s", cc.conn.DBPath), "errMsg", err.Error())
		delete(s.connections, key)
	}
}
