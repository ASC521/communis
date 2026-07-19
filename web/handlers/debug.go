package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime/debug"

	userstore "github.com/ASC521/communis/user-store"
	"github.com/ASC521/communis/web/assets"
)

func ConnCacheStateGet(
	htmlRenderer *assets.HTMLRenderer,
	logger *slog.Logger,
	dss *userstore.SQLiteConnManager,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state := dss.GetState()
		bytes, err := json.Marshal(state)
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, err = w.Write(bytes)
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}
	}
}

func GetDebugBuildInfo() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bi, ok := debug.ReadBuildInfo()
		if !ok {
			w.Write([]byte("failed to read build info"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(bi.String()))
	}
}
