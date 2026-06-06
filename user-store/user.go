package userstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/ASC521/communis/dbx/sqlitex"
	"golang.org/x/crypto/bcrypt"
)

const (
	userDBDir     = "user-databases"
	sqliteTimeFmt = "2006-01-02 15:04:05"
)

type IndexDBRepository struct {
	db *sqlitex.SQLiteDB
}

func NewIndexDBRepository(db *sqlitex.SQLiteDB) *IndexDBRepository {
	return &IndexDBRepository{db: db}
}

func (r *IndexDBRepository) DBVersionBefore(ctx context.Context, latestVer int) ([]UserDatabase, error) {
	sql := `SELECT user_databases.id, user_databases.user_id, user_databases.db_path, user_databases.db_version
		FROM user_databases
		INNER JOIN users ON user_databases.user_id = users.id
		WHERE db_version < ? AND users.is_admin = 0;`
	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()
	rows, err := r.db.Read.QueryContext(ctxWTO, sql, latestVer)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	userDBs := []UserDatabase{}
	for rows.Next() {
		userDB := UserDatabase{}
		err = rows.Scan(&userDB.ID, &userDB.UserID, &userDB.Path, &userDB.Version)
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

func (r *IndexDBRepository) UpdateDBVersion(ctx context.Context, userID int64, version int) error {
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

func (r *IndexDBRepository) GetUserDB(ctx context.Context, userID int64) (UserDatabase, error) {
	q := `SELECT id, db_path, db_version FROM user_databases WHERE user_id = ?;`
	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	userDB := UserDatabase{UserID: userID}
	row := r.db.Read.QueryRowContext(ctxWTO, q, userID)
	err := row.Scan(&userDB.ID, &userDB.Path, &userDB.Version)
	if err != nil {
		return UserDatabase{}, err
	}
	return userDB, nil
}

func (r *IndexDBRepository) CreateAdminUser(ctx context.Context, username, password string) (int64, error) {
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

func (r *IndexDBRepository) CreateUserAndDB(ctx context.Context, userName, password string) (int64, error) {

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return 0, err
	}

	userStmt := `INSERT INTO users (name, hashed_password, is_admin) VALUES (?, ?, ?);`
	dbStmt := `INSERT INTO user_databases (user_id, db_path, db_version) VALUES (?, ?, ?)`
	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	return sqlitex.WithTransaction(r.db.Write, ctxWTO, func(ctx context.Context, tx *sql.Tx) (int64, error) {
		result, err := tx.ExecContext(ctx, userStmt, userName, hashedPassword, false)
		if err != nil {
			return -1, err
		}
		userID, err := result.LastInsertId()
		if err != nil {
			return 0, err
		}
		dbPath := filepath.Join(userDBDir, fmt.Sprintf("%v.db", userID))
		_, err = tx.ExecContext(ctx, dbStmt, userID, dbPath, 0)
		if err != nil {
			return 0, err
		}

		return userID, err
	})
}

func (r *IndexDBRepository) AuthenticateUser(ctx context.Context, username, password string) (User, error) {
	q := `SELECT id, name, hashed_password, is_admin, created_at_utc, last_login_utc FROM users WHERE name = ?;`
	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	user := User{}
	var hashedPassword []byte
	var createdStr, lastLoginStr sql.NullString
	row := r.db.Read.QueryRowContext(ctxWTO, q, username)
	err := row.Scan(&user.ID, &user.Name, &hashedPassword, &user.IsAdmin, &createdStr, &lastLoginStr)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrInvalidCredentials
		} else {
			return User{}, err
		}
	}

	err = bcrypt.CompareHashAndPassword(hashedPassword, []byte(password))
	if err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return User{}, ErrInvalidCredentials
		} else {
			return User{}, err
		}
	}

	if createdStr.Valid {
		createdAt, err := time.ParseInLocation(sqliteTimeFmt, createdStr.String, time.UTC)
		if err != nil {
			return User{}, err
		}
		user.CreatedAtUTC = createdAt
	}
	if lastLoginStr.Valid {
		lastLogin, err := time.ParseInLocation(sqliteTimeFmt, lastLoginStr.String, time.UTC)
		if err != nil {
			return User{}, err
		}
		user.LastLoginUTC = lastLogin
	}

	return user, nil
}

func (r *IndexDBRepository) IsAdminUser(ctx context.Context, userID int64) (bool, error) {
	q := `SELECT is_admin FROM users WHERE id = ?;`
	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	isAdmin := false
	err := r.db.Read.QueryRowContext(ctxWTO, q, userID).Scan(&isAdmin)
	if err != nil {
		return false, err
	}
	return isAdmin, nil
}

