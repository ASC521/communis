package handlers

import (
	"log/slog"
	"net/http"

	"github.com/ASC521/communis/models"
	"github.com/alexedwards/scs/v2"
)

func GetAdmin(
	tc *TemplateCache,
	logger *slog.Logger,
	indexRepo models.IndexRepository,
	sessionManager *scs.SessionManager,
) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		users, err := indexRepo.ListUsers(r.Context())
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		data := TemplateData{
			IsAuthenticated: isAuthenticated(r, sessionManager),
			Users:           users,
		}

		tc.RenderPage(logger, w, r, http.StatusOK, "admin.tmpl", data)
	}
}
