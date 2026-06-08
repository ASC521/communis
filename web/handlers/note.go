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

	datastore "github.com/ASC521/communis/data-store"
	userstore "github.com/ASC521/communis/user-store"
	"github.com/ASC521/communis/web/handlers/validator"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alexedwards/scs/v2"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
)

type noteForm struct {
	ID                  int64
	Title               string
	Content             string
	TagIds              []int64
	SectionID           int64
	ReferenceNoteIds    []int64
	ReferencedByNoteIds []int64
	Errors              map[string]string
}

type searchForm struct {
	Query string
}

func renderNote(markdownContent, theme string) (template.HTML, error) {

	var style highlighting.Option
	if theme == "dark" {
		style = highlighting.WithStyle("dracula")
	} else {
		style = highlighting.WithStyle("tango")
	}

	md := goldmark.New(
		goldmark.WithExtensions(
			highlighting.NewHighlighting(
				style,
				highlighting.WithFormatOptions(
					chromahtml.WithLineNumbers(true),
					chromahtml.WithClasses(true),
					chromahtml.ClassPrefix("renderedmd-"),
				),
			),
			extension.NewTable(),
		),
	)
	b := new(bytes.Buffer)

	err := md.Convert([]byte(markdownContent), b)
	if err != nil {
		return "", err
	}
	return template.HTML(b.String()), nil
}

func parseNoteForm(r *http.Request) (noteForm, error) {
	err := r.ParseForm()
	if err != nil {
		return noteForm{}, err
	}

	nf := noteForm{ID: 0}
	pid := r.PathValue("id")
	if pid != "" {
		id, err := strconv.ParseInt(pid, 10, 64)
		if err != nil {
			return noteForm{}, err
		}
		nf.ID = id
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
		nf.SectionID = sid
	}

	refNotesF := r.PostForm["reference-notes"]
	refNoteIds := make([]int64, len(refNotesF))
	for i, rni := range refNotesF {
		rid, err := strconv.ParseInt(rni, 10, 64)
		if err != nil {
			return noteForm{}, err
		}
		refNoteIds[i] = rid
	}
	nf.ReferenceNoteIds = refNoteIds

	refByNotesF := r.PostForm["referenced-by-notes"]
	refByNoteIds := make([]int64, len(refByNotesF))
	for i, rbi := range refByNotesF {
		rbid, err := strconv.ParseInt(rbi, 10, 64)
		if err != nil {
			return noteForm{}, err
		}
		refByNoteIds[i] = rbid
	}
	nf.ReferencedByNoteIds = refByNoteIds

	return nf, nil
}

func validateNoteForm(ctx context.Context, nf noteForm, notesRepo *datastore.SQLite) (map[string]string, error) {
	fe := map[string]string{}

	// Note Id Validation
	if nf.ID > 0 {
		_, err := notesRepo.FindNoteByID(ctx, nf.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				fe["id"] = fmt.Sprintf("id %d does not exist", nf.ID)
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
	if err == nil && nid != nf.ID {
		fe["title"] = fmt.Sprintf("title %s exists", nf.Title)
	}

	// Section Validation
	if nf.SectionID <= 0 {
		fe["section"] = "section does not exist"
	} else {
		_, err = notesRepo.FindSectionById(ctx, nf.SectionID)
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
				if !slices.ContainsFunc(tags, func(t datastore.Tag) bool {
					return t.ID == tid
				}) {
					fe["tags"] = fmt.Sprintf("tags %v does not exist", tid)
				}
			}
		}

	}

	return fe, nil

}

type noteCreateData struct {
	BaseData
	Form                   noteForm
	Tags                   []datastore.Tag
	Sections               []datastore.Section
	RenderedNote           renderedNotePageData
	SelectedReferenceNotes []datastore.NoteDetail
	ReferencedByNotes      []datastore.NoteDetail
}

