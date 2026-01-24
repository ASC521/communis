package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/ASC521/communis/models"
	"github.com/ASC521/communis/web/handlers/validator"
)

func parseIdFromPath(r *http.Request) (int64, error) {
	pathSectionId := r.PathValue("id")
	if pathSectionId == "" {
		return 0, errors.New("no id found in path")
	}

	sectionId, err := strconv.ParseInt(pathSectionId, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("section id %v is not a valid int", pathSectionId)
	}

	return sectionId, nil
}

type newSectionForm struct {
	Method      string
	Id          int64
	Name        string
	FieldErrors map[string]string
}

func parseSectionFormFromRequest(r *http.Request) (newSectionForm, error) {
	err := r.ParseForm()
	if err != nil {
		return newSectionForm{}, err
	}

	sectionName := r.PostForm.Get("section-name")

	sectionForm := newSectionForm{
		Method:      r.Method,
		Id:          0,
		Name:        sectionName,
		FieldErrors: map[string]string{},
	}

	if r.Method == "PUT" {
		sectionId, err := parseIdFromPath(r)
		if err != nil {
			return newSectionForm{}, err
		}
		sectionForm.Id = sectionId
	}

	return sectionForm, nil

}

func validateSectionForm(form *newSectionForm) {

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
	sr models.SectionRepository,
) http.Handler {

	type td struct {
		FormData newSectionForm
		Sections []models.Section
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		sections, err := sr.ListAll()
		if err != nil {
			serverError(logger, w, r, err)
			return
		}

		tc.RenderPage(logger, w, r, http.StatusOK, "section-list.tmpl", td{Sections: sections})
	})
}

func SectionPost(
	tc *TemplateCache,
	logger *slog.Logger,
	sr models.SectionRepository,
) http.Handler {
	type td struct {
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		form, err := parseSectionFormFromRequest(r)
		if err != nil {
			http.Error(w, "failed to parse form", http.StatusUnprocessableEntity)
			return
		}
		validateSectionForm(&form)
		if len(form.FieldErrors) > 0 {
			tc.RenderPartial(logger, w, r, http.StatusUnprocessableEntity, "post-section.tmpl", "new-section-form", form)
			return
		}

		_, err = sr.Create(models.Section{Name: form.Name})
		if err != nil {
			serverError(logger, w, r, err)
			return
		}

		w.Header().Add("HX-Redirect", "/section")
		w.WriteHeader(http.StatusSeeOther)
	})
}

func SectionNewGet(tc *TemplateCache, logger *slog.Logger) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tc.RenderPartial(logger, w, r, http.StatusOK, "post-section.tmpl", "new-section-form", newSectionForm{FieldErrors: map[string]string{}})
	})
}

func SectionDelete(
	tc *TemplateCache,
	logger *slog.Logger,
	sr models.SectionRepository,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		sectionId, err := parseIdFromPath(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		err = sr.Delete(sectionId)
		if err != nil {
			serverError(logger, w, r, err)
			return
		}
		w.Header().Add("HX-Redirect", "/section")
		w.WriteHeader(http.StatusSeeOther)
	})

}

func SectionPut(
	tc *TemplateCache,
	logger *slog.Logger,
	sr models.SectionRepository,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		form, err := parseSectionFormFromRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		validateSectionForm(&form)
		if len(form.FieldErrors) > 0 {
			tc.RenderPartial(logger, w, r, http.StatusOK, "put-section.tmpl", "update-section", form)
			return
		}

		err = sr.Update(models.Section{Id: form.Id, Name: form.Name})
		if err != nil {
			serverError(logger, w, r, err)
			return
		}

		w.Header().Add("HX-Redirect", fmt.Sprintf("/section/%v/%v", form.Id, slugify(form.Name)))
		w.WriteHeader(http.StatusSeeOther)
	})
}

func SectionEditGet(
	tc *TemplateCache,
	logger *slog.Logger,
	sr models.SectionRepository,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sectionId, err := parseIdFromPath(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		section, err := sr.FindById(sectionId)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				http.Error(w, fmt.Sprintf("section %v not found", sectionId), http.StatusNotFound)
				return
			}

			serverError(logger, w, r, err)
			return
		}

		sectionForm := newSectionForm{Id: section.Id, Name: section.Name, FieldErrors: map[string]string{}}
		tc.RenderPartial(logger, w, r, http.StatusOK, "put-section.tmpl", "update-section", sectionForm)
	})
}

func SectionViewGet(
	tc *TemplateCache,
	logger *slog.Logger,
	nr models.NoteRepository,
	sr models.SectionRepository,
) http.Handler {
	type td struct {
		Section     models.Section
		NoteDetails []models.NoteDetail
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		sec, err := sr.FindById(id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				http.Error(w, "section not found", http.StatusNotFound)
				return
			}
			serverError(logger, w, r, err)
			return
		}

		nds, err := nr.InSection(id)
		if err != nil {
			serverError(logger, w, r, err)
			return
		}

		tc.RenderPage(
			logger,
			w,
			r,
			http.StatusOK,
			"section-view.tmpl",
			td{Section: sec, NoteDetails: nds},
		)
	})
}
