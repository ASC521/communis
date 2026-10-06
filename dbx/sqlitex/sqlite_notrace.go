//go:build !sqlite_trace && !trace

package sqlitex

import (
	"errors"
	"log/slog"
	"time"

	sqlite "github.com/mattn/go-sqlite3"
)

const traceSupported = false

func configureTrace(conn *sqlite.SQLiteConn, logger *slog.Logger, threshold time.Duration) error {
	return errors.New("sqlite tracing requested but binary built without sqlite_trace tag")
}
