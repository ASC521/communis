package sqlite

import (
	"context"
	"fmt"

	"github.com/ASC521/communis/dbx/sqlitex"
	"github.com/ASC521/communis/models"
)

type sectionRepository struct {
	db  *sqlitex.SQLiteDB
	ctx context.Context
}

func NewSectionRepository(db *sqlitex.SQLiteDB, ctx context.Context) *sectionRepository {
	return &sectionRepository{db: db, ctx: ctx}
}

func (r *sectionRepository) Create(s *models.Section) (int64, error) {

	ctxWTO, cancel := context.WithTimeout(r.ctx, r.db.Opts.QueryTimeout)
	defer cancel()
	res, err := r.db.Write.ExecContext(ctxWTO, "INSERT INTO sections (name) VALUES (?);", s.Name)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()

}

func (r *sectionRepository) FindById(id int64) (*models.Section, error) {
	sql := "SELECT id, name FROM sections WHERE id = ?;"
	ctxWTO, cancel := context.WithTimeout(r.ctx, r.db.Opts.QueryTimeout)
	defer cancel()

	nb := models.Section{}
	err := r.db.Read.QueryRowContext(ctxWTO, sql, id).Scan(&nb.Id, &nb.Name)
	if err != nil {
		return nil, err
	}
	return &nb, nil
}

func (r *sectionRepository) Update(s *models.Section) error {

	sql := "UPDATE sections SET name = ? WHERE id = ?;"
	ctxWTO, cancel := context.WithTimeout(r.ctx, r.db.Opts.QueryTimeout)
	defer cancel()

	_, err := r.db.Write.ExecContext(ctxWTO, sql, s.Name, s.Id)
	if err != nil {
		return err
	}

	return nil
}

func (r *sectionRepository) Delete(id int64) error {

	sql := "DELETE FROM sections WHERE id = ?;"
	ctxWTO, cancel := context.WithTimeout(r.ctx, r.db.Opts.QueryTimeout)
	defer cancel()

	_, err := r.db.Write.ExecContext(ctxWTO, sql, id)
	if err != nil {
		return err
	}
	return nil
}

func (r *sectionRepository) List(limit, offset int) (*models.PaginatedSections, error) {
	if limit <= 0 {
		limit = 10
	}

	if offset < 0 {
		offset = 0
	}

	query := `SELECT id, name FROM sections ORDER BY id ASC LIMIT ? OFFSET ?;`
	ctxWTO, cancel := context.WithTimeout(r.ctx, r.db.Opts.QueryTimeout)
	defer cancel()
	rows, err := r.db.Read.QueryContext(ctxWTO, query, limit+1, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query notebooks: %w", err)
	}
	defer rows.Close()

	nbs := make([]*models.Section, 0, limit)
	for rows.Next() {
		nb := &models.Section{}
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

	return &models.PaginatedSections{Sections: nbs, Limit: limit, Offset: offset, HasMore: hasMore, NextOffset: nextOffset}, nil
}
