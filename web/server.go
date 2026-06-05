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
	"github.com/ASC521/communis/hdx"
	userstore "github.com/ASC521/communis/user-store/sqlite"
	"github.com/ASC521/communis/web/handlers"

	"github.com/alexedwards/scs/v2"
)

//go:embed "static"
var staticFiles embed.FS

//go:embed "html"
var htmlFiles embed.FS

type ServerConfig struct {
	Host                string
	Port                uint
	HTTPSEnabled        bool
	CertFile            string
	KeyFile             string
	Debug               bool
	IgnoredLoggingPaths []string
}

func ConfigToServerConfig(conf *config.Config) (ServerConfig, error) {

	svrConf := ServerConfig{
		Debug: conf.Debug,
	}

	if conf.WebHost == "" {
		return ServerConfig{}, errors.New("web host is required")
	}
	svrConf.Host = conf.WebHost

	if conf.WebPort == 0 {
		return ServerConfig{}, errors.New("web port is required")
	}
	svrConf.Port = conf.WebPort

	if conf.WebEnableHTTPS {
		svrConf.HTTPSEnabled = true
		if conf.WebCert == "" {
			return ServerConfig{}, errors.New("https enabled - certificate file required")
		}
		certFile, err := hdx.ResolveFile(conf.WebCert)
		if err != nil {
			return ServerConfig{}, fmt.Errorf("failed to resolve path to certificate file: %s", err.Error())
		}
		svrConf.CertFile = certFile

		keyFile, err := hdx.ResolveFile(conf.WebKey)
		if err != nil {
			return ServerConfig{}, fmt.Errorf("failed to resolve path to key file: %s", err.Error())
		}
		svrConf.KeyFile = keyFile
	}

	svrConf.IgnoredLoggingPaths = make([]string, len(conf.WebLoggingIgnoredPaths))
	for i, ip := range conf.WebLoggingIgnoredPaths {
		svrConf.IgnoredLoggingPaths[i] = ip.Pattern
	}

	return svrConf, nil

}

func RunServer(conf ServerConfig, dsm *userstore.SQLiteDataStoreActor, logger *slog.Logger) error {
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
	serverLogger := logger.WithGroup("SERVER")

	tc, err := handlers.NewTemplateCache(htmlFiles, conf.Debug)
	if err != nil {
		return err
	}

	dsm.Start(ctx, wg)
	err = dsm.RunMigrations(ctx)
	if err != nil {
		return err
	}

	sessionManager := scs.New()
	sessionManager.Store = sqlitex.NewSessionStore(dsm.IndexDB)

	// Check if initial setup needs to be run
	initialSetupNeeded, err := dsm.UserStore.InitialSetupNeeded(ctx)
	if err != nil {
		return err
	}
	serverLogger.Info(fmt.Sprintf("initial setup = %v", initialSetupNeeded))

	handler := routes(serverLogger, tc, dsm, sessionManager, conf.IgnoredLoggingPaths, conf.Debug, &initialSetupNeeded)

	srv := &http.Server{
		Addr:    net.JoinHostPort(conf.Host, strconv.Itoa(int(conf.Port))),
		Handler: handler,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS13,
		},
		ErrorLog: slog.NewLogLogger(serverLogger.Handler(), slog.LevelError),
	}

	shutdownError := make(chan error)
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		s := <-quit

		serverLogger.Info("caught signal", "signal", s.String())

		ctxWTO, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		err := srv.Shutdown(ctxWTO)
		if err != nil {
			shutdownError <- err
		}

		serverLogger.Info("completing background tasks", "addr", srv.Addr)

		dsm.Stop()

		wg.Wait()
		shutdownError <- nil
	}()

	serverLogger.Info("starting server", "addr", srv.Addr)
	if conf.HTTPSEnabled {
		err = srv.ListenAndServeTLS(conf.CertFile, conf.KeyFile)
	} else {
		err = srv.ListenAndServe()
	}

	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	err = <-shutdownError
	if err != nil {
		return err
	}

	serverLogger.Info("stopped server", "addr", srv.Addr)

	return nil
}
