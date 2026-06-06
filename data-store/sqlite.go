package datastore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ASC521/communis/dbx/sqlitex"
)

const (
	sqliteTimeFmt = "2006-01-02 15:04:05"
	insNoteFTSSql = `INSERT INTO notes_details_fts(rowid, title, content, tags_txt)
		       SELECT notes_details.id, notes_details.title, notes_details.content, notes_details.tags_txt
		       FROM notes_details
		       WHERE notes_details.id = ?;`
	delNoteFTSSql = `INSERT INTO notes_details_fts(notes_details_fts, rowid, title, content, tags_txt)
		       SELECT 'delete', notes_details.id, notes_details.title, notes_details.content, notes_details.tags_txt
		       FROM notes_details
		       WHERE notes_details.id = ?;`
	delTagFTSSql = `INSERT INTO notes_details_fts(notes_details_fts, rowid, title, content, tags_txt)
		   SELECT 'delete', notes_details.id,  title, content, tags_txt
		   FROM notes_details, json_each(notes_details.tags_json)
		   WHERE json_extract(value, '$.id') = ?;`
	insTagFTSSql = `INSERT INTO notes_details_fts(rowid, title, content, tags_txt)
		   SELECT notes_details.id, notes_details.title, notes_details.content, notes_details.tags_txt
		   FROM notes_details, json_each(notes_details.tags_json)
		   WHERE json_extract(value, '$.id') = ?;`
)

type SQLite struct {
	db *sqlitex.SQLiteDB
}

func NewSQLite(db *sqlitex.SQLiteDB) *SQLite {
	return &SQLite{db: db}
}

// NOTE FUNCTIONS

