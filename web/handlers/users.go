package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/ASC521/communis/models"
	"github.com/ASC521/communis/services"
	"github.com/ASC521/communis/web/handlers/validator"
	"github.com/alexedwards/scs/v2"
	"github.com/mitchellh/go-homedir"
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

func validateUserForm(form *userForm) {

	if form.Name == "" {
		form.FieldErrors["name"] = "Name cannot be empty"
	}

	if form.PlainTextPassword == "" {
		form.FieldErrors["password"] = "Password cannot be empty"
	} else if !validator.MinChars(form.PlainTextPassword, 8) {
		form.FieldErrors["password"] = "Password must be at least 8 characters"
	}
}

func DeleteUser(
	tc *TemplateCache,
	logger *slog.Logger,
	indexRepo models.IndexRepository,
	sqliteConnSvc *services.SQLiteConnService,
	dbDirectory string,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userId, err := parseIdFromPath(r)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		err = sqliteConnSvc.RemoveConn(r.Context(), userId)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}
		userDB, err := indexRepo.GetUserDB(r.Context(), userId)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		err = indexRepo.DeleteUser(r.Context(), userId)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		// TODO: this shouldn't be happening here - config object should have aboslute path
		dbd, err := homedir.Expand(dbDirectory)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}
		err = os.Remove(filepath.Join(dbd, userDB.Path))
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		w.Header().Set("HX-Redirect", "/admin")
		w.WriteHeader(http.StatusOK)
	}
}

func GetUserCreate(
	tc *TemplateCache,
	logger *slog.Logger,
	indexRepo models.IndexRepository,
	sessionManager *scs.SessionManager,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tc.RenderPartial(logger, w, r, http.StatusOK, "user-new", nil)
	}
}

// PostUser creates a new user in the index database and bootstraps a new user database.
func PostUser(tc *TemplateCache, logger *slog.Logger, indexRepo models.IndexRepository, connSvc *services.SQLiteConnService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userForm, err := parseUserFormFromRequest(r)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		validateUserForm(&userForm)
		if len(userForm.FieldErrors) > 0 {
			tc.RenderPartial(logger, w, r, http.StatusUnprocessableEntity, "user-new", userForm)
			return
		}

		dbPath := fmt.Sprintf("notes/%s.db", strings.ToLower(userForm.Name))
		userId, err := indexRepo.CreateUserAndDB(r.Context(), userForm.Name, userForm.PlainTextPassword, userForm.IsAdmin, dbPath)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		err = connSvc.CreateDB(r.Context(), userId)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		user, err := indexRepo.GetUser(r.Context(), userId)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		tc.RenderPartial(logger, w, r, http.StatusCreated, "user-row", user)

	}
}

func GetUserLogin(
	tc *TemplateCache,
	logger *slog.Logger,
	indexRepo models.IndexRepository,
	sessionManager *scs.SessionManager,
) http.Handler {

	type td struct {
		BaseData
		Form userForm
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		data := td{
			Form:     userForm{},
			BaseData: newBase(r, sessionManager),
		}
		tc.RenderPage(logger, w, r, http.StatusOK, "login.tmpl", data)

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

		user, err := indexRepo.AuthenticateUser(r.Context(), userForm.Name, userForm.PlainTextPassword)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		err = indexRepo.UpdateUserLastLoginToNow(r.Context(), user.Id)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		err = sessionManager.RenewToken(r.Context())
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		sessionManager.Put(r.Context(), "authenticatedUserId", user.Id)
		if user.IsAdmin {
			http.Redirect(w, r, "/admin", http.StatusSeeOther)
		} else {
			http.Redirect(w, r, "/", http.StatusSeeOther)
		}

	})
}

func PostUserLogout(
	tc *TemplateCache,
	logger *slog.Logger,
	sessionManager *scs.SessionManager,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		err := sessionManager.RenewToken(r.Context())
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		sessionManager.Remove(r.Context(), "authenticatedUserId")
		w.Header().Set("HX-Redirect", "/login")
		w.WriteHeader(http.StatusOK)
	})
}

func GetUser(
	tc *TemplateCache,
	logger *slog.Logger,
	indexRepo models.IndexRepository,
	sessionManager *scs.SessionManager,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userId, err := parseIdFromPath(r)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		user, err := indexRepo.GetUser(r.Context(), userId)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		tc.RenderPartial(logger, w, r, http.StatusOK, "user-row", user)

	}
}

func GetUserEdit(
	tc *TemplateCache,
	logger *slog.Logger,
	indexRepo models.IndexRepository,
	sessionManager *scs.SessionManager,
) http.HandlerFunc {

	type td struct {
		BaseData
		User models.User
	}

	return func(w http.ResponseWriter, r *http.Request) {
		userId, err := parseIdFromPath(r)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		user, err := indexRepo.GetUser(r.Context(), userId)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}
		data := td{
			BaseData: newBase(r, sessionManager),
			User:     user,
		}

		tc.RenderPartial(logger, w, r, http.StatusOK, "user-edit", data)
	}
}
