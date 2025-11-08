package dbx

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"

	"github.com/ASC521/communis/dbx/sqlitex"
	_ "modernc.org/sqlite"
)

//go:embed sql
var embeddedMigrationsFS embed.FS
var migrationRegex = regexp.MustCompile(`^(?<number>\d+)_(?<name>.*)\.(?<direction>up|down)\.sql$`)

type MigrationDriver interface {
	AddVersionTable() error
	RunMigration(sql string, version uint) error
	Version() (uint, error)
}

type migration struct {
	Version  uint
	upFile   string
	downFile string
	Name     string
}

func findVersionIndex(migrations []migration, version uint) int {
	for i, m := range migrations {
		if m.Version == version {
			return i
		}
	}
	return -1
}

func loadMigrations() ([]migration, error) {
	des, err := embeddedMigrationsFS.ReadDir("sql")
	if err != nil {
		return nil, err
	}
	migs := []migration{}
	for _, de := range des {
		fn := de.Name()
		matches := migrationRegex.FindStringSubmatch(fn)
		if len(matches) != 4 {
			// ToDo:  add logging of ignored migrations file
			continue
		}

		ver, err := strconv.Atoi(matches[1])
		if err != nil {
			return nil, err
		}

		i := findVersionIndex(migs, uint(ver))
		if i == -1 {
			mig := migration{Version: uint(ver), Name: matches[2]}
			migs = append(migs, mig)
			i = findVersionIndex(migs, uint(ver))
		}
		mig := &migs[i]
		if matches[3] == "up" {
			mig.upFile = fn
		} else {
			mig.downFile = fn
		}
	}

	slices.SortFunc(migs, func(a, b migration) int {
		return int(a.Version) - int(b.Version)
	})

	return migs, nil
}

type Migrator struct {
	driver       MigrationDriver
	embeddedRoot string
	Migrations   []migration
}

func NewMigrator(driver MigrationDriver) (*Migrator, error) {

	migs, err := loadMigrations()
	if err != nil {
		return nil, err
	}

	mig := &Migrator{driver: driver, embeddedRoot: "sql", Migrations: migs}

	return mig, nil
}

func NewSQLiteMigrator(ctx context.Context, db *sqlitex.SQLiteDB) (*Migrator, error) {
	sd, err := sqlitex.NewSQLiteMigrationDriver(db, ctx)
	if err != nil {
		return nil, err
	}
	migs, err := loadMigrations()
	if err != nil {
		return nil, err
	}
	return &Migrator{driver: sd, embeddedRoot: "sql", Migrations: migs}, nil
}

func (m *Migrator) First() (uint, error) {
	if len(m.Migrations) == 0 {
		return 0, os.ErrNotExist
	}

	return m.Migrations[0].Version, nil
}

func (m *Migrator) Prev(currVersion uint) (uint, error) {
	i := findVersionIndex(m.Migrations, currVersion)
	if i <= 0 {
		return 0, os.ErrNotExist
	}

	return m.Migrations[i-1].Version, nil
}

func (m *Migrator) Next(currVersion uint) (uint, error) {
	i := findVersionIndex(m.Migrations, currVersion)
	if i == len(m.Migrations)-1 || i == -1 {
		return 0, os.ErrNotExist
	}

	return m.Migrations[i+1].Version, nil
}

func (m *Migrator) Bootstrap() error {
	err := m.driver.AddVersionTable()
	if err != nil {
		return err
	}

	currVer, err := m.First()
	if err != nil {
		return err
	}

	for {
		mig, sql, err := m.ReadUp(currVer)
		if err != nil {
			return err
		}

		err = m.driver.RunMigration(sql, currVer)
		if err != nil {
			return fmt.Errorf("migration %s failed: %w", mig.Name, err)
		}

		currVer, err = m.Next(currVer)
		if errors.Is(err, os.ErrNotExist) {
			break
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *Migrator) Up() error {

	cv, err := m.driver.Version()
	if err != nil {
		return err
	}

	for {
		if cv == 0 {
			cv, err = m.First()
			if err != nil {
				return err
			}

		} else {
			cv, err = m.Next(cv)
			if errors.Is(err, os.ErrNotExist) {
				break
			}
			if err != nil {
				return err
			}
		}

		mig, sql, err := m.ReadUp(cv)
		if err != nil {
			return fmt.Errorf("migration %s failed: %w", mig.Name, err)
		}

		err = m.driver.RunMigration(sql, cv)
		if err != nil {
			return err
		}
		slog.Info(fmt.Sprintf("database migrated to version %v - %s", cv, mig.Name), "version", cv, "name", mig.Name)
	}

	return nil

}

func (m *Migrator) Down() error {
	currVer, err := m.driver.Version()
	if err != nil {
		return err
	}

	for {

		prevVer, prevErr := m.Prev(currVer)
		mig, sql, err := m.ReadDown(currVer)
		if err != nil {
			return err
		}
		err = m.driver.RunMigration(sql, prevVer)
		if err != nil {
			return fmt.Errorf("migration %s failed: %w", mig.Name, err)
		}
		slog.Info(fmt.Sprintf("database migrated to version %v - %s", prevVer, mig.Name), "version", prevVer, "name", mig.Name)
		if errors.Is(prevErr, os.ErrNotExist) {
			break
		}
		if prevErr != nil {
			return err
		}

	}

	return nil
}

func (m *Migrator) ReadUp(version uint) (migration, string, error) {
	i := findVersionIndex(m.Migrations, version)
	if i == -1 {
		return migration{}, "", os.ErrNotExist
	}

	mig := m.Migrations[i]
	path := filepath.Join(m.embeddedRoot, mig.upFile)
	data, err := embeddedMigrationsFS.ReadFile(path)
	if err != nil {
		return migration{}, "", err
	}
	return mig, string(data), nil
}

func (m *Migrator) ReadDown(version uint) (migration, string, error) {
	i := findVersionIndex(m.Migrations, version)
	if i == -1 {
		return migration{}, "", os.ErrNotExist
	}

	mig := m.Migrations[i]
	path := filepath.Join(m.embeddedRoot, mig.downFile)
	data, err := embeddedMigrationsFS.ReadFile(path)
	if err != nil {
		return migration{}, "", err
	}
	return mig, string(data), nil
}

func (m *Migrator) Version() (uint, error) {
	return m.driver.Version()
}
