package handlers

import (
	"log/slog"
	"net/http"

	"github.com/ASC521/communis/models"
)

func HomeGet(
	tc *TemplateCache,
	logger *slog.Logger,
	nr models.NoteRepository,
) http.Handler {

	type templateData struct {
		Sections      []*models.Section
		ModifiedNotes []*models.NoteDetail
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		mn, err := nr.RecentUpdates(5)
		if err != nil {
			serverError(logger, w, r, err)
			return
		}

		tc.RenderPage(logger, w, r, http.StatusOK, "home.tmpl", templateData{ModifiedNotes: mn})
	})
}
