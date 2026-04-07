package models

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalidCredentials = errors.New("models: invalid credentials")
	ErrDuplicateUserName  = errors.New("models: duplicate username")
)

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

type Note struct {
	Id               int64
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
	Id    int64  `json:"id"`
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
	Id             int64
	Title          string
	TitleHighlight string
	ContentSnippet string
	TagNames       string
}

type NotesRepository interface {
	// Tag Methods
	CreateTag(ctx context.Context, t Tag) (int64, error)
	FindTagById(ctx context.Context, id int64) (Tag, error)
	FindTagByName(ctx context.Context, name string) (Tag, error)
	UpdateTag(ctx context.Context, t Tag) error
	DeleteTag(ctx context.Context, id int64) error
	ListAllTags(ctx context.Context) ([]Tag, error)
	ListTags(ctx context.Context, limit, offset int) (PaginatedTags, error)
	QueryTags(ctx context.Context, ids []int64) ([]Tag, error)

	// Section Methods
	CreateSection(ctx context.Context, s Section) (int64, error)
	FindSectionById(ctx context.Context, id int64) (Section, error)
	FindSectionByName(ctx context.Context, name string) (Section, error)
	UpdateSection(ctx context.Context, s Section) error
	DeleteSection(ctx context.Context, id int64) error
	ListAllSections(ctx context.Context) ([]Section, error)

	// Note Methods
	CreateNote(ctx context.Context, title, content string, sectionId int64, tagIds, referenceNoteIds []int64) (int64, error)
	NoteExists(ctx context.Context, title string) (int64, error)
	FindNoteById(ctx context.Context, id int64) (Note, error)
	UpdateNote(ctx context.Context, id int64, title, content string, sectionId int64, tagIds, referenceNoteIds []int64) error
	DeleteNote(ctx context.Context, id int64) error
	ListNotes(ctx context.Context, limit, offset int) (PaginatedNotes, error)
	SearchNotes(ctx context.Context, query string) ([]NoteSearchResult, error)
	RecentlyUpdatedNotes(ctx context.Context, limit int) ([]NoteDetail, error)
	NotesInSection(ctx context.Context, sectionId int64) ([]NoteDetail, error)
	NotesWithTag(ctx context.Context, tagId int64) ([]NoteDetail, error)
	GetNoteDetailByIds(ctx context.Context, ids []int64) ([]NoteDetail, error)
}

type User struct {
	Id           int64
	Name         string
	IsAdmin      bool
	CreatedAtUTC time.Time
	LastLoginUTC time.Time
	Theme        string
}

type UserDatabase struct {
	Id      int64
	UserId  int64
	Path    string
	Version int
}

type IndexRepository interface {
	DBVersionBefore(ctx context.Context, latestVer int) ([]UserDatabase, error)
	UpdateDBVersion(ctx context.Context, id int64, version int) error
	GetUserDB(ctx context.Context, userId int64) (UserDatabase, error)
	CreateAdminUser(ctx context.Context, username, password string) (int64, error)
	CreateUserAndDB(ctx context.Context, userName, password string) (int64, error)
	AuthenticateUser(ctx context.Context, username, password string) (User, error)
	IsAdminUser(ctx context.Context, userId int64) (bool, error)
	GetUser(ctx context.Context, userId int64) (User, error)
	ListUsers(ctx context.Context) ([]User, error)
	UpdateUser(ctx context.Context, id int64, name string) (User, error)
	UpdateUserLastLoginToNow(ctx context.Context, id int64) error
	UpdateUserPassword(ctx context.Context, id int64, password string) error
	DeleteUser(ctx context.Context, id int64) error
	UpdateUserTheme(ctx context.Context, id int64, theme string) error
	NameExists(ctx context.Context, name string) (bool, error)
}
