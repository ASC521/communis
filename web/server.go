package web

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/ASC521/communis/cache"
	"github.com/ASC521/communis/config"
	"github.com/ASC521/communis/dbx/migrations"
	"github.com/ASC521/communis/dbx/sqlitex"
	"github.com/ASC521/communis/models"
	"github.com/ASC521/communis/repository/sqlite"
	"github.com/ASC521/communis/web/handlers"
)

//go:embed "static"
var staticFiles embed.FS

//go:embed "html"
var htmlFiles embed.FS

func setupLogging(c *config.Config) *slog.Logger {
	opts := slog.HandlerOptions{}
	if c.VerboseLogging {
		opts.Level = slog.LevelDebug
	}

	h := slog.NewTextHandler(os.Stdout, &opts)

	return slog.New(h)
}

func connectToDatabase(conf *config.Config, ctx context.Context, logger *slog.Logger) (models.IndexRepository, *cache.TTLCache[string, *sqlitex.SQLiteDB], error) {
	err := config.ValidSQLite(conf.SQLite)
	if err != nil {
		return nil, nil, err
	}

	notesConnCache := cache.NewTTL[string, *sqlitex.SQLiteDB](func(key string, value *sqlitex.SQLiteDB) {
		err := value.Close()
		if err != nil {
			logger.Error(fmt.Sprintf("failed to close %s db connection", key), "erMsg", err.Error())
		}
	})

	indexDBFP := filepath.Join(conf.SQLite.DBDirectory, conf.SQLite.IndexDBFileName)
	indexDB, err := sqlitex.NewSQLiteDB(ctx, indexDBFP,
		sqlitex.WithBusyTimeout(conf.SQLite.BusyTimeout),
		sqlitex.WithCacheSize(conf.SQLite.CacheSize),
		sqlitex.WithForeignKeys(conf.SQLite.ForeignKeys),
		sqlitex.WithJournalMode(conf.SQLite.JournalMode),
		sqlitex.WithSynchronous(conf.SQLite.Synchronous),
		sqlitex.WithTempStore(conf.SQLite.TempStore),
	)
	if err != nil {
		return nil, notesConnCache, err
	}

	indexMigrations, err := migrations.Load(conf.SQLite.IndexDBMigrations)
	if err != nil {
		indexDB.Close()
		return nil, notesConnCache, err
	}
	indexDBMigrationDriver := sqlitex.NewMigrationDriver(indexDB, ctx)

	emptyIndexDB, err := indexDBMigrationDriver.IsEmpty()
	if err != nil {
		indexDB.Close()
		return nil, notesConnCache, err
	}

	if emptyIndexDB {
		logger.Info("fresh database - bootstrapping all the way up")
		_, err = migrations.Bootstrap(indexMigrations, indexDBMigrationDriver)
		if err != nil {
			indexDB.Close()
			return nil, nil, fmt.Errorf("failed to bootstrap new datbase: %w", err)
		}

		return sqlite.NewIndexDBRepository(ctx, indexDB.QueryTimeout), nil, nil
	}

	isLatest, err := migrations.IsLatest(indexMigrations, indexDBMigrationDriver)
	if err != nil {
		indexDB.Close()
		return nil, notesConnCache, fmt.Errorf("failed to determine if database on latest migration: %w", err)
	}

	if !isLatest {
		logger.Info("database not on latest version - running migrations up")
		_, err = migrations.Up(indexMigrations, indexDBMigrationDriver)
		if err != nil {
			indexDB.Close()
			return nil, notesConnCache, fmt.Errorf("failed to run migrations up: %w", err)
		}

		return sqlite.NewIndexDBRepository(ctx, indexDB.QueryTimeout), nil, nil
	}

	// Check notes db for available migrations
	notesMigrations, err := migrations.Load(conf.SQLite.NotesDBMigrations)
	if err != nil {
		indexDB.Close()
		return nil, notesConnCache, err
	}

	latestNotesMigration, err := migrations.Latest(notesMigrations)
	if err != nil {
		indexDB.Close()
		return nil, notesConnCache, err
	}

	indexRepository := sqlite.NewIndexDBRepository(ctx, indexDB.QueryTimeout)
	dbsToUpgrade, err := indexRepository.DBVersionBefore(indexDB.Read, int(latestNotesMigration.Version))
	if err != nil {
		indexDB.Close()
		return nil, notesConnCache, err
	}

	for _, dbInfo := range dbsToUpgrade {
		// TODO: This is embarassingly parallelisable and should be rewritten for concurrency
		notesDBFP := filepath.Join(conf.SQLite.DBDirectory, dbInfo.DBPath)
		notesDB, err := sqlitex.NewSQLiteDB(ctx, notesDBFP,
			sqlitex.WithBusyTimeout(conf.SQLite.BusyTimeout),
			sqlitex.WithCacheSize(conf.SQLite.CacheSize),
			sqlitex.WithForeignKeys(conf.SQLite.ForeignKeys),
			sqlitex.WithJournalMode(conf.SQLite.JournalMode),
			sqlitex.WithSynchronous(conf.SQLite.Synchronous),
			sqlitex.WithTempStore(conf.SQLite.TempStore),
		)
		if err != nil {
			indexDB.Close()
			return nil, notesConnCache, err
		}

		notesDBMigrationDriver := sqlitex.NewMigrationDriver(notesDB, ctx)
		version, err := migrations.Up(notesMigrations, notesDBMigrationDriver)
		if err != nil {
			indexDB.Close()
			return nil, notesConnCache, err
		}
		indexRepository.UpdateDBVersion(indexDB.Write, dbInfo.UserId, version)

	}

	logger.Info("database on latest version - leaving it be")
	logger.Info("database configured", slog.Any("config", indexDB.LogDBConfig()))

	return indexRepository, notesConnCache, nil

}

