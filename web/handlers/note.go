package handlers

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"slices"
	"strconv"

	"github.com/ASC521/communis/models"
	"github.com/ASC521/communis/web/handlers/validator"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
)

const newNote = "NEW-NOTE"
const editNote = "EDIT-NOTE"

type tempDataCreateNote struct {
	Type        string
	FormData    noteForm
	AllTags     []models.Tag
	AllSections []models.Section
	FieldErrors map[string]string
}

type noteForm struct {
	Id        int64
	Title     string
	Content   string
	TagIds    []int64
	SectionId int64
}

func parseNoteForm(r *http.Request) (noteForm, error) {
	err := r.ParseForm()
	if err != nil {
		return noteForm{}, err
	}

	nf := noteForm{Id: 0}
	pid := r.PathValue("id")
	if pid != "" {
		id, err := strconv.ParseInt(pid, 10, 64)
		if err != nil {
			return noteForm{}, err
		}
		nf.Id = id
	}

	title := r.PostForm.Get("title")
	nf.Title = title

	content := r.PostForm.Get("note-content")
	nf.Content = content

	tagsF := r.PostForm["tags"]
	tagIds := make([]int64, len(tagsF))
	for i, t := range tagsF {
		tid, err := strconv.ParseInt(t, 10, 64)
		if err != nil {
			return noteForm{}, err
		}
		tagIds[i] = tid
	}
	nf.TagIds = tagIds

	secF := r.PostForm.Get("section")
	if secF != "" {
		sid, err := strconv.ParseInt(secF, 10, 64)
		if err != nil {
			return noteForm{}, err
		}
		nf.SectionId = sid
	}

	return nf, nil
}

func validateNoteForm(nf noteForm, nr models.NoteRepository, sr models.SectionRepository, tr models.TagRepository) (map[string]string, error) {
	fe := map[string]string{}

	// Note Id Validation
	if nf.Id > 0 {
		_, err := nr.FindById(nf.Id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				fe["id"] = fmt.Sprintf("id %d does not exist", nf.Id)
			}
			return nil, err
		}
	}

	// Title Validation
	if !validator.NotBlank(nf.Title) {
		fe["title"] = "title cannot be empty"

	}
	if !validator.MaxChars(nf.Title, 100) {
		fe["title"] = "title cannot be more than 100 characters"
	}

	nid, err := nr.Exists(nf.Title)
	// Check for database error - filter out sql.ErrNoRows
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if err == nil && nid != nf.Id {
		fe["title"] = fmt.Sprintf("title %s exists", nf.Title)
	}

	// Section Validation
	if nf.SectionId <= 0 {
		fe["section"] = "section does not exist"
	} else {
		_, err = sr.FindById(nf.SectionId)
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				return nil, err
			}
			fe["section"] = "section does not exist"
		}
	}

	// Tag Validation
	if len(nf.TagIds) > 0 {
		tags, err := tr.Query(nf.TagIds)
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				return nil, err
			}
			fe["tags"] = fmt.Sprintf("tag %v does not exist", nf.TagIds)
		}

		if len(tags) != len(nf.TagIds) {
			for _, tid := range nf.TagIds {
				if !slices.ContainsFunc(tags, func(t models.Tag) bool {
					return t.Id == tid
				}) {
					fe["tags"] = fmt.Sprintf("tags %v does not exist", tid)
				}
			}
		}

	}

	return fe, nil

}

func NoteNewGet(
	tc *TemplateCache,
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

		td := tempDataCreateNote{
			Type:        newNote,
			FormData:    noteForm{},
			AllTags:     tags,
			AllSections: sec,
			FieldErrors: map[string]string{},
		}
		tc.RenderPage(logger, w, r, http.StatusOK, "note-create.tmpl", td)
	})
}

