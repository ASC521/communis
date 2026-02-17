package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

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

func (r *indexDBRepository) DBVersionBefore(ctx context.Context, latestVer int) ([]models.UserDatabase, error) {
	sql := `SELECT user_databases.id, user_databases.db_path, user_databases.db_version
		FROM user_databases
		INNER JOIN users ON user_databases.user_id = users.id
		WHERE db_version < ? AND users.is_admin = 0;`
	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()
	rows, err := r.db.Read.QueryContext(ctxWTO, sql, latestVer)
	if err != nil {
		return nil, err
	}

	userDBs := []models.UserDatabase{}
	for rows.Next() {
		userDB := models.UserDatabase{}
		err = rows.Scan(&userDB.UserId, &userDB.Path, &userDB.Version)
		if err != nil {
			return nil, err
		}
		userDBs = append(userDBs, userDB)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return userDBs, nil
}

func (r *indexDBRepository) UpdateDBVersion(ctx context.Context, userID int64, version int) error {
	updateStmt := `UPDATE user_databases SET db_version = ? WHERE user_id = ?;`

	_, err := sqlitex.WithTransaction(r.db.Write, ctx, func(ctx context.Context, tx *sql.Tx) (int, error) {
		_, err := tx.Exec(updateStmt, version, userID)
		if err != nil {
			return -1, err
		}

		return int(userID), nil
	})

	return err
}

func (r *indexDBRepository) GetUserDB(ctx context.Context, userId int64) (models.UserDatabase, error) {
	q := `SELECT id, db_path, db_version FROM user_databases WHERE user_id = ?;`
	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	userDB := models.UserDatabase{UserId: userId}
	row := r.db.Read.QueryRowContext(ctxWTO, q, userId)
	err := row.Scan(&userDB.Id, &userDB.Path, &userDB.Version)
	if err != nil {
		return models.UserDatabase{}, err
	}
	return userDB, nil
}

func (r *indexDBRepository) CreateAdminUser(ctx context.Context, username, password string) (int64, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return 0, err
	}

	stmt := `INSERT INTO users (name, hashed_password, is_admin) VALUES (?, ?, ?)`
	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	res, err := r.db.Write.ExecContext(ctxWTO, stmt, username, hashedPassword, true)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *indexDBRepository) CreateUserAndDB(ctx context.Context, userName, password string, isAdmin bool, dbPath string) (int64, error) {

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return 0, err
	}

	userStmt := `INSERT INTO users (name, hashed_password, is_admin) VALUES (?, ?, ?);`
	dbStmt := `INSERT INTO user_databases (user_id, db_path, db_version) VALUES (?, ?, ?)`
	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	return sqlitex.WithTransaction(r.db.Write, ctxWTO, func(ctx context.Context, tx *sql.Tx) (int64, error) {
		result, err := tx.ExecContext(ctx, userStmt, userName, hashedPassword, isAdmin)
		if err != nil {
			return -1, err
		}
		userId, err := result.LastInsertId()
		if err != nil {
			return 0, err
		}
		_, err = tx.ExecContext(ctx, dbStmt, userId, dbPath, 0)
		if err != nil {
			return 0, err
		}

		return userId, err
	})
}

func (r *indexDBRepository) AuthenticateUser(ctx context.Context, username, password string) (models.User, error) {
	q := `SELECT id, name, hashed_password, is_admin, created_at_utc, last_login_utc FROM users WHERE name = ?;`
	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	user := models.User{}
	var hashedPassword []byte
	var createdStr, lastLoginStr sql.NullString
	row := r.db.Read.QueryRowContext(ctxWTO, q, username)
	err := row.Scan(&user.Id, &user.Name, &hashedPassword, &user.IsAdmin, &createdStr, &lastLoginStr)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.User{}, models.ErrInvalidCredentials
		} else {
			return models.User{}, err
		}
	}

	err = bcrypt.CompareHashAndPassword(hashedPassword, []byte(password))
	if err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return models.User{}, models.ErrInvalidCredentials
		} else {
			return models.User{}, err
		}
	}

	if createdStr.Valid {
		createdAt, err := time.ParseInLocation(sqliteTimeFmt, createdStr.String, time.UTC)
		if err != nil {
			return models.User{}, err
		}
		user.CreatedAtUTC = createdAt
	}
	if lastLoginStr.Valid {
		lastLogin, err := time.ParseInLocation(sqliteTimeFmt, lastLoginStr.String, time.UTC)
		if err != nil {
			return models.User{}, err
		}
		user.LastLoginUTC = lastLogin
	}

	return user, nil
}

