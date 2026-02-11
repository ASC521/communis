package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/ASC521/communis/dbx/sqlitex"
	"github.com/ASC521/communis/models"
	"golang.org/x/crypto/bcrypt"
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

func (r *indexDBRepository) CreateUser(ctx context.Context, user models.User) (int64, error) {

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.PTPassword), 12)
	if err != nil {
		return 0, err
	}

	q := `INSERT INTO users (name, hashed_password, db_path, db_version, is_admin) VALUES (?, ?, ?, ?, ?);`
	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	return sqlitex.WithTransaction(r.db.Write, ctxWTO, func(ctx context.Context, tx *sql.Tx) (int64, error) {
		result, err := tx.ExecContext(ctx, q, user.Name, hashedPassword, user.DBPath, user.DBVersion, user.IsAdmin)
		if err != nil {
			return -1, err
		}

		return result.LastInsertId()
	})
}

func (r *indexDBRepository) AuthenticateUser(ctx context.Context, username, password string) (int64, error) {
	q := `SELECT id, hashed_password FROM users WHERE name = ?;`
	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	var id int64
	var hashedPassword []byte
	row := r.db.Read.QueryRowContext(ctxWTO, q, username)
	err := row.Scan(&id, &hashedPassword)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, models.ErrInvalidCredentials
		} else {
			return 0, err
		}

	}

	err = bcrypt.CompareHashAndPassword(hashedPassword, []byte(password))
	if err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return 0, models.ErrInvalidCredentials
		} else {
			return 0, err
		}

	}

	return id, nil
}
