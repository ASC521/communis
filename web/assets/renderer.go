package assets

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"runtime/debug"
	"slices"

	"github.com/ASC521/communis/slug"
)

//go:embed "html" "static"
var files embed.FS

var (
	HTMLFiles   = sub(files, "html")
	StaticFiles = sub(files, "static")
)

func sub(f embed.FS, dir string) fs.FS {
	sub, err := fs.Sub(f, dir)
	if err != nil {
		panic(err)
	}
	return sub
}

type BaseData struct {
	IsAuthenticated bool
	IsAdmin         bool
	UserId          int64
	Theme           string
}

type HTMLRenderer struct {
	debug           bool
	templateFS      fs.FS
	sharedTemplates *template.Template
}

func NewHTMLRenderer(files fs.FS, debug bool, sharedTemplateFiles ...string) (*HTMLRenderer, error) {
	funcMap := template.FuncMap{
		"slugify":      slug.Slugify,
		"safeHTML":     func(s string) template.HTML { return template.HTML(s) },
		"containsInt":  slices.Contains[[]int64],
		"jsonMarshall": json.Marshal,
	}

	sharedTemplates, err := template.New("").Funcs(funcMap).ParseFS(files, sharedTemplateFiles...)
	if err != nil {
		return nil, err
	}

	return &HTMLRenderer{templateFS: files, sharedTemplates: sharedTemplates, debug: debug}, nil
}

func (r *HTMLRenderer) Render(
	w http.ResponseWriter,
	status int,
	data any,
	templateName string,
	additionalTemplateFiles ...string,
) error {
	ts, err := r.sharedTemplates.Clone()
	if err != nil {
		return err
	}

	if len(additionalTemplateFiles) > 0 {
		ts, err = ts.ParseFS(r.templateFS, additionalTemplateFiles...)
		if err != nil {
			return err
		}
	}

	buf := new(bytes.Buffer)
	err = ts.ExecuteTemplate(buf, templateName, data)
	if err != nil {
		return err
	}

	w.WriteHeader(status)
	buf.WriteTo(w)

	return nil
}

func (r *HTMLRenderer) RenderError(w http.ResponseWriter, err error) {
	if r.debug {
		trace := string(debug.Stack())
		body := fmt.Sprintf("%s\n%s", err, trace)
		http.Error(w, body, http.StatusInternalServerError)
		return
	}
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}
