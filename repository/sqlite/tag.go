package sqlite

import (
	"context"
	"fmt"

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
	ctxWTO, cancel := context.WithTimeout(r.ctx, r.db.Opts.QueryTimeout)
	defer cancel()

	res, err := r.db.Write.ExecContext(ctxWTO, sql, t.Name)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()

}

func (r *tagRepository) FindById(id int64) (*models.Tag, error) {
	sql := "SELECT id, name FROM tags WHERE id = ?;"
	ctxWTO, cancel := context.WithTimeout(r.ctx, r.db.Opts.QueryTimeout)
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
	ctxWTO, cancel := context.WithTimeout(r.ctx, r.db.Opts.QueryTimeout)
	defer cancel()

	_, err := r.db.Write.ExecContext(ctxWTO, sql, t.Name, t.Id)
	if err != nil {
		return err
	}

	return nil
}

func (r *tagRepository) Delete(id int64) error {

	sql := "DELETE FROM tags WHERE id = ?;"
	ctxWTO, cancel := context.WithTimeout(r.ctx, r.db.Opts.QueryTimeout)
	defer cancel()

	_, err := r.db.Write.ExecContext(ctxWTO, sql, id)
	if err != nil {
		return err
	}
	return nil

}

func (r *tagRepository) List(limit, offset int) (*models.PaginatedTags, error) {

	if limit <= 0 {
		limit = 10
	}

	if offset < 0 {
		offset = 0
	}

	query := `SELECT id, name FROM tags ORDER BY id ASC LIMIT ? OFFSET ?;`
	ctxWTO, cancel := context.WithTimeout(r.ctx, r.db.Opts.QueryTimeout)
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