func NoteNewGet(
	tc *TemplateCache,
	logger *slog.Logger,
	dss *userstore.SQLiteConnManager,
	sessionManager *scs.SessionManager,
) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		notesRepo, err := GetNotesDataStore(r, dss)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		sec, err := notesRepo.ListAllSections(r.Context())
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		tags, err := notesRepo.ListAllTags(r.Context())
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		data := noteCreateData{
			BaseData:               newBase(r),
			Form:                   noteForm{SectionID: 1},
			Tags:                   tags,
			Sections:               sec,
			RenderedNote:           renderedNotePageData{IsPreview: true},
			SelectedReferenceNotes: []datastore.NoteDetail{},
		}
		tc.RenderPage(logger, w, r, http.StatusOK, "note-create.tmpl", data)
	}
}

func NotePost(
	tc *TemplateCache,
	logger *slog.Logger,
	dss *userstore.SQLiteConnManager,
) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		nf, err := parseNoteForm(r)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		notesRepo, err := GetNotesDataStore(r, dss)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		fe, err := validateNoteForm(r.Context(), nf, notesRepo)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		if len(fe) > 0 {
			logger.Debug("Not creation did not pass validation", "fieldErrors", fe)

			nf.Errors = fe
			allTags, err := notesRepo.ListAllTags(r.Context())
			if err != nil {
				tc.RenderError(logger, w, r, err)
				return
			}

			secs, err := notesRepo.ListAllSections(r.Context())
			if err != nil {
				tc.RenderError(logger, w, r, err)
				return
			}

			data := noteCreateData{
				BaseData: newBase(r),
				Sections: secs,
				Tags:     allTags,
				Form:     nf,
			}
			tc.RenderPage(logger, w, r, http.StatusUnprocessableEntity, "note-create.tmpl", data)
			return
		}
		id, err := notesRepo.CreateNote(
			r.Context(),
			nf.Title,
			nf.Content,
			nf.SectionID,
			nf.TagIds,
			nf.ReferenceNoteIds,
		)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		ru := fmt.Sprintf("/note/%v/%s", id, slugify(nf.Title))
		w.Header().Set("HX-Redirect", ru)
		w.WriteHeader(http.StatusOK)
	}
}

func NoteEditGet(
	tc *TemplateCache,
	logger *slog.Logger,
	dss *userstore.SQLiteConnManager,
	sessionManager *scs.SessionManager,
) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id < 1 {
			http.NotFound(w, r)
			return
		}

		notesRepo, err := GetNotesDataStore(r, dss)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		n, err := notesRepo.FindNoteByID(r.Context(), id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				http.NotFound(w, r)
				return
			}

			tc.RenderError(logger, w, r, err)
			return
		}

		sec, err := notesRepo.ListAllSections(r.Context())
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		tags, err := notesRepo.ListAllTags(r.Context())
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		tids := make([]int64, len(n.Tags)+1)
		for i, t := range n.Tags {
			tids[i] = t.ID
		}
		nf := noteForm{
			ID:        id,
			Title:     n.Title,
			Content:   n.Content,
			TagIds:    tids,
			SectionID: n.Section.ID,
			Errors:    map[string]string{},
		}

		data := noteCreateData{
			BaseData:               newBase(r),
			RenderedNote:           renderedNotePageData{IsPreview: true},
			Sections:               sec,
			Tags:                   tags,
			Form:                   nf,
			SelectedReferenceNotes: n.ReferenceNotes,
			ReferencedByNotes:      n.ReferenceByNotes,
		}
		tc.RenderPage(logger, w, r, http.StatusOK, "note-create.tmpl", data)
	}
}

