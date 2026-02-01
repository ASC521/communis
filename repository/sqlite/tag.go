package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/ASC521/communis/dbx/sqlitex"
	"github.com/ASC521/communis/models"
)

const delTagFTSSql = `INSERT INTO notes_details_fts(notes_details_fts, rowid, title, content, tags_txt)
		   SELECT 'delete', notes_details.id,  title, content, tags_txt
		   FROM notes_details, json_each(notes_details.tags_json)
		   WHERE json_extract(value, '$.id') = ?;`

const insTagFTSSql = `INSERT INTO notes_details_fts(rowid, title, content, tags_txt)
		   SELECT notes_details.id, notes_details.title, notes_details.content, notes_details.tags_txt
		   FROM notes_details, json_each(notes_details.tags_json)
		   WHERE json_extract(value, '$.id') = ?;`

type tagRepository struct {
	db  *sqlitex.SQLiteDB
	ctx context.Context
}

func NewTagRepository(db *sqlitex.SQLiteDB, ctx context.Context) *tagRepository {
	return &tagRepository{db: db, ctx: ctx}
}

func (r *tagRepository) Create(t *models.Tag) (int64, error) {

	sql := "INSERT INTO tags (name) VALUES (?);"
	ctxWTO, cancel := context.WithTimeout(r.ctx, r.db.QueryTimeout)
	defer cancel()

	res, err := r.db.Write.ExecContext(ctxWTO, sql, t.Name)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()

}

func (r *tagRepository) CheckMissing(names []string) ([]string, error) {
	valPlaceholders := make([]string, len(names))
	args := make([]any, len(names))
	for i, n := range names {
		valPlaceholders[i] = "(?)"
		args[i] = n
	}
	valuesClause := strings.Join(valPlaceholders, ",")

	q := fmt.Sprintf(`
		WITH input_values(name) AS (VALUES %s)
		SELECT input_values.name
		FROM input_values
		WHERE name NOT IN (SELECT name from tags);`,
		valuesClause,
	)

	ctxWTO, cancel := context.WithTimeout(r.ctx, r.db.QueryTimeout)
	defer cancel()

	rows, err := r.db.Read.QueryContext(ctxWTO, q, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query database for tags: %w", err)
	}
	defer rows.Close()

	missing := []string{}
	for rows.Next() {
		var t string
		err = rows.Scan(&t)
		if err != nil {
			return nil, err
		}
		missing = append(missing, t)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating missing tags: %w", err)
	}

	return missing, nil

}

func (r *tagRepository) FindById(id int64) (models.Tag, error) {
	sql := "SELECT id, name FROM tags WHERE id = ?;"
	ctxWTO, cancel := context.WithTimeout(r.ctx, r.db.QueryTimeout)
	defer cancel()

	t := models.Tag{}
	err := r.db.Read.QueryRowContext(ctxWTO, sql, id).Scan(&t.Id, &t.Name)
	if err != nil {
		return models.Tag{}, err
	}
	return t, nil
}

func (r *tagRepository) FindByName(name string) (models.Tag, error) {
	sql := "SELECT id, name FROM tags WHERE name = ?;"
	ctxWTO, cancel := context.WithTimeout(r.ctx, r.db.QueryTimeout)
	defer cancel()

	t := models.Tag{}
	err := r.db.Read.QueryRowContext(ctxWTO, sql, name).Scan(&t.Id, &t.Name)
	if err != nil {
		return models.Tag{}, err
	}
	return t, nil
}

func (r *tagRepository) Update(t models.Tag) error {

	_, err := sqlitex.WithTransaction(r.db.Write, r.ctx, func(ctx context.Context, tx *sql.Tx) (int, error) {

		_, err := tx.Exec(delTagFTSSql, t.Id)
		if err != nil {
			return -1, err
		}

		sql := "UPDATE tags SET name = ? WHERE id = ?;"
		_, err = tx.Exec(sql, t.Name, t.Id)
		if err != nil {
			return -1, err
		}

		_, err = tx.Exec(insTagFTSSql, t.Id)
		if err != nil {
			return -1, err
		}
		return 1, nil
	})
	return err
}

func (r *tagRepository) Delete(id int64) error {
	_, err := sqlitex.WithTransaction(r.db.Write, r.ctx, func(ctx context.Context, tx *sql.Tx) (int, error) {

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

func (r *tagRepository) ListAll() ([]models.Tag, error) {
	query := "SELECT id, name FROM tags ORDER BY name ASC"
	ctxWTO, cancel := context.WithTimeout(r.ctx, r.db.QueryTimeout)
	defer cancel()

	rows, err := r.db.Read.QueryContext(ctxWTO, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query tags: %w", err)
	}
	defer rows.Close()

	ts := []models.Tag{}
	for rows.Next() {
		t := models.Tag{}
		err = rows.Scan(&t.Id, &t.Name)
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

func (r *tagRepository) List(limit, offset int) (models.PaginatedTags, error) {

	if limit <= 0 {
		limit = 10
	}

	if offset < 0 {
		offset = 0
	}

	query := `SELECT id, name FROM tags ORDER BY id ASC LIMIT ? OFFSET ?;`
	ctxWTO, cancel := context.WithTimeout(r.ctx, r.db.QueryTimeout)
	defer cancel()

	rows, err := r.db.Read.QueryContext(ctxWTO, query, limit+1, offset)
	if err != nil {
		return models.PaginatedTags{}, fmt.Errorf("failed to query tags: %w", err)
	}
	defer rows.Close()

	ts := make([]models.Tag, 0, limit)
	for rows.Next() {
		t := models.Tag{}
		err = rows.Scan(&t.Id, &t.Name)
		if err != nil {
			return models.PaginatedTags{}, err
		}

		ts = append(ts, t)
	}

	if err = rows.Err(); err != nil {
		return models.PaginatedTags{}, fmt.Errorf("error iterating tags: %w", err)
	}

	var nextOffset *int
	hasMore := len(ts) > limit
	if hasMore {
		ts = ts[:limit]
		next := offset + limit
		nextOffset = &next
	}

	return models.PaginatedTags{Tags: ts, Limit: limit, Offset: offset, HasMore: hasMore, NextOffset: nextOffset}, nil

}

func (r *tagRepository) Query(ids []int64) ([]models.Tag, error) {
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

	ctxWTO, cancel := context.WithTimeout(r.ctx, r.db.QueryTimeout)
	defer cancel()

	rows, err := r.db.Read.QueryContext(ctxWTO, q, args...)
	if err != nil {
		return nil, err
	}

	tags := []models.Tag{}
	for rows.Next() {
		var t models.Tag
		err = rows.Scan(&t.Id, &t.Name)
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
