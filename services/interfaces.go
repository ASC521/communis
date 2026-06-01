package services

import (
	"context"
	"sync"

	"github.com/ASC521/communis/dbx/sqlitex"
	"github.com/ASC521/communis/models"
)

type DataStoreService interface {
	GetNotesStore(ctx context.Context, key int64) (models.NotesRepository, error)
	Remove(key int64) error
	CreateDB(ctx context.Context, key int64) error
	DeleteDB(ctx context.Context, key int64) error
	GetIndexDatabase() *sqlitex.SQLiteDB
	GetUserStore() models.IndexRepository
	RunMigrations(ctx context.Context) error
	Stop() error
	Start(ctx context.Context, wg *sync.WaitGroup)
	GetState() CacheState
}
