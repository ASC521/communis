package web

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"runtime/debug"
	"slices"
	"time"

	"github.com/ASC521/communis/dbx/sqlitex"
	"github.com/ASC521/communis/models"
	"github.com/ASC521/communis/services"
	"github.com/ASC521/communis/web/handlers"
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

func requireAuth(sessionManager *scs.SessionManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authUserId := sessionManager.GetInt64(r.Context(), "authenticatedUserId")
			if authUserId == 0 {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}

			// Set the "Cache-Control: no-store" header so that pages
			// which require authentication are not stored in the users
			// browser cache (or other intermediary cache).
			w.Header().Add("Cache-Control", "no-store")
			next.ServeHTTP(w, r)
		})
	}
}

func requireAuthAndDB(
	sessionManager *scs.SessionManager,
	sqliteConnSvc *services.SQLiteConnService,
	logger *slog.Logger,
	tc *handlers.TemplateCache,
) func(http.Handler) http.Handler {

	return func(next http.Handler) http.Handler {
		hFunc := func(w http.ResponseWriter, r *http.Request) {

			authUserId := sessionManager.GetInt64(r.Context(), "authenticatedUserId")
			if authUserId == 0 {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}

			db, err := sqliteConnSvc.GetConn(r.Context(), authUserId)
			if err != nil {
				tc.RenderError(logger, w, r, err)
				return
			}

			// Set the "Cache-Control: no-store" header so that pages
			// which require authentication are not stored in the users
			// browser cache (or other intermediary cache).
			w.Header().Add("Cache-Control", "no-store")
			next.ServeHTTP(w, r.WithContext(sqlitex.NewContext(r.Context(), db)))
		}

		return http.HandlerFunc(hFunc)
	}
}

func requireAdmin(
	sessionManager *scs.SessionManager,
	indexRepo models.IndexRepository,
) func(http.Handler) http.Handler {

	return func(next http.Handler) http.Handler {
		handlerFunc := func(w http.ResponseWriter, r *http.Request) {
			authUserId := sessionManager.GetInt64(r.Context(), "authenticatedUserId")
			if authUserId == 0 {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}

			isAdmin, err := indexRepo.IsAdminUser(r.Context(), authUserId)
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			if !isAdmin {
				http.Redirect(w, r, "/login", http.StatusForbidden)
				return
			}

			w.Header().Add("Cache-Control", "no-store")
			next.ServeHTTP(w, r)
		}

		return http.HandlerFunc(handlerFunc)
	}

}
