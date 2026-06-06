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

type SQLite struct {
	DB *sqlitex.SQLiteDB
}

func NewSQLite(db *sqlitex.SQLiteDB) *SQLite {
	return &SQLite{DB: db}
}

func (r *SQLite) DBVersionBefore(ctx context.Context, latestVer int) ([]UserDatabase, error) {
	sql := `SELECT user_databases.id, user_databases.user_id, user_databases.db_path, user_databases.db_version
		FROM user_databases
		INNER JOIN users ON user_databases.user_id = users.id
		WHERE db_version < ? AND users.is_admin = 0;`
	ctxWTO, cancel := context.WithTimeout(ctx, r.DB.QueryTimeout)
	defer cancel()
	rows, err := r.DB.Read.QueryContext(ctxWTO, sql, latestVer)
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

func (r *SQLite) UpdateDBVersion(ctx context.Context, userID int64, version int) error {
	updateStmt := `UPDATE user_databases SET db_version = ? WHERE user_id = ?;`

	_, err := sqlitex.WithTransaction(r.DB.Write, ctx, func(ctx context.Context, tx *sql.Tx) (int, error) {
		_, err := tx.Exec(updateStmt, version, userID)
		if err != nil {
			return -1, err
		}

		return int(userID), nil
	})

	return err
}

func (r *SQLite) GetUserDB(ctx context.Context, userID int64) (UserDatabase, error) {
	q := `SELECT id, db_path, db_version FROM user_databases WHERE user_id = ?;`
	ctxWTO, cancel := context.WithTimeout(ctx, r.DB.QueryTimeout)
	defer cancel()

	userDB := UserDatabase{UserID: userID}
	row := r.DB.Read.QueryRowContext(ctxWTO, q, userID)
	err := row.Scan(&userDB.ID, &userDB.Path, &userDB.Version)
	if err != nil {
		return UserDatabase{}, err
	}
	return userDB, nil
}

func (r *SQLite) CreateAdminUser(ctx context.Context, username, password string) (int64, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return 0, err
	}

	stmt := `INSERT INTO users (name, hashed_password, is_admin) VALUES (?, ?, ?)`
	ctxWTO, cancel := context.WithTimeout(ctx, r.DB.QueryTimeout)
	defer cancel()

	res, err := r.DB.Write.ExecContext(ctxWTO, stmt, username, hashedPassword, true)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *SQLite) CreateUserAndDB(ctx context.Context, userName, password string) (int64, error) {

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return 0, err
	}

	userStmt := `INSERT INTO users (name, hashed_password, is_admin) VALUES (?, ?, ?);`
	dbStmt := `INSERT INTO user_databases (user_id, db_path, db_version) VALUES (?, ?, ?)`
	ctxWTO, cancel := context.WithTimeout(ctx, r.DB.QueryTimeout)
	defer cancel()

	return sqlitex.WithTransaction(r.DB.Write, ctxWTO, func(ctx context.Context, tx *sql.Tx) (int64, error) {
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

func (r *SQLite) AuthenticateUser(ctx context.Context, username, password string) (User, error) {
	q := `SELECT id, name, hashed_password, is_admin, created_at_utc, last_login_utc FROM users WHERE name = ?;`
	ctxWTO, cancel := context.WithTimeout(ctx, r.DB.QueryTimeout)
	defer cancel()

	user := User{}
	var hashedPassword []byte
	var createdStr, lastLoginStr sql.NullString
	row := r.DB.Read.QueryRowContext(ctxWTO, q, username)
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

func (r *SQLite) IsAdminUser(ctx context.Context, userID int64) (bool, error) {
	q := `SELECT is_admin FROM users WHERE id = ?;`
	ctxWTO, cancel := context.WithTimeout(ctx, r.DB.QueryTimeout)
	defer cancel()

	isAdmin := false
	err := r.DB.Read.QueryRowContext(ctxWTO, q, userID).Scan(&isAdmin)
	if err != nil {
		return false, err
	}
	return isAdmin, nil
}

func (r *SQLite) GetUser(ctx context.Context, id int64) (User, error) {
	q := `SELECT id, name, is_admin, created_at_utc, last_login_utc, theme FROM users WHERE id = ?;`
	ctxWTO, cancel := context.WithTimeout(ctx, r.DB.QueryTimeout)
	defer cancel()

	row := r.DB.Read.QueryRowContext(ctxWTO, q, id)
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

func (r *SQLite) UpdateUser(ctx context.Context, id int64, name string) (User, error) {
	stmt := `UPDATE users SET name = ? WHERE id = ?
		RETURNING id, name, is_admin, created_at_utc, last_login_utc, theme;`

	ctxWTO, cancel := context.WithTimeout(ctx, r.DB.QueryTimeout)
	defer cancel()

	row := r.DB.Write.QueryRowContext(ctxWTO, stmt, name, id)
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

func (r *SQLite) UpdateUserPassword(ctx context.Context, id int64, password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return err
	}

	stmt := `UPDATE users SET hashed_password = ? WHERE id = ?;`
	ctxWTO, cancel := context.WithTimeout(ctx, r.DB.QueryTimeout)
	defer cancel()

	_, err = r.DB.Write.ExecContext(ctxWTO, stmt, hashedPassword, id)
	return err
}

func (r *SQLite) UpdateUserLastLoginToNow(ctx context.Context, id int64) error {
	stmt := `UPDATE users SET last_login_utc = datetime('now') WHERE id = ?;`

	ctxWTO, cancel := context.WithTimeout(ctx, r.DB.QueryTimeout)
	defer cancel()

	_, err := r.DB.Write.ExecContext(ctxWTO, stmt, id)
	return err
}

func (r *SQLite) ListUsers(ctx context.Context) ([]User, error) {
	q := `SELECT id, name, is_admin, created_at_utc, last_login_utc FROM users`
	ctxWTO, cancel := context.WithTimeout(ctx, r.DB.QueryTimeout)
	defer cancel()

	rows, err := r.DB.Read.QueryContext(ctxWTO, q)
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

func (r *SQLite) DeleteUser(ctx context.Context, id int64) error {
	s := `DELETE FROM users WHERE id = ?;`
	ctxWTO, cancel := context.WithTimeout(ctx, r.DB.QueryTimeout)
	defer cancel()

	_, err := r.DB.Write.ExecContext(ctxWTO, s, id)
	return err
}

func (r *SQLite) UpdateUserTheme(ctx context.Context, id int64, theme string) error {
	s := `UPDATE users SET theme = ? WHERE id =?;`
	ctxWTO, cancel := context.WithTimeout(ctx, r.DB.QueryTimeout)
	defer cancel()

	_, err := r.DB.Write.ExecContext(ctxWTO, s, theme, id)
	return err
}

func (r *SQLite) NameExists(ctx context.Context, name string) (bool, error) {
	q := `SELECT EXISTS(SELECT 1 FROM users WHERE name = ?);`
	ctxWTO, cancel := context.WithTimeout(ctx, r.DB.QueryTimeout)
	defer cancel()

	var exists bool
	row := r.DB.Read.QueryRowContext(ctxWTO, q, name)
	err := row.Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil

}

func (r *SQLite) InitialSetupNeeded(ctx context.Context) (bool, error) {
	q := `SELECT EXISTS(SELECT 1 FROM users WHERE is_admin = true)`
	ctxWTO, cancel := context.WithTimeout(ctx, r.DB.QueryTimeout)
	defer cancel()

	var exists bool
	row := r.DB.Read.QueryRowContext(ctxWTO, q)
	err := row.Scan(&exists)
	if err != nil {
		return false, err
	}
	return !exists, nil
}
