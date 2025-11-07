package models

import "time"

type Tag struct {
	Id   int64
	Name string
}

type TagRepository interface {
	Create(t *Tag) (int64, error)
	FindById(id int64) (*Tag, error)
	Update(t *Tag) error
	Delete(id int64) error
	List(limit, offset int) ([]*Tag, error)
}

type Notebook struct {
	Id   int64
	Name string
}

type NotebookRepository interface {
	Create(t *Notebook) (int64, error)
	FindById(id int64) (*Notebook, error)
	Update(t *Notebook) error
	Delete(id int64) error
	List(limit, offset int) ([]*Notebook, error)
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

type NoteRepository interface {
	Create(t *Note) (int64, error)
	FindById(id int64) (*Note, error)
	Update(t *Note) error
	Delete(id int64) error
	List(limit, offset int) ([]*Note, error)
}
