package web

import (
	"context"
	"crypto/tls"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/ASC521/communis/config"
	"github.com/ASC521/communis/dbx/sqlitex"
	"github.com/ASC521/communis/services"
	"github.com/ASC521/communis/web/handlers"

	"github.com/alexedwards/scs/v2"
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

	h := slog.NewTextHandler(os.Stderr, &opts)

	return slog.New(h)
}

func RunServer(conf *config.Config) error {
	logger := setupLogging(conf)

	ctx := context.Background()
	wg := &sync.WaitGroup{}

	fmt.Fprint(os.Stdout, `
------------------------------------------
 ____ ____ ____ ____ ____ ____ ____ ____
||c |||o |||m |||m |||u |||n |||i |||s ||
||__|||__|||__|||__|||__|||__|||__|||__||
|/__\|/__\|/__\|/__\|/__\|/__\|/__\|/__\|

------------------------------------------

`)

	tc, err := handlers.NewTemplateCache(htmlFiles, conf.Web.Debug)
	if err != nil {
		return err
	}

	dataStoreConfig, err := services.ConfigToSQLiteDataStoreConfig(*conf)
	if err != nil {
		return err
	}
	dataStoreSvc, err := services.NewSQLiteDataStoreActor(wg, time.Hour*12, dataStoreConfig, logger)
	if err != nil {
		return err
	}
	err = dataStoreSvc.RunMigrations(ctx)
	if err != nil {
		return err
	}

	sessionManager := scs.New()
	sessionManager.Store = sqlitex.NewSessionStore(dataStoreSvc.GetIndexDatabase())

	// Check if initial setup needs to be run
	initialSetupNeeded, err := dataStoreSvc.GetUserStore().InitialSetupNeeded(ctx)
	if err != nil {
		return err
	}
	logger.Info(fmt.Sprintf("initial setup = %v", initialSetupNeeded))

	handler := routes(logger, tc, dataStoreSvc, sessionManager, conf.Web.LoggingIgnoredPaths, conf.Web.Debug, &initialSetupNeeded)

	srv := &http.Server{
		Addr:    net.JoinHostPort(conf.Web.Host, strconv.Itoa(int(conf.Web.Port))),
		Handler: handler,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS13,
		},
	}

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

		dataStoreSvc.Stop()

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