func RunServer(conf *config.Config) error {
	logger := setupLogging(conf)

	ctx := context.Background()

	fmt.Fprint(os.Stdout, `
------------------------------------------
 ____ ____ ____ ____ ____ ____ ____ ____ 
||c |||o |||m |||m |||u |||n |||i |||s ||
||__|||__|||__|||__|||__|||__|||__|||__||
|/__\|/__\|/__\|/__\|/__\|/__\|/__\|/__\|

------------------------------------------

`)

	tc, err := handlers.NewTemplateCache(htmlFiles)
	if err != nil {
		return err
	}
	_, notesDBCache, err := connectToDatabase(conf, ctx, logger)
	if err != nil {
		return err
	}
	defer notesDBCache.Shutdown()

	// TODO: Clean up indexRepository connection

	// TODO: Remove this. Doing this to compile and be able to run CLI
	db, _ := notesDBCache.Get("need-to-just-compile")

	nr := sqlite.NewNoteRepository(db, ctx)
	tr := sqlite.NewTagRepository(db, ctx)
	sr := sqlite.NewSectionRepository(db, ctx)

	handler := routes(logger, tc, nr, tr, sr)

	srv := &http.Server{
		Addr:    net.JoinHostPort(conf.Web.Host, strconv.Itoa(int(conf.Web.Port))),
		Handler: handler,
	}

	wg := sync.WaitGroup{}
	shutdownError := make(chan error)
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		s := <-quit

		logger.Info("caught signal", "signal", s.String())

		ctxWTO, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		err := srv.Shutdown(ctxWTO)
		if err != nil {
			shutdownError <- err
		}

		logger.Info("completing background tasks", "addr", srv.Addr)

		wg.Wait()
		shutdownError <- nil
	}()

	logger.Info("starting server", "addr", srv.Addr)
	err = srv.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	err = <-shutdownError
	if err != nil {
		return err
	}

	logger.Info("stopped server", "addr", srv.Addr)

	return nil
}
