package handlers

import (
	"log/slog"
	"net/http"

	"github.com/ASC521/communis/models"
)

func SectionGet(
	tc *TemplateCache,
	logger *slog.Logger,
	sr models.SectionRepository,
) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		sections, err := sr.ListAll()
		if err != nil {
			serverError(logger, w, r, err)
			return
		}

		tc.RenderPage(logger, w, r, http.StatusOK, "section-list.tmpl", sections)
	})
}
