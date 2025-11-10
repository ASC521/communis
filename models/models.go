package models

import "time"

type Tag struct {
	Id   int64  `json:"id"`
	Name string `json:"name"`
}

type PaginatedTags struct {
	Tags       []*Tag
	Limit      int
	Offset     int
	HasMore    bool
	NextOffset *int
}

type TagRepository interface {
	Create(t *Tag) (int64, error)
	FindById(id int64) (*Tag, error)
	Update(t *Tag) error
	Delete(id int64) error
	List(limit, offset int) (*PaginatedTags, error)
}

type Notebook struct {
	Id   int64
	Name string
}

type PaginatedNotebooks struct {
	Notebooks  []*Notebook
	Limit      int
	Offset     int
	HasMore    bool
	NextOffset *int
}

type NotebookRepository interface {
	Create(n *Notebook) (int64, error)
	FindById(id int64) (*Notebook, error)
	Update(n *Notebook) error
	Delete(id int64) error
	List(limit, offset int) (*PaginatedNotebooks, error)
}

type Note struct {
	Id            int64
	Title         string
	Content       string
	Notebook      Notebook
	Tags          []Tag
	CreatedAt     time.Time
	LastUpdatedAt time.Time
}

type PaginatedNotes struct {
	Notes      []*Note
	Limit      int
	Offset     int
	HasMore    bool
	NextOffset *int
}

type NoteRepository interface {
	Create(n *Note) (int64, error)
	FindById(id int64) (*Note, error)
	Update(n *Note) error
	Delete(id int64) error
	List(limit, offset int) (*PaginatedNotes, error)
}
