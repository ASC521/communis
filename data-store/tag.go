package datastore

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/ASC521/communis/dbx/sqlitex"
)

const delTagFTSSql = `INSERT INTO notes_details_fts(notes_details_fts, rowid, title, content, tags_txt)
		   SELECT 'delete', notes_details.id,  title, content, tags_txt
		   FROM notes_details, json_each(notes_details.tags_json)
		   WHERE json_extract(value, '$.id') = ?;`

const insTagFTSSql = `INSERT INTO notes_details_fts(rowid, title, content, tags_txt)
		   SELECT notes_details.id, notes_details.title, notes_details.content, notes_details.tags_txt
		   FROM notes_details, json_each(notes_details.tags_json)
		   WHERE json_extract(value, '$.id') = ?;`

func (r *NotesRepository) CreateTag(ctx context.Context, t Tag) (int64, error) {

	sql := "INSERT INTO tags (name) VALUES (?);"
	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	res, err := r.db.Write.ExecContext(ctxWTO, sql, t.Name)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()

}

func (r *NotesRepository) FindTagById(ctx context.Context, id int64) (Tag, error) {
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

func (r *NotesRepository) FindTagByName(ctx context.Context, name string) (Tag, error) {
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

func (r *NotesRepository) UpdateTag(ctx context.Context, t Tag) error {

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

func (r *NotesRepository) DeleteTag(ctx context.Context, id int64) error {
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

func (r *NotesRepository) ListAllTags(ctx context.Context) ([]Tag, error) {
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

func (r *NotesRepository) ListTags(ctx context.Context, limit, offset int) (PaginatedTags, error) {

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

func (r *NotesRepository) QueryTags(ctx context.Context, ids []int64) ([]Tag, error) {
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
