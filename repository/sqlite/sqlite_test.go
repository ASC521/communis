package sqlite_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ASC521/communis/dbx"
	"github.com/ASC521/communis/dbx/sqlitex"
	"github.com/ASC521/communis/models"
	"github.com/ASC521/communis/repository/sqlite"
)

func bootstrapInMemoryDB(ctx context.Context) (*sqlitex.SQLiteDB, error, func() error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to find current directory: %w", err), nil
	}

	p := filepath.Dir(cwd)
	r := filepath.Dir(p)
	testPath := filepath.Join(r, "test")
	err = os.Mkdir(testPath, 0700)
	if err != nil {
		return nil, fmt.Errorf("failed to make test directory: %w", err), nil
	}

	cleanUp := func() error {
		return os.RemoveAll(testPath)
	}

	db, err := sqlitex.NewSQLiteDB(ctx, filepath.Join(testPath, "test.db"))
	if err != nil {
		cleanUp()
		return nil, fmt.Errorf("failed to create sqlite database: %w", err), nil
	}

	mig, err := dbx.NewSQLiteMigrator(ctx, db)
	if err != nil {
		cleanUp()
		return nil, fmt.Errorf("failed to create sqlite migrator: %w", err), nil
	}
	err = mig.Bootstrap()
	if err != nil {
		cleanUp()
		return nil, fmt.Errorf("failed to bootstrap database: %w", err), nil
	}

	return db, nil, cleanUp

}

func TestSQLiteNotebookRepository(t *testing.T) {

	nbs := make([]*models.Notebook, 0, 19)
	for i := range 20 {
		nbs = append(nbs, &models.Notebook{Name: fmt.Sprintf("notebook-%v", i+1)})
	}

	ctx := context.Background()
	db, err, cleanUp := bootstrapInMemoryDB(ctx)
	if err != nil {
		t.Fatalf("failed to bootstrap sqlite database: %v", err)
	}
	defer cleanUp()

	nbRepo := sqlite.NewNotebookRepository(db, ctx)

	tcs := []struct {
		Name  string
		TFunc func(models.NotebookRepository) error
	}{
		{
			Name: "Create",
			TFunc: func(nr models.NotebookRepository) error {
				for _, nb := range nbs {
					id, err := nr.Create(nb)
					if err != nil {
						return fmt.Errorf("failed to create notebook %s: %w", nb.Name, err)
					}
					if id == 0 {
						return fmt.Errorf("id not returned for %s", nb.Name)
					}
					nb.Id = id
				}

				return nil
			},
		},
		{
			Name: "FindById",
			TFunc: func(nr models.NotebookRepository) error {
				nb := nbs[2]
				nbQ, err := nr.FindById(nb.Id)
				if err != nil {
					return fmt.Errorf("failed to find notebook by id: %w", err)
				}
				if nbQ.Name != nbs[2].Name {
					return fmt.Errorf("find by id returned an unexpected notebook, got %v, want %v", nbQ, nbs[2])
				}

				return nil
			},
		},
		{
			Name: "Update",
			TFunc: func(nr models.NotebookRepository) error {
				onb := nbs[10]
				nnb := &models.Notebook{Id: onb.Id, Name: onb.Name}
				nnb.Name = "notebook-25"
				err := nr.Update(nnb)
				if err != nil {
					return fmt.Errorf("failed to update notebook: %w", err)
				}
				qnb, err := nr.FindById(onb.Id)
				if err != nil {
					return fmt.Errorf("failed to find updated notebook by id: %w", err)
				}
				if qnb.Name != nnb.Name {
					return fmt.Errorf("notebook update failed: got %v, want %v", qnb.Name, nnb.Name)
				}
				return nil
			},
		},
		{
			Name: "Delete",
			TFunc: func(nr models.NotebookRepository) error {
				nb := nbs[12]
				err := nr.Delete(nb.Id)
				if err != nil {
					return fmt.Errorf("notebook delete failed: %w", err)
				}

				dnb, _ := nr.FindById(nb.Id)
				if dnb != nil {
					return fmt.Errorf("deleted notebook id was returned")
				}

				return nil
			},
		},
		{
			Name: "List",
			TFunc: func(nr models.NotebookRepository) error {
				set1, err := nr.List(10, 0)
				if err != nil {
					return fmt.Errorf("failed to list notebooks: %w", err)
				}
				if len(set1.Notebooks) != 10 {
					return fmt.Errorf("expected list 1 to have 10, got %v", len(set1.Notebooks))
				}

				if !set1.HasMore {
					return fmt.Errorf("expected list 1 indates there is no more data, expecting to indicate more data available")
				}

				if set1.NextOffset == nil {
					return fmt.Errorf("set 1 next offset is nil but indicates there is more data; expecting NextOffset to be non nil")
				}
				set2, err := nr.List(set1.Limit, *set1.NextOffset)
				if err != nil {
					return fmt.Errorf("failed to list notebooks: %w", err)
				}
				if len(set2.Notebooks) != 9 {
					return fmt.Errorf("expected list 2 to have 9, got %v", len(set2.Notebooks))
				}

				return nil
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			err := tc.TFunc(nbRepo)
			if err != nil {
				t.Fatal(err)
			}
		})
	}

	db.Close()
}

