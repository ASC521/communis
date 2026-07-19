package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	userstore "github.com/ASC521/communis/user-store"
	"github.com/ASC521/communis/web/assets"
	"github.com/ASC521/communis/web/handlers/validator"
	"github.com/alexedwards/scs/v2"
)

func validatePassword(password, confirmedPassword string, errors map[string]string) {
	switch {
	case password == "":
		errors["password"] = "password cannot be empty"
	case !validator.MinChars(password, 8):
		errors["password"] = "password must be at least 8 characters"
	case password != confirmedPassword:
		errors["password"] = "passwords must match"
	}
}

type userEditForm struct {
	ID          int64
	UserName    string
	FieldErrors map[string]string
}

func parseUserEditFormFromRequest(r *http.Request) (userEditForm, error) {
	err := r.ParseForm()
	if err != nil {
		return userEditForm{}, err
	}

	var id int64
	idStr := r.PostForm.Get("id")
	if idStr == "" {
		return userEditForm{}, errors.New("missing user id from form")
	}
	id, err = strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return userEditForm{}, err
	}

	name := r.PostForm.Get("username")
	form := userEditForm{
		ID:          id,
		UserName:    name,
		FieldErrors: map[string]string{},
	}

	return form, nil
}

func DeleteUser(
	htmlRenderer *assets.HTMLRenderer,
	logger *slog.Logger,
	indexRepo *userstore.SQLite,
	dss *userstore.SQLiteConnManager,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := parseIDFromPath(r)
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}

		user, err := indexRepo.GetUser(r.Context(), userID)
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}

		if user.IsAdmin {
			err = dss.Remove(user.ID)
			if err != nil {
				logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
				htmlRenderer.RenderError(w, err)
				return
			}
		} else {
			err = dss.DeleteDB(r.Context(), userID)
			if err != nil {
				logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
				htmlRenderer.RenderError(w, err)
				return
			}
		}

		err = indexRepo.DeleteUser(r.Context(), userID)
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}

		w.Header().Set("HX-Redirect", "/admin")
		w.WriteHeader(http.StatusOK)
	}
}

// GetUserCreate writes an HTML partial template containing a form to create a new user form.
func GetUserCreate(
	htmlRenderer *assets.HTMLRenderer,
	logger *slog.Logger,
	indexRepo *userstore.SQLite,
	sessionManager *scs.SessionManager,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := htmlRenderer.Render(w, http.StatusOK, nil, "partial:user:new")
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
		}
	}
}

// PostUser creates a new user in the index database and bootstraps a new user database.
func PostUser(
	htmlRenderer *assets.HTMLRenderer,
	logger *slog.Logger,
	indexRepo *userstore.SQLite,
	dss *userstore.SQLiteConnManager,
) http.HandlerFunc {
	type newUserForm struct {
		Name        string
		Password    string
		IsAdmin     bool
		FieldErrors map[string]string
	}

	return func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}

		nuf := newUserForm{FieldErrors: map[string]string{}}
		nuf.Name = r.PostForm.Get("username")
		if nuf.Name == "" {
			nuf.FieldErrors["name"] = "username cannot be blank"
		}
		nuf.Password = r.PostForm.Get("password")
		if nuf.Password == "" {
			nuf.FieldErrors["password"] = "password cannot be blank"
		} else if !validator.MinChars(nuf.Password, 8) {
			nuf.FieldErrors["password"] = "password must be at least 8 characters"
		} // TODO: Add function with basic complexity text for passwords

		nuf.IsAdmin = r.PostForm.Get("is-admin") == "on"

		if len(nuf.FieldErrors) > 0 {
			htmlRenderer.Render(w, http.StatusUnprocessableEntity, nuf, "partial:user:new")
			return
		}

		exists, err := indexRepo.NameExists(r.Context(), nuf.Name)
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}
		if exists {
			nuf.FieldErrors["name"] = "username already exists"
			htmlRenderer.Render(w, http.StatusUnprocessableEntity, nuf, "partial:user:new")
			return
		}

		if nuf.IsAdmin {
			_, err = indexRepo.CreateAdminUser(r.Context(), nuf.Name, nuf.Password)
			if err != nil {
				logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
				htmlRenderer.RenderError(w, err)
				return
			}
		} else {
			userID, err := indexRepo.CreateUserAndDB(r.Context(), nuf.Name, nuf.Password)
			if err != nil {
				logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
				htmlRenderer.RenderError(w, err)
				return
			}

			err = dss.CreateDB(r.Context(), userID)
			if err != nil {
				logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
				htmlRenderer.RenderError(w, err)
				return
			}
		}
		w.Header().Add("HX-Redirect", "/admin")
		w.WriteHeader(http.StatusSeeOther)
	}
}

