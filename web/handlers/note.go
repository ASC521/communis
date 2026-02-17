package handlers

import (
	"bytes"
	"context"
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
	"github.com/alexedwards/scs/v2"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
)

type noteForm struct {
	Id        int64
	Title     string
	Content   string
	TagIds    []int64
	SectionId int64
	Errors    map[string]string
}

type searchForm struct {
	Query string
}

func renderNote(n models.Note) (models.RenderedNote, error) {

	md := goldmark.New(
		goldmark.WithExtensions(
			highlighting.NewHighlighting(
				highlighting.WithStyle("dracula"),
			),
		),
	)
	b := new(bytes.Buffer)
	err := md.Convert([]byte(n.Content), b)
	if err != nil {
		return models.RenderedNote{}, err
	}

	rn := models.RenderedNote{
		Id:          n.Id,
		Title:       n.Title,
		Section:     n.Section,
		HTMLContent: template.HTML(b.String()),
		Tags:        n.Tags,
	}
	return rn, nil
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

func validateNoteForm(ctx context.Context, nf noteForm, notesRepo models.NotesRepository) (map[string]string, error) {
	fe := map[string]string{}

	// Note Id Validation
	if nf.Id > 0 {
		_, err := notesRepo.FindNoteById(ctx, nf.Id)
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

	nid, err := notesRepo.NoteExists(ctx, nf.Title)
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
		_, err = notesRepo.FindSectionById(ctx, nf.SectionId)
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				return nil, err
			}
			fe["section"] = "section does not exist"
		}
	}

	// Tag Validation
	if len(nf.TagIds) > 0 {
		tags, err := notesRepo.QueryTags(ctx, nf.TagIds)
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

func parseNoteFromNoteForm(nf noteForm) models.Note {
	ts := make([]models.Tag, len(nf.TagIds))
	for i, tid := range nf.TagIds {
		ts[i] = models.Tag{Id: tid}
	}
	return models.Note{
		Id:      nf.Id,
		Title:   nf.Title,
		Content: nf.Content,
		Section: models.Section{Id: nf.SectionId},
		Tags:    ts,
	}
}

func NoteNewGet(
	tc *TemplateCache,
	logger *slog.Logger,
	newNotesRepo getNotesRepo,
) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		notesRepo, ok := newNotesRepo(r)
		if !ok {
			serverError(logger, w, r, ErrNotesRepoNotFound)
			return
		}

		sec, err := notesRepo.ListAllSections(r.Context())
		if err != nil {
			serverError(logger, w, r, err)
			return
		}

		tags, err := notesRepo.ListAllTags(r.Context())
		if err != nil {
			serverError(logger, w, r, err)
			return
		}

		td := TemplateData{
			Form:         noteForm{SectionId: 1},
			Tags:         tags,
			Sections:     sec,
			RenderedNote: models.RenderedNote{IsPreview: true},
		}
		tc.RenderPage(logger, w, r, http.StatusOK, "note-create.tmpl", td)
	})
}

func NotePost(
	tc *TemplateCache,
	logger *slog.Logger,
	newNotesRepo getNotesRepo,
	sessionManager *scs.SessionManager,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		nf, err := parseNoteForm(r)
		if err != nil {
			serverError(logger, w, r, err)
			return
		}
		notesRepo, ok := newNotesRepo(r)
		if !ok {
			serverError(logger, w, r, ErrNotesRepoNotFound)
			return
		}
		fe, err := validateNoteForm(r.Context(), nf, notesRepo)
		if err != nil {
			serverError(logger, w, r, err)
			return
		}

		if len(fe) > 0 {
			logger.Debug("Not creation did not pass validation", "fieldErrors", fe)

			nf.Errors = fe
			allTags, err := notesRepo.ListAllTags(r.Context())
			if err != nil {
				serverError(logger, w, r, err)
				return
			}

			secs, err := notesRepo.ListAllSections(r.Context())
			if err != nil {
				serverError(logger, w, r, err)
				return
			}

			data := TemplateData{
				IsAuthenticated: isAuthenticated(r, sessionManager),
				Sections:        secs,
				Tags:            allTags,
				Form:            nf,
			}
			tc.RenderPage(logger, w, r, http.StatusUnprocessableEntity, "note-create.tmpl", data)
			return
		}
		n := parseNoteFromNoteForm(nf)
		id, err := notesRepo.CreateNote(r.Context(), n)
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
	newNotesRepo getNotesRepo,
	sessionManager *scs.SessionManager,
) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id < 1 {
			http.NotFound(w, r)
			return
		}

		notesRepo, ok := newNotesRepo(r)
		if !ok {
			serverError(logger, w, r, ErrNotesRepoNotFound)
			return
		}
		n, err := notesRepo.FindNoteById(r.Context(), id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				http.NotFound(w, r)
				return
			}

			serverError(logger, w, r, err)
			return
		}

		sec, err := notesRepo.ListAllSections(r.Context())
		if err != nil {
			serverError(logger, w, r, err)
			return
		}

		tags, err := notesRepo.ListAllTags(r.Context())
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
			Errors:    map[string]string{},
		}

		data := TemplateData{
			IsAuthenticated: isAuthenticated(r, sessionManager),
			RenderedNote:    models.RenderedNote{IsPreview: true},
			Sections:        sec,
			Tags:            tags,
			Form:            nf,
		}
		tc.RenderPage(logger, w, r, http.StatusOK, "note-create.tmpl", data)
	})
}

