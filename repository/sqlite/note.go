package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ASC521/communis/dbx/sqlitex"
	"github.com/ASC521/communis/models"
)

const sqliteTimeFmt = "2006-01-02 15:04:05"
const insNoteFTSSql = `INSERT INTO notes_details_fts(rowid, title, content, tags_txt)
		       SELECT notes_details.id, notes_details.title, notes_details.content, notes_details.tags_txt
		       FROM notes_details
		       WHERE notes_details.id = ?;`
const delNoteFTSSql = `INSERT INTO notes_details_fts(notes_details_fts, rowid, title, content, tags_txt)
		       SELECT 'delete', notes_details.id, notes_details.title, notes_details.content, notes_details.tags_txt
		       FROM notes_details
		       WHERE notes_details.id = ?;`

func parseNoteDetailsFromRows(rows *sql.Rows) ([]models.NoteDetail, error) {
	nds := []models.NoteDetail{}
	for rows.Next() {
		nd := models.NoteDetail{}
		err := rows.Scan(&nd.Id, &nd.Title)
		if err != nil {
			return nil, err
		}
		nds = append(nds, nd)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return nds, nil
}

func (r *NotesRepository) CreateNote(
	ctx context.Context,
	title string,
	content string,
	sectionId int64,
	tagIds []int64,
	referenceNoteIds []int64,
) (int64, error) {
	return sqlitex.WithTransaction(r.db.Write, ctx, func(ctx context.Context, tx *sql.Tx) (int64, error) {
		res, err := tx.Exec("INSERT INTO notes (title, content, section) VALUES (?, ?, ?);", title, content, sectionId)
		if err != nil {
			return 0, fmt.Errorf("failed to insert new note: %w", err)
		}
		nid, err := res.LastInsertId()
		if err != nil {
			return 0, fmt.Errorf("failed to retrieve id of inserted note: %w", err)
		}

		if nid == 0 {
			return 0, fmt.Errorf("id of created note not returned")
		}

		tagLen := len(tagIds)
		if tagLen > 0 {
			tgstmt := "INSERT INTO notes_tags (note_id, tag_id) VALUES"
			tgArgs := []any{}
			for i, t := range tagIds {
				if i != len(tagIds)-1 {
					tgstmt += " (?, ?),"
				} else {
					tgstmt += " (?, ?);"
				}
				tgArgs = append(tgArgs, nid, t)
			}
			_, err = tx.Exec(tgstmt, tgArgs...)
			if err != nil {
				return 0, fmt.Errorf("failed to insert tags associated with note %v: %v", nid, err)
			}
		}

		refNoteLen := len(referenceNoteIds)
		if refNoteLen > 0 {
			rnstmt := "INSERT INTO reference_notes (note_id, ref_note_id) VALUES "
			rnargs := []any{}
			for i, rnid := range referenceNoteIds {
				if i != len(referenceNoteIds)-1 {
					rnstmt += " (?, ?),"
				} else {
					rnstmt += " (?, ?);"
				}
				rnargs = append(rnargs, nid, rnid)
			}

			_, err = tx.Exec(rnstmt, rnargs...)
			if err != nil {
				return 0, fmt.Errorf("failed to insert reference notes for note_id %v: %v", nid, err)
			}
		}

		_, err = tx.Exec(insNoteFTSSql, nid)
		if err != nil {
			return -1, err
		}

		return nid, nil

	})
}

func (r *NotesRepository) NoteExists(ctx context.Context, title string) (int64, error) {
	q := `SELECT id FROM notes WHERE title = ?;`
	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	var id int64
	err := r.db.Read.QueryRowContext(ctxWTO, q, title).Scan(&id)
	if err != nil {
		return -1, err
	}

	return id, nil
}

func (r *NotesRepository) FindNoteById(ctx context.Context, id int64) (models.Note, error) {
	q := `
     SELECT id, section_id, section_name, title, content, created_at_utc, last_updated_at_utc, tags_json, reference_notes_json, reference_by_notes_json
     FROM notes_details
     WHERE id = ?;`
	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	var n models.Note
	var tagJSON, createdStr, updatedStr, refNotesJSON, refByNotesJSON string
	row := r.db.Read.QueryRowContext(ctxWTO, q, id)
	err := row.Scan(&n.Id, &n.Section.Id, &n.Section.Name, &n.Title, &n.Content, &createdStr, &updatedStr, &tagJSON, &refNotesJSON, &refByNotesJSON)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Note{}, err
		}
		return models.Note{}, fmt.Errorf("failed to scan row for note: %w", err)
	}

	err = json.Unmarshal([]byte(tagJSON), &n.Tags)
	if err != nil {
		return models.Note{}, fmt.Errorf("failed to parse tags json: %w", err)
	}

	created, err := time.ParseInLocation(sqliteTimeFmt, createdStr, time.UTC)
	if err != nil {
		return models.Note{}, fmt.Errorf("failed to parse created at time: %w", err)
	}
	n.CreatedAt = created

	updated, err := time.ParseInLocation(sqliteTimeFmt, updatedStr, time.UTC)
	if err != nil {
		return models.Note{}, fmt.Errorf("failed to parse updated at time: %w", err)
	}
	n.LastUpdatedAt = updated

	err = json.Unmarshal([]byte(refNotesJSON), &n.ReferenceNotes)
	if err != nil {
		return models.Note{}, fmt.Errorf("failed to parse reference notes: %w", err)
	}
	err = json.Unmarshal([]byte(refByNotesJSON), &n.ReferenceByNotes)
	if err != nil {
		return models.Note{}, fmt.Errorf("failed to parse reference by notes: %w", err)
	}

	return n, nil
}

