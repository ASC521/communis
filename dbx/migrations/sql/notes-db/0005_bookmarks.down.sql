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
COALESCE(t.tags_json, JSON('[]')) AS tags_json,
COALESCE(t.tags_txt, '') AS tags_txt,
COALESCE(ref_notes.reference_notes_json, JSON('[]')) as reference_notes_json,
COALESCE(ref_by_notes.notes_json, JSON('[]')) as reference_by_notes_json
FROM notes n
LEFT JOIN notes_tags nt ON n.id = nt.note_id
LEFT JOIN sections s ON s.id = n.section
LEFT JOIN (
	SELECT 
	nt.note_id,
	JSON_GROUP_ARRAY(JSON_OBJECT('id', t.id, 'name', t.name)) as tags_json,
	GROUP_CONCAT(t.name, ' ') as tags_txt
	FROM notes_tags AS nt
	INNER JOIN tags AS t ON t.id = nt.tag_id
	GROUP BY nt.note_id
) AS t ON t.note_id = n.id
LEFT JOIN (
	SELECT 
	rn.note_id, 
	JSON_GROUP_ARRAY(JSON_OBJECT('id', rn.ref_note_id, 'title', n.title)) as reference_notes_json
	FROM reference_notes AS rn
	JOIN notes AS n ON n.id = rn.ref_note_id
	GROUP BY rn.note_id
) AS ref_notes ON ref_notes.note_id = n.id
LEFT JOIN (
	SELECT 
	rn.ref_note_id as note_id, 
	JSON_GROUP_ARRAY(JSON_OBJECT('id', rn.note_id, 'title', n.title)) as notes_json
	FROM reference_notes AS rn
	JOIN notes AS n ON n.id = rn.note_id
	GROUP BY rn.ref_note_id
) AS ref_by_notes ON ref_by_notes.note_id = n.id;

ALTER TABLE notes DROP COLUMN bookmark;
