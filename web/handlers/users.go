package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/ASC521/communis/models"
	"github.com/alexedwards/scs/v2"
)

type loginForm struct {
	Name        string
	Password    string
	FieldErrors map[string]string
}

func GetUserLogin(
	tc *TemplateCache,
	logger *slog.Logger,
	indexRepo models.IndexRepository,
	sessionManager *scs.SessionManager,
) http.Handler {

	type td struct {
		BaseData
		Form loginForm
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		data := td{
			Form:     loginForm{},
			BaseData: newBase(r),
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

	type td struct {
		BaseData
		Form loginForm
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		err := r.ParseForm()
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		lf := loginForm{FieldErrors: map[string]string{}}
		lf.Name = r.PostForm.Get("name")
		if lf.Name == "" {
			lf.FieldErrors["error"] = "username cannot be empty"
		}

		lf.Password = r.PostForm.Get("password")
		if lf.Password == "" {
			lf.FieldErrors["error"] = "password cannot be empty"
		}

		if len(lf.FieldErrors) > 0 {
			data := td{
				BaseData: newBase(r),
				Form:     lf,
			}
			tc.RenderPage(logger, w, r, http.StatusUnprocessableEntity, "login.tmpl", data)
			return
		}

		user, err := indexRepo.AuthenticateUser(r.Context(), lf.Name, lf.Password)
		if err != nil {
			if errors.Is(err, models.ErrInvalidCredentials) {
				lf.FieldErrors["error"] = "username or password is incorrect"
				data := td{
					BaseData: newBase(r),
					Form:     lf,
				}
				tc.RenderPage(logger, w, r, http.StatusForbidden, "login.tmpl", data)
				return
			}

			tc.RenderError(logger, w, r, err)
			return
		}

		err = indexRepo.UpdateUserLastLoginToNow(r.Context(), user.ID)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		err = sessionManager.RenewToken(r.Context())
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		sessionManager.Put(r.Context(), "authenticatedUserId", user.ID)
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

func PutUserTheme(
	tc *TemplateCache,
	logger *slog.Logger,
	indexRepo models.IndexRepository,
) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		userId, err := parseIDFromPath(r)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		ctxUserId := getUserIDFromRequest(r)
		if ctxUserId == 0 {
			tc.RenderError(logger, w, r, errors.New("user id missing from request context"))
			return
		}

		if userId != ctxUserId {
			tc.RenderError(logger, w, r, errors.New("authenticated user id does not match path user id"))
			return
		}

		userTheme := getUserThemeFromRequest(r)
		if userTheme == "" {
			tc.RenderError(logger, w, r, errors.New("user theme not set in request"))
			return
		}

		if userTheme == "dark" {
			userTheme = "light"
		} else {
			userTheme = "dark"
		}
		err = indexRepo.UpdateUserTheme(r.Context(), ctxUserId, userTheme)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		currentUrl := r.Header.Get("HX-Current-URL")
		if currentUrl == "" {
			tc.RenderError(logger, w, r, errors.New("HX-Current-URL request header not set"))
			return
		}

		w.Header().Set("HX-Redirect", currentUrl)
		w.WriteHeader(http.StatusOK)
	}
}
