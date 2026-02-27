package handlers

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/ASC521/communis/models"
	"github.com/ASC521/communis/services"
)

var ErrUserIdNotFound = errors.New("user id not found in request context")

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

func parseIdFromPath(r *http.Request) (int64, error) {
	pathId := r.PathValue("id")
	if pathId == "" {
		return 0, errors.New("no id found in path")
	}

	id, err := strconv.ParseInt(pathId, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("id %v is not a valid int", pathId)
	}

	return id, nil
}

var ErrNotesRepoNotFound = errors.New("notes repository not found in context")

func GetNotesRepo(r *http.Request, dss services.DataStoreService) (models.NotesRepository, error) {

	userId, ok := r.Context().Value(userIdContextKey).(int64)
	if !ok {
		return nil, ErrUserIdNotFound
	}
	notesRepo, err := dss.GetNotesStore(r.Context(), userId)
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

func getUserIdFromRequest(r *http.Request) int64 {
	userId, ok := r.Context().Value(userIdContextKey).(int64)
	if !ok {
		return 0
	}

	return userId
}

func getUserThemeFromRequest(r *http.Request) string {
	userTheme, ok := r.Context().Value(userThemeContextKey).(string)
	if !ok {
		return ""
	}

	return userTheme
}
