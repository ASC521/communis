package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/ASC521/communis/services"
)

func ConnCacheStateGet(
	tc *TemplateCache,
	logger *slog.Logger,
	dss *services.SQLiteDataStoreActor,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state := dss.GetState()
		bytes, err := json.Marshal(state)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, err = w.Write(bytes)
		if err != nil {
			tc.RenderError(logger, w, r, err)
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