func (r *IndexDBRepository) GetUser(ctx context.Context, id int64) (User, error) {
	q := `SELECT id, name, is_admin, created_at_utc, last_login_utc, theme FROM users WHERE id = ?;`
	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	row := r.db.Read.QueryRowContext(ctxWTO, q, id)
	user := User{}
	var createdStr, lastLoginStr sql.NullString
	err := row.Scan(&user.ID, &user.Name, &user.IsAdmin, &createdStr, &lastLoginStr, &user.Theme)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrInvalidCredentials
		} else {
			return User{}, err
		}
	}

	if createdStr.Valid {
		createdAt, err := time.ParseInLocation(sqliteTimeFmt, createdStr.String, time.UTC)
		if err != nil {
			return User{}, err
		}
		user.CreatedAtUTC = createdAt
	}
	if lastLoginStr.Valid {
		lastLogin, err := time.ParseInLocation(sqliteTimeFmt, lastLoginStr.String, time.UTC)
		if err != nil {
			return User{}, err
		}
		user.LastLoginUTC = lastLogin
	}

	return user, nil

}

func (r *IndexDBRepository) UpdateUser(ctx context.Context, id int64, name string) (User, error) {
	stmt := `UPDATE users SET name = ? WHERE id = ?
		RETURNING id, name, is_admin, created_at_utc, last_login_utc, theme;`

	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	row := r.db.Write.QueryRowContext(ctxWTO, stmt, name, id)
	user := User{}
	var createdStr, lastLoginStr sql.NullString
	err := row.Scan(&user.ID, &user.Name, &user.IsAdmin, &createdStr, &lastLoginStr, &user.Theme)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrInvalidCredentials
		} else {
			return User{}, err
		}
	}

	if createdStr.Valid {
		createdAt, err := time.ParseInLocation(sqliteTimeFmt, createdStr.String, time.UTC)
		if err != nil {
			return User{}, err
		}
		user.CreatedAtUTC = createdAt
	}
	if lastLoginStr.Valid {
		lastLogin, err := time.ParseInLocation(sqliteTimeFmt, lastLoginStr.String, time.UTC)
		if err != nil {
			return User{}, err
		}
		user.LastLoginUTC = lastLogin
	}

	return user, nil
}

func (r *IndexDBRepository) UpdateUserPassword(ctx context.Context, id int64, password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return err
	}

	stmt := `UPDATE users SET hashed_password = ? WHERE id = ?;`
	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	_, err = r.db.Write.ExecContext(ctxWTO, stmt, hashedPassword, id)
	return err
}

func (r *IndexDBRepository) UpdateUserLastLoginToNow(ctx context.Context, id int64) error {
	stmt := `UPDATE users SET last_login_utc = datetime('now') WHERE id = ?;`

	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	_, err := r.db.Write.ExecContext(ctxWTO, stmt, id)
	return err
}

func (r *IndexDBRepository) ListUsers(ctx context.Context) ([]User, error) {
	q := `SELECT id, name, is_admin, created_at_utc, last_login_utc FROM users`
	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	rows, err := r.db.Read.QueryContext(ctxWTO, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []User{}
	for rows.Next() {
		user := User{}
		var createdStr, lastLoginStr sql.NullString
		err := rows.Scan(&user.ID, &user.Name, &user.IsAdmin, &createdStr, &lastLoginStr)
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

func (r *IndexDBRepository) DeleteUser(ctx context.Context, id int64) error {
	s := `DELETE FROM users WHERE id = ?;`
	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	_, err := r.db.Write.ExecContext(ctxWTO, s, id)
	return err
}

func (r *IndexDBRepository) UpdateUserTheme(ctx context.Context, id int64, theme string) error {
	s := `UPDATE users SET theme = ? WHERE id =?;`
	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	_, err := r.db.Write.ExecContext(ctxWTO, s, theme, id)
	return err
}

func (r *IndexDBRepository) NameExists(ctx context.Context, name string) (bool, error) {
	q := `SELECT EXISTS(SELECT 1 FROM users WHERE name = ?);`
	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	var exists bool
	row := r.db.Read.QueryRowContext(ctxWTO, q, name)
	err := row.Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil

}

func (r *IndexDBRepository) InitialSetupNeeded(ctx context.Context) (bool, error) {
	q := `SELECT EXISTS(SELECT 1 FROM users WHERE is_admin = true)`
	ctxWTO, cancel := context.WithTimeout(ctx, r.db.QueryTimeout)
	defer cancel()

	var exists bool
	row := r.db.Read.QueryRowContext(ctxWTO, q)
	err := row.Scan(&exists)
	if err != nil {
		return false, err
	}
	return !exists, nil
}
