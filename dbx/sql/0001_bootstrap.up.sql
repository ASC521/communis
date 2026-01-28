CREATE TABLE IF NOT EXISTS sections (
       id INTEGER PRIMARY KEY,
       name TEXT NOT NULL UNIQUE
) strict;

INSERT INTO sections (name) VALUES ('01 Inbox');

CREATE TABLE IF NOT EXISTS tags (
       id INTEGER PRIMARY KEY,
       name TEXT NOT NULL UNIQUE
) strict;

CREATE TABLE IF NOT EXISTS notes (
       id INTEGER PRIMARY KEY,
       title TEXT UNIQUE NOT NULL,
       content TEXT,
       section INTEGER NOT NULL,
       created_at_utc TEXT DEFAULT (datetime('now')),
       last_updated_at_utc TEXT DEFAULT (datetime('now')),
       FOREIGN KEY(section) REFERENCES sections(id) ON DELETE RESTRICT
) strict;

CREATE TABLE IF NOT EXISTS notes_tags (
       note_id INTEGER NOT NULL,
       tag_id INTEGER NOT NULL,
       PRIMARY KEY (note_id, tag_id)
       FOREIGN KEY (note_id) REFERENCES notes(id) ON DELETE CASCADE,
       FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE
) strict;
