package sqlite

import (
	"context"
	"fmt"

	"github.com/ASC521/communis/dbx/sqlitex"
	"github.com/ASC521/communis/models"
)

type notebookRepository struct {
	db  *sqlitex.SQLiteDB
	ctx context.Context
}

func NewNotebookRepository(db *sqlitex.SQLiteDB, ctx context.Context) *notebookRepository {
	return &notebookRepository{db: db, ctx: ctx}
}

func (r *notebookRepository) Create(n *models.Notebook) (int64, error) {

	res, err := sqlitex.Exec(r.db, r.ctx, "INSERT INTO notebooks (name) VALUES (?);", n.Name)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()

}

func (r *notebookRepository) FindById(id int64) (*models.Notebook, error) {
	sql := "SELECT id, name FROM notebooks WHERE id = ?;"
	row := sqlitex.QueryRow(r.db, r.ctx, sql, id)
	nb := models.Notebook{}
	err := row.Scan(&nb.Id, &nb.Name)
	if err != nil {
		return nil, err
	}
	return &nb, nil
}

func (r *notebookRepository) Update(n *models.Notebook) error {
	_, err := sqlitex.Exec(r.db, r.ctx, "UPDATE notebooks SET name = ? WHERE id = ?;", n.Name, n.Id)
	if err != nil {
		return err
	}

	return nil
}

func (r *notebookRepository) Delete(id int64) error {

	_, err := sqlitex.Exec(r.db, r.ctx, "DELETE FROM notebooks WHERE id = ?;", id)
	if err != nil {
		return err
	}
	return nil
}

func (r *notebookRepository) List(limit, offset int) (*models.PaginatedNotebooks, error) {
	if limit <= 0 {
		limit = 10
	}

	if offset < 0 {
		offset = 0
	}

	query := `SELECT id, name FROM notebooks ORDER BY id ASC LIMIT ? OFFSET ?;`
	rows, err := sqlitex.Query(r.db, r.ctx, query, limit+1, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query notebooks: %w", err)
	}
	defer rows.Close()

	nbs := make([]*models.Notebook, 0, limit)
	for rows.Next() {
		nb := &models.Notebook{}
		err = rows.Scan(&nb.Id, &nb.Name)
		if err != nil {
			return nil, err
		}

		nbs = append(nbs, nb)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating notebooks: %w", err)
	}

	var nextOffset *int
	hasMore := len(nbs) > limit
	if hasMore {
		nbs = nbs[:limit]
		next := offset + limit
		nextOffset = &next
	}

	return &models.PaginatedNotebooks{Notebooks: nbs, Limit: limit, Offset: offset, HasMore: hasMore, NextOffset: nextOffset}, nil
}
