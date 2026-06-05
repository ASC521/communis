package web

import (
	"log/slog"
	"net/http"

	userstore "github.com/ASC521/communis/user-store/sqlite"
	"github.com/ASC521/communis/web/handlers"
	"github.com/alexedwards/scs/v2"
)

func routes(
	logger *slog.Logger,
	tc *handlers.TemplateCache,
	dss *userstore.SQLiteDataStoreActor,
	sessionManager *scs.SessionManager,
	ignoredLoggingPaths []string,
	debugEnabled bool,
	setupRequired *bool,
) http.Handler {

	mux := http.NewServeMux()

	baseChain := handlers.Chain{
		handlers.RecoverPanic(logger),
		handlers.RequestLogger(ignoredLoggingPaths, logger),
		handlers.CommonHeaders, handlers.CrossOriginProtection,
		sessionManager.LoadAndSave,
		handlers.Authenticate(sessionManager, dss.UserStore),
		handlers.InitialSetup(setupRequired),
	}
	authReq := handlers.Chain{handlers.RequireAuth, handlers.RedirectAdmin}
	adminReq := handlers.Chain{handlers.RequireAdmin}

	mux.Handle("GET /static/", http.FileServerFS(staticFiles))

	mux.Handle("GET /note/{id}/{slug}", authReq.Then(handlers.NoteViewGet(tc, logger, dss, sessionManager)))
	mux.Handle("GET /note/new", authReq.Then(handlers.NoteNewGet(tc, logger, dss, sessionManager)))
	mux.Handle("POST /note/preview/{id}", authReq.Then(handlers.NotePreviewPost(tc, logger, dss)))
	mux.Handle("POST /note", authReq.Then(handlers.NotePost(tc, logger, dss)))
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

	mux.Handle("GET /login", handlers.GetUserLogin(tc, logger, dss.UserStore, sessionManager))
	mux.Handle("POST /login", handlers.PostUserLogin(tc, logger, dss.UserStore, sessionManager))
	mux.Handle("DELETE /session", handlers.RequireAuth((handlers.PostUserLogout(tc, logger, sessionManager))))

	mux.Handle("GET /admin", adminReq.Then(handlers.GetAdmin(tc, logger, dss.UserStore, sessionManager)))
	mux.Handle("GET /user/new", adminReq.Then(handlers.GetUserCreate(tc, logger, dss.UserStore, sessionManager)))
	mux.Handle("POST /user", adminReq.Then(handlers.PostUser(tc, logger, dss.UserStore, dss)))
	mux.Handle("GET /user/{id}", adminReq.Then(handlers.GetUser(tc, logger, dss.UserStore, sessionManager)))
	mux.Handle("GET /user/{id}/edit", adminReq.Then(handlers.GetUserEdit(tc, logger, dss.UserStore, sessionManager)))
	mux.Handle("DELETE /user/{id}", adminReq.Then(handlers.DeleteUser(tc, logger, dss.UserStore, dss)))
	mux.Handle("PUT /user/{id}", adminReq.Then(handlers.PutUser(tc, logger, dss.UserStore)))
	mux.Handle("PUT /user/{id}/password", adminReq.Then(handlers.PutUserPassword(tc, logger, dss.UserStore)))

	mux.Handle("GET /setup", handlers.GetSetup(tc, logger, setupRequired))
	mux.Handle("POST /setup", handlers.PostSetup(tc, logger, setupRequired, dss.UserStore, sessionManager))

	mux.Handle("PUT /user/{id}/theme", handlers.RequireAuth(handlers.PutUserTheme(tc, logger, dss.UserStore)))

	if debugEnabled {
		mux.Handle("GET /debug/conn-cache-state", adminReq.Then(handlers.ConnCacheStateGet(tc, logger, dss)))
		mux.Handle("GET /debug/build-info", authReq.Then(handlers.GetDebugBuildInfo()))
	}

	return baseChain.Then(mux)
}