func (r *NotesRepository) GetNoteDetailByIds(ctx context.Context, ids []int64) ([]models.NoteDetail, error) {

	args := make([]any, len(ids))
	placeholders := make([]string, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	placeholderStr := strings.Join(placeholders, ", ")

	q := fmt.Sprintf(`SELECT id, title FROM notes WHERE id IN (%s);`, placeholderStr)
	rows, err := r.db.Read.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}

	noteDetails := []models.NoteDetail{}
	for rows.Next() {
		nd := models.NoteDetail{}
		err := rows.Scan(&nd.Id, &nd.Title)
		if err != nil {
			return nil, err
		}
		noteDetails = append(noteDetails, nd)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return noteDetails, nil

}

func (r *NotesRepository) UpdateNote(
	ctx context.Context,
	id int64,
	title string,
	content string,
	sectionId int64,
	tagIds []int64,
	referenceNoteIds []int64,
) error {
	_, err := sqlitex.WithTransaction(r.db.Write, ctx, func(ctx context.Context, tx *sql.Tx) (int, error) {

		_, err := tx.Exec(delNoteFTSSql, id)
		if err != nil {
			return -1, err
		}

		s := `UPDATE notes SET title = ?, content = ?, section = ?, last_updated_at_utc = datetime('now') WHERE id = ?`
		_, err = tx.Exec(s, title, content, sectionId, id)
		if err != nil {
			return -1, fmt.Errorf("failed to update note %v: %w", id, err)
		}

		ds := `DELETE FROM notes_tags WHERE note_id = ?;`
		_, err = tx.Exec(ds, id)
		if err != nil {
			return -1, fmt.Errorf("failed to remove tags associated with note: %w", err)
		}

		tagsLen := len(tagIds)
		if tagsLen > 0 {
			tgstmt := "INSERT INTO notes_tags (note_id, tag_id) VALUES"
			tgArgs := []any{}
			for i, t := range tagIds {
				if i != tagsLen-1 {
					tgstmt += " (?, ?),"
				} else {
					tgstmt += " (?, ?);"
				}
				tgArgs = append(tgArgs, id, t)
			}
			_, err = tx.Exec(tgstmt, tgArgs...)
			if err != nil {
				return 0, fmt.Errorf("failed to insert tags associated with note %v: %v", id, err)
			}
		}

		drn := `DELETE FROM reference_notes WHERE note_id =?;`
		_, err = tx.Exec(drn, id)
		if err != nil {
			return -1, fmt.Errorf("failed to remove reference notes associated with note %v: %v", id, err)
		}

		refNoteLen := len(referenceNoteIds)
		if refNoteLen > 0 {
			rnstmt := "INSERT INTO reference_notes (note_id, ref_note_id) VALUES "
			rnargs := []any{}
			for i, rnid := range referenceNoteIds {
				if i != refNoteLen-1 {
					rnstmt += " (?, ?),"
				} else {
					rnstmt += " (?, ?);"
				}
				rnargs = append(rnargs, id, rnid)
			}

			_, err = tx.Exec(rnstmt, rnargs...)
			if err != nil {
				return 0, fmt.Errorf("failed to insert reference notes for note_id %v: %v", id, err)
			}
		}

		_, err = tx.Exec(insNoteFTSSql, id)
		if err != nil {
			return -1, err
		}

		return 1, nil

	})

	return err
}

func (r *NotesRepository) DeleteNote(ctx context.Context, id int64) error {
	_, err := sqlitex.WithTransaction(r.db.Write, ctx, func(ctx context.Context, tx *sql.Tx) (int, error) {

		_, err := tx.Exec(delNoteFTSSql, id)
		if err != nil {
			return -1, err
		}

		s := `DELETE FROM notes WHERE id = ?;`
		_, err = tx.Exec(s, id)
		if err != nil {
			return -1, fmt.Errorf("failed to delete note %v: %w", id, err)
		}

		return 1, nil

	})

	return err
}

func (r *NotesRepository) ListNotes(ctx context.Context, limit, offset int) (models.PaginatedNotes, error) {

	if limit <= 0 {
		limit = 10
	}

	if offset < 0 {
		offset = 0
	}

	query := `
	SELECT n.id,  n.title
	FROM notes AS n
	ORDER BY n.id ASC LIMIT ? OFFSET ?;`
	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	rows, err := r.db.Read.QueryContext(ctxWTO, query, limit+1, offset)
	if err != nil {
		return models.PaginatedNotes{}, fmt.Errorf("failed to query notes: %w", err)
	}
	defer rows.Close()

	ns := make([]*models.NoteDetail, 0, limit)
	for rows.Next() {
		n := &models.NoteDetail{}
		err = rows.Scan(&n.Id, &n.Title)
		if err != nil {
			return models.PaginatedNotes{}, fmt.Errorf("failed to scan rows for note: %w", err)
		}

		ns = append(ns, n)
	}

	if err = rows.Err(); err != nil {
		return models.PaginatedNotes{}, fmt.Errorf("error iterating notes: %w", err)
	}

	var nextOffset *int
	hasMore := len(ns) > limit
	if hasMore {
		ns = ns[:limit]
		next := offset + limit
		nextOffset = &next
	}

	return models.PaginatedNotes{
		Notes:      ns,
		Limit:      limit,
		Offset:     offset,
		HasMore:    hasMore,
		NextOffset: nextOffset,
	}, nil

}

func (r *NotesRepository) SearchNotes(ctx context.Context, q string) ([]models.NoteSearchResult, error) {

	sql := `SELECT DISTINCT
		  nd.id,
		  nd.title,
		  highlight(notes_details_fts, 0, '<mark>', '</mark>') as title_highlight,
		  snippet(notes_details_fts, 1, '<mark>', '</mark>', '...', 100) as content_snippet,
		  IFNULL(highlight(notes_details_fts, 2, '<mark>', '</mark>'), '') AS tags_txt
		FROM notes_details_fts AS fts
		INNER JOIN notes_details AS nd
		ON fts.rowid = nd.id
		WHERE notes_details_fts MATCH ?
		ORDER BY rank
		LIMIT 50;`
	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	rows, err := r.db.Read.QueryContext(ctxWTO, sql, q)
	if err != nil {
		return nil, err
	}

	srs := []models.NoteSearchResult{}
	for rows.Next() {
		sr := models.NoteSearchResult{}
		err = rows.Scan(&sr.Id, &sr.Title, &sr.TitleHighlight, &sr.ContentSnippet, &sr.TagNames)
		if err != nil {
			return nil, err
		}
		srs = append(srs, sr)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating search results: %w", err)
	}

	return srs, nil

}

func (r *NotesRepository) RecentlyUpdatedNotes(ctx context.Context, limit int) ([]models.NoteDetail, error) {

	sql := `SELECT n.id, n.title
		FROM notes as n
		ORDER BY last_updated_at_utc DESC
		LIMIT ?;`
	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	rows, err := r.db.Read.QueryContext(ctxWTO, sql, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return parseNoteDetailsFromRows(rows)
}

func (r *NotesRepository) NotesInSection(ctx context.Context, secId int64) ([]models.NoteDetail, error) {
	sql := `SELECT n.id, n.title
		FROM notes as n
		WHERE n.section = ?
		ORDER BY title ASC`

	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	rows, err := r.db.Read.QueryContext(ctxWTO, sql, secId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return parseNoteDetailsFromRows(rows)
}

func (r *NotesRepository) NotesWithTag(ctx context.Context, tagId int64) ([]models.NoteDetail, error) {
	sql := `SELECT n.id, n.title
		FROM notes as n
		INNER JOIN notes_tags as nt
		ON n.id = nt.note_id
		WHERE nt.tag_id = ?;`

	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	rows, err := r.db.Read.QueryContext(ctxWTO, sql, tagId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return parseNoteDetailsFromRows(rows)
}
