package web

import (
	"log/slog"
	"net/http"

	"github.com/ASC521/communis/models"
	"github.com/ASC521/communis/web/handlers"
)

// TODO: Move this handler to the handlers package
func handleHome(
	tc *handlers.TemplateCache,
	logger *slog.Logger,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tc.RenderPage(logger, w, r, http.StatusOK, "home.tmpl", nil)
	})
}

func routes(
	logger *slog.Logger,
	tc *handlers.TemplateCache,
	nr models.NoteRepository,
	tr models.TagRepository,
	sr models.SectionRepository,
) http.Handler {

	mux := http.NewServeMux()

	mux.Handle("GET /static/", http.FileServerFS(staticFiles))

	mux.Handle("GET /new", handlers.NoteNewGet(tc, logger, tr, sr))
	mux.Handle("POST /new", handlers.NoteNewPost(tc, logger, nr, sr, tr))

	mux.Handle("GET /note/{id}/{slug}", handlers.NoteViewGet(tc, logger, nr))

	mux.Handle("GET /edit/{id}/{slug}", handlers.NoteEditGet(tc, logger, nr, sr, tr))
	mux.Handle("POST /edit/{id}/{slug}", handlers.NoteEditPost(tc, logger, nr, sr, tr))

	mux.Handle("GET /section", handlers.SectionGet(tc, logger, sr))
	mux.Handle("GET /search", handlers.NoteSearchGet(tc, logger, nr))

	mux.Handle("GET /{$}", handleHome(tc, logger))
	baseChain := chain{recoverPanic(logger), requestLogger([]string{}, logger)}
	return baseChain.then(mux)
}
