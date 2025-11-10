CREATE VIEW notes_details AS
SELECT 
n.id,
n.section as section_id,
s.name as section_name,
n.title,
n.content,
n.created_at_utc,
n.last_updated_at_utc,
JSON_GROUP_ARRAY(JSON_OBJECT('id', t.id, 'name', t.name)) as tags
FROM notes n
LEFT JOIN notes_tags nt ON n.id = nt.note_id
LEFT JOIN tags t ON t.id = nt.tag_id
LEFT JOIN sections s ON s.id = n.section
GROUP BY n.id;
