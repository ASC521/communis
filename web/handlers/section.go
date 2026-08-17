package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	datastore "github.com/ASC521/communis/data-store"
	"github.com/ASC521/communis/slug"
	userstore "github.com/ASC521/communis/user-store"
	"github.com/ASC521/communis/web/assets"
	"github.com/ASC521/communis/web/handlers/validator"
	"github.com/alexedwards/scs/v2"
)

type sectionForm struct {
	Method      string
	ID          int64
	Name        string
	FieldErrors map[string]string
}

func parseSectionFormFromRequest(r *http.Request) (sectionForm, error) {
	err := r.ParseForm()
	if err != nil {
		return sectionForm{}, err
	}

	sectionName := r.PostForm.Get("section-name")

	form := sectionForm{
		Method:      r.Method,
		ID:          0,
		Name:        sectionName,
		FieldErrors: map[string]string{},
	}

	if r.Method == "PUT" {
		sectionID, err := parseIDFromPath(r)
		if err != nil {
			return sectionForm{}, err
		}
		form.ID = sectionID
	}

	return form, nil
}

func validateSectionForm(form *sectionForm) {
	if form.Name == "" {
		form.FieldErrors["name"] = "Cannot be empty"
	}

	if !validator.MaxChars(form.Name, 25) {
		form.FieldErrors["name"] = "Cannot be more than 25 characters"
	}

	if form.Method == "PUT" && form.ID == 0 {
		form.FieldErrors["id"] = "Id cannot be empty"
	}
}

func SectionGet(
	htmlRenderer *assets.HTMLRenderer,
	logger *slog.Logger,
	dss *userstore.SQLiteConnManager,
	sessionManager *scs.SessionManager,
) http.HandlerFunc {
	type td struct {
		assets.BaseData
		Sections []datastore.Section
	}

	return func(w http.ResponseWriter, r *http.Request) {
		notesRepo, err := GetNotesDataStore(r, dss)
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}

		sections, err := notesRepo.ListAllSections(r.Context())
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}

		data := td{
			BaseData: extractBaseDataFromRequest(r),
			Sections: sections,
		}

		if err = htmlRenderer.Render(w, http.StatusOK, data, "base", "pages/section-list.tmpl"); err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
		}
	}
}

func SectionPost(
	htmlRenderer *assets.HTMLRenderer,
	logger *slog.Logger,
	dss *userstore.SQLiteConnManager,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		form, err := parseSectionFormFromRequest(r)
		if err != nil {
			http.Error(w, "failed to parse form", http.StatusUnprocessableEntity)
			return
		}

		validateSectionForm(&form)
		if len(form.FieldErrors) > 0 {
			err = htmlRenderer.Render(w, http.StatusUnprocessableEntity, form, "partial:section:new")
			if err != nil {
				logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
				htmlRenderer.RenderError(w, err)
			}
			return
		}

		notesRepo, err := GetNotesDataStore(r, dss)
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}

		_, err = notesRepo.CreateSection(r.Context(), datastore.Section{Name: form.Name})
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}

		w.Header().Add("HX-Redirect", "/section")
		w.WriteHeader(http.StatusSeeOther)
	}
}

func SectionNewGet(htmlRenderer *assets.HTMLRenderer, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := htmlRenderer.Render(w, http.StatusOK, sectionForm{FieldErrors: map[string]string{}}, "partial:section:new"); err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
		}
	}
}

func SectionDelete(
	htmlRenderer *assets.HTMLRenderer,
	logger *slog.Logger,
	dss *userstore.SQLiteConnManager,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sectionID, err := parseIDFromPath(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		notesRepo, err := GetNotesDataStore(r, dss)
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}

		err = notesRepo.DeleteSection(r.Context(), sectionID)
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}
		w.Header().Add("HX-Redirect", "/section")
		w.WriteHeader(http.StatusSeeOther)
	}
}

func SectionPut(
	htmlRenderer *assets.HTMLRenderer,
	logger *slog.Logger,
	dss *userstore.SQLiteConnManager,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		form, err := parseSectionFormFromRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		validateSectionForm(&form)
		if len(form.FieldErrors) > 0 {
			htmlRenderer.Render(w, http.StatusOK, form, "partial:section:update")
			return
		}

		notesRepo, err := GetNotesDataStore(r, dss)
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}

		err = notesRepo.UpdateSection(r.Context(), datastore.Section{ID: form.ID, Name: form.Name})
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}

		w.Header().Add("HX-Redirect", fmt.Sprintf("/section/%v/%v", form.ID, slug.Slugify(form.Name)))
		w.WriteHeader(http.StatusSeeOther)
	}
}

func SectionEditGet(
	htmlRenderer *assets.HTMLRenderer,
	logger *slog.Logger,
	dss *userstore.SQLiteConnManager,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sectionID, err := parseIDFromPath(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		notesRepo, err := GetNotesDataStore(r, dss)
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}

		section, err := notesRepo.FindSectionById(r.Context(), sectionID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				http.Error(w, fmt.Sprintf("section %v not found", sectionID), http.StatusNotFound)
				return
			}

			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}

		sectionForm := sectionForm{ID: section.ID, Name: section.Name, FieldErrors: map[string]string{}}
		if err = htmlRenderer.Render(w, http.StatusOK, sectionForm, "partial:section:update"); err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
		}
	}
}

func SectionViewGet(
	htmlRenderer *assets.HTMLRenderer,
	logger *slog.Logger,
	dss *userstore.SQLiteConnManager,
	sessionManager *scs.SessionManager,
) http.HandlerFunc {
	type td struct {
		assets.BaseData
		Section     datastore.Section
		NoteDetails []datastore.NoteDetail
	}
	return func(w http.ResponseWriter, r *http.Request) {
		sid := r.PathValue("id")
		if sid == "" {
			http.Error(w, "section id not found", http.StatusNotFound)
			return
		}

		id, err := strconv.ParseInt(sid, 10, 64)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		notesRepo, err := GetNotesDataStore(r, dss)
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}

		sec, err := notesRepo.FindSectionById(r.Context(), id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				http.Error(w, "section not found", http.StatusNotFound)
				return
			}
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}

		nds, err := notesRepo.NotesInSection(r.Context(), id)
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}

		data := td{
			BaseData:    extractBaseDataFromRequest(r),
			Section:     sec,
			NoteDetails: nds,
		}

		if err = htmlRenderer.Render(w, http.StatusOK, data, "base", "pages/section-view.tmpl"); err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
		}
	}
}
