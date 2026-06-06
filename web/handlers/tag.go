package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	datastore "github.com/ASC521/communis/data-store"
	userstore "github.com/ASC521/communis/user-store"
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
		tagId, err := parseIDFromPath(r)
		if err != nil {
			return tagForm{}, err
		}
		form.Id = tagId
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
	tc *TemplateCache,
	logger *slog.Logger,
	dss *userstore.SQLiteConnManager,
	sessionManager *scs.SessionManager,
) http.HandlerFunc {

	type td struct {
		BaseData
		Tags []datastore.Tag
	}

	return func(w http.ResponseWriter, r *http.Request) {
		notesRepo, err := GetNotesDataStore(r, dss)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		allTags, err := notesRepo.ListAllTags(r.Context())
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		data := td{
			BaseData: newBase(r),
			Tags:     allTags,
		}
		tc.RenderPage(logger, w, r, http.StatusOK, "tags-list.tmpl", data)
	}
}

func TagViewGet(
	tc *TemplateCache,
	logger *slog.Logger,
	dss *userstore.SQLiteConnManager,
	sessionManager *scs.SessionManager,
) http.HandlerFunc {
	type td struct {
		BaseData
		Tag         datastore.Tag
		NoteDetails []datastore.NoteDetail
	}
	return func(w http.ResponseWriter, r *http.Request) {
		tagId, err := parseIDFromPath(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		notesRepo, err := GetNotesDataStore(r, dss)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		tag, err := notesRepo.FindTagById(r.Context(), tagId)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		noteDetails, err := notesRepo.NotesWithTag(r.Context(), tagId)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		data := td{
			BaseData:    newBase(r),
			NoteDetails: noteDetails,
			Tag:         tag,
		}

		tc.RenderPage(logger, w, r, http.StatusOK, "tag-view.tmpl", data)

	}
}

func TagEditGet(
	tc *TemplateCache,
	logger *slog.Logger,
	dss *userstore.SQLiteConnManager,
) http.HandlerFunc {

	type td struct {
		Id          int64
		Name        string
		FieldErrors map[string]string
	}
	return func(w http.ResponseWriter, r *http.Request) {
		tagId, err := parseIDFromPath(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		notesRepo, err := GetNotesDataStore(r, dss)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		tag, err := notesRepo.FindTagById(r.Context(), tagId)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		tc.RenderPartial(logger, w, r, http.StatusOK, "put-tag", td{Id: tag.ID, Name: tag.Name, FieldErrors: map[string]string{}})

	}
}

func TagPut(
	tc *TemplateCache,
	logger *slog.Logger,
	dss *userstore.SQLiteConnManager,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		notesRepo, err := GetNotesDataStore(r, dss)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		form, err := parseTagFormFromRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		validateTagForm(r.Context(), &form, notesRepo)

		if len(form.FieldErrors) > 0 {
			tc.RenderPartial(logger, w, r, http.StatusOK, "put-tag", form)
			return
		}

		err = notesRepo.UpdateTag(r.Context(), datastore.Tag{ID: form.Id, Name: form.Name})
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		w.Header().Add("HX-Redirect", fmt.Sprintf("/tag/%v/%v", form.Id, slugify(form.Name)))
		w.WriteHeader(http.StatusSeeOther)

	}
}

func TagDelete(
	tc *TemplateCache,
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
			tc.RenderError(logger, w, r, err)
			return
		}

		err = notesRepo.DeleteTag(r.Context(), tagID)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		w.Header().Add("HX-Redirect", "/index")
		w.WriteHeader(http.StatusSeeOther)
	}
}

func TagPost(
	tc *TemplateCache,
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
			tc.RenderError(logger, w, r, err)
			return
		}

		err = validateTagForm(r.Context(), &form, notesRepo)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		if len(form.FieldErrors) > 0 {
			msg := ""
			for _, e := range form.FieldErrors {
				msg += fmt.Sprintf("<p>%s</p>\n", e)
			}
			tc.RenderPartial(logger, w, r, http.StatusUnprocessableEntity, "new-tag", td{ErrMsg: msg})
			return
		}

		id, err := notesRepo.CreateTag(r.Context(), datastore.Tag{Name: form.Name})
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		slog.Info(fmt.Sprintf("tag %d successfully created", id), "tagId", id)
		tc.RenderPartial(
			logger,
			w,
			r,
			http.StatusCreated,
			"new-tag",
			td{
				SuccessMsg: fmt.Sprintf("Tag %s created", form.Name),
				Tag:        &datastore.Tag{ID: id, Name: form.Name},
			},
		)

	}
}
