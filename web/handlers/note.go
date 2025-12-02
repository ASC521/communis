package handlers

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/ASC521/communis/models"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
)

type createNoteForm struct {
	Title       string
	Content     string
	Section     string
	Tags        map[string]bool
	AllTags     []*models.Tag
	AllSections []*models.Section
	FieldErrors map[string]string
}

func NoteCreateGet(
	tc map[string]*template.Template,
	logger *slog.Logger,
	tr models.TagRepository,
	sr models.SectionRepository,
) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		ts, ok := tc["create-note.tmpl"]
		if !ok {
			serverError(logger, w, r, errors.New("template new-note does not exist"))
			return
		}

		sec, err := sr.ListAll()
		if err != nil {
			serverError(logger, w, r, err)
			return
		}

		tags, err := tr.ListAll()
		if err != nil {
			serverError(logger, w, r, err)
			return
		}

		w.WriteHeader(http.StatusOK)
		err = ts.ExecuteTemplate(w, "base", createNoteForm{AllSections: sec, AllTags: tags, Title: "", Tags: map[string]bool{}})
		if err != nil {
			serverError(logger, w, r, err)
		}
	})
}

func NoteCreatePost(
	tc map[string]*template.Template,
	logger *slog.Logger,
	nr models.NoteRepository,
	sr models.SectionRepository,
	tr models.TagRepository,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			logger.Error("failed to parse form", "errMsg", err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		nf := createNoteForm{
			FieldErrors: map[string]string{},
		}

		title := r.PostForm.Get("title")
		if strings.TrimSpace(title) == "" {
			nf.FieldErrors["title"] = "Title is required"
		} else if utf8.RuneCountInString(title) > 100 {
			nf.FieldErrors["title"] = "This field cannot be more than 100 characters long"
		} else {
			exists, err := nr.Exists(title)
			if err != nil {
				serverError(logger, w, r, err)
				return
			}
			if exists {
				nf.FieldErrors["title"] = fmt.Sprintf("Title %s already exists", title)

			}
		}

		secForm := r.PostForm.Get("section")
		var section *models.Section
		if strings.TrimSpace(secForm) == "" {
			nf.FieldErrors["section"] = "This field cannot be empty"
		} else {
			section, err = sr.FindByName(secForm)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					nf.FieldErrors["section"] = fmt.Sprintf("Section %s does not exist", secForm)
				} else {
					serverError(logger, w, r, err)
					return
				}
			}
		}

		tagsForm := r.PostForm["tags"]
		selectedTags := []models.Tag{}
		var missing []string
		if len(tagsForm) > 0 {
			selectedTags, missing, err = tr.Query(tagsForm)
			if err != nil {
				serverError(logger, w, r, err)
				return
			}

			if len(missing) > 0 {
				mstr := strings.Join(missing, ", ")
				nf.FieldErrors["tags"] = fmt.Sprintf("tags %s have not been created", mstr)
			}
		}

		if len(nf.FieldErrors) > 0 {

			allTags, err := tr.ListAll()
			if err != nil {
				serverError(logger, w, r, err)
				return
			}

			secs, err := sr.ListAll()
			if err != nil {
				serverError(logger, w, r, err)
				return
			}

			tm := map[string]bool{}
			for _, t := range selectedTags {
				tm[t.Name] = true
			}

			nf.AllTags = allTags
			nf.AllSections = secs
			nf.Section = secForm
			nf.Tags = tm
			nf.Content = r.PostForm.Get("content")
			nf.Title = title

			ts, ok := tc["create-note.tmpl"]
			if !ok {
				serverError(logger, w, r, errors.New("template create-note does not exist"))
				return
			}
			err = ts.ExecuteTemplate(w, "base", nf)
			if err != nil {
				serverError(logger, w, r, err)
			}

			return

		}

		note := models.Note{
			Title:   title,
			Content: r.PostForm.Get("content"),
			Section: *section,
			Tags:    selectedTags,
		}

		id, err := nr.Create(&note)
		if err != nil {
			serverError(logger, w, r, err)
			return
		}

		http.Redirect(w, r, fmt.Sprintf("/note/%v/%s", id, slugify(note.Title)), http.StatusSeeOther)

	})
}

func NoteViewGet(
	tc map[string]*template.Template,
	logger *slog.Logger,
	nr models.NoteRepository,
) http.Handler {

	type viewNoteData struct {
		Note        models.Note
		HTMLContent template.HTML
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id < 1 {
			http.NotFound(w, r)
			return
		}

		n, err := nr.FindById(id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				http.NotFound(w, r)
				return
			}

			serverError(logger, w, r, err)
			return
		}

		expSlug := slugify(n.Title)
		actualSlug := r.PathValue("slug")
		if expSlug != actualSlug {
			http.Redirect(w, r, fmt.Sprintf("/note/%v/%s", id, expSlug), http.StatusMovedPermanently)
			return
		}

		md := goldmark.New(
			goldmark.WithExtensions(
				highlighting.NewHighlighting(
					highlighting.WithStyle("dracula"),
				),
			),
		)
		b := new(bytes.Buffer)
		err = md.Convert([]byte(n.Content), b)
		if err != nil {
			serverError(logger, w, r, err)
			return
		}

		vnd := viewNoteData{Note: *n, HTMLContent: template.HTML(b.String())}
		renderTemplate(tc, logger, w, r, http.StatusOK, "view-note.tmpl", vnd)

	})
}
