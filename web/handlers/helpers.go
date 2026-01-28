package handlers

import (
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

// The serverError helper writes a log entry at Error level (including the request
// method and URI as attributes), then sends a generic 500 Internal Server Error
// response to the user.
func serverError(logger *slog.Logger, w http.ResponseWriter, r *http.Request, err error) {
	logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

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
		return 0, fmt.Errorf("section id %v is not a valid int", pathId)
	}

	return id, nil
}
