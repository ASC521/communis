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
	ListAll() ([]*Tag, error)
	List(limit, offset int) (*PaginatedTags, error)
	Query(names []string) (found []Tag, missing []string, err error)
}

type Section struct {
	Id   int64
	Name string
}

type PaginatedSections struct {
	Sections   []*Section
	Limit      int
	Offset     int
	HasMore    bool
	NextOffset *int
}

type SectionRepository interface {
	Create(s *Section) (int64, error)
	FindById(id int64) (*Section, error)
	FindByName(name string) (*Section, error)
	Update(s *Section) error
	Delete(id int64) error
	ListAll() ([]*Section, error)
	List(limit, offset int) (*PaginatedSections, error)
}

type Note struct {
	Id            int64
	Title         string
	Content       string
	Section       Section
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
	FindByTitle(title string) (*Note, error)
	FindById(id int) (*Note, error)
	Update(n *Note) error
	Delete(id int64) error
	List(limit, offset int) (*PaginatedNotes, error)
}
