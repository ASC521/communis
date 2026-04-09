package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/ASC521/communis/services"
)

func ConnCacheStateGet(
	tc *TemplateCache,
	logger *slog.Logger,
	dss services.DataStoreService,
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
