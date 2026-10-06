DROP VIEW IF EXISTS "main"."notes_details";

CREATE VIEW notes_details AS
SELECT 
n.id,
n.section as section_id,
s.name as section_name,
n.title,
n.content,
n.created_at_utc,
n.last_updated_at_utc,
n.bookmark,
(SELECT COALESCE(JSON_GROUP_ARRAY(JSON_OBJECT('id', tags.id, 'name', tags.name)), JSON('[]')) FROM notes_tags JOIN tags ON notes_tags.tag_id = tags.id WHERE notes_tags.note_id = n.id) AS tags_json,
(SELECT COALESCE(GROUP_CONCAT(tags.name, ' '), '') FROM notes_tags JOIN tags ON notes_tags.tag_id = tags.id WHERE notes_tags.note_id = n.id) AS tags_txt,
(SELECT COALESCE(JSON_GROUP_ARRAY(JSON_OBJECT('id', reference_notes.ref_note_id, 'title', notes.title)), JSON('[]')) FROM reference_notes JOIN notes ON notes.id = reference_notes.ref_note_id WHERE reference_notes.note_id = n.id) as reference_notes_json,
(SELECT COALESCE(JSON_GROUP_ARRAY(JSON_OBJECT('id', reference_notes.note_id, 'title', notes.title)), JSON('[]')) FROM reference_notes JOIN notes ON notes.id = reference_notes.note_id WHERE reference_notes.ref_note_id = n.id) as reference_by_notes_json
FROM notes n
LEFT JOIN sections s ON s.id = n.section;

CREATE INDEX IF NOT EXISTS index_ref_note_id ON reference_notes(ref_note_id);
