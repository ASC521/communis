package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"runtime/debug"
	"slices"
	"strings"
	"time"

	userstore "github.com/ASC521/communis/user-store"
	"github.com/alexedwards/scs/v2"
)

type Chain []func(http.Handler) http.Handler

func (c Chain) Then(h http.Handler) http.Handler {
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

func RequestLogger(ignoreRE []string, logger *slog.Logger) func(next http.Handler) http.Handler {

	routerLogger := logger.WithGroup("ROUTER")
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
				routerLogger.Info(
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

func RecoverPanic(logger *slog.Logger) func(next http.Handler) http.Handler {
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

func Authenticate(sessionManager *scs.SessionManager, userStore *userstore.SQLite) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authUserID := sessionManager.GetInt64(r.Context(), "authenticatedUserId")
			if authUserID == 0 {
				ctx := context.WithValue(r.Context(), isAuthenticatedContextKey, false)
				ctx = context.WithValue(ctx, isAdminContextKey, false)
				ctx = context.WithValue(ctx, userThemeContextKey, "light")
				r = r.WithContext(ctx)
			} else {
				user, err := userStore.GetUser(r.Context(), authUserID)
				if err != nil {
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					return
				}
				ctx := context.WithValue(r.Context(), isAuthenticatedContextKey, true)
				ctx = context.WithValue(ctx, isAdminContextKey, user.IsAdmin)
				ctx = context.WithValue(ctx, userIDContextKey, user.ID)
				ctx = context.WithValue(ctx, userThemeContextKey, user.Theme)
				r = r.WithContext(ctx)
			}

			// Set the "Cache-Control: no-store" header so that pages
			// which require authentication are not stored in the users
			// browser cache (or other intermediary cache).
			w.Header().Add("Cache-Control", "no-store")
			next.ServeHTTP(w, r)

		})
	}
}

func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		isAuthenticated, ok := r.Context().Value(isAuthenticatedContextKey).(bool)
		if !ok {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		if !isAuthenticated {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func RedirectAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		isAdmin, ok := r.Context().Value(isAdminContextKey).(bool)
		if !ok {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		if isAdmin {
			http.Redirect(w, r, "/admin", http.StatusSeeOther)
			return
		}

		next.ServeHTTP(w, r)

	})
}

func RequireAdmin(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		isAuthenticated, ok := r.Context().Value(isAuthenticatedContextKey).(bool)
		if !ok {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		if !isAuthenticated {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		isAdmin, ok := r.Context().Value(isAdminContextKey).(bool)
		if !ok {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		if !isAdmin {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func InitialSetup(setupRequired *bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.EscapedPath()
			if *setupRequired && path != "/setup" && !strings.Contains(path, "/static/") {
				http.Redirect(w, r, "/setup", http.StatusSeeOther)
				return
			}
			next.ServeHTTP(w, r)
		})
	}

}

func CommonHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-eval'; img-src 'self'; font-src 'self'; style-src 'self' 'sha256-faU7yAF8NxuMTNEwVmBz+VcYeIoBQ2EMHW3WaVxCvnk='")
		w.Header().Set("Referrer-Policy", "origin-when-cross-origin")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "deny")
		w.Header().Set("X-XSS-Protection", "0")
		next.ServeHTTP(w, r)
	})
}

func CrossOriginProtection(next http.Handler) http.Handler {
	cop := http.NewCrossOriginProtection()
	return cop.Handler(next)
}
