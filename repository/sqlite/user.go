package sqlite

import (
	"context"
	"database/sql"

	"github.com/ASC521/communis/dbx/sqlitex"
	"github.com/ASC521/communis/models"
)

type indexDBRepository struct {
	db *sqlitex.SQLiteDB
}

func NewIndexDBRepository(db *sqlitex.SQLiteDB) *indexDBRepository {
	return &indexDBRepository{db: db}
}

func (r *indexDBRepository) DBVersionBefore(ctx context.Context, latestVer int) ([]models.NotesDBInfo, error) {
	sql := `SELECT id, db_path, db_version FROM users WHERE db_version < ? AND is_admin = 0;`
	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()
	rows, err := r.db.Read.QueryContext(ctxWTO, sql, latestVer)
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

func (r *indexDBRepository) UpdateDBVersion(ctx context.Context, userID int64, version int) error {
	updateStmt := `UPDATE users SET db_version = ? WHERE id = ?;`

	_, err := sqlitex.WithTransaction(r.db.Write, ctx, func(ctx context.Context, tx *sql.Tx) (int, error) {
		_, err := tx.Exec(updateStmt, version, userID)
		if err != nil {
			return -1, err
		}

		return int(userID), nil
	})

	return err
}

func (r *indexDBRepository) GetUserDB(ctx context.Context, userId int64) (models.NotesDBInfo, error) {
	q := `SELECT id, db_path, db_version FROM users WHERE id = ?;`
	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	dbInfo := models.NotesDBInfo{}
	row := r.db.Read.QueryRowContext(ctxWTO, q, userId)
	err := row.Scan(&dbInfo.UserId, &dbInfo.DBPath, &dbInfo.DBVersion)
	if err != nil {
		return models.NotesDBInfo{}, err
	}
	return dbInfo, nil
}
