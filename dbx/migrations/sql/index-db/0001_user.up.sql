CREATE TABLE IF NOT EXISTS users (
       id INTEGER PRIMARY KEY,
       name TEXT UNIQUE NOT NULL,
       hashed_password TEXT NOT NULL,
       db_path TEXT NOT NULL,
       db_version INTEGER NOT NULL,
       is_admin INTEGER NOT NULL DEFAULT 0
) strict;

CREATE TABLE sessions (
	token TEXT PRIMARY KEY,
	data BLOB NOT NULL,
	expiry REAL NOT NULL
);

CREATE INDEX sessions_expiry_idx ON sessions(expiry);
