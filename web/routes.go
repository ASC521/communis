package web

import (
	"log/slog"
	"net/http"

	"github.com/ASC521/communis/cache"
	"github.com/ASC521/communis/config"
	"github.com/ASC521/communis/dbx/sqlitex"
	"github.com/ASC521/communis/models"
	"github.com/ASC521/communis/web/handlers"
	"github.com/alexedwards/scs/v2"
)

func routes(
	logger *slog.Logger,
	tc *handlers.TemplateCache,
	indexRepo models.IndexRepository,
	notesDBConnCache *cache.TTLCache[int64, *sqlitex.SQLiteDB],
	sessionManager *scs.SessionManager,
	conf *config.Config,
) http.Handler {

	mux := http.NewServeMux()

	baseChain := chain{recoverPanic(logger), requestLogger([]string{}, logger)}
	dynamic := chain{sessionManager.LoadAndSave}
	authMiddleware := requireAuthentication(sessionManager, notesDBConnCache, indexRepo, conf, logger)
	authReq := append(dynamic, authMiddleware)

	mux.Handle("GET /static/", http.FileServerFS(staticFiles))

	mux.Handle("GET /note/{id}/{slug}", authReq.then(handlers.NoteViewGet(tc, logger, handlers.GetSQLiteNotesRepo, sessionManager)))
	mux.Handle("GET /note/new", authReq.then(handlers.NoteNewGet(tc, logger, handlers.GetSQLiteNotesRepo)))
	mux.Handle("POST /note/preview", authReq.then(handlers.NotePreviewPost(tc, logger, handlers.GetSQLiteNotesRepo)))
	mux.Handle("POST /note", authReq.then(handlers.NotePost(tc, logger, handlers.GetSQLiteNotesRepo, sessionManager)))
	mux.Handle("PUT /note/{id}/{slug}", authReq.then(handlers.NotePut(tc, logger, handlers.GetSQLiteNotesRepo, sessionManager)))
	mux.Handle("DELETE /note/{id}/{slug}", authReq.then(handlers.NoteDelete(tc, logger, handlers.GetSQLiteNotesRepo)))
	mux.Handle("GET /edit/{id}/{slug}", authReq.then(handlers.NoteEditGet(tc, logger, handlers.GetSQLiteNotesRepo, sessionManager)))

	mux.Handle("GET /section", authReq.then(handlers.SectionGet(tc, logger, handlers.GetSQLiteNotesRepo, sessionManager)))
	mux.Handle("POST /section", authReq.then(handlers.SectionPost(tc, logger, handlers.GetSQLiteNotesRepo)))
	mux.Handle("PUT /section/{id}", authReq.then(handlers.SectionPut(tc, logger, handlers.GetSQLiteNotesRepo)))
	mux.Handle("DELETE /section/{id}", authReq.then(handlers.SectionDelete(tc, logger, handlers.GetSQLiteNotesRepo)))
	mux.Handle("GET /section/new", authReq.then(handlers.SectionNewGet(tc, logger)))
	mux.Handle("GET /section/{id}/{slug}", authReq.then(handlers.SectionViewGet(tc, logger, handlers.GetSQLiteNotesRepo, sessionManager)))
	mux.Handle("GET /section/{id}/edit", authReq.then(handlers.SectionEditGet(tc, logger, handlers.GetSQLiteNotesRepo)))

	mux.Handle("GET /search", authReq.then(handlers.NoteSearchGet(tc, logger, handlers.GetSQLiteNotesRepo, sessionManager)))

	mux.Handle("GET /index", authReq.then(handlers.TagGet(tc, logger, handlers.GetSQLiteNotesRepo, sessionManager)))
	mux.Handle("POST /tag", authReq.then(handlers.TagPost(tc, logger, handlers.GetSQLiteNotesRepo)))
	mux.Handle("PUT /tag/{id}", authReq.then(handlers.TagPut(tc, logger, handlers.GetSQLiteNotesRepo)))
	mux.Handle("DELETE /tag/{id}", authReq.then(handlers.TagDelete(tc, logger, handlers.GetSQLiteNotesRepo)))
	mux.Handle("GET /tag/{id}/{slug}", authReq.then(handlers.TagViewGet(tc, logger, handlers.GetSQLiteNotesRepo, sessionManager)))
	mux.Handle("GET /tag/{id}/edit", authReq.then(handlers.TagEditGet(tc, logger, handlers.GetSQLiteNotesRepo)))

	mux.Handle("GET /{$}", authReq.then(handlers.HomeGet(tc, logger, handlers.GetSQLiteNotesRepo, sessionManager)))

	mux.Handle("GET /login", dynamic.then(handlers.GetUserLogin(tc, logger, indexRepo, sessionManager)))
	mux.Handle("POST /login", dynamic.then(handlers.PostUserLogin(tc, logger, indexRepo, sessionManager)))
	mux.Handle("GET /user/new", dynamic.then(handlers.GetUserCreate(tc, logger, indexRepo, sessionManager)))
	mux.Handle("POST /user", dynamic.then(handlers.PostUserCreate(tc, logger, indexRepo, conf)))
	mux.Handle("POST /logout", authReq.then(handlers.PostUserLogout(tc, logger, sessionManager)))

	return baseChain.then(mux)
}