func NotePut(
	tc *TemplateCache,
	logger *slog.Logger,
	dss *userstore.SQLiteConnManager,
	sessionManager *scs.SessionManager,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		nf, err := parseNoteForm(r)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}
		notesRepo, err := GetNotesDataStore(r, dss)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		fe, err := validateNoteForm(r.Context(), nf, notesRepo)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		if nf.ID <= 0 {
			logger.Warn("received a PUT request for a new note")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if len(fe) > 0 {
			nf.Errors = fe

			allTags, err := notesRepo.ListAllTags(r.Context())
			if err != nil {
				tc.RenderError(logger, w, r, err)
				return
			}

			secs, err := notesRepo.ListAllSections(r.Context())
			if err != nil {
				tc.RenderError(logger, w, r, err)
				return
			}

			data := noteCreateData{
				BaseData: newBase(r),
				Sections: secs,
				Tags:     allTags,
				Form:     nf,
			}

			tc.RenderPage(logger, w, r, http.StatusUnprocessableEntity, "note-create.tmpl", data)
			return

		}

		err = notesRepo.UpdateNote(
			r.Context(),
			nf.ID,
			nf.Title,
			nf.Content,
			nf.SectionID,
			nf.TagIds,
			nf.ReferenceNoteIds,
		)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		ru := fmt.Sprintf("/note/%v/%s", nf.ID, slugify(nf.Title))
		w.Header().Set("HX-Redirect", ru)
		w.WriteHeader(http.StatusOK)
	}
}

type renderedNotePageData struct {
	Note         datastore.Note
	RenderedHTML template.HTML
	IsPreview    bool
}

func NotePreviewPost(
	tc *TemplateCache,
	logger *slog.Logger,
	dss *userstore.SQLiteConnManager,
) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		nf, err := parseNoteForm(r)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}
		notesRepo, err := GetNotesDataStore(r, dss)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}
		ts := make([]datastore.Tag, len(nf.TagIds))
		for i, tid := range nf.TagIds {
			et, err := notesRepo.FindTagById(r.Context(), tid)
			if err != nil {
				slog.Error(fmt.Sprintf("failed to enrich tag %v from database", tid), "errMsg", err.Error())
				continue
			}

			ts[i] = et
		}

		refNotes, err := notesRepo.GetNoteDetailByIds(r.Context(), nf.ReferenceNoteIds)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		refByNotes, err := notesRepo.GetNoteDetailByIds(r.Context(), nf.ReferencedByNoteIds)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		n := datastore.Note{
			ID:               nf.ID,
			Title:            nf.Title,
			Content:          nf.Content,
			Section:          datastore.Section{ID: nf.SectionID},
			ReferenceNotes:   refNotes,
			ReferenceByNotes: refByNotes,
			Tags:             ts,
			Bookmark:         false,
		}

		sec, err := notesRepo.FindSectionById(r.Context(), n.Section.ID)
		if err != nil {
			slog.Error(fmt.Sprintf("failed to enrich section %v from database", n.Section.ID), "errMsg", err.Error())
		} else {
			n.Section.Name = sec.Name
		}

		userTheme := getUserThemeFromRequest(r)
		if userTheme == "" {
			tc.RenderError(logger, w, r, errors.New("user theme not set in request"))
			return
		}

		rHTML, err := renderNote(n.Content, userTheme)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		tc.RenderPartial(logger, w, r, http.StatusOK, "rendered-note", renderedNotePageData{Note: n, RenderedHTML: rHTML, IsPreview: true})
	}

}

