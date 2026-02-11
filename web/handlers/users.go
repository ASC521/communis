package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/ASC521/communis/config"
	"github.com/ASC521/communis/dbx/migrations"
	"github.com/ASC521/communis/dbx/sqlitex"
	"github.com/ASC521/communis/models"
	"github.com/alexedwards/scs/v2"
)

type userForm struct {
	Method            string
	Id                int64
	Name              string
	PlainTextPassword string
	IsAdmin           bool
	FieldErrors       map[string]string
}

func parseUserFormFromRequest(r *http.Request) (userForm, error) {
	err := r.ParseForm()
	if err != nil {
		return userForm{}, err
	}

	name := r.PostForm.Get("username")
	password := r.PostForm.Get("password")
	form := userForm{
		Method:            r.Method,
		Id:                0,
		Name:              name,
		PlainTextPassword: password,
		FieldErrors:       map[string]string{},
	}

	return form, nil
}

// func PostUser() http.Handler     {}
// func PutUser() http.Handler      {}
// func GetUser() http.Handler      {}
// func DeleteUser() http.Handler   {}

func GetUserCreate(tc *TemplateCache, logger *slog.Logger, indexRepo models.IndexRepository) http.Handler {
	type td struct {
		Form userForm
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tc.RenderPage(logger, w, r, http.StatusOK, "user-create.tmpl", td{Form: userForm{}})
	})
}

// PostUserCreate creates a new user in the index database and bootstraps a new user database.
func PostUserCreate(tc *TemplateCache, logger *slog.Logger, indexRepo models.IndexRepository, conf *config.Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userForm, err := parseUserFormFromRequest(r)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		user := models.User{
			Name:       userForm.Name,
			PTPassword: userForm.PlainTextPassword,
			IsAdmin:    userForm.IsAdmin,
			DBPath:     fmt.Sprintf("notes/%s.db", strings.ToLower(userForm.Name)),
			DBVersion:  0,
		}

		_, err = indexRepo.CreateUser(r.Context(), user)
		if err != nil {
			serverError(logger, w, r, err)
			return
		}

		notesDBFP := filepath.Join(conf.SQLite.DBDirectory, user.DBPath)
		notesDB, err := sqlitex.NewSQLiteDB(notesDBFP,
			sqlitex.WithBusyTimeout(conf.SQLite.BusyTimeout),
			sqlitex.WithCacheSize(conf.SQLite.CacheSize),
			sqlitex.WithForeignKeys(conf.SQLite.ForeignKeys),
			sqlitex.WithJournalMode(conf.SQLite.JournalMode),
			sqlitex.WithSynchronous(conf.SQLite.Synchronous),
			sqlitex.WithTempStore(conf.SQLite.TempStore),
		)
		if err != nil {
			serverError(logger, w, r, err)
			return
		}

		notesDBMigrationDriver := sqlitex.NewMigrationDriver(notesDB, r.Context())
		migs, err := migrations.Load(conf.SQLite.NotesDBMigrations)
		if err != nil {
			serverError(logger, w, r, err)
			return
		}
		_, err = migrations.Bootstrap(r.Context(), migs, notesDBMigrationDriver)
		if err != nil {
			serverError(logger, w, r, err)
			return
		}
		http.Redirect(w, r, "/login", http.StatusSeeOther)

	})
}

func GetUserLogin(
	tc *TemplateCache,
	logger *slog.Logger,
	indexRepo models.IndexRepository,
	sessionManager *scs.SessionManager,
) http.Handler {

	type td struct {
		Form userForm
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		tc.RenderPage(logger, w, r, http.StatusOK, "login.tmpl", td{Form: userForm{}})

	})
}

func PostUserLogin(
	tc *TemplateCache,
	logger *slog.Logger,
	indexRepo models.IndexRepository,
	sessionManager *scs.SessionManager,
) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		userForm, err := parseUserFormFromRequest(r)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		userId, err := indexRepo.AuthenticateUser(r.Context(), userForm.Name, userForm.PlainTextPassword)
		if err != nil {
			serverError(logger, w, r, err)
			return
		}

		err = sessionManager.RenewToken(r.Context())
		if err != nil {
			serverError(logger, w, r, err)
			return
		}

		sessionManager.Put(r.Context(), "authenticatedUserId", userId)
		http.Redirect(w, r, "/", http.StatusSeeOther)

	})
}
