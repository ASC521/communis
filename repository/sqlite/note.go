package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/ASC521/communis/dbx/sqlitex"
	"github.com/ASC521/communis/models"
)

const sqliteTimeFmt = "2006-01-02 15:04:05"

type noteRepository struct {
	db  *sqlitex.SQLiteDB
	ctx context.Context
}

func NewNoteRepository(db *sqlitex.SQLiteDB, ctx context.Context) *noteRepository {
	return &noteRepository{db: db, ctx: ctx}
}

func (r *noteRepository) Create(n *models.Note) (int64, error) {
	return sqlitex.WithTransaction(r.db, r.ctx, func(ctx context.Context, tx *sql.Tx) (int64, error) {
		res, err := tx.Exec("INSERT INTO notes (title, content, section) VALUES (?, ?, ?);", n.Title, n.Content, n.Section.Id)
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

		for _, t := range n.Tags {
			_, err = tx.Exec("INSERT INTO notes_tags (note_id, tag_id) VALUES (?, ?);", nid, t.Id)
			if err != nil {
				return 0, fmt.Errorf("failed to insert note tag mapping for note %v tag %v: %w", nid, t.Id, err)
			}
		}

		return nid, nil

	})
}

func (r *noteRepository) Exists(title string) (bool, error) {
	q := `SELECT id FROM notes WHERE title = ?;`
	ctxWTO, cancel := context.WithTimeout(r.ctx, r.db.QueryTimeout)
	defer cancel()

	var id string
	err := r.db.Read.QueryRowContext(ctxWTO, q, title).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func (r *noteRepository) FindById(id int64) (*models.Note, error) {
	q := `
     SELECT id, section_id, section_name, title, content, created_at_utc, last_updated_at_utc, tags
     FROM notes_details
     WHERE id = ?;`
	ctxWTO, cancel := context.WithTimeout(r.ctx, r.db.QueryTimeout)
	defer cancel()

	var n models.Note
	var tagJSON, createdStr, updatedStr string
	err := r.db.Read.QueryRowContext(ctxWTO, q, id).Scan(&n.Id, &n.Section.Id, &n.Section.Name, &n.Title, &n.Content, &createdStr, &updatedStr, &tagJSON)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to scan row for note: %w", err)
	}

	err = json.Unmarshal([]byte(tagJSON), &n.Tags)
	if err != nil {
		return nil, fmt.Errorf("failed to parse tags json: %w", err)
	}

	created, err := time.ParseInLocation(sqliteTimeFmt, createdStr, time.UTC)
	if err != nil {
		return nil, fmt.Errorf("failed to parse created at time: %w", err)
	}
	n.CreatedAt = created

	updated, err := time.ParseInLocation(sqliteTimeFmt, updatedStr, time.UTC)
	if err != nil {
		return nil, fmt.Errorf("failed to parse updated at time: %w", err)
	}
	n.LastUpdatedAt = updated

	return &n, nil
}

func (r *noteRepository) Update(n *models.Note) error {
	_, err := sqlitex.WithTransaction(r.db, r.ctx, func(ctx context.Context, tx *sql.Tx) (int, error) {
		s := `UPDATE notes SET title = ?, content = ?, section = ?, last_updated_at_utc = datetime('now') WHERE id = ?`
		_, err := tx.Exec(s, n.Title, n.Content, n.Section.Id, n.Id)
		if err != nil {
			return -1, fmt.Errorf("failed to update note %v: %w", n.Id, err)
		}

		ds := `DELETE FROM notes_tags WHERE note_id = ?;`
		_, err = tx.Exec(ds, n.Id)
		if err != nil {
			return -1, fmt.Errorf("failed to remove tags associated with note: %w", err)
		}
		ts := `INSERT INTO notes_tags (note_id, tag_id) VALUES (?, ?);`
		for _, tag := range n.Tags {
			_, err := tx.Exec(ts, n.Id, tag.Id)
			if err != nil {
				return -1, fmt.Errorf("failed to upsert tag %v: %w", tag.Id, err)
			}
		}

		return 1, nil

	})

	return err
}

func (r *noteRepository) Delete(id int64) error {
	s := `DELETE FROM notes WHERE id = ?;`
	ctxWTO, cancel := context.WithTimeout(r.ctx, r.db.QueryTimeout)
	defer cancel()

	_, err := r.db.Write.ExecContext(ctxWTO, s, id)
	return err
}

func (r *noteRepository) List(limit, offset int) (*models.PaginatedNotes, error) {

	if limit <= 0 {
		limit = 10
	}

	if offset < 0 {
		offset = 0
	}

	query := "SELECT id, section_id, section_name, title, content, created_at_utc, last_updated_at_utc, tags FROM notes_details ORDER BY id ASC LIMIT ? OFFSET ?;"
	ctxWTO, cancel := context.WithTimeout(r.ctx, r.db.QueryTimeout)
	defer cancel()

	rows, err := r.db.Read.QueryContext(ctxWTO, query, limit+1, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query notes: %w", err)
	}
	defer rows.Close()

	ns := make([]*models.Note, 0, limit)
	for rows.Next() {
		n := &models.Note{}
		var tagJSON, createdStr, updatedStr string
		err = rows.Scan(&n.Id, &n.Section.Id, &n.Section.Name, &n.Title, &n.Content, &createdStr, &updatedStr, &tagJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to scan rows for note: %w", err)
		}

		err = json.Unmarshal([]byte(tagJSON), &n.Tags)
		if err != nil {
			return nil, fmt.Errorf("failed to parse tags json: %w", err)
		}

		created, err := time.ParseInLocation(sqliteTimeFmt, createdStr, time.UTC)
		if err != nil {
			return nil, fmt.Errorf("failed to parse created at time: %w", err)
		}
		n.CreatedAt = created

		updated, err := time.ParseInLocation(sqliteTimeFmt, updatedStr, time.UTC)
		if err != nil {
			return nil, fmt.Errorf("failed to parse updated at time: %w", err)
		}
		n.LastUpdatedAt = updated

		ns = append(ns, n)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating notes: %w", err)
	}

	var nextOffset *int
	hasMore := len(ns) > limit
	if hasMore {
		ns = ns[:limit]
		next := offset + limit
		nextOffset = &next
	}

	return &models.PaginatedNotes{
		Notes:      ns,
		Limit:      limit,
		Offset:     offset,
		HasMore:    hasMore,
		NextOffset: nextOffset,
	}, nil

}
