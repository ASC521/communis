//go:build sqlite_trace || trace

package sqlitex

import (
	"log/slog"
	"time"

	sqlite "github.com/mattn/go-sqlite3"
)

const traceSupported = true

func configureTrace(conn *sqlite.SQLiteConn, logger *slog.Logger, threshold time.Duration) error {
	callback := func(info sqlite.TraceInfo) int {
		switch info.EventCode {
		case sqlite.TraceStmt:
			logger.Debug("statement prepared", "stmt_handle", info.StmtHandle, "expanded_sql", info.ExpandedSQL, "conn_handle", info.ConnHandle)
		case sqlite.TraceProfile:
			stmtDuration := time.Duration(info.RunTimeNanosec)
			if stmtDuration > threshold {
				logger.Warn("statement executed", "stmt_handle", info.StmtHandle, "duration", stmtDuration.String(), "conn_handle", info.ConnHandle)
			} else {
				logger.Debug("statement executed", "stmt_handle", info.StmtHandle, "duration", stmtDuration.String(), "conn_handle", info.ConnHandle)
			}

		default:
		}
		// The integer return value from the callback is currently ignored,
		// though this may change in future releases. Callback implementations
		// should return zero to ensure future compatibility.
		return 0
	}

	return conn.SetTrace(&sqlite.TraceConfig{
		Callback:        callback,
		EventMask:       sqlite.TraceProfile | sqlite.TraceStmt,
		WantExpandedSQL: true,
	})
}
