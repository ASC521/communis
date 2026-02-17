CREATE TABLE IF NOT EXISTS users (
       id INTEGER PRIMARY KEY,
       name TEXT UNIQUE NOT NULL,
       hashed_password BLOB NOT NULL,
       is_admin INTEGER NOT NULL DEFAULT 0,
       created_at_utc TEXT DEFAULT (datetime('now')),
       last_login_utc TEXT
) strict;

CREATE TABLE user_databases (
       id INTEGER PRIMARY KEY,
       user_id INTEGER NOT NULL UNIQUE,
       db_path TEXT NOT NULL,
       db_version INTEGER NOT NULL,
       FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) strict;

CREATE TABLE sessions (
	token TEXT PRIMARY KEY,
	data BLOB NOT NULL,
	expiry REAL NOT NULL
);

CREATE INDEX sessions_expiry_idx ON sessions(expiry);
