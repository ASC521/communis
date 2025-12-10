package handlers

import (
	"bytes"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
)

// The serverError helper writes a log entry at Error level (including the request
// method and URI as attributes), then sends a generic 500 Internal Server Error
// response to the user.
func serverError(logger *slog.Logger, w http.ResponseWriter, r *http.Request, err error) {
	logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

// The renderTemplate helper will execute and write out named template.
// Template is executed in a two stage process to catch any run time template execution errors.
func renderTemplate(
	tc map[string]*template.Template,
	logger *slog.Logger,
	w http.ResponseWriter,
	r *http.Request,
	status int,
	page string,
	data any,
) {
	ts, ok := tc[page]
	if !ok {
		serverError(logger, w, r, fmt.Errorf("the template %s does not exist", page))
		return
	}

	buf := new(bytes.Buffer)

	err := ts.ExecuteTemplate(buf, "layout", data)
	if err != nil {
		serverError(logger, w, r, err)
		return
	}

	w.WriteHeader(status)
	buf.WriteTo(w)
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
