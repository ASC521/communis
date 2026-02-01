package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/ASC521/communis/dbx/sqlitex"
	"github.com/ASC521/communis/models"
)

type indexDBRepository struct {
	ctx          context.Context
	queryTimeout time.Duration
}

func NewIndexDBRepository(ctx context.Context, timeout time.Duration) *indexDBRepository {
	return &indexDBRepository{ctx: ctx, queryTimeout: timeout}
}

func (r *indexDBRepository) DBVersionBefore(db *sql.DB, latestVer int) ([]models.NotesDBInfo, error) {
	sql := `SELECT id, db_path, db_version FROM users WHERE db_version < ? AND is_admin = 0;`
	ctxWTO, cancel := context.WithTimeout(r.ctx, r.queryTimeout)
	defer cancel()
	rows, err := db.QueryContext(ctxWTO, sql, latestVer)
	if err != nil {
		return nil, err
	}

	dbInfos := []models.NotesDBInfo{}
	for rows.Next() {
		dbInfo := models.NotesDBInfo{}
		err = rows.Scan(&dbInfo.UserId, &dbInfo.DBPath, &dbInfo.DBVersion)
		if err != nil {
			return nil, err
		}
		dbInfos = append(dbInfos, dbInfo)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return dbInfos, nil
}

func (r *indexDBRepository) UpdateDBVersion(db *sql.DB, userID int64, version int) error {
	updateStmt := `UPDATE users SET db_version = ? WHERE id = ?;`

	_, err := sqlitex.WithTransaction(db, r.ctx, func(ctx context.Context, tx *sql.Tx) (int, error) {
		_, err := tx.Exec(updateStmt, version, userID)
		if err != nil {
			return -1, err
		}

		return int(userID), nil
	})

	return err

}
