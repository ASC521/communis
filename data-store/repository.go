package datastore

import "github.com/ASC521/communis/dbx/sqlitex"

type NotesRepository struct {
	db *sqlitex.SQLiteDB
}

func NewNotesRepository(db *sqlitex.SQLiteDB) *NotesRepository {
	return &NotesRepository{db: db}
}