func NotePut(
	tc *TemplateCache,
	logger *slog.Logger,
	newNotesRepo getNotesRepo,
	sessionManager *scs.SessionManager,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		nf, err := parseNoteForm(r)
		if err != nil {
			serverError(logger, w, r, err)
			return
		}
		notesRepo, ok := newNotesRepo(r)
		if !ok {
			serverError(logger, w, r, ErrNotesRepoNotFound)
			return
		}
		fe, err := validateNoteForm(r.Context(), nf, notesRepo)
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
			nf.Errors = fe

			allTags, err := notesRepo.ListAllTags(r.Context())
			if err != nil {
				serverError(logger, w, r, err)
				return
			}

			secs, err := notesRepo.ListAllSections(r.Context())
			if err != nil {
				serverError(logger, w, r, err)
				return
			}

			data := TemplateData{
				IsAuthenticated: isAuthenticated(r, sessionManager),
				Sections:        secs,
				Tags:            allTags,
				Form:            nf,
			}

			tc.RenderPage(logger, w, r, http.StatusUnprocessableEntity, "note-create.tmpl", data)
			return

		}

		n := parseNoteFromNoteForm(nf)
		err = notesRepo.UpdateNote(r.Context(), n)
		if err != nil {
			serverError(logger, w, r, err)
			return
		}

		ru := fmt.Sprintf("/note/%v/%s", nf.Id, slugify(nf.Title))
		w.Header().Set("HX-Redirect", ru)
		w.WriteHeader(http.StatusOK)
	})
}

func NotePreviewPost(
	tc *TemplateCache,
	logger *slog.Logger,
	newNotesRepo getNotesRepo,
) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nf, err := parseNoteForm(r)
		if err != nil {
			serverError(logger, w, r, err)
			return
		}
		n := parseNoteFromNoteForm(nf)

		notesRepo, ok := newNotesRepo(r)
		if !ok {
			serverError(logger, w, r, ErrNotesRepoNotFound)
			return
		}
		for i, tag := range n.Tags {
			et, err := notesRepo.FindTagById(r.Context(), tag.Id)
			if err != nil {
				slog.Error(fmt.Sprintf("failed to enrich tag %v from database", tag.Id), "errMsg", err.Error())
				continue
			}
			n.Tags[i].Name = et.Name
		}

		sec, err := notesRepo.FindSectionById(r.Context(), n.Section.Id)
		if err != nil {
			slog.Error(fmt.Sprintf("failed to enrich section %v from database", n.Section.Id), "errMsg", err.Error())
		} else {
			n.Section.Name = sec.Name
		}

		rn, err := renderNote(n)
		if err != nil {
			serverError(logger, w, r, err)
			return
		}
		rn.IsPreview = true

		tc.RenderPartial(logger, w, r, http.StatusOK, "rendered-note", rn)
	})

}

func NoteViewGet(
	tc *TemplateCache,
	logger *slog.Logger,
	newNotesRepo getNotesRepo,
	sessionManager *scs.SessionManager,
) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id < 1 {
			http.NotFound(w, r)
			return
		}
		notesRepo, ok := newNotesRepo(r)
		if !ok {
			serverError(logger, w, r, ErrNotesRepoNotFound)
			return
		}
		n, err := notesRepo.FindNoteById(r.Context(), id)
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

		rn, err := renderNote(n)
		if err != nil {
			serverError(logger, w, r, err)
			return
		}
		rn.IsPreview = false

		data := TemplateData{
			IsAuthenticated: isAuthenticated(r, sessionManager),
			RenderedNote:    rn,
		}

		tc.RenderPage(logger, w, r, http.StatusOK, "note-view.tmpl", data)
	})
}

func NoteSearchGet(
	tc *TemplateCache,
	logger *slog.Logger,
	newNotesRepo getNotesRepo,
	sessionManager *scs.SessionManager,
) http.Handler {

	type templateData struct {
		Notes []models.NoteSearchResult
		Query string
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		data := TemplateData{
			IsAuthenticated: isAuthenticated(r, sessionManager),
		}
		q := r.URL.Query().Get("q")

		if q != "" {
			notesRepo, ok := newNotesRepo(r)
			if !ok {
				serverError(logger, w, r, ErrNotesRepoNotFound)
				return
			}
			srs, err := notesRepo.SearchNotes(r.Context(), `"`+q+`"`)
			if err != nil {
				serverError(logger, w, r, err)
				return
			}
			data.SearchResults = srs
			data.Form = searchForm{Query: q}
		} else {
			data.SearchResults = []models.NoteSearchResult{}
			data.Form = searchForm{Query: ""}
		}

		name := r.Header.Get("Hx-Source")
		if name == "input#search" {
			tc.RenderPartial(logger, w, r, http.StatusOK, "note-table", data.SearchResults)
			return
		}
		tc.RenderPage(logger, w, r, http.StatusOK, "search.tmpl", data)

	})
}

func NoteDelete(
	tc *TemplateCache,
	logger *slog.Logger,
	newNotesRepo getNotesRepo,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id < 1 {
			http.NotFound(w, r)
			return
		}
		notesRepo, ok := newNotesRepo(r)
		if !ok {
			serverError(logger, w, r, ErrNotesRepoNotFound)
			return
		}
		err = notesRepo.DeleteNote(r.Context(), id)
		if err != nil {
			serverError(logger, w, r, err)
			return
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
	})
}
