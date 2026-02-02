package web

import (
	"log/slog"
	"net/http"

	"github.com/ASC521/communis/cache"
	"github.com/ASC521/communis/dbx/sqlitex"
	"github.com/ASC521/communis/models"
	"github.com/ASC521/communis/web/handlers"
)

func routes(
	logger *slog.Logger,
	tc *handlers.TemplateCache,
	index models.IndexRepository,
	notesDBConnCache *cache.TTLCache[string, *sqlitex.SQLiteDB],
) http.Handler {

	mux := http.NewServeMux()

	mux.Handle("GET /static/", http.FileServerFS(staticFiles))

	mux.Handle("GET /note/{id}/{slug}", handlers.NoteViewGet(tc, logger, handlers.GetSQLiteNotesRepo))
	mux.Handle("GET /note/new", handlers.NoteNewGet(tc, logger, handlers.GetSQLiteNotesRepo))
	mux.Handle("POST /note/preview", handlers.NotePreviewPost(tc, logger, handlers.GetSQLiteNotesRepo))
	mux.Handle("POST /note", handlers.NotePost(tc, logger, handlers.GetSQLiteNotesRepo))
	mux.Handle("PUT /note/{id}/{slug}", handlers.NotePut(tc, logger, handlers.GetSQLiteNotesRepo))
	mux.Handle("DELETE /note/{id}/{slug}", handlers.NoteDelete(tc, logger, handlers.GetSQLiteNotesRepo))
	mux.Handle("GET /edit/{id}/{slug}", handlers.NoteEditGet(tc, logger, handlers.GetSQLiteNotesRepo))

	mux.Handle("GET /section", handlers.SectionGet(tc, logger, handlers.GetSQLiteNotesRepo))
	mux.Handle("POST /section", handlers.SectionPost(tc, logger, handlers.GetSQLiteNotesRepo))
	mux.Handle("PUT /section/{id}", handlers.SectionPut(tc, logger, handlers.GetSQLiteNotesRepo))
	mux.Handle("DELETE /section/{id}", handlers.SectionDelete(tc, logger, handlers.GetSQLiteNotesRepo))
	mux.Handle("GET /section/new", handlers.SectionNewGet(tc, logger))
	mux.Handle("GET /section/{id}/{slug}", handlers.SectionViewGet(tc, logger, handlers.GetSQLiteNotesRepo))
	mux.Handle("GET /section/{id}/edit", handlers.SectionEditGet(tc, logger, handlers.GetSQLiteNotesRepo))

	mux.Handle("GET /search", handlers.NoteSearchGet(tc, logger, handlers.GetSQLiteNotesRepo))

	mux.Handle("GET /index", handlers.TagGet(tc, logger, handlers.GetSQLiteNotesRepo))
	mux.Handle("POST /tag", handlers.TagPost(tc, logger, handlers.GetSQLiteNotesRepo))
	mux.Handle("PUT /tag/{id}", handlers.TagPut(tc, logger, handlers.GetSQLiteNotesRepo))
	mux.Handle("DELETE /tag/{id}", handlers.TagDelete(tc, logger, handlers.GetSQLiteNotesRepo))
	mux.Handle("GET /tag/{id}/{slug}", handlers.TagViewGet(tc, logger, handlers.GetSQLiteNotesRepo))
	mux.Handle("GET /tag/{id}/edit", handlers.TagEditGet(tc, logger, handlers.GetSQLiteNotesRepo))

	mux.Handle("GET /{$}", handlers.HomeGet(tc, logger, handlers.GetSQLiteNotesRepo))

	baseChain := chain{recoverPanic(logger), requestLogger([]string{}, logger)}
	return baseChain.then(mux)
}
