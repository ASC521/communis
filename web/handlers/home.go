package handlers

import (
	"log/slog"
	"net/http"

	"github.com/ASC521/communis/models"
	"github.com/alexedwards/scs/v2"
)

func HomeGet(
	tc *TemplateCache,
	logger *slog.Logger,
	newNotesRepo getNotesRepo,
	sessionManager *scs.SessionManager,
) http.Handler {

	type td struct {
		BaseData
		NoteDetails []models.NoteDetail
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		notesRepo, ok := newNotesRepo(r)
		if !ok {
			tc.RenderError(logger, w, r, ErrNotesRepoNotFound)
			return
		}
		mn, err := notesRepo.RecentlyUpdatedNotes(r.Context(), 5)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}
		data := td{
			NoteDetails: mn,
			BaseData:    newBase(r, sessionManager),
		}

		tc.RenderPage(logger, w, r, http.StatusOK, "home.tmpl", data)
	})
}
