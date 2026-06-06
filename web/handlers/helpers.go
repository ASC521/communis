package handlers

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	datastore "github.com/ASC521/communis/data-store"
	userstore "github.com/ASC521/communis/user-store"
)

var ErrUserIDNotFound = errors.New("user id not found in request context")

func slugify(s string) string {

	s = strings.ToLower(s)

	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")

	reg := regexp.MustCompile("[^a-z0-9-]+")
	s = reg.ReplaceAllString(s, "")

	reg = regexp.MustCompile("-+")
	s = reg.ReplaceAllString(s, "-")

	s = strings.Trim(s, "-")

	maxLen := 100
	if len(s) > maxLen {
		s = s[:maxLen]
		s = strings.TrimRight(s, "-")
	}

	if s == "" {
		return "untitled"
	}

	return s
}

func safeHTML(s string) template.HTML {
	return template.HTML(s)
}

func parseIDFromPath(r *http.Request) (int64, error) {
	pathID := r.PathValue("id")
	if pathID == "" {
		return 0, errors.New("no id found in path")
	}

	id, err := strconv.ParseInt(pathID, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("id %v is not a valid int", pathID)
	}

	return id, nil
}

var ErrNotesRepoNotFound = errors.New("notes repository not found in context")

func GetNotesDataStore(r *http.Request, dss *userstore.SQLiteConnManager) (*datastore.SQLite, error) {

	userID, ok := r.Context().Value(userIDContextKey).(int64)
	if !ok {
		return nil, ErrUserIDNotFound
	}
	notesRepo, err := dss.GetNotesStore(r.Context(), userID)
	if err != nil {
		return nil, err
	}
	return notesRepo, nil
}

func isAuthenticated(r *http.Request) bool {
	isAuth, ok := r.Context().Value(isAuthenticatedContextKey).(bool)
	if !ok {
		return false
	}

	return isAuth
}

func isAdmin(r *http.Request) bool {
	isAdmin, ok := r.Context().Value(isAdminContextKey).(bool)
	if !ok {
		return false
	}
	return isAdmin
}

func getUserIDFromRequest(r *http.Request) int64 {
	userID, ok := r.Context().Value(userIDContextKey).(int64)
	if !ok {
		return 0
	}

	return userID
}

func getUserThemeFromRequest(r *http.Request) string {
	userTheme, ok := r.Context().Value(userThemeContextKey).(string)
	if !ok {
		return ""
	}

	return userTheme
}
