package handlers

import (
	"log/slog"
	"net/http"

	"github.com/ASC521/communis/models"
)

func HomeGet(
	tc *TemplateCache,
	logger *slog.Logger,
	newNotesRepo getNotesRepo,
) http.Handler {

	type templateData struct {
		Sections      []*models.Section
		ModifiedNotes []models.NoteDetail
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		notesRepo, ok := newNotesRepo(r)
		if !ok {
			serverError(logger, w, r, ErrNotesRepoNotFound)
			return
		}
		mn, err := notesRepo.RecentlyUpdatedNotes(r.Context(), 5)
		if err != nil {
			serverError(logger, w, r, err)
			return
		}

		tc.RenderPage(logger, w, r, http.StatusOK, "home.tmpl", templateData{ModifiedNotes: mn})
	})
}
