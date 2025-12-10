package web

import (
	"html/template"
	"log/slog"
	"net/http"

	"github.com/ASC521/communis/models"
	"github.com/ASC521/communis/web/handlers"
)

// TODO: Move this handler to the handlers package
func handleHome(
	tc map[string]*template.Template,
	logger *slog.Logger,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		ts, ok := tc["home/home.tmpl"]
		if !ok {
			logger.Error("template home does not exist", "method", r.Method, "uri", r.URL.RequestURI())
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		err := ts.ExecuteTemplate(w, "layout", nil)
		if err != nil {
			logger.Error("failed to execute template", "errMsg", err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
	})

}

func routes(
	logger *slog.Logger,
	tc map[string]*template.Template,
	nr models.NoteRepository,
	tr models.TagRepository,
	sr models.SectionRepository,
) http.Handler {

	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.FileServerFS(staticFiles))

	baseChain := chain{recoverPanic(logger), requestLogger([]string{}, logger)}

	mux.Handle("GET /note/create", handlers.NoteCreateGet(tc, logger, tr, sr))
	mux.Handle("POST /note/create", handlers.NoteCreatePost(tc, logger, nr, sr, tr))
	mux.Handle("GET /note/{id}/{slug}", handlers.NoteViewGet(tc, logger, nr))
	mux.Handle("GET /edit/{id}/{slug}", handlers.NoteEditGet(tc, logger, nr, sr, tr))
	mux.Handle("POST /edit/{id}/{slug}", handlers.NoteEditPost(tc, logger, nr, sr, tr))
	mux.Handle("GET /section", handlers.SectionGet(tc, logger, sr))

	mux.Handle("GET /{$}", handleHome(tc, logger))

	return baseChain.then(mux)
}
