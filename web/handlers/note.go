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

	"github.com/ASC521/communis/models"
	"github.com/ASC521/communis/web/handlers/validator"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
)

type createNoteForm struct {
	ActionDest  string
	Note        models.Note
	AllTags     []*models.Tag
	AllSections []*models.Section
	validator.Validator
}

func validCreateNoteForm(r *http.Request, nr models.NoteRepository, sr models.SectionRepository, tr models.TagRepository) (createNoteForm, error) {

	err := r.ParseForm()
	if err != nil {
		return createNoteForm{}, err
	}

	nf := createNoteForm{Note: models.Note{Content: r.PostForm.Get("content")}}

	pid := r.PathValue("id")
	if pid != "" {
		id, err := strconv.ParseInt(pid, 10, 64)
		if err != nil {
			return createNoteForm{}, err
		}
		nf.Note.Id = id
	}

	title := r.PostForm.Get("title")
	nf.CheckField(validator.NotBlank(title), "title", "this field cannot be empty")
	nf.CheckField(validator.MaxChars(title, 100), "title", "this field cannot be more than 100 characters")
	nid, err := nr.Exists(title)
	if err != nil {
		return createNoteForm{}, err
	}

	if nf.Note.Id != 0 && nid != nf.Note.Id {
		nf.CheckField(!validator.Equals(nid, -1), "title", "title already exists")
	}

	nf.Note.Title = title

	secForm := r.PostForm.Get("section")
	nf.CheckField(validator.NotBlank(secForm), "section", "this field cannot be empty")
	section, err := sr.FindByName(secForm)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			nf.AddFieldError("section", fmt.Sprintf("section %s does not exist", secForm))
		} else {
			return createNoteForm{}, err
		}
	}
	nf.Note.Section = *section

	tagsForm := r.PostForm["tags-list"]
	if len(tagsForm) > 0 {
		// TODO: I don't like this API for validating tags.  Update it.
		validTags, missing, err := tr.Query(tagsForm)
		if err != nil {
			return createNoteForm{}, err
		}

		if len(missing) > 0 {
			mstr := strings.Join(missing, ", ")
			nf.AddFieldError("tags", fmt.Sprintf("tags %s have not been created", mstr))
		}
		nf.Note.Tags = validTags
	}

	return nf, nil

}

func NoteCreateGet(
	tc map[string]*template.Template,
	logger *slog.Logger,
	tr models.TagRepository,
	sr models.SectionRepository,
) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

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

		nf := createNoteForm{AllSections: sec, AllTags: tags, ActionDest: "/note/create"}
		renderTemplate(tc, logger, w, r, http.StatusOK, "create-note-tmpl", nf)
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

		vnf, err := validCreateNoteForm(r, nr, sr, tr)
		if err != nil {
			serverError(logger, w, r, err)
			return
		}

		if !vnf.Valid() {

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

			vnf.AllTags = allTags
			vnf.AllSections = secs
			vnf.ActionDest = "/note/create"
			renderTemplate(tc, logger, w, r, http.StatusUnprocessableEntity, "create-note.tmpl", vnf)
			return
		}

		id, err := nr.Create(&vnf.Note)
		if err != nil {
			serverError(logger, w, r, err)
			return
		}

		http.Redirect(w, r, fmt.Sprintf("/note/%v/%s", id, slugify(vnf.Note.Title)), http.StatusSeeOther)

	})
}

func NoteEditGet(
	tc map[string]*template.Template,
	logger *slog.Logger,
	nr models.NoteRepository,
	sr models.SectionRepository,
	tr models.TagRepository,
) http.Handler {

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

		nf := createNoteForm{
			Note:        *n,
			AllTags:     tags,
			AllSections: sec,
			ActionDest:  fmt.Sprintf("/edit/%v/%s", id, slugify(n.Title)),
		}

		renderTemplate(tc, logger, w, r, http.StatusOK, "create-note.tmpl", nf)

	})
}

func NoteEditPost(
	tc map[string]*template.Template,
	logger *slog.Logger,
	nr models.NoteRepository,
	sr models.SectionRepository,
	tr models.TagRepository,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		vnf, err := validCreateNoteForm(r, nr, sr, tr)
		if err != nil {
			serverError(logger, w, r, err)
			return
		}

		id := r.PathValue("id")
		if !vnf.Valid() {

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

			vnf.AllTags = allTags
			vnf.AllSections = secs
			vnf.ActionDest = fmt.Sprintf("/edit/%v/%s", id, r.PathValue("slug"))

			renderTemplate(tc, logger, w, r, http.StatusUnprocessableEntity, "create-note.tmpl", vnf)
			return

		}

		err = nr.Update(&vnf.Note)
		if err != nil {
			serverError(logger, w, r, err)
			return
		}

		http.Redirect(w, r, fmt.Sprintf("/note/%v/%s", id, slugify(vnf.Note.Title)), http.StatusSeeOther)

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
