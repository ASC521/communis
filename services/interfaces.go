package services

import (
	"context"

	"github.com/ASC521/communis/models"
)

type DataStoreService interface {
	GetNotesStore(ctx context.Context, key int64) (models.NotesRepository, error)
	Remove(ctx context.Context, key int64) error
	CreateDB(ctx context.Context, key int64) error
	DeleteDB(ctx context.Context, key int64) error
	GetUserStore() models.IndexRepository
	RunMigrations(ctx context.Context) error
	Stop()
}
