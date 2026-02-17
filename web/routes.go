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
	authDBMiddleware := requireAuthAndDB(sessionManager, notesDBConnCache, indexRepo, conf, logger)
	authDBReq := append(dynamic, authDBMiddleware)
	authReq := append(dynamic, requireAuth(sessionManager))
	adminReq := append(dynamic, requireAdmin(sessionManager, indexRepo))

	mux.Handle("GET /static/", http.FileServerFS(staticFiles))

	mux.Handle("GET /note/{id}/{slug}", authDBReq.then(handlers.NoteViewGet(tc, logger, handlers.GetSQLiteNotesRepo, sessionManager)))
	mux.Handle("GET /note/new", authDBReq.then(handlers.NoteNewGet(tc, logger, handlers.GetSQLiteNotesRepo)))
	mux.Handle("POST /note/preview", authDBReq.then(handlers.NotePreviewPost(tc, logger, handlers.GetSQLiteNotesRepo)))
	mux.Handle("POST /note", authDBReq.then(handlers.NotePost(tc, logger, handlers.GetSQLiteNotesRepo, sessionManager)))
	mux.Handle("PUT /note/{id}/{slug}", authDBReq.then(handlers.NotePut(tc, logger, handlers.GetSQLiteNotesRepo, sessionManager)))
	mux.Handle("DELETE /note/{id}/{slug}", authDBReq.then(handlers.NoteDelete(tc, logger, handlers.GetSQLiteNotesRepo)))
	mux.Handle("GET /edit/{id}/{slug}", authDBReq.then(handlers.NoteEditGet(tc, logger, handlers.GetSQLiteNotesRepo, sessionManager)))

	mux.Handle("GET /section", authDBReq.then(handlers.SectionGet(tc, logger, handlers.GetSQLiteNotesRepo, sessionManager)))
	mux.Handle("POST /section", authDBReq.then(handlers.SectionPost(tc, logger, handlers.GetSQLiteNotesRepo)))
	mux.Handle("PUT /section/{id}", authDBReq.then(handlers.SectionPut(tc, logger, handlers.GetSQLiteNotesRepo)))
	mux.Handle("DELETE /section/{id}", authDBReq.then(handlers.SectionDelete(tc, logger, handlers.GetSQLiteNotesRepo)))
	mux.Handle("GET /section/new", authDBReq.then(handlers.SectionNewGet(tc, logger)))
	mux.Handle("GET /section/{id}/{slug}", authDBReq.then(handlers.SectionViewGet(tc, logger, handlers.GetSQLiteNotesRepo, sessionManager)))
	mux.Handle("GET /section/{id}/edit", authDBReq.then(handlers.SectionEditGet(tc, logger, handlers.GetSQLiteNotesRepo)))

	mux.Handle("GET /search", authDBReq.then(handlers.NoteSearchGet(tc, logger, handlers.GetSQLiteNotesRepo, sessionManager)))

	mux.Handle("GET /index", authDBReq.then(handlers.TagGet(tc, logger, handlers.GetSQLiteNotesRepo, sessionManager)))
	mux.Handle("POST /tag", authDBReq.then(handlers.TagPost(tc, logger, handlers.GetSQLiteNotesRepo)))
	mux.Handle("PUT /tag/{id}", authDBReq.then(handlers.TagPut(tc, logger, handlers.GetSQLiteNotesRepo)))
	mux.Handle("DELETE /tag/{id}", authDBReq.then(handlers.TagDelete(tc, logger, handlers.GetSQLiteNotesRepo)))
	mux.Handle("GET /tag/{id}/{slug}", authDBReq.then(handlers.TagViewGet(tc, logger, handlers.GetSQLiteNotesRepo, sessionManager)))
	mux.Handle("GET /tag/{id}/edit", authDBReq.then(handlers.TagEditGet(tc, logger, handlers.GetSQLiteNotesRepo)))

	mux.Handle("GET /{$}", authDBReq.then(handlers.HomeGet(tc, logger, handlers.GetSQLiteNotesRepo, sessionManager)))

	mux.Handle("GET /login", dynamic.then(handlers.GetUserLogin(tc, logger, indexRepo, sessionManager)))
	mux.Handle("POST /login", dynamic.then(handlers.PostUserLogin(tc, logger, indexRepo, sessionManager)))
	mux.Handle("POST /logout", authReq.then(handlers.PostUserLogout(tc, logger, sessionManager)))

	mux.Handle("GET /admin", adminReq.thenFunc(handlers.GetAdmin(tc, logger, indexRepo, sessionManager)))
	mux.Handle("GET /user/new", adminReq.thenFunc(handlers.GetUserCreate(tc, logger, indexRepo, sessionManager)))
	mux.Handle("POST /user", adminReq.thenFunc(handlers.PostUser(tc, logger, indexRepo, conf)))
	mux.Handle("GET /user/{id}", adminReq.thenFunc(handlers.GetUser(tc, logger, indexRepo, sessionManager)))
	mux.Handle("GET /user/{id}/edit", adminReq.thenFunc(handlers.GetUserEdit(tc, logger, indexRepo, sessionManager)))
	mux.Handle("DELETE /user/{id}", adminReq.thenFunc(handlers.DeleteUser(tc, logger, indexRepo, notesDBConnCache, conf.SQLite.DBDirectory)))

	return baseChain.then(mux)
}
