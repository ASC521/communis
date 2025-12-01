package web

import (
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"slices"
	"time"
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
					logger.Error(fmt.Sprintf("%v", pv), "method", r.Method, "uri", r.URL.RequestURI())
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
