package handlers

import (
	"html/template"
	"log/slog"
	"net/http"

	"github.com/ASC521/communis/models"
)

func SectionGet(
	tc map[string]*template.Template,
	logger *slog.Logger,
	sr models.SectionRepository,
) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		sections, err := sr.ListAll()
		if err != nil {
			serverError(logger, w, r, err)
			return
		}

		renderTemplate(tc, logger, w, r, http.StatusOK, "section.tmpl", sections)
	})
}
