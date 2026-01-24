package models

import "time"

type Tag struct {
	Id   int64  `json:"id"`
	Name string `json:"name"`
}

type PaginatedTags struct {
	Tags       []Tag
	Limit      int
	Offset     int
	HasMore    bool
	NextOffset *int
}

type TagRepository interface {
	Create(t *Tag) (int64, error)
	FindById(id int64) (Tag, error)
	FindByName(name string) (Tag, error)
	Update(t *Tag) error
	Delete(id int64) error
	ListAll() ([]Tag, error)
	List(limit, offset int) (PaginatedTags, error)
	Query(ids []int64) ([]Tag, error)
}

type Section struct {
	Id   int64
	Name string
}

type PaginatedSections struct {
	Sections   []Section
	Limit      int
	Offset     int
	HasMore    bool
	NextOffset *int
}

type SectionRepository interface {
	Create(s Section) (int64, error)
	FindById(id int64) (Section, error)
	FindByName(name string) (Section, error)
	Update(s Section) error
	Delete(id int64) error
	ListAll() ([]Section, error)
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

type NoteDetail struct {
	Id            int64
	Title         string
	CreatedAt     time.Time
	LastUpdatedAt time.Time
}

type PaginatedNotes struct {
	Notes      []*NoteDetail
	Limit      int
	Offset     int
	HasMore    bool
	NextOffset *int
}

type NoteSearchResult struct {
	Id             int64
	Title          string
	TitleHighlight string
	ContentSnippet string
	TagNames       string
}

type NoteRepository interface {
	Create(n Note) (int64, error)
	Exists(title string) (int64, error)
	FindById(id int64) (Note, error)
	Update(n Note) error
	Delete(id int64) error
	List(limit, offset int) (PaginatedNotes, error)
	Search(q string) ([]NoteSearchResult, error)
	RecentUpdates(limit uint) ([]NoteDetail, error)
	InSection(secId int64) ([]NoteDetail, error)
}
