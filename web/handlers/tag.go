package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/ASC521/communis/models"
	"github.com/ASC521/communis/web/handlers/validator"
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
		tagId, err := parseIdFromPath(r)
		if err != nil {
			return tagForm{}, err
		}
		form.Id = tagId
	}

	return form, nil
}

func validateTagForm(ctx context.Context, tf *tagForm, nr models.NotesRepository) error {

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
	newNotesRepo getNotesRepo,
) http.Handler {

	type td struct {
		Tags []models.Tag
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		notesRepo, ok := newNotesRepo(r)
		if !ok {
			serverError(logger, w, r, ErrNotesRepoNotFound)
			return
		}
		allTags, err := notesRepo.ListAllTags(r.Context())
		if err != nil {
			serverError(logger, w, r, err)
			return
		}

		tc.RenderPage(logger, w, r, http.StatusOK, "tags-list.tmpl", td{Tags: allTags})
	})
}

func TagViewGet(
	tc *TemplateCache,
	logger *slog.Logger,
	newNotesRepo getNotesRepo,
) http.Handler {
	type td struct {
		Tag         models.Tag
		NoteDetails []models.NoteDetail
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tagId, err := parseIdFromPath(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		notesRepo, ok := newNotesRepo(r)
		if !ok {
			serverError(logger, w, r, ErrNotesRepoNotFound)
			return
		}

		tag, err := notesRepo.FindTagById(r.Context(), tagId)
		if err != nil {
			serverError(logger, w, r, err)
			return
		}

		noteDetails, err := notesRepo.NotesWithTag(r.Context(), tagId)
		if err != nil {
			serverError(logger, w, r, err)
			return
		}

		tc.RenderPage(logger, w, r, http.StatusOK, "tag-view.tmpl", td{Tag: tag, NoteDetails: noteDetails})

	})
}

func TagEditGet(
	tc *TemplateCache,
	logger *slog.Logger,
	newNotesRepo getNotesRepo,
) http.Handler {

	type td struct {
		Id          int64
		Name        string
		FieldErrors map[string]string
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tagId, err := parseIdFromPath(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		notesRepo, ok := newNotesRepo(r)
		if !ok {
			serverError(logger, w, r, ErrNotesRepoNotFound)
			return
		}

		tag, err := notesRepo.FindTagById(r.Context(), tagId)
		if err != nil {
			serverError(logger, w, r, err)
			return
		}

		tc.RenderPartial(logger, w, r, http.StatusOK, "put-tag.tmpl", "put-tag", td{Id: tag.Id, Name: tag.Name, FieldErrors: map[string]string{}})

	})
}

func TagPut(
	tc *TemplateCache,
	logger *slog.Logger,
	newNotesRepo getNotesRepo,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		notesRepo, ok := newNotesRepo(r)
		if !ok {
			serverError(logger, w, r, ErrNotesRepoNotFound)
			return
		}
		form, err := parseTagFormFromRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		validateTagForm(r.Context(), &form, notesRepo)

		if len(form.FieldErrors) > 0 {
			tc.RenderPartial(logger, w, r, http.StatusOK, "put-tag.tmpl", "put-tag", form)
			return
		}

		err = notesRepo.UpdateTag(r.Context(), models.Tag{Id: form.Id, Name: form.Name})
		if err != nil {
			serverError(logger, w, r, err)
			return
		}

		w.Header().Add("HX-Redirect", fmt.Sprintf("/tag/%v/%v", form.Id, slugify(form.Name)))
		w.WriteHeader(http.StatusSeeOther)

	})
}

func TagDelete(
	tc *TemplateCache,
	logger *slog.Logger,
	newNotesRepo getNotesRepo,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tagId, err := parseIdFromPath(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		notesRepo, ok := newNotesRepo(r)
		if !ok {
			serverError(logger, w, r, ErrNotesRepoNotFound)
			return
		}

		err = notesRepo.DeleteTag(r.Context(), tagId)
		if err != nil {
			serverError(logger, w, r, err)
			return
		}

		w.Header().Add("HX-Redirect", "/index")
		w.WriteHeader(http.StatusSeeOther)
	})
}

func TagPost(
	tc *TemplateCache,
	logger *slog.Logger,
	newNotesRepo getNotesRepo,
) http.Handler {

	type td struct {
		ErrMsg     string
		SuccessMsg string
		Tag        *models.Tag
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		form, err := parseTagFormFromRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		notesRepo, ok := newNotesRepo(r)
		if !ok {
			serverError(logger, w, r, ErrNotesRepoNotFound)
			return
		}

		err = validateTagForm(r.Context(), &form, notesRepo)
		if err != nil {
			serverError(logger, w, r, err)
			return
		}

		if len(form.FieldErrors) > 0 {
			msg := ""
			for _, e := range form.FieldErrors {
				msg += fmt.Sprintf("<p>%s</p>\n", e)
			}
			tc.RenderPartial(logger, w, r, http.StatusUnprocessableEntity, "new-tag.tmpl", "new-tag", td{ErrMsg: msg})
			return
		}

		id, err := notesRepo.CreateTag(r.Context(), models.Tag{Name: form.Name})
		if err != nil {
			serverError(logger, w, r, err)
			return
		}

		slog.Info(fmt.Sprintf("tag %d successfully created", id), "tagId", id)
		tc.RenderPartial(
			logger,
			w,
			r,
			http.StatusCreated,
			"new-tag.tmpl",
			"new-tag",
			td{
				SuccessMsg: fmt.Sprintf("Tag %s created", form.Name),
				Tag:        &models.Tag{Id: id, Name: form.Name},
			},
		)

	})
}
