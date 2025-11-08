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

	res, err := sqlitex.Exec(r.db, r.ctx, "INSERT INTO tags (name) VALUES (?);", t.Name)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()

}

func (r *tagRepository) FindById(id int64) (*models.Tag, error) {
	sql := "SELECT id, name FROM tags WHERE id = ?;"
	row := sqlitex.QueryRow(r.db, r.ctx, sql, id)
	t := models.Tag{}
	err := row.Scan(&t.Id, &t.Name)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *tagRepository) Update(t *models.Tag) error {
	_, err := sqlitex.Exec(r.db, r.ctx, "UPDATE tags SET name = ? WHERE id = ?;", t.Name, t.Id)
	if err != nil {
		return err
	}

	return nil
}

func (r *tagRepository) Delete(id int64) error {

	_, err := sqlitex.Exec(r.db, r.ctx, "DELETE FROM tags WHERE id = ?;", id)
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
	rows, err := sqlitex.Query(r.db, r.ctx, query, limit+1, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query notebooks: %w", err)
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
		return nil, fmt.Errorf("error iterating notebooks: %w", err)
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
