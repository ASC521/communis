CREATE TABLE IF NOT EXISTS notebooks (
       id INTEGER PRIMARY KEY,
       name TEXT NOT NULL
) strict;

CREATE TABLE IF NOT EXISTS tags (
       id INTEGER PRIMARY KEY,
       name TEXT UNIQUE NOT NULL
) strict;

CREATE TABLE IF NOT EXISTS notes (
       id INTEGER PRIMARY KEY,
       title TEXT NOT NULL,
       content TEXT,
       notebook INTEGER NOT NULL,
       created_at_utc TEXT DEFAULT (datetime('now')),
       FOREIGN KEY(notebook) REFERENCES notebooks(id)
) strict;

CREATE TABLE IF NOT EXISTS note_tags (
       note_id INTEGER NOT NULL,
       tag_id INTEGER NOT NULL,
       PRIMARY KEY (note_id, tag_id)
       FOREIGN KEY (note_id) REFERENCES notes(id),
       FOREIGN KEY (tag_id) REFERENCES tags(id)
) strict;