func (r *indexDBRepository) IsAdminUser(ctx context.Context, userId int64) (bool, error) {
	q := `SELECT is_admin FROM users WHERE id = ?;`
	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	isAdmin := false
	err := r.db.Read.QueryRowContext(ctxWTO, q, userId).Scan(&isAdmin)
	if err != nil {
		return false, err
	}
	return isAdmin, nil
}

func (r *indexDBRepository) GetUser(ctx context.Context, id int64) (models.User, error) {
	q := `SELECT id, name, is_admin, created_at_utc, last_login_utc FROM users WHERE id = ?;`
	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	user := models.User{}
	var createdStr, lastLoginStr sql.NullString
	row := r.db.Read.QueryRowContext(ctxWTO, q, id)
	err := row.Scan(&user.Id, &user.Name, &user.IsAdmin, &createdStr, &lastLoginStr)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.User{}, models.ErrInvalidCredentials
		} else {
			return models.User{}, err
		}
	}

	if createdStr.Valid {
		createdAt, err := time.ParseInLocation(sqliteTimeFmt, createdStr.String, time.UTC)
		if err != nil {
			return models.User{}, err
		}
		user.CreatedAtUTC = createdAt
	}
	if lastLoginStr.Valid {
		lastLogin, err := time.ParseInLocation(sqliteTimeFmt, lastLoginStr.String, time.UTC)
		if err != nil {
			return models.User{}, err
		}
		user.LastLoginUTC = lastLogin
	}

	return user, nil

}

func (r *indexDBRepository) UpdateUser(ctx context.Context, id int64, name string, isAdmin bool) error {
	stmt := `UPDATE users SET name = ?, is_admin = ? WHERE id = ?;`

	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	_, err := r.db.Write.ExecContext(ctxWTO, stmt, name, isAdmin, id)
	return err
}

func (r *indexDBRepository) UpdateUserLastLoginToNow(ctx context.Context, id int64) error {
	stmt := `UPDATE users SET last_login_utc = datetime('now') WHERE id = ?;`

	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	_, err := r.db.Write.ExecContext(ctxWTO, stmt, id)
	return err
}

func (r *indexDBRepository) ListUsers(ctx context.Context) ([]models.User, error) {
	q := `SELECT id, name, is_admin, created_at_utc, last_login_utc FROM users`
	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	rows, err := r.db.Read.QueryContext(ctxWTO, q)
	if err != nil {
		return nil, err
	}

	users := []models.User{}
	for rows.Next() {
		user := models.User{}
		var createdStr, lastLoginStr sql.NullString
		err := rows.Scan(&user.Id, &user.Name, &user.IsAdmin, &createdStr, &lastLoginStr)
		if err != nil {
			return nil, err
		}

		if createdStr.Valid {
			createdAt, err := time.ParseInLocation(sqliteTimeFmt, createdStr.String, time.UTC)
			if err != nil {
				return nil, err
			}
			user.CreatedAtUTC = createdAt
		}
		if lastLoginStr.Valid {
			lastLogin, err := time.ParseInLocation(sqliteTimeFmt, lastLoginStr.String, time.UTC)
			if err != nil {
				return nil, err
			}
			user.LastLoginUTC = lastLogin
		}

		users = append(users, user)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}

func (r *indexDBRepository) DeleteUser(ctx context.Context, id int64) error {
	s := `DELETE FROM users WHERE id = ?;`
	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	_, err := r.db.Write.ExecContext(ctxWTO, s, id)
	return err
}
