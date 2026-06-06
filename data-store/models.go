package datastore

import (
	"time"
)

type Tag struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type PaginatedTags struct {
	Tags       []Tag
	Limit      int
	Offset     int
	HasMore    bool
	NextOffset *int
}

type Section struct {
	ID   int64
	Name string
}

type PaginatedSections struct {
	Sections   []Section
	Limit      int
	Offset     int
	HasMore    bool
	NextOffset *int
}

type Note struct {
	ID               int64
	Title            string
	Content          string
	Section          Section
	Tags             []Tag
	CreatedAt        time.Time
	LastUpdatedAt    time.Time
	ReferenceNotes   []NoteDetail
	ReferenceByNotes []NoteDetail
}

type NoteDetail struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
}

type PaginatedNotes struct {
	Notes      []*NoteDetail
	Limit      int
	Offset     int
	HasMore    bool
	NextOffset *int
}

type NoteSearchResult struct {
	ID             int64
	Title          string
	TitleHighlight string
	ContentSnippet string
	TagNames       string
}
