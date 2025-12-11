package handlers

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"path/filepath"
)

type TemplateCache struct {
	pages    map[string]*template.Template
	partials map[string]*template.Template
}

func NewTemplateCache(files fs.FS) (*TemplateCache, error) {

	tc := &TemplateCache{
		pages:    map[string]*template.Template{},
		partials: map[string]*template.Template{},
	}

	funcMap := template.FuncMap{
		"slugify": slugify,
	}

	partialFiles, err := fs.Glob(files, "html/partials/*.tmpl")
	if err != nil {
		return nil, err
	}

	for _, partialFile := range partialFiles {
		name := filepath.Base(partialFile)
		ts, err := template.New(name).Funcs(funcMap).ParseFS(files, partialFile)
		if err != nil {
			return nil, err
		}

		tc.partials[name] = ts
	}

	pageFiles, err := fs.Glob(files, "html/pages/*.tmpl")
	if err != nil {
		return nil, err
	}

	for _, pageFile := range pageFiles {
		name := filepath.Base(pageFile)
		tempFiles := []string{"html/layout.tmpl", pageFile}
		tempFiles = append(tempFiles, partialFiles...)
		ts, err := template.New(name).Funcs(funcMap).ParseFS(files, tempFiles...)
		if err != nil {
			return nil, err
		}

		tc.pages[name] = ts

	}

	return tc, nil

}

func (t *TemplateCache) render(
	ts *template.Template,
	logger *slog.Logger,
	w http.ResponseWriter,
	r *http.Request,
	status int,
	data any,
) {
	buf := new(bytes.Buffer)
	err := ts.ExecuteTemplate(buf, "layout", data)
	if err != nil {
		serverError(logger, w, r, err)
	}
	w.WriteHeader(status)
	buf.WriteTo(w)
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
		serverError(logger, w, r, fmt.Errorf("template %s does not exist", tempName))
		return
	}
	buf := new(bytes.Buffer)
	err := ts.ExecuteTemplate(buf, "layout", data)
	if err != nil {
		serverError(logger, w, r, err)
	}
	w.WriteHeader(status)
	buf.WriteTo(w)
}

func (t *TemplateCache) RenderPartial(
	logger *slog.Logger,
	w http.ResponseWriter,
	r *http.Request,
	status int,
	tempName string,
	data any,
) {
	ts, ok := t.partials[tempName]
	if !ok {
		serverError(logger, w, r, fmt.Errorf("template %s does not exist", tempName))
		return
	}
	buf := new(bytes.Buffer)
	err := ts.ExecuteTemplate(buf, tempName, data)
	if err != nil {
		serverError(logger, w, r, err)
	}
	w.WriteHeader(status)
	buf.WriteTo(w)

}