func NoteViewGet(
	tc *TemplateCache,
	logger *slog.Logger,
	dss *userstore.SQLiteConnManager,
	sessionManager *scs.SessionManager,
) http.HandlerFunc {

	type td struct {
		BaseData
		RenderedNote renderedNotePageData
	}

	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id < 1 {
			http.NotFound(w, r)
			return
		}

		notesRepo, err := GetNotesDataStore(r, dss)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		n, err := notesRepo.FindNoteByID(r.Context(), id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				http.NotFound(w, r)
				return
			}

			tc.RenderError(logger, w, r, err)
			return
		}

		expSlug := slugify(n.Title)
		actualSlug := r.PathValue("slug")
		if expSlug != actualSlug {
			http.Redirect(w, r, fmt.Sprintf("/note/%v/%s", id, expSlug), http.StatusMovedPermanently)
			return
		}

		userTheme := getUserThemeFromRequest(r)
		if userTheme == "" {
			tc.RenderError(logger, w, r, errors.New("user theme not set in request"))
			return
		}
		rHTML, err := renderNote(n.Content, userTheme)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		data := td{
			BaseData: newBase(r),
			RenderedNote: renderedNotePageData{
				Note:         n,
				RenderedHTML: rHTML,
				IsPreview:    false,
			},
		}

		tc.RenderPage(logger, w, r, http.StatusOK, "note-view.tmpl", data)
	}
}

func NoteSearchGet(
	tc *TemplateCache,
	logger *slog.Logger,
	dss *userstore.SQLiteConnManager,
	sessionManager *scs.SessionManager,
) http.HandlerFunc {

	type td struct {
		BaseData
		SearchResults []datastore.NoteSearchResult
		Form          searchForm
	}
	return func(w http.ResponseWriter, r *http.Request) {

		data := td{
			BaseData: newBase(r),
		}
		q := r.URL.Query().Get("q")

		if q != "" {

			notesRepo, err := GetNotesDataStore(r, dss)
			if err != nil {
				tc.RenderError(logger, w, r, err)
				return
			}

			srs, err := notesRepo.SearchNotes(r.Context(), `"`+q+`"`)
			if err != nil {
				tc.RenderError(logger, w, r, err)
				return
			}
			data.SearchResults = srs
			data.Form = searchForm{Query: q}
		} else {
			data.SearchResults = []datastore.NoteSearchResult{}
			data.Form = searchForm{Query: ""}
		}

		switch r.Header.Get("Hx-Source") {
		case "input#search":
			tc.RenderPartial(logger, w, r, http.StatusOK, "note-table", data.SearchResults)
		case "input#ref-notes-search":
			tc.RenderPartial(logger, w, r, http.StatusOK, "ref-notes-search-results", data.SearchResults)
		default:
			tc.RenderPage(logger, w, r, http.StatusOK, "search.tmpl", data)
		}

	}
}

func NoteDelete(
	tc *TemplateCache,
	logger *slog.Logger,
	dss *userstore.SQLiteConnManager,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id < 1 {
			http.NotFound(w, r)
			return
		}

		notesRepo, err := GetNotesDataStore(r, dss)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		err = notesRepo.DeleteNote(r.Context(), id)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		w.Header().Set("HX-Redirect", "/")
		w.WriteHeader(http.StatusOK)
	}
}

func ReferenceNoteSelectPost(
	tc *TemplateCache,
	logger *slog.Logger,
) http.HandlerFunc {
	type td struct {
		Title string
		ID    int64
	}
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseIDFromPath(r)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		err = r.ParseForm()
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}
		title := r.PostForm.Get("title")
		if title == "" {
			tc.RenderError(logger, w, r, errors.New("hx-vals title is an empty string"))
			return
		}

		tc.RenderPartial(logger, w, r, http.StatusOK, "selected-ref-note", td{ID: id, Title: title})
	}
}

func ReferenceNoteSelectDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}
}

func NoteBookmarkPutDelete(
	tc *TemplateCache,
	logger *slog.Logger,
	connMgr *userstore.SQLiteConnManager,
	setBookmark bool,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id < 1 {
			http.NotFound(w, r)
			return
		}
		dataStore, err := GetNotesDataStore(r, connMgr)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}
		err = dataStore.SetNoteBookmark(r.Context(), id, setBookmark)
		if err != nil {
			tc.RenderError(logger, w, r, err)
			return
		}

		tc.RenderPartial(logger, w, r, http.StatusOK, "bookmarked-button", datastore.Note{ID: id, Bookmark: setBookmark})

	}
}
