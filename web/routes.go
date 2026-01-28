package web

import (
	"log/slog"
	"net/http"

	"github.com/ASC521/communis/models"
	"github.com/ASC521/communis/web/handlers"
)

func routes(
	logger *slog.Logger,
	tc *handlers.TemplateCache,
	nr models.NoteRepository,
	tr models.TagRepository,
	sr models.SectionRepository,
) http.Handler {

	mux := http.NewServeMux()

	mux.Handle("GET /static/", http.FileServerFS(staticFiles))

	mux.Handle("GET /note/{id}/{slug}", handlers.NoteViewGet(tc, logger, nr))
	mux.Handle("GET /note/new", handlers.NoteNewGet(tc, logger, tr, sr))
	mux.Handle("POST /note/preview", handlers.NotePreviewPost(tc, logger, tr, sr))
	mux.Handle("POST /note", handlers.NotePost(tc, logger, nr, sr, tr))
	mux.Handle("PUT /note/{id}/{slug}", handlers.NotePut(tc, logger, nr, sr, tr))
	mux.Handle("DELETE /note/{id}/{slug}", handlers.NoteDelete(tc, logger, nr))
	mux.Handle("GET /edit/{id}/{slug}", handlers.NoteEditGet(tc, logger, nr, sr, tr))

	mux.Handle("GET /section", handlers.SectionGet(tc, logger, sr))
	mux.Handle("POST /section", handlers.SectionPost(tc, logger, sr))
	mux.Handle("PUT /section/{id}", handlers.SectionPut(tc, logger, sr))
	mux.Handle("DELETE /section/{id}", handlers.SectionDelete(tc, logger, sr))
	mux.Handle("GET /section/new", handlers.SectionNewGet(tc, logger))
	mux.Handle("GET /section/{id}/{slug}", handlers.SectionViewGet(tc, logger, nr, sr))
	mux.Handle("GET /section/{id}/edit", handlers.SectionEditGet(tc, logger, sr))

	mux.Handle("GET /search", handlers.NoteSearchGet(tc, logger, nr))

	mux.Handle("GET /index", handlers.TagGet(tc, logger, tr))
	mux.Handle("POST /tag", handlers.TagPost(tc, logger, tr))
	mux.Handle("PUT /tag/{id}", handlers.TagPut(tc, logger, tr))
	mux.Handle("DELETE /tag/{id}", handlers.TagDelete(tc, logger, tr))
	mux.Handle("GET /tag/{id}/{slug}", handlers.TagViewGet(tc, logger, nr, tr))
	mux.Handle("GET /tag/{id}/edit", handlers.TagEditGet(tc, logger, tr))

	mux.Handle("GET /{$}", handlers.HomeGet(tc, logger, nr))
	baseChain := chain{recoverPanic(logger), requestLogger([]string{}, logger)}
	return baseChain.then(mux)
}
