package web

import (
	"context"
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
)

func setupLogging(c *config.Config) *slog.Logger {
	opt := &slog.HandlerOptions{}

	if c.VerboseLogging {
		opt.Level = slog.LevelDebug
	}

	h := slog.NewTextHandler(os.Stdout, opt)

	return slog.New(h)
}

func RunServer(conf *config.Config) error {

	logger := setupLogging(conf)
	mux := http.NewServeMux()
	addRoutes(mux, logger)

	srv := &http.Server{
		Addr:    net.JoinHostPort(conf.Web.Host, strconv.Itoa(int(conf.Web.Port))),
		Handler: mux,
	}

	wg := sync.WaitGroup{}
	ctx := context.Background()
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

	fmt.Fprint(os.Stdout, `
------------------------------------------
 ____ ____ ____ ____ ____ ____ ____ ____ 
||c |||o |||m |||m |||u |||n |||i |||s ||
||__|||__|||__|||__|||__|||__|||__|||__||
|/__\|/__\|/__\|/__\|/__\|/__\|/__\|/__\|

------------------------------------------

`)

	logger.Info("starting server", "addr", srv.Addr)
	err := srv.ListenAndServe()
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
