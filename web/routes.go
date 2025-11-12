package web

import (
	"html/template"
	"log/slog"
	"net/http"

	"github.com/ASC521/communis/models"
)

func handleHome(
	tc map[string]*template.Template,
	logger *slog.Logger,
	nr models.NoteRepository,
	tr models.TagRepository,
	sr models.SectionRepository,
) http.Handler {

	type tempData struct {
		notes    []models.Note
		tags     []models.Tag
		sections []models.Section
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		ts, ok := tc["home.tmpl"]
		if !ok {
			logger.Error("template home does not exist", "method", r.Method, "uri", r.URL.RequestURI())
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		err := ts.ExecuteTemplate(w, "base", nil)
		if err != nil {
			logger.Error("failed to execute template", "errMsg", err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
	})

}

func addRoutes(
	mux *http.ServeMux,
	logger *slog.Logger,
	tc map[string]*template.Template,
	nr models.NoteRepository,
	tr models.TagRepository,
	sr models.SectionRepository,
) {

	mux.Handle("GET /static/", http.FileServerFS(staticFiles))

	baseChain := chain{RequestLogger([]string{}, logger)}

	mux.Handle("GET /{$}", baseChain.then(handleHome(tc, logger, nr, tr, sr)))

}
