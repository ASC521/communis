package web

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"slices"
	"time"

	"github.com/ASC521/communis/cache"
	"github.com/ASC521/communis/config"
	"github.com/ASC521/communis/dbx/sqlitex"
	"github.com/ASC521/communis/models"
	"github.com/alexedwards/scs/v2"
)

type chain []func(http.Handler) http.Handler

func (c chain) thenFunc(h http.HandlerFunc) http.Handler {
	return c.then(h)
}

func (c chain) then(h http.Handler) http.Handler {
	for _, mw := range slices.Backward(c) {
		h = mw(h)
	}

	return h
}

type responseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func wrapResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w}
}

func (rw *responseWriter) Status() int {
	return rw.status
}

func (rw *responseWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}

	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
	rw.wroteHeader = true
}

func requestLogger(ignoreRE []string, logger *slog.Logger) func(next http.Handler) http.Handler {

	var ignore []*regexp.Regexp
	for _, re := range ignoreRE {
		regex, err := regexp.Compile(re)
		if err != nil {
			logger.Warn(fmt.Sprintf("Unable to exclude url path pattern %s - it is not a valid regex", re), "errMsg", err.Error())
			continue
		}
		ignore = append(ignore, regex)
	}

	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			start := time.Now().UTC()
			wrw := wrapResponseWriter(w)
			next.ServeHTTP(wrw, r)

			exclude := false
			for _, regex := range ignore {
				if regex.MatchString(r.URL.Path) {
					exclude = true
					break
				}
			}

			if !exclude {
				logger.Info(
					fmt.Sprintf("%s %s from %s", r.Method, r.URL.EscapedPath(), r.RemoteAddr),
					"respStatus", wrw.Status(),
					"method", r.Method,
					"path", r.URL.EscapedPath(),
					"duration", time.Since(start),
					"remoteAddr", r.RemoteAddr)
			}
		}

		return http.HandlerFunc(fn)
	}
}

func recoverPanic(logger *slog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				pv := recover()

				if pv != nil {
					w.Header().Set("Connection", "close")
					st := string(debug.Stack())
					logger.Error(
						fmt.Sprintf("%v", pv),
						"method", r.Method,
						"uri", r.URL.RequestURI(),
						"stacktrace", string(debug.Stack()))
					fmt.Fprint(os.Stderr, st+"\n")
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)

				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

func requireAuthentication(
	sessionManager *scs.SessionManager,
	notesDBCache *cache.TTLCache[int64, *sqlitex.SQLiteDB],
	indexRepo models.IndexRepository,
	conf *config.Config,
	logger *slog.Logger,
) func(http.Handler) http.Handler {

	return func(next http.Handler) http.Handler {
		hFunc := func(w http.ResponseWriter, r *http.Request) {

			authUserId := sessionManager.GetInt64(r.Context(), "authenticatedUserId")
			if authUserId == 0 {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}

			notesDB, ok := notesDBCache.Get(authUserId)
			if !ok {
				logger.Debug("cache miss - create new db connection")
				dbInfo, err := indexRepo.GetUserDB(r.Context(), authUserId)
				if err != nil {
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					return
				}

				dbFP := filepath.Join(conf.SQLite.DBDirectory, dbInfo.DBPath)
				notesDB, err = sqlitex.NewSQLiteDB(dbFP,
					sqlitex.WithBusyTimeout(conf.SQLite.BusyTimeout),
					sqlitex.WithCacheSize(conf.SQLite.CacheSize),
					sqlitex.WithForeignKeys(conf.SQLite.ForeignKeys),
					sqlitex.WithJournalMode(conf.SQLite.JournalMode),
					sqlitex.WithSynchronous(conf.SQLite.Synchronous),
					sqlitex.WithTempStore(conf.SQLite.TempStore),
				)

				if err != nil {
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					return
				}

				notesDBCache.Set(authUserId, notesDB, 24*time.Hour)
				logger.Debug(fmt.Sprintf("added connection to %s to cache", notesDB.DBPath))

			}

			// Set the "Cache-Control: no-store" header so that pages
			// which require authentication are not stored in the users
			// browser cache (or other intermediary cache).
			w.Header().Add("Cache-Control", "no-store")
			next.ServeHTTP(w, r.WithContext(sqlitex.NewContext(r.Context(), notesDB)))
		}

		return http.HandlerFunc(hFunc)
	}
}
