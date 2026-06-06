package handlers

import (
	"log/slog"
	"net/http"

	datastore "github.com/ASC521/communis/data-store"
	userstore "github.com/ASC521/communis/user-store"
	"github.com/alexedwards/scs/v2"
)

func HomeGet(
	tc *TemplateCache,
	logger *slog.Logger,
	dss *userstore.SQLiteConnManager,
	sessionManager *scs.SessionManager,
) http.HandlerFunc {

	type td struct {
		BaseData
		NoteDetails []datastore.NoteDetail
	}

	return func(w http.ResponseWriter, r *http.Request) {
		notesRepo, err := GetNotesDataStore(r, dss)
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
