package web

import (
	"log/slog"
	"net/http"

	"github.com/ASC521/communis/services"
	"github.com/ASC521/communis/web/handlers"
	"github.com/alexedwards/scs/v2"
)

func routes(
	logger *slog.Logger,
	tc *handlers.TemplateCache,
	dss services.DataStoreService,
	sessionManager *scs.SessionManager,
	ignoredLoggingPaths []string,
) http.Handler {

	indexRepo := dss.GetUserStore()

	mux := http.NewServeMux()

	baseChain := handlers.Chain{handlers.RecoverPanic(logger), handlers.RequestLogger(ignoredLoggingPaths, logger), handlers.CommonHeaders, handlers.CrossOriginProtection}
	dynamic := handlers.Chain{sessionManager.LoadAndSave, handlers.Authenticate(sessionManager, indexRepo)}
	authReq := append(dynamic, handlers.RequireAuth)
	adminReq := append(dynamic, handlers.RequireAdmin)

	mux.Handle("GET /static/", http.FileServerFS(staticFiles))

	mux.Handle("GET /note/{id}/{slug}", authReq.Then(handlers.NoteViewGet(tc, logger, dss, sessionManager)))
	mux.Handle("GET /note/new", authReq.Then(handlers.NoteNewGet(tc, logger, dss, sessionManager)))
	mux.Handle("POST /note/preview", authReq.Then(handlers.NotePreviewPost(tc, logger, dss)))
	mux.Handle("POST /note", authReq.Then(handlers.NotePost(tc, logger, dss, sessionManager)))
	mux.Handle("PUT /note/{id}/{slug}", authReq.Then(handlers.NotePut(tc, logger, dss, sessionManager)))
	mux.Handle("DELETE /note/{id}/{slug}", authReq.Then(handlers.NoteDelete(tc, logger, dss)))
	mux.Handle("GET /edit/{id}/{slug}", authReq.Then(handlers.NoteEditGet(tc, logger, dss, sessionManager)))

	mux.Handle("POST /ref-notes/select/{id}", authReq.Then(handlers.ReferenceNoteSelectPost(tc, logger)))
	mux.Handle("DELETE /ref-notes/select/{id}", authReq.Then(handlers.ReferenceNoteSelectDelete()))

	mux.Handle("GET /section", authReq.Then(handlers.SectionGet(tc, logger, dss, sessionManager)))
	mux.Handle("POST /section", authReq.Then(handlers.SectionPost(tc, logger, dss)))
	mux.Handle("PUT /section/{id}", authReq.Then(handlers.SectionPut(tc, logger, dss)))
	mux.Handle("DELETE /section/{id}", authReq.Then(handlers.SectionDelete(tc, logger, dss)))
	mux.Handle("GET /section/new", authReq.Then(handlers.SectionNewGet(tc, logger)))
	mux.Handle("GET /section/{id}/{slug}", authReq.Then(handlers.SectionViewGet(tc, logger, dss, sessionManager)))
	mux.Handle("GET /section/{id}/edit", authReq.Then(handlers.SectionEditGet(tc, logger, dss)))

	mux.Handle("GET /search", authReq.Then(handlers.NoteSearchGet(tc, logger, dss, sessionManager)))

	mux.Handle("GET /index", authReq.Then(handlers.TagGet(tc, logger, dss, sessionManager)))
	mux.Handle("POST /tag", authReq.Then(handlers.TagPost(tc, logger, dss)))
	mux.Handle("PUT /tag/{id}", authReq.Then(handlers.TagPut(tc, logger, dss)))
	mux.Handle("DELETE /tag/{id}", authReq.Then(handlers.TagDelete(tc, logger, dss)))
	mux.Handle("GET /tag/{id}/{slug}", authReq.Then(handlers.TagViewGet(tc, logger, dss, sessionManager)))
	mux.Handle("GET /tag/{id}/edit", authReq.Then(handlers.TagEditGet(tc, logger, dss)))

	mux.Handle("GET /{$}", authReq.Then(handlers.HomeGet(tc, logger, dss, sessionManager)))

	mux.Handle("GET /login", dynamic.Then(handlers.GetUserLogin(tc, logger, indexRepo, sessionManager)))
	mux.Handle("POST /login", dynamic.Then(handlers.PostUserLogin(tc, logger, indexRepo, sessionManager)))
	mux.Handle("DELETE /session", authReq.Then(handlers.PostUserLogout(tc, logger, sessionManager)))

	mux.Handle("GET /admin", adminReq.Then(handlers.GetAdmin(tc, logger, indexRepo, sessionManager)))
	mux.Handle("GET /user/new", adminReq.Then(handlers.GetUserCreate(tc, logger, indexRepo, sessionManager)))
	mux.Handle("POST /user", adminReq.Then(handlers.PostUser(tc, logger, indexRepo, dss)))
	mux.Handle("GET /user/{id}", adminReq.Then(handlers.GetUser(tc, logger, indexRepo, sessionManager)))
	mux.Handle("GET /user/{id}/edit", adminReq.Then(handlers.GetUserEdit(tc, logger, indexRepo, sessionManager)))
	mux.Handle("DELETE /user/{id}", adminReq.Then(handlers.DeleteUser(tc, logger, indexRepo, dss)))
	mux.Handle("PUT /user/{id}", adminReq.Then(handlers.PutUser(tc, logger, indexRepo)))
	mux.Handle("PUT /user/{id}/password", adminReq.Then(handlers.PutUserPassword(tc, logger, indexRepo)))

	mux.Handle("PUT /user/{id}/theme", authReq.Then(handlers.PutUserTheme(tc, logger, indexRepo)))

	return baseChain.Then(mux)
}