func GetAdmin(
	tc *assets.HTMLRenderer,
	logger *slog.Logger,
	indexRepo *userstore.SQLite,
	sessionManager *scs.SessionManager,
) http.HandlerFunc {
	type td struct {
		assets.BaseData
		Users []userstore.User
	}

	return func(w http.ResponseWriter, r *http.Request) {
		users, err := indexRepo.ListUsers(r.Context())
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			tc.RenderError(w, err)
			return
		}

		data := td{
			BaseData: extractBaseDataFromRequest(r),
			Users:    users,
		}

		if err = tc.Render(w, http.StatusOK, data, "base", "pages/admin.tmpl"); err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			tc.RenderError(w, err)
		}
	}
}

func PutUser(
	tc *assets.HTMLRenderer,
	logger *slog.Logger,
	indexRepo *userstore.SQLite,
) http.HandlerFunc {
	type td struct {
		assets.BaseData
		Form userEditForm
		User userstore.User
	}

	return func(w http.ResponseWriter, r *http.Request) {
		pathUserID, err := parseIDFromPath(r)
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			tc.RenderError(w, err)
			return
		}

		userEditForm, err := parseUserEditFormFromRequest(r)
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			tc.RenderError(w, err)
			return
		}

		if userEditForm.ID != pathUserID {
			err = errors.New("form id does not match path id")
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			tc.RenderError(w, err)
			return
		}

		if userEditForm.UserName == "" {
			userEditForm.FieldErrors["username"] = "username cannot be empty"
		} else {
			exists, err := indexRepo.NameExists(r.Context(), userEditForm.UserName)
			if err != nil {
				logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
				tc.RenderError(w, err)
				return
			}
			if exists {
				userEditForm.FieldErrors["username"] = "name already exists"
			}
		}

		if len(userEditForm.FieldErrors) > 0 {
			user, err := indexRepo.GetUser(r.Context(), userEditForm.ID)
			if err != nil {
				logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
				tc.RenderError(w, err)
				return
			}
			data := td{
				BaseData: extractBaseDataFromRequest(r),
				Form:     userEditForm,
				User:     user,
			}

			tc.Render(w, http.StatusUnprocessableEntity, data, "partial:user:edit")
			return
		}

		user, err := indexRepo.UpdateUser(r.Context(), userEditForm.ID, userEditForm.UserName)
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			tc.RenderError(w, err)
			return
		}

		if err = tc.Render(w, http.StatusOK, user, "partial:user:updated"); err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			tc.RenderError(w, err)
		}
	}
}

type changePasswordForm struct {
	ID                int64
	Name              string
	Password          string
	ConfirmedPassword string
	FieldErrors       map[string]string
}

func PutUserPassword(
	htmlRenderer *assets.HTMLRenderer,
	logger *slog.Logger,
	indexRepo *userstore.SQLite,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}

		passChgForm := changePasswordForm{
			Password:          r.PostForm.Get("password"),
			ConfirmedPassword: r.PostForm.Get("confirmed-password"),
			Name:              r.PostForm.Get("name"),
			FieldErrors:       map[string]string{},
		}

		idStr := r.PostForm.Get("id")
		if idStr == "" {
			err = errors.New("id missing from form")
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		} else {
			passChgForm.ID, err = strconv.ParseInt(idStr, 10, 64)
			if err != nil {
				logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
				htmlRenderer.RenderError(w, err)
				return
			}
		}

		if passChgForm.Password == "" {
			passChgForm.FieldErrors["error"] = "password cannot be empty"
		} else if passChgForm.ConfirmedPassword == "" {
			passChgForm.FieldErrors["error"] = "confirmed password cannot be empty"
		} else if !validator.MinChars(passChgForm.Password, 8) {
			passChgForm.FieldErrors["error"] = "password must be at least 8 characters"
		} else if passChgForm.Password != passChgForm.ConfirmedPassword {
			passChgForm.FieldErrors["error"] = "passwords do not match"
		}

		if len(passChgForm.FieldErrors) > 0 {
			htmlRenderer.Render(w, http.StatusUnprocessableEntity, passChgForm, "partial:user:change-password-form")
			return
		}

		err = indexRepo.UpdateUserPassword(r.Context(), passChgForm.ID, passChgForm.Password)
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}

		w.Header().Add("HX-Redirect", "/admin")
		w.WriteHeader(http.StatusSeeOther)
	}
}