func TestSQLiteTagRepository(t *testing.T) {

	ts := make([]*models.Tag, 0, 19)
	for i := range 20 {
		ts = append(ts, &models.Tag{Name: fmt.Sprintf("tag-%v", i+1)})
	}

	ctx := context.Background()
	db, err, cleanUp := bootstrapInMemoryDB(ctx)
	if err != nil {
		t.Fatalf("failed to bootstrap sqlite database: %v", err)
	}
	defer cleanUp()

	nbRepo := sqlite.NewTagRepository(db, ctx)

	tcs := []struct {
		Name  string
		TFunc func(models.TagRepository) error
	}{
		{
			Name: "Create",
			TFunc: func(tr models.TagRepository) error {
				for _, t := range ts {
					id, err := tr.Create(t)
					if err != nil {
						return fmt.Errorf("failed to create tag %s: %w", t.Name, err)
					}
					if id == 0 {
						return fmt.Errorf("id not returned for %s", t.Name)
					}
					t.Id = id
				}

				return nil
			},
		},
		{
			Name: "FindById",
			TFunc: func(tr models.TagRepository) error {
				t := ts[2]
				tq, err := tr.FindById(t.Id)
				if err != nil {
					return fmt.Errorf("failed to find tag by id: %w", err)
				}
				if tq.Name != ts[2].Name {
					return fmt.Errorf("find by id returned an unexpected tag, got %v, want %v", tq, ts[2])
				}

				return nil
			},
		},
		{
			Name: "Update",
			TFunc: func(tr models.TagRepository) error {
				ot := ts[10]
				nt := &models.Tag{Id: ot.Id, Name: ot.Name}
				nt.Name = "notebook-25"
				err := tr.Update(nt)
				if err != nil {
					return fmt.Errorf("failed to update tag: %w", err)
				}
				qt, err := tr.FindById(ot.Id)
				if err != nil {
					return fmt.Errorf("failed to find updated tag by id: %w", err)
				}
				if qt.Name != nt.Name {
					return fmt.Errorf("tag update failed: got %v, want %v", qt.Name, nt.Name)
				}
				return nil
			},
		},
		{
			Name: "Delete",
			TFunc: func(tr models.TagRepository) error {
				t := ts[12]
				err := tr.Delete(t.Id)
				if err != nil {
					return fmt.Errorf("tag delete failed: %w", err)
				}

				dnb, _ := tr.FindById(t.Id)
				if dnb != nil {
					return fmt.Errorf("deleted tag id was returned")
				}

				return nil
			},
		},
		{
			Name: "List",
			TFunc: func(tr models.TagRepository) error {
				set1, err := tr.List(10, 0)
				if err != nil {
					return fmt.Errorf("failed to list tags: %w", err)
				}
				if len(set1.Tags) != 10 {
					return fmt.Errorf("expected list 1 to have 10, got %v", len(set1.Tags))
				}

				if !set1.HasMore {
					return fmt.Errorf("expected list 1 indates there is no more data, expecting to indicate more data available")
				}

				if set1.NextOffset == nil {
					return fmt.Errorf("set 1 next offset is nil but indicates there is more data; expecting NextOffset to be non nil")
				}
				set2, err := tr.List(set1.Limit, *set1.NextOffset)
				if err != nil {
					return fmt.Errorf("failed to list tags: %w", err)
				}
				if len(set2.Tags) != 9 {
					return fmt.Errorf("expected list 2 to have 9, got %v", len(set2.Tags))
				}

				return nil
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			err := tc.TFunc(nbRepo)
			if err != nil {
				t.Fatal(err)
			}
		})
	}

	db.Close()
}
