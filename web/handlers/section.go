package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/ASC521/communis/models"
	"github.com/ASC521/communis/services"
	"github.com/ASC521/communis/web/handlers/validator"
	"github.com/alexedwards/scs/v2"
)

type sectionForm struct {
	Method      string
	Id          int64
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
		Id:          0,
		Name:        sectionName,
		FieldErrors: map[string]string{},
	}

	if r.Method == "PUT" {
		sectionId, err := parseIdFromPath(r)
		if err != nil {
			return sectionForm{}, err
		}
		form.Id = sectionId
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

	if form.Method == "PUT" && form.Id == 0 {
		form.FieldErrors["id"] = "Id cannot be empty"
	}

}

func SectionGet(
	tc *TemplateCache,
	logger *slog.Logger,
	dss services.DataStoreService,
	sessionManager *scs.SessionManager,
) http.HandlerFunc {

	type td struct {
		BaseData
		Sections []models.Section
	}

	return func(w http.ResponseWriter, r *http.Request) {

		notesRepo, err := GetNotesRepo(r, dss)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		sections, err := notesRepo.ListAllSections(r.Context())
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		data := td{
			BaseData: newBase(r),
			Sections: sections,
		}

		tc.RenderPage(logger, w, r, http.StatusOK, "section-list.tmpl", data)
	}
}

func SectionPost(
	tc *TemplateCache,
	logger *slog.Logger,
	dss services.DataStoreService,
) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		form, err := parseSectionFormFromRequest(r)
		if err != nil {
			http.Error(w, "failed to parse form", http.StatusUnprocessableEntity)
			return
		}

		validateSectionForm(&form)
		if len(form.FieldErrors) > 0 {
			tc.RenderPartial(logger, w, r, http.StatusUnprocessableEntity, "new-section-form", form)
			return
		}

		notesRepo, err := GetNotesRepo(r, dss)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		_, err = notesRepo.CreateSection(r.Context(), models.Section{Name: form.Name})
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		w.Header().Add("HX-Redirect", "/section")
		w.WriteHeader(http.StatusSeeOther)
	}
}

func SectionNewGet(tc *TemplateCache, logger *slog.Logger) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		tc.RenderPartial(logger, w, r, http.StatusOK, "new-section-form", sectionForm{FieldErrors: map[string]string{}})
	}
}

func SectionDelete(
	tc *TemplateCache,
	logger *slog.Logger,
	dss services.DataStoreService,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		sectionId, err := parseIdFromPath(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		notesRepo, err := GetNotesRepo(r, dss)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		err = notesRepo.DeleteSection(r.Context(), sectionId)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}
		w.Header().Add("HX-Redirect", "/section")
		w.WriteHeader(http.StatusSeeOther)
	}

}

func SectionPut(
	tc *TemplateCache,
	logger *slog.Logger,
	dss services.DataStoreService,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		form, err := parseSectionFormFromRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		validateSectionForm(&form)
		if len(form.FieldErrors) > 0 {
			tc.RenderPartial(logger, w, r, http.StatusOK, "update-section", form)
			return
		}

		notesRepo, err := GetNotesRepo(r, dss)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		err = notesRepo.UpdateSection(r.Context(), models.Section{Id: form.Id, Name: form.Name})
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		w.Header().Add("HX-Redirect", fmt.Sprintf("/section/%v/%v", form.Id, slugify(form.Name)))
		w.WriteHeader(http.StatusSeeOther)
	}
}

func SectionEditGet(
	tc *TemplateCache,
	logger *slog.Logger,
	dss services.DataStoreService,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sectionId, err := parseIdFromPath(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		notesRepo, err := GetNotesRepo(r, dss)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		section, err := notesRepo.FindSectionById(r.Context(), sectionId)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				http.Error(w, fmt.Sprintf("section %v not found", sectionId), http.StatusNotFound)
				return
			}

			tc.RenderError(logger, w, r, err)
			return
		}

		sectionForm := sectionForm{Id: section.Id, Name: section.Name, FieldErrors: map[string]string{}}
		tc.RenderPartial(logger, w, r, http.StatusOK, "update-section", sectionForm)
	}
}

func SectionViewGet(
	tc *TemplateCache,
	logger *slog.Logger,
	dss services.DataStoreService,
	sessionManager *scs.SessionManager,
) http.HandlerFunc {
	type td struct {
		BaseData
		Section     models.Section
		NoteDetails []models.NoteDetail
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

		notesRepo, err := GetNotesRepo(r, dss)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		sec, err := notesRepo.FindSectionById(r.Context(), id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				http.Error(w, "section not found", http.StatusNotFound)
				return
			}
			tc.RenderError(logger, w, r, err)
			return
		}

		nds, err := notesRepo.NotesInSection(r.Context(), id)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		data := td{
			BaseData:    newBase(r),
			Section:     sec,
			NoteDetails: nds,
		}

		tc.RenderPage(
			logger,
			w,
			r,
			http.StatusOK,
			"section-view.tmpl",
			data,
		)
	}
}
