package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	datastore "github.com/ASC521/communis/data-store"
	"github.com/ASC521/communis/slug"
	userstore "github.com/ASC521/communis/user-store"
	"github.com/ASC521/communis/web/assets"
	"github.com/ASC521/communis/web/handlers/validator"
	"github.com/alexedwards/scs/v2"
)

type tagForm struct {
	Method      string
	Id          int64
	Name        string
	FieldErrors map[string]string
}

func parseTagFormFromRequest(r *http.Request) (tagForm, error) {
	err := r.ParseForm()
	if err != nil {
		return tagForm{}, err
	}

	name := r.PostForm.Get("tag-name")
	form := tagForm{
		Method:      r.Method,
		Id:          0,
		Name:        name,
		FieldErrors: map[string]string{},
	}

	if r.Method == "PUT" {
		tagID, err := parseIDFromPath(r)
		if err != nil {
			return tagForm{}, err
		}
		form.Id = tagID
	}

	return form, nil
}

func validateTagForm(ctx context.Context, tf *tagForm, nr *datastore.SQLite) error {
	if !validator.NotBlank(tf.Name) {
		tf.FieldErrors["name"] = "Cannot be empty"
	}

	if !validator.MaxChars(tf.Name, 25) {
		tf.FieldErrors["name"] = "Cannot be more than 25 characters"
	}

	_, err := nr.FindTagByName(ctx, tf.Name)
	if err == nil {
		tf.FieldErrors["name"] = "Tag already exists"
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	return nil
}

func TagGet(
	htmlRenderer *assets.HTMLRenderer,
	logger *slog.Logger,
	dss *userstore.SQLiteConnManager,
	sessionManager *scs.SessionManager,
) http.HandlerFunc {
	type td struct {
		assets.BaseData
		Tags []datastore.Tag
	}

	return func(w http.ResponseWriter, r *http.Request) {
		notesRepo, err := GetNotesDataStore(r, dss)
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}

		allTags, err := notesRepo.ListAllTags(r.Context())
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}

		data := td{
			BaseData: extractBaseDataFromRequest(r),
			Tags:     allTags,
		}
		if err = htmlRenderer.Render(w, http.StatusOK, data, "base", "pages/tags-list.tmpl"); err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
		}
	}
}

func TagViewGet(
	htmlRenderer *assets.HTMLRenderer,
	logger *slog.Logger,
	dss *userstore.SQLiteConnManager,
	sessionManager *scs.SessionManager,
) http.HandlerFunc {
	type td struct {
		assets.BaseData
		Tag         datastore.Tag
		NoteDetails []datastore.NoteDetail
	}
	return func(w http.ResponseWriter, r *http.Request) {
		tagID, err := parseIDFromPath(r)
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

		tag, err := notesRepo.FindTagById(r.Context(), tagID)
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}

		noteDetails, err := notesRepo.NotesWithTag(r.Context(), tagID)
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}

		data := td{
			BaseData:    extractBaseDataFromRequest(r),
			NoteDetails: noteDetails,
			Tag:         tag,
		}

		if err = htmlRenderer.Render(w, http.StatusOK, data, "base", "pages/tag-view.tmpl"); err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
		}
	}
}

func TagEditGet(
	htmlRenderer *assets.HTMLRenderer,
	logger *slog.Logger,
	dss *userstore.SQLiteConnManager,
) http.HandlerFunc {
	type td struct {
		Id          int64
		Name        string
		FieldErrors map[string]string
	}
	return func(w http.ResponseWriter, r *http.Request) {
		tagID, err := parseIDFromPath(r)
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

		tag, err := notesRepo.FindTagById(r.Context(), tagID)
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}

		if err = htmlRenderer.Render(w, http.StatusOK, td{Id: tag.ID, Name: tag.Name, FieldErrors: map[string]string{}}, "partial:tag:put"); err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
		}
	}
}

func TagPut(
	htmlRenderer *assets.HTMLRenderer,
	logger *slog.Logger,
	dss *userstore.SQLiteConnManager,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		notesRepo, err := GetNotesDataStore(r, dss)
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}

		form, err := parseTagFormFromRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		validateTagForm(r.Context(), &form, notesRepo)

		if len(form.FieldErrors) > 0 {
			if err = htmlRenderer.Render(w, http.StatusOK, form, "partial:tag:put"); err != nil {
				logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
				htmlRenderer.RenderError(w, err)
			}
			return
		}

		err = notesRepo.UpdateTag(r.Context(), datastore.Tag{ID: form.Id, Name: form.Name})
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}

		w.Header().Add("HX-Redirect", fmt.Sprintf("/tag/%v/%v", form.Id, slug.Slugify(form.Name)))
		w.WriteHeader(http.StatusSeeOther)
	}
}

func TagDelete(
	htmlRenderer *assets.HTMLRenderer,
	logger *slog.Logger,
	dss *userstore.SQLiteConnManager,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tagID, err := parseIDFromPath(r)
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

		err = notesRepo.DeleteTag(r.Context(), tagID)
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}

		w.Header().Add("HX-Redirect", "/index")
		w.WriteHeader(http.StatusSeeOther)
	}
}

func TagPost(
	htmlRenderer *assets.HTMLRenderer,
	logger *slog.Logger,
	dss *userstore.SQLiteConnManager,
) http.HandlerFunc {
	type td struct {
		ErrMsg     string
		SuccessMsg string
		Tag        *datastore.Tag
	}

	return func(w http.ResponseWriter, r *http.Request) {
		form, err := parseTagFormFromRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		notesRepo, err := GetNotesDataStore(r, dss)
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}

		err = validateTagForm(r.Context(), &form, notesRepo)
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}

		if len(form.FieldErrors) > 0 {
			var msg strings.Builder
			for _, e := range form.FieldErrors {
				fmt.Fprintf(&msg, "<p>%s</p>\n", e)
			}
			if err = htmlRenderer.Render(w, http.StatusUnprocessableEntity, td{ErrMsg: msg.String()}, "partial:tag:new"); err != nil {
				logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
				htmlRenderer.RenderError(w, err)
			}
			return
		}

		id, err := notesRepo.CreateTag(r.Context(), datastore.Tag{Name: form.Name})
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}

		data := td{
			SuccessMsg: fmt.Sprintf("Tag %s created", form.Name),
			Tag:        &datastore.Tag{ID: id, Name: form.Name},
		}
		slog.Info(fmt.Sprintf("tag %d successfully created", id), "tagId", id)
		if err = htmlRenderer.Render(w, http.StatusCreated, data, "partial:tag:new"); err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
		}
	}
}
