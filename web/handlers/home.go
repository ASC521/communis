package handlers

import (
	"log/slog"
	"net/http"

	"github.com/ASC521/communis/models"
	"github.com/ASC521/communis/services"
	"github.com/alexedwards/scs/v2"
)

func HomeGet(
	tc *TemplateCache,
	logger *slog.Logger,
	dss *services.SQLiteDataStoreActor,
	sessionManager *scs.SessionManager,
) http.HandlerFunc {

	type td struct {
		BaseData
		NoteDetails []models.NoteDetail
	}

	return func(w http.ResponseWriter, r *http.Request) {
		notesRepo, err := GetNotesRepo(r, dss)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		mn, err := notesRepo.RecentlyUpdatedNotes(r.Context(), 5)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}
		data := td{
			NoteDetails: mn,
			BaseData:    newBase(r),
		}

		tc.RenderPage(logger, w, r, http.StatusOK, "home.tmpl", data)
	}
}