func NotePost(
	tc *TemplateCache,
	logger *slog.Logger,
	nr models.NoteRepository,
	sr models.SectionRepository,
	tr models.TagRepository,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		nf, err := parseNoteForm(r)
		if err != nil {
			serverError(logger, w, r, err)
			return
		}

		fe, err := validateNoteForm(nf, nr, sr, tr)
		if err != nil {
			serverError(logger, w, r, err)
			return
		}

		if len(fe) > 0 {
			logger.Debug("Not creation did not pass validation", "fieldErrors", fe)

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

			td := tempDataCreateNote{
				Type:        newNote,
				FormData:    nf,
				AllTags:     allTags,
				AllSections: secs,
				FieldErrors: fe,
			}
			tc.RenderPage(logger, w, r, http.StatusUnprocessableEntity, "note-create.tmpl", td)
			return
		}

		ts := make([]models.Tag, len(nf.TagIds))
		for i, tid := range nf.TagIds {
			ts[i] = models.Tag{Id: tid}
		}
		n := models.Note{
			Title:   nf.Title,
			Content: nf.Content,
			Section: models.Section{Id: nf.SectionId},
			Tags:    ts,
		}

		id, err := nr.Create(n)
		if err != nil {
			serverError(logger, w, r, err)
			return
		}

		http.Redirect(w, r, fmt.Sprintf("/note/%v/%s", id, slugify(nf.Title)), http.StatusSeeOther)

	})
}

func NoteEditGet(
	tc *TemplateCache,
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

		tids := make([]int64, len(n.Tags)+1)
		for i, t := range n.Tags {
			tids[i] = t.Id
		}
		nf := noteForm{
			Id:        id,
			Title:     n.Title,
			Content:   n.Content,
			TagIds:    tids,
			SectionId: n.Section.Id,
		}

		td := tempDataCreateNote{
			Type:        editNote,
			FormData:    nf,
			AllTags:     tags,
			AllSections: sec,
			FieldErrors: map[string]string{},
		}
		tc.RenderPage(logger, w, r, http.StatusOK, "note-create.tmpl", td)
	})
}

func NotePut(
	tc *TemplateCache,
	logger *slog.Logger,
	nr models.NoteRepository,
	sr models.SectionRepository,
	tr models.TagRepository,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		nf, err := parseNoteForm(r)
		if err != nil {
			serverError(logger, w, r, err)
			return
		}
		fe, err := validateNoteForm(nf, nr, sr, tr)
		if err != nil {
			serverError(logger, w, r, err)
			return
		}

		if nf.Id <= 0 {
			logger.Warn("received a PUT request for a new note")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if len(fe) > 0 {

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

			td := tempDataCreateNote{
				Type:        editNote,
				FormData:    nf,
				AllTags:     allTags,
				AllSections: secs,
				FieldErrors: fe,
			}

			tc.RenderPage(logger, w, r, http.StatusUnprocessableEntity, "note-create.tmpl", td)
			return

		}

		ts := make([]models.Tag, len(nf.TagIds))
		for i, tid := range nf.TagIds {
			ts[i] = models.Tag{Id: tid}
		}
		n := models.Note{
			Id:      nf.Id,
			Title:   nf.Title,
			Content: nf.Content,
			Section: models.Section{Id: nf.SectionId},
			Tags:    ts,
		}
		err = nr.Update(n)
		if err != nil {
			serverError(logger, w, r, err)
			return
		}

		ru := fmt.Sprintf("/note/%v/%s", nf.Id, slugify(nf.Title))
		w.Header().Set("HX-Redirect", ru)
		w.WriteHeader(http.StatusOK)
	})
}

func NoteViewGet(
	tc *TemplateCache,
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

		vnd := viewNoteData{Note: n, HTMLContent: template.HTML(b.String())}
		tc.RenderPage(logger, w, r, http.StatusOK, "note-view.tmpl", vnd)
	})
}

func NoteSearchGet(
	tc *TemplateCache,
	logger *slog.Logger,
	nr models.NoteRepository,
) http.Handler {

	type templateData struct {
		Notes []models.NoteSearchResult
		Query string
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		td := templateData{}
		q := r.URL.Query().Get("q")

		if q != "" {
			srs, err := nr.Search(`"` + q + `"`)
			if err != nil {
				serverError(logger, w, r, err)
				return
			}
			td.Notes = srs
			td.Query = q
		} else {
			td.Notes = []models.NoteSearchResult{}
			td.Query = ""
		}

		name := r.Header.Get("Hx-Source")
		if name == "input#search" {
			tc.RenderPartial(logger, w, r, http.StatusOK, "note-table.tmpl", "note-table", td)
			return
		}
		tc.RenderPage(logger, w, r, http.StatusOK, "search.tmpl", td)

	})
}

func NoteDelete(
	tc *TemplateCache,
	logger *slog.Logger,
	nr models.NoteRepository,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id < 1 {
			http.NotFound(w, r)
			return
		}

		err = nr.Delete(id)
		if err != nil {
			serverError(logger, w, r, err)
			return
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
	})
}
