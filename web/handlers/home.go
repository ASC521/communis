package handlers

import (
	"log/slog"
	"net/http"

	datastore "github.com/ASC521/communis/data-store"
	userstore "github.com/ASC521/communis/user-store"
	"github.com/ASC521/communis/web/assets"
	"github.com/alexedwards/scs/v2"
)

func HomeGet(
	htmlRenderer *assets.HTMLRenderer,
	logger *slog.Logger,
	dss *userstore.SQLiteConnManager,
	sessionManager *scs.SessionManager,
) http.HandlerFunc {
	type td struct {
		assets.BaseData
		RecentNotes     []datastore.NoteDetail
		BookmarkedNotes []datastore.NoteDetail
		ReferencedNotes []datastore.NoteDetail
		HubNotes        []datastore.NoteDetail
		CareNotes       []datastore.NoteDetail
		Sections        []datastore.Section
	}

	limit := 10

	return func(w http.ResponseWriter, r *http.Request) {
		notesRepo, err := GetNotesDataStore(r, dss)
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}

		mn, err := notesRepo.RecentlyModifiedNotes(r.Context(), limit)
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}
		bn, err := notesRepo.BookmarkedNotes(r.Context(), limit)
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}
		rn, err := notesRepo.MostReferencedNotes(r.Context(), limit)
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}
		hn, err := notesRepo.NoteWithMostReferences(r.Context(), limit)
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}
		needCare, err := notesRepo.NotesWithNoContent(r.Context(), limit)
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}
		secs, err := notesRepo.RecentlyActiveSections(r.Context(), limit)
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}
		data := td{
			RecentNotes:     mn,
			BookmarkedNotes: bn,
			ReferencedNotes: rn,
			Sections:        secs,
			CareNotes:       needCare,
			HubNotes:        hn,
			BaseData:        extractBaseDataFromRequest(r),
		}

		if err = htmlRenderer.Render(w, http.StatusOK, data, "base", "pages/home.tmpl"); err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
		}
	}
}