func GetUserEdit(
	htmlRenderer *assets.HTMLRenderer,
	logger *slog.Logger,
	indexRepo *userstore.SQLite,
	sessionManager *scs.SessionManager,
) http.HandlerFunc {
	type td struct {
		assets.BaseData
		User               userstore.User
		EditUserForm       userEditForm
		ChangePasswordForm changePasswordForm
	}

	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := parseIDFromPath(r)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		user, err := indexRepo.GetUser(r.Context(), userID)
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}
		data := td{
			BaseData: extractBaseDataFromRequest(r),
			EditUserForm: userEditForm{
				ID:          userID,
				UserName:    user.Name,
				FieldErrors: map[string]string{},
			},
			User: user,
			ChangePasswordForm: changePasswordForm{
				ID:          user.ID,
				Name:        user.Name,
				FieldErrors: map[string]string{},
			},
		}

		if err = htmlRenderer.Render(w, http.StatusOK, data, "partial:user:edit"); err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
		}
	}
}

func GetUser(
	htmlRenderer *assets.HTMLRenderer,
	logger *slog.Logger,
	indexRepo *userstore.SQLite,
	sessionManager *scs.SessionManager,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := parseIDFromPath(r)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		user, err := indexRepo.GetUser(r.Context(), userID)
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}

		if err = htmlRenderer.Render(w, http.StatusOK, user, "partial:user:replace-edit-form"); err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
		}
	}
}

type setupUserForm struct {
	Username        string
	Password        string
	ConfirmPassword string
	FieldErrors     map[string]string
}

type setupData struct {
	assets.BaseData
	SetupUserForm setupUserForm
}

func GetSetup(
	htmlRenderer *assets.HTMLRenderer,
	logger *slog.Logger,
	setupRequired *bool,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !*setupRequired {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		data := setupData{
			BaseData:      extractBaseDataFromRequest(r),
			SetupUserForm: setupUserForm{FieldErrors: map[string]string{}},
		}

		if err := htmlRenderer.Render(w, http.StatusOK, data, "base", "pages/setup.tmpl"); err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
		}
	}
}

func PostSetup(
	htmlRenderer *assets.HTMLRenderer,
	logger *slog.Logger,
	setupRequired *bool,
	indexRepo *userstore.SQLite,
	sessionManager *scs.SessionManager,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !*setupRequired {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		err := r.ParseForm()
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}

		suf := setupUserForm{
			Username:        r.PostForm.Get("username"),
			Password:        r.PostForm.Get("password"),
			ConfirmPassword: r.PostForm.Get("confirm-password"),
			FieldErrors:     map[string]string{},
		}

		if suf.Username == "" {
			suf.FieldErrors["username"] = "username cannot be empty"
		}

		validatePassword(suf.Password, suf.ConfirmPassword, suf.FieldErrors)

		if len(suf.FieldErrors) > 0 {
			data := setupData{
				BaseData:      extractBaseDataFromRequest(r),
				SetupUserForm: suf,
			}
			htmlRenderer.Render(w, http.StatusOK, data, "base", "pages/setup.tmpl")
			return
		}

		userID, err := indexRepo.CreateAdminUser(r.Context(), suf.Username, suf.Password)
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}

		err = indexRepo.UpdateUserLastLoginToNow(r.Context(), userID)
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}

		err = sessionManager.RenewToken(r.Context())
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}

		*setupRequired = false
		sessionManager.Put(r.Context(), "authenticatedUserId", userID)
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
	}
}
