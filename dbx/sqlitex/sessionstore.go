package sqlitex

import (
	"database/sql"
	"fmt"
	"log"
	"time"
)

// SessionStore represents the session store.
type SessionStore struct {
	db          *SQLiteDB
	stopCleanup chan bool
	tableName   string
}

type Config struct {
	// CleanUpInterval is the interval between each cleanup operation.
	// If set to 0, the cleanup operation is disabled.
	CleanUpInterval time.Duration

	// TableName is the name of the table where the session data will be stored.
	// If not set, it will default to "sessions".
	TableName string
}

// New returns a new SQLite3Store instance, with a background cleanup goroutine
// that runs every 5 minutes to remove expired session data.
func NewSessionStore(db *SQLiteDB) *SessionStore {
	return NewSessionStoreWithConfig(db, Config{
		CleanUpInterval: 5 * time.Minute,
	})
}

// NewWithCleanupInterval returns a new SQLite3Store instance. The cleanupInterval
// parameter controls how frequently expired session data is removed by the
// background cleanup goroutine. Setting it to 0 prevents the cleanup goroutine
// from running (i.e. expired sessions will not be removed).
func NewSessionStoreWithCleanupInterval(db *SQLiteDB, cleanupInterval time.Duration) *SessionStore {
	return NewSessionStoreWithConfig(db, Config{
		CleanUpInterval: cleanupInterval,
	})
}

// NewWithConfig returns a new SQLite3Store instance with the given configuration.
// If the TableName field is empty, it will be set to "sessions".
// If the CleanUpInterval field is 0, the cleanup goroutine will not be started.
func NewSessionStoreWithConfig(db *SQLiteDB, config Config) *SessionStore {
	if config.TableName == "" {
		config.TableName = "sessions"
	}

	s := &SessionStore{db: db, tableName: config.TableName}

	if config.CleanUpInterval > 0 {
		s.stopCleanup = make(chan bool)
		go s.startCleanup(config.CleanUpInterval)
	}

	return s
}

// Find returns the data for a given session token from the SQLite3Store instance.
// If the session token is not found or is expired, the returned exists flag will
// be set to false.
func (s *SessionStore) Find(token string) (b []byte, exists bool, err error) {
	stmt := fmt.Sprintf("SELECT data FROM %s WHERE token = $1 AND julianday('now') < expiry", s.tableName)
	row := s.db.Read.QueryRow(stmt, token)
	err = row.Scan(&b)
	if err == sql.ErrNoRows {
		return nil, false, nil
	} else if err != nil {
		return nil, false, err
	}
	return b, true, nil
}

// Commit adds a session token and data to the SQLite3Store instance with the
// given expiry time. If the session token already exists, then the data and expiry
// time are updated.
func (s *SessionStore) Commit(token string, b []byte, expiry time.Time) error {
	stmt := fmt.Sprintf("REPLACE INTO %s (token, data, expiry) VALUES ($1, $2, julianday($3))", s.tableName)
	_, err := s.db.Write.Exec(stmt, token, b, expiry.UTC().Format("2006-01-02T15:04:05.999"))
	return err
}

// Delete removes a session token and corresponding data from the SQLite3Store
// instance.
func (s *SessionStore) Delete(token string) error {
	stmt := fmt.Sprintf("DELETE FROM %s WHERE token = $1", s.tableName)
	_, err := s.db.Write.Exec(stmt, token)
	return err
}

// All returns a map containing the token and data for all active (i.e.
// not expired) sessions in the SQLite3Store instance.
func (s *SessionStore) All() (map[string][]byte, error) {
	stmt := fmt.Sprintf("SELECT token, data FROM %s WHERE julianday('now') < expiry", s.tableName)
	rows, err := s.db.Read.Query(stmt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sessions := make(map[string][]byte)

	for rows.Next() {
		var (
			token string
			data  []byte
		)

		err = rows.Scan(&token, &data)
		if err != nil {
			return nil, err
		}

		sessions[token] = data
	}

	err = rows.Err()
	if err != nil {
		return nil, err
	}

	return sessions, nil
}

func (s *SessionStore) startCleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	for {
		select {
		case <-ticker.C:
			err := s.deleteExpired()
			if err != nil {
				log.Println(err)
			}
		case <-s.stopCleanup:
			ticker.Stop()
			return
		}
	}
}

// StopCleanup terminates the background cleanup goroutine for the SQLite3Store
// instance. It's rare to terminate this; generally SQLite3Store instances and
// their cleanup goroutines are intended to be long-lived and run for the lifetime
// of your application.
//
// There may be occasions though when your use of the SQLite3Store is transient.
// An example is creating a new SQLite3Store instance in a test function. In this
// scenario, the cleanup goroutine (which will run forever) will prevent the
// SQLite3Store object from being garbage collected even after the test function
// has finished. You can prevent this by manually calling StopCleanup.
func (s *SessionStore) StopCleanup() {
	if s.stopCleanup != nil {
		s.stopCleanup <- true
	}
}

func (s *SessionStore) deleteExpired() error {
	stmt := fmt.Sprintf("DELETE FROM %s WHERE expiry < julianday('now')", s.tableName)
	_, err := s.db.Write.Exec(stmt)
	return err
}
