package handlers

import (
	"log/slog"
	"net/http"

	"github.com/alexedwards/scs/v2"
)

func HomeGet(
	tc *TemplateCache,
	logger *slog.Logger,
	newNotesRepo getNotesRepo,
	sessionManager *scs.SessionManager,
) http.Handler {

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
		td := TemplateData{
			NoteDetails:     mn,
			IsAuthenticated: isAuthenticated(r, sessionManager),
		}

		tc.RenderPage(logger, w, r, http.StatusOK, "home.tmpl", td)
	})
}