func parseNoteDetailsFromRows(rows *sql.Rows) ([]NoteDetail, error) {
	nds := []NoteDetail{}
	for rows.Next() {
		nd := NoteDetail{}
		err := rows.Scan(&nd.ID, &nd.Title)
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

func (r *SQLite) CreateNote(
	ctx context.Context,
	title string,
	content string,
	sectionID int64,
	tagIds []int64,
	referenceNoteIds []int64,
) (int64, error) {
	return sqlitex.WithTransaction(r.db.Write, ctx, func(ctx context.Context, tx *sql.Tx) (int64, error) {
		res, err := tx.Exec("INSERT INTO notes (title, content, section) VALUES (?, ?, ?);", title, content, sectionID)
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
			var tgstmt strings.Builder
			tgstmt.WriteString("INSERT INTO notes_tags (note_id, tag_id) VALUES")
			tgArgs := []any{}
			for i, t := range tagIds {
				if i != len(tagIds)-1 {
					tgstmt.WriteString(" (?, ?),")
				} else {
					tgstmt.WriteString(" (?, ?);")
				}
				tgArgs = append(tgArgs, nid, t)
			}
			_, err = tx.Exec(tgstmt.String(), tgArgs...)
			if err != nil {
				return 0, fmt.Errorf("failed to insert tags associated with note %v: %v", nid, err)
			}
		}

		refNoteLen := len(referenceNoteIds)
		if refNoteLen > 0 {
			var rnstmt strings.Builder
			rnstmt.WriteString("INSERT INTO reference_notes (note_id, ref_note_id) VALUES ")
			rnargs := []any{}
			for i, rnid := range referenceNoteIds {
				if i != len(referenceNoteIds)-1 {
					rnstmt.WriteString(" (?, ?),")
				} else {
					rnstmt.WriteString(" (?, ?);")
				}
				rnargs = append(rnargs, nid, rnid)
			}

			_, err = tx.Exec(rnstmt.String(), rnargs...)
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

func (r *SQLite) NoteExists(ctx context.Context, title string) (int64, error) {
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

func (r *SQLite) FindNoteByID(ctx context.Context, id int64) (Note, error) {
	q := `
     SELECT id, section_id, section_name, title, content, created_at_utc, last_updated_at_utc, tags_json, reference_notes_json, reference_by_notes_json
     FROM notes_details
     WHERE id = ?;`
	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	var n Note
	var tagJSON, createdStr, updatedStr, refNotesJSON, refByNotesJSON string
	row := r.db.Read.QueryRowContext(ctxWTO, q, id)
	err := row.Scan(&n.ID, &n.Section.ID, &n.Section.Name, &n.Title, &n.Content, &createdStr, &updatedStr, &tagJSON, &refNotesJSON, &refByNotesJSON)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Note{}, err
		}
		return Note{}, fmt.Errorf("failed to scan row for note: %w", err)
	}

	err = json.Unmarshal([]byte(tagJSON), &n.Tags)
	if err != nil {
		return Note{}, fmt.Errorf("failed to parse tags json: %w", err)
	}

	created, err := time.ParseInLocation(sqliteTimeFmt, createdStr, time.UTC)
	if err != nil {
		return Note{}, fmt.Errorf("failed to parse created at time: %w", err)
	}
	n.CreatedAt = created

	updated, err := time.ParseInLocation(sqliteTimeFmt, updatedStr, time.UTC)
	if err != nil {
		return Note{}, fmt.Errorf("failed to parse updated at time: %w", err)
	}
	n.LastUpdatedAt = updated

	err = json.Unmarshal([]byte(refNotesJSON), &n.ReferenceNotes)
	if err != nil {
		return Note{}, fmt.Errorf("failed to parse reference notes: %w", err)
	}
	err = json.Unmarshal([]byte(refByNotesJSON), &n.ReferenceByNotes)
	if err != nil {
		return Note{}, fmt.Errorf("failed to parse reference by notes: %w", err)
	}

	return n, nil
}

func (r *SQLite) GetNoteDetailByIds(ctx context.Context, ids []int64) ([]NoteDetail, error) {

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

	noteDetails := []NoteDetail{}
	for rows.Next() {
		nd := NoteDetail{}
		err := rows.Scan(&nd.ID, &nd.Title)
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

func (r *SQLite) UpdateNote(
	ctx context.Context,
	id int64,
	title string,
	content string,
	sectionID int64,
	tagIds []int64,
	referenceNoteIds []int64,
) error {
	_, err := sqlitex.WithTransaction(r.db.Write, ctx, func(ctx context.Context, tx *sql.Tx) (int, error) {

		_, err := tx.Exec(delNoteFTSSql, id)
		if err != nil {
			return -1, err
		}

		s := `UPDATE notes SET title = ?, content = ?, section = ?, last_updated_at_utc = datetime('now') WHERE id = ?`
		_, err = tx.Exec(s, title, content, sectionID, id)
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
			var tgstmt strings.Builder
			tgstmt.WriteString("INSERT INTO notes_tags (note_id, tag_id) VALUES")
			tgArgs := []any{}
			for i, t := range tagIds {
				if i != tagsLen-1 {
					tgstmt.WriteString(" (?, ?),")
				} else {
					tgstmt.WriteString(" (?, ?);")
				}
				tgArgs = append(tgArgs, id, t)
			}
			_, err = tx.Exec(tgstmt.String(), tgArgs...)
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
			var rnstmt strings.Builder
			rnstmt.WriteString("INSERT INTO reference_notes (note_id, ref_note_id) VALUES ")
			rnargs := []any{}
			for i, rnid := range referenceNoteIds {
				if i != refNoteLen-1 {
					rnstmt.WriteString(" (?, ?),")
				} else {
					rnstmt.WriteString(" (?, ?);")
				}
				rnargs = append(rnargs, id, rnid)
			}

			_, err = tx.Exec(rnstmt.String(), rnargs...)
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

func (r *SQLite) DeleteNote(ctx context.Context, id int64) error {
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

func (r *SQLite) ListNotes(ctx context.Context, limit, offset int) (PaginatedNotes, error) {

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
		return PaginatedNotes{}, fmt.Errorf("failed to query notes: %w", err)
	}
	defer rows.Close()

	ns := make([]*NoteDetail, 0, limit)
	for rows.Next() {
		n := &NoteDetail{}
		err = rows.Scan(&n.ID, &n.Title)
		if err != nil {
			return PaginatedNotes{}, fmt.Errorf("failed to scan rows for note: %w", err)
		}

		ns = append(ns, n)
	}

	if err = rows.Err(); err != nil {
		return PaginatedNotes{}, fmt.Errorf("error iterating notes: %w", err)
	}

	var nextOffset *int
	hasMore := len(ns) > limit
	if hasMore {
		ns = ns[:limit]
		next := offset + limit
		nextOffset = &next
	}

	return PaginatedNotes{
		Notes:      ns,
		Limit:      limit,
		Offset:     offset,
		HasMore:    hasMore,
		NextOffset: nextOffset,
	}, nil

}

func (r *SQLite) SearchNotes(ctx context.Context, q string) ([]NoteSearchResult, error) {

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

	srs := []NoteSearchResult{}
	for rows.Next() {
		sr := NoteSearchResult{}
		err = rows.Scan(&sr.ID, &sr.Title, &sr.TitleHighlight, &sr.ContentSnippet, &sr.TagNames)
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

func (r *SQLite) RecentlyUpdatedNotes(ctx context.Context, limit int) ([]NoteDetail, error) {

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

func (r *SQLite) NotesInSection(ctx context.Context, secID int64) ([]NoteDetail, error) {
	sql := `SELECT n.id, n.title
		FROM notes as n
		WHERE n.section = ?
		ORDER BY title ASC`

	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	rows, err := r.db.Read.QueryContext(ctxWTO, sql, secID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return parseNoteDetailsFromRows(rows)
}

func (r *SQLite) NotesWithTag(ctx context.Context, tagID int64) ([]NoteDetail, error) {
	sql := `SELECT n.id, n.title
		FROM notes as n
		INNER JOIN notes_tags as nt
		ON n.id = nt.note_id
		WHERE nt.tag_id = ?;`

	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	rows, err := r.db.Read.QueryContext(ctxWTO, sql, tagID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return parseNoteDetailsFromRows(rows)
}

// NOTE FUNCTIONS

// SECTION FUNCTIONS

func (r *SQLite) CreateSection(ctx context.Context, s Section) (int64, error) {

	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()
	res, err := r.db.Write.ExecContext(ctxWTO, "INSERT INTO sections (name) VALUES (?);", s.Name)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()

}

func (r *SQLite) FindSectionById(ctx context.Context, id int64) (Section, error) {
	sql := "SELECT id, name FROM sections WHERE id = ?;"
	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	nb := Section{}
	err := r.db.Read.QueryRowContext(ctxWTO, sql, id).Scan(&nb.ID, &nb.Name)
	if err != nil {
		return Section{}, err
	}
	return nb, nil
}

func (r *SQLite) FindSectionByName(ctx context.Context, name string) (Section, error) {
	sql := "SELECT id, name FROM sections WHERE name = ?;"
	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	nb := Section{}
	err := r.db.Read.QueryRowContext(ctxWTO, sql, name).Scan(&nb.ID, &nb.Name)
	if err != nil {
		return Section{}, err
	}
	return nb, nil
}

func (r *SQLite) UpdateSection(ctx context.Context, s Section) error {

	sql := "UPDATE sections SET name = ? WHERE id = ?;"
	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	_, err := r.db.Write.ExecContext(ctxWTO, sql, s.Name, s.ID)
	if err != nil {
		return err
	}

	return nil
}

func (r *SQLite) DeleteSection(ctx context.Context, id int64) error {

	sql := "DELETE FROM sections WHERE id = ?;"
	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	_, err := r.db.Write.ExecContext(ctxWTO, sql, id)
	if err != nil {
		return err
	}
	return nil
}

func (r *SQLite) ListAllSections(ctx context.Context) ([]Section, error) {
	query := "SELECT id, name FROM sections ORDER BY name ASC"
	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()
	rows, err := r.db.Read.QueryContext(ctxWTO, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query sections: %w", err)
	}
	defer rows.Close()

	var secs []Section
	for rows.Next() {
		sec := Section{}
		err = rows.Scan(&sec.ID, &sec.Name)
		if err != nil {
			return nil, err
		}

		secs = append(secs, sec)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating sections: %w", err)
	}

	return secs, nil
}

// SECTION FUNCTIONS

// TAG FUNCTIONS

func (r *SQLite) CreateTag(ctx context.Context, t Tag) (int64, error) {

	sql := "INSERT INTO tags (name) VALUES (?);"
	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	res, err := r.db.Write.ExecContext(ctxWTO, sql, t.Name)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()

}

func (r *SQLite) FindTagById(ctx context.Context, id int64) (Tag, error) {
	sql := "SELECT id, name FROM tags WHERE id = ?;"
	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	t := Tag{}
	err := r.db.Read.QueryRowContext(ctxWTO, sql, id).Scan(&t.ID, &t.Name)
	if err != nil {
		return Tag{}, err
	}
	return t, nil
}

func (r *SQLite) FindTagByName(ctx context.Context, name string) (Tag, error) {
	sql := "SELECT id, name FROM tags WHERE name = ?;"
	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	t := Tag{}
	err := r.db.Read.QueryRowContext(ctxWTO, sql, name).Scan(&t.ID, &t.Name)
	if err != nil {
		return Tag{}, err
	}
	return t, nil
}

func (r *SQLite) UpdateTag(ctx context.Context, t Tag) error {

	_, err := sqlitex.WithTransaction(r.db.Write, ctx, func(ctx context.Context, tx *sql.Tx) (int, error) {

		_, err := tx.Exec(delTagFTSSql, t.ID)
		if err != nil {
			return -1, err
		}

		sql := "UPDATE tags SET name = ? WHERE id = ?;"
		_, err = tx.Exec(sql, t.Name, t.ID)
		if err != nil {
			return -1, err
		}

		_, err = tx.Exec(insTagFTSSql, t.ID)
		if err != nil {
			return -1, err
		}
		return 1, nil
	})
	return err
}

func (r *SQLite) DeleteTag(ctx context.Context, id int64) error {
	_, err := sqlitex.WithTransaction(r.db.Write, ctx, func(ctx context.Context, tx *sql.Tx) (int, error) {

		_, err := tx.Exec(delTagFTSSql, id)
		if err != nil {
			return -1, err
		}
		sql := "DELETE FROM tags WHERE id = ?;"
		_, err = tx.Exec(sql, id)
		if err != nil {
			return -1, err
		}
		_, err = tx.Exec(insTagFTSSql, id)
		if err != nil {
			return -1, err
		}
		return 1, nil
	})

	return err

}

func (r *SQLite) ListAllTags(ctx context.Context) ([]Tag, error) {
	query := "SELECT id, name FROM tags ORDER BY name ASC"
	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	rows, err := r.db.Read.QueryContext(ctxWTO, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query tags: %w", err)
	}
	defer rows.Close()

	ts := []Tag{}
	for rows.Next() {
		t := Tag{}
		err = rows.Scan(&t.ID, &t.Name)
		if err != nil {
			return nil, err
		}

		ts = append(ts, t)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tags: %w", err)
	}

	return ts, nil

}

func (r *SQLite) ListTags(ctx context.Context, limit, offset int) (PaginatedTags, error) {

	if limit <= 0 {
		limit = 10
	}

	if offset < 0 {
		offset = 0
	}

	query := `SELECT id, name FROM tags ORDER BY id ASC LIMIT ? OFFSET ?;`
	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	rows, err := r.db.Read.QueryContext(ctxWTO, query, limit+1, offset)
	if err != nil {
		return PaginatedTags{}, fmt.Errorf("failed to query tags: %w", err)
	}
	defer rows.Close()

	ts := make([]Tag, 0, limit)
	for rows.Next() {
		t := Tag{}
		err = rows.Scan(&t.ID, &t.Name)
		if err != nil {
			return PaginatedTags{}, err
		}

		ts = append(ts, t)
	}

	if err = rows.Err(); err != nil {
		return PaginatedTags{}, fmt.Errorf("error iterating tags: %w", err)
	}

	var nextOffset *int
	hasMore := len(ts) > limit
	if hasMore {
		ts = ts[:limit]
		next := offset + limit
		nextOffset = &next
	}

	return PaginatedTags{Tags: ts, Limit: limit, Offset: offset, HasMore: hasMore, NextOffset: nextOffset}, nil

}

func (r *SQLite) QueryTags(ctx context.Context, ids []int64) ([]Tag, error) {
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))

	for i, n := range ids {
		placeholders[i] = "?"
		args[i] = n
	}

	q := fmt.Sprintf(`
		SELECT id, name
		FROM tags
		WHERE id in (%s);
		`,
		strings.Join(placeholders, ","),
	)

	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	rows, err := r.db.Read.QueryContext(ctxWTO, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tags := []Tag{}
	for rows.Next() {
		var t Tag
		err = rows.Scan(&t.ID, &t.Name)
		if err != nil {
			return nil, err
		}

		tags = append(tags, t)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating sql rows: %w", err)
	}
	return tags, nil
}

// TAG FUNCTIONS
