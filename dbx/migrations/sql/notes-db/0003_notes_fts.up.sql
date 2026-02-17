CREATE VIRTUAL TABLE IF NOT EXISTS notes_details_fts USING fts5(title, content, tags_txt, content='notes_details', content_rowid='id');

INSERT INTO notes_details_fts(rowid, title, content, tags_txt)
SELECT id, title, content, tags_txt
FROM notes_details;
