package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/ASC521/communis/models"
	"github.com/ASC521/communis/web/handlers/validator"
)

func TagPost(
	tc *TemplateCache,
	logger *slog.Logger,
	tr models.TagRepository,
) http.Handler {

	type td struct {
		ErrMsg     string
		SuccessMsg string
		Tag        *models.Tag
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		err := r.ParseForm()
		if err != nil {
			serverError(logger, w, r, err)
			return
		}
		name := r.PostForm.Get("tag-name")

		v := validator.Validator{}
		v.CheckField(validator.NotBlank(name), "name", "name cannot be empty")
		v.CheckField(validator.MaxChars(name, 25), "name", "name cannot be more than 25 characters")

		if len(v.FieldErrors) > 0 {
			msg := ""
			for _, e := range v.FieldErrors {
				msg += fmt.Sprintf("<p>%s</p>\n", e)
			}
			tc.RenderPartial(logger, w, r, http.StatusUnprocessableEntity, "new-tag.tmpl", "new-tag", td{ErrMsg: msg})
			return
		}

		_, err = tr.FindByName(name)
		if err == nil {
			tc.RenderPartial(logger, w, r, http.StatusUnprocessableEntity, "new-tag.tmpl", "new-tag", td{ErrMsg: fmt.Sprintf("Tag %s exists", name)})
			return
		}
		if !errors.Is(err, sql.ErrNoRows) {
			serverError(logger, w, r, err)
			return
		}

		id, err := tr.Create(&models.Tag{Name: name})
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
				SuccessMsg: fmt.Sprintf("Tag %s created", name),
				Tag:        &models.Tag{Id: id, Name: name},
			},
		)

	})
}
