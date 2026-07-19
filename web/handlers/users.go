package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	userstore "github.com/ASC521/communis/user-store"
	"github.com/ASC521/communis/web/assets"
	"github.com/alexedwards/scs/v2"
)

type loginForm struct {
	Name        string
	Password    string
	FieldErrors map[string]string
}

func GetUserLogin(
	htmlRenderer *assets.HTMLRenderer,
	logger *slog.Logger,
	indexRepo *userstore.SQLite,
	sessionManager *scs.SessionManager,
) http.Handler {
	type td struct {
		assets.BaseData
		Form loginForm
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := td{
			Form:     loginForm{},
			BaseData: extractBaseDataFromRequest(r),
		}
		if err := htmlRenderer.Render(w, http.StatusOK, data, "base", "pages/login.tmpl"); err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
		}
	})
}

func PostUserLogin(
	htmlRenderer *assets.HTMLRenderer,
	logger *slog.Logger,
	indexRepo *userstore.SQLite,
	sessionManager *scs.SessionManager,
) http.Handler {
	type td struct {
		assets.BaseData
		Form loginForm
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
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
				BaseData: extractBaseDataFromRequest(r),
				Form:     lf,
			}
			if err = htmlRenderer.Render(w, http.StatusUnprocessableEntity, data, "base", "pages/login.tmpl"); err != nil {
				logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
				htmlRenderer.RenderError(w, err)
			}
			return
		}

		user, err := indexRepo.AuthenticateUser(r.Context(), lf.Name, lf.Password)
		if err != nil {
			if errors.Is(err, userstore.ErrInvalidCredentials) {
				lf.FieldErrors["error"] = "username or password is incorrect"
				data := td{
					BaseData: extractBaseDataFromRequest(r),
					Form:     lf,
				}
				if err = htmlRenderer.Render(w, http.StatusForbidden, data, "base", "pages/login.tmpl"); err != nil {
					logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
					htmlRenderer.RenderError(w, err)
				}
				return
			}

			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}

		err = indexRepo.UpdateUserLastLoginToNow(r.Context(), user.ID)
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

		sessionManager.Put(r.Context(), "authenticatedUserId", user.ID)
		if user.IsAdmin {
			http.Redirect(w, r, "/admin", http.StatusSeeOther)
		} else {
			http.Redirect(w, r, "/", http.StatusSeeOther)
		}
	})
}

func PostUserLogout(
	htmlRenderer *assets.HTMLRenderer,
	logger *slog.Logger,
	sessionManager *scs.SessionManager,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := sessionManager.RenewToken(r.Context())
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}

		sessionManager.Remove(r.Context(), "authenticatedUserId")
		w.Header().Set("HX-Redirect", "/login")
		w.WriteHeader(http.StatusOK)
	})
}

func PutUserTheme(
	htmlRenderer *assets.HTMLRenderer,
	logger *slog.Logger,
	indexRepo *userstore.SQLite,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := parseIDFromPath(r)
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}

		ctxUserID := getUserIDFromRequest(r)
		if ctxUserID == 0 {
			err = errors.New("user id missing from request context")
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}

		if userID != ctxUserID {
			err = errors.New("authenticated user id does not match path user id")
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}

		userTheme := getUserThemeFromRequest(r)
		if userTheme == "" {
			err = errors.New("user theme not set in request")
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}

		if userTheme == "dark" {
			userTheme = "light"
		} else {
			userTheme = "dark"
		}
		err = indexRepo.UpdateUserTheme(r.Context(), ctxUserID, userTheme)
		if err != nil {
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}

		currentURL := r.Header.Get("HX-Current-URL")
		if currentURL == "" {
			err = errors.New("HX-Current-URL request header not set")
			logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			htmlRenderer.RenderError(w, err)
			return
		}

		w.Header().Set("HX-Redirect", currentURL)
		w.WriteHeader(http.StatusOK)
	}
}
