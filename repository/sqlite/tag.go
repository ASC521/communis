package sqlite

import (
	"context"
	"fmt"
	"strings"

	"github.com/ASC521/communis/dbx/sqlitex"
	"github.com/ASC521/communis/models"
)

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

func (r *tagRepository) FindById(id int64) (*models.Tag, error) {
	sql := "SELECT id, name FROM tags WHERE id = ?;"
	ctxWTO, cancel := context.WithTimeout(r.ctx, r.db.QueryTimeout)
	defer cancel()

	t := models.Tag{}
	err := r.db.Read.QueryRowContext(ctxWTO, sql, id).Scan(&t.Id, &t.Name)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *tagRepository) Update(t *models.Tag) error {

	sql := "UPDATE tags SET name = ? WHERE id = ?;"
	ctxWTO, cancel := context.WithTimeout(r.ctx, r.db.QueryTimeout)
	defer cancel()

	_, err := r.db.Write.ExecContext(ctxWTO, sql, t.Name, t.Id)
	if err != nil {
		return err
	}

	return nil
}

func (r *tagRepository) Delete(id int64) error {

	sql := "DELETE FROM tags WHERE id = ?;"
	ctxWTO, cancel := context.WithTimeout(r.ctx, r.db.QueryTimeout)
	defer cancel()

	_, err := r.db.Write.ExecContext(ctxWTO, sql, id)
	if err != nil {
		return err
	}
	return nil

}

func (r *tagRepository) ListAll() ([]*models.Tag, error) {
	query := "SELECT id, name FROM tags ORDER BY id ASC"
	ctxWTO, cancel := context.WithTimeout(r.ctx, r.db.QueryTimeout)
	defer cancel()

	rows, err := r.db.Read.QueryContext(ctxWTO, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query tags: %w", err)
	}
	defer rows.Close()

	ts := []*models.Tag{}
	for rows.Next() {
		t := &models.Tag{}
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

func (r *tagRepository) List(limit, offset int) (*models.PaginatedTags, error) {

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
		return nil, fmt.Errorf("failed to query tags: %w", err)
	}
	defer rows.Close()

	ts := make([]*models.Tag, 0, limit)
	for rows.Next() {
		t := &models.Tag{}
		err = rows.Scan(&t.Id, &t.Name)
		if err != nil {
			return nil, err
		}

		ts = append(ts, t)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tags: %w", err)
	}

	var nextOffset *int
	hasMore := len(ts) > limit
	if hasMore {
		ts = ts[:limit]
		next := offset + limit
		nextOffset = &next
	}

	return &models.PaginatedTags{Tags: ts, Limit: limit, Offset: offset, HasMore: hasMore, NextOffset: nextOffset}, nil

}

func (r *tagRepository) Query(names []string) (found []models.Tag, missing []string, err error) {
	placeholders := make([]string, len(names))
	args := make([]any, len(names))

	for i, n := range names {
		placeholders[i] = "?"
		args[i] = n
	}

	q := fmt.Sprintf(`
		SELECT id, name
		FROM tags
		WHERE name in (%s);
		`,
		strings.Join(placeholders, ","),
	)

	ctxWTO, cancel := context.WithTimeout(r.ctx, r.db.QueryTimeout)
	defer cancel()

	rows, err := r.db.Read.QueryContext(ctxWTO, q, args...)
	if err != nil {
		return nil, nil, err
	}

	fset := map[string]bool{}
	for rows.Next() {
		var t models.Tag
		err = rows.Scan(&t.Id, &t.Name)
		if err != nil {
			return nil, nil, err
		}

		fset[t.Name] = true

		found = append(found, t)
	}

	if err = rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("failed iterating sql rows: %w", err)
	}

	for _, n := range names {
		_, exists := fset[n]
		if !exists {
			missing = append(missing, n)
		}
	}

	return found, missing, nil
}
