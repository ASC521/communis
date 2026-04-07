package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"path/filepath"
	"runtime/debug"
	"slices"
)

type BaseData struct {
	IsAuthenticated bool
	UserId          int64
	Theme           string
}

func newBase(r *http.Request) BaseData {
	return BaseData{
		IsAuthenticated: isAuthenticated(r),
		UserId:          getUserIdFromRequest(r),
		Theme:           getUserThemeFromRequest(r),
	}
}

type TemplateCache struct {
	debug    bool
	pages    map[string]*template.Template
	partials *template.Template
}

func NewTemplateCache(files fs.FS, debug bool) (*TemplateCache, error) {

	funcMap := template.FuncMap{
		"slugify":      slugify,
		"safeHTML":     safeHTML,
		"containsInt":  slices.Contains[[]int64],
		"jsonMarshall": json.Marshal,
	}

	partialsTemplate, err := template.New("partials").Funcs(funcMap).ParseFS(files, "html/partials/*.tmpl")
	if err != nil {
		return nil, err
	}

	pageFiles, err := fs.Glob(files, "html/pages/*.tmpl")
	if err != nil {
		return nil, err
	}

	pages := map[string]*template.Template{}
	for _, pageFile := range pageFiles {
		name := filepath.Base(pageFile)
		tempFiles := []string{"html/layout.tmpl", pageFile}
		temp, err := partialsTemplate.Clone()
		if err != nil {
			return nil, err
		}
		temp, err = temp.Funcs(funcMap).ParseFS(files, tempFiles...)
		if err != nil {
			return nil, err
		}

		pages[name] = temp

	}

	return &TemplateCache{pages: pages, partials: partialsTemplate, debug: debug}, nil

}

func (t *TemplateCache) RenderPage(
	logger *slog.Logger,
	w http.ResponseWriter,
	r *http.Request,
	status int,
	tempName string,
	data any,
) {
	ts, ok := t.pages[tempName]
	if !ok {
		t.RenderError(logger, w, r, fmt.Errorf("template %s does not exist", tempName))
		return
	}
	buf := new(bytes.Buffer)
	err := ts.ExecuteTemplate(buf, "layout", data)
	if err != nil {
		t.RenderError(logger, w, r, err)
		return
	}
	w.WriteHeader(status)
	buf.WriteTo(w)
}

func (t *TemplateCache) RenderPartial(
	logger *slog.Logger,
	w http.ResponseWriter,
	r *http.Request,
	status int,
	name string,
	data any,
) {
	buf := new(bytes.Buffer)
	err := t.partials.ExecuteTemplate(buf, name, data)
	if err != nil {
		t.RenderError(logger, w, r, err)
		return
	}
	w.WriteHeader(status)
	buf.WriteTo(w)
}

func (t *TemplateCache) RenderError(
	logger *slog.Logger,
	w http.ResponseWriter,
	r *http.Request,
	err error,
) {
	method := r.Method
	uri := r.URL.RequestURI()

	logger.Error(err.Error(), "method", method, "uri", uri)
	if t.debug {
		trace := string(debug.Stack())
		body := fmt.Sprintf("%s\n%s", err, trace)
		http.Error(w, body, http.StatusInternalServerError)
		return
	}
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}
