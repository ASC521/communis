package sqlite

import (
	"context"
	"fmt"

	"github.com/ASC521/communis/models"
)

func (r *NotesRepository) CreateSection(ctx context.Context, s models.Section) (int64, error) {

	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()
	res, err := r.db.Write.ExecContext(ctxWTO, "INSERT INTO sections (name) VALUES (?);", s.Name)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()

}

func (r *NotesRepository) FindSectionById(ctx context.Context, id int64) (models.Section, error) {
	sql := "SELECT id, name FROM sections WHERE id = ?;"
	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	nb := models.Section{}
	err := r.db.Read.QueryRowContext(ctxWTO, sql, id).Scan(&nb.ID, &nb.Name)
	if err != nil {
		return models.Section{}, err
	}
	return nb, nil
}

func (r *NotesRepository) FindSectionByName(ctx context.Context, name string) (models.Section, error) {
	sql := "SELECT id, name FROM sections WHERE name = ?;"
	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	nb := models.Section{}
	err := r.db.Read.QueryRowContext(ctxWTO, sql, name).Scan(&nb.ID, &nb.Name)
	if err != nil {
		return models.Section{}, err
	}
	return nb, nil
}

func (r *NotesRepository) UpdateSection(ctx context.Context, s models.Section) error {

	sql := "UPDATE sections SET name = ? WHERE id = ?;"
	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	_, err := r.db.Write.ExecContext(ctxWTO, sql, s.Name, s.ID)
	if err != nil {
		return err
	}

	return nil
}

func (r *NotesRepository) DeleteSection(ctx context.Context, id int64) error {

	sql := "DELETE FROM sections WHERE id = ?;"
	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	_, err := r.db.Write.ExecContext(ctxWTO, sql, id)
	if err != nil {
		return err
	}
	return nil
}

func (r *NotesRepository) ListAllSections(ctx context.Context) ([]models.Section, error) {
	query := "SELECT id, name FROM sections ORDER BY name ASC"
	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()
	rows, err := r.db.Read.QueryContext(ctxWTO, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query sections: %w", err)
	}
	defer rows.Close()

	var secs []models.Section
	for rows.Next() {
		sec := models.Section{}
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
