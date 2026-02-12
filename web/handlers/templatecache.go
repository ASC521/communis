package handlers

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"path/filepath"
	"slices"

	"github.com/ASC521/communis/models"
)

type TemplateData struct {
	IsAuthenticated bool
	Note            models.Note
	RenderedNote    models.RenderedNote
	Sections        []models.Section
	Section         models.Section
	Tags            []models.Tag
	Tag             models.Tag
	NoteDetails     []models.NoteDetail
	SearchResults   []models.NoteSearchResult
	Form            any
}

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
		"slugify":     slugify,
		"safeHTML":    safeHTML,
		"containsInt": slices.Contains[[]int64],
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

func (t *TemplateCache) RenderPage(
	logger *slog.Logger,
	w http.ResponseWriter,
	r *http.Request,
	status int,
	tempName string,
	data TemplateData,
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
	tempFileName string,
	tempName string,
	data any,
) {
	ts, ok := t.partials[tempFileName]
	if !ok {
		serverError(logger, w, r, fmt.Errorf("template %s does not exist", tempFileName))
		return
	}
	buf := new(bytes.Buffer)
	err := ts.ExecuteTemplate(buf, tempName, data)
	if err != nil {
		serverError(logger, w, r, err)
		return
	}
	w.WriteHeader(status)
	buf.WriteTo(w)
}
