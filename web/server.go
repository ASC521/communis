package web

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
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

	"github.com/ASC521/communis/config"
	"github.com/ASC521/communis/dbx"
	"github.com/ASC521/communis/dbx/sqlitex"
	"github.com/ASC521/communis/repository/sqlite"
)

//go:embed "static"
var staticFiles embed.FS

//go:embed "html"
var htmlFiles embed.FS

func setupLogging(c *config.Config) *slog.Logger {
	opt := &slog.HandlerOptions{}

	if c.VerboseLogging {
		opt.Level = slog.LevelDebug
	}

	h := slog.NewTextHandler(os.Stdout, opt)

	return slog.New(h)
}

func connectToDatabase(c *config.Config, ctx context.Context, logger *slog.Logger) (*sqlitex.SQLiteDB, error) {
	err := config.ValidSQLite(c.SQLite)
	if err != nil {
		return nil, err
	}

	db, err := sqlitex.NewSQLiteDB(ctx, c.SQLite.FilePath,
		sqlitex.WithBusyTimeout(c.SQLite.BusyTimeout),
		sqlitex.WithCacheSize(c.SQLite.CacheSize),
		sqlitex.WithForeignKeys(c.SQLite.ForeignKeys),
		sqlitex.WithJournalMode(c.SQLite.JournalMode),
		sqlitex.WithSynchronous(c.SQLite.Synchronous),
		sqlitex.WithTempStore(c.SQLite.TempStore),
	)
	if err != nil {
		return nil, err
	}

	mig, err := dbx.NewSQLiteMigrator(ctx, db)
	if err != nil {
		db.Close()
		return nil, err
	}

	emptyDB, err := mig.IsEmpty()
	if err != nil {
		db.Close()
		return nil, err
	}

	if emptyDB {
		logger.Info("fresh database - bootstrapping all the way up")
		err = mig.Bootstrap()
		if err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to bootstrap new datbase: %w", err)
		}
		return db, nil

	}

	isLatest, err := mig.IsLatest()
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to determine if database on latest migration: %w", err)
	}

	if !isLatest {
		logger.Info("database not on latest version - running migrations up")
		err = mig.Up()
		if err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to run migrations up: %w", err)
		}

		return db, nil
	}

	logger.Info("database on latest version - leaving it be")

	logger.Info("database configured", slog.Any("config", db.LogDBConfig()))

	return db, nil

}

func newTemplateCache() (map[string]*template.Template, error) {
	cache := map[string]*template.Template{}

	pages, err := fs.Glob(htmlFiles, "html/pages/*.tmpl")
	if err != nil {
		return nil, err
	}

	for _, page := range pages {
		name := filepath.Base(page)

		files := []string{
			"html/base.tmpl",
			page,
		}

		ts, err := template.New(name).ParseFS(htmlFiles, files...)
		if err != nil {
			return nil, err
		}

		cache[name] = ts

	}

	return cache, nil
}

func RunServer(conf *config.Config) error {

	ctx := context.Background()

	tc, err := newTemplateCache()
	if err != nil {
		return err
	}

	logger := setupLogging(conf)

	fmt.Fprint(os.Stdout, `
------------------------------------------
 ____ ____ ____ ____ ____ ____ ____ ____ 
||c |||o |||m |||m |||u |||n |||i |||s ||
||__|||__|||__|||__|||__|||__|||__|||__||
|/__\|/__\|/__\|/__\|/__\|/__\|/__\|/__\|

------------------------------------------

`)

	db, err := connectToDatabase(conf, ctx, logger)
	if err != nil {
		return err
	}
	defer db.Close()

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
