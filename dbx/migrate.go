package dbx

import (
	"context"
	"embed"
	"errors"
	"fmt"
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
	IsEmpty() (bool, error)
}

type migration struct {
	Version  uint
	upFile   string
	downFile string
	Name     string
}

var emptyMigration = migration{Version: 0, Name: "Empty Database"}

func findVersionIndex(migrations []migration, version uint) int {
	for i, m := range migrations {
		if m.Version == version {
			return i
		}
	}
	return -1
}

func loadMigrations(migPath string) ([]migration, error) {
	des, err := embeddedMigrationsFS.ReadDir(migPath)
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

func NewMigrator(driver MigrationDriver, migPath string) (*Migrator, error) {

	migs, err := loadMigrations(migPath)
	if err != nil {
		return nil, err
	}

	mig := &Migrator{driver: driver, embeddedRoot: migPath, Migrations: migs}

	return mig, nil
}

func NewSQLiteMigrator(ctx context.Context, db *sqlitex.SQLiteDB, migPath string) (*Migrator, error) {
	sd, err := sqlitex.NewSQLiteMigrationDriver(db, ctx)
	if err != nil {
		return nil, err
	}
	migs, err := loadMigrations(migPath)
	if err != nil {
		return nil, err
	}
	return &Migrator{driver: sd, embeddedRoot: migPath, Migrations: migs}, nil
}

func (m *Migrator) First() (migration, error) {
	if len(m.Migrations) == 0 {
		return migration{}, os.ErrNotExist
	}

	return m.Migrations[0], nil
}

func (m *Migrator) Latest() (migration, error) {
	if len(m.Migrations) == 0 {
		return migration{}, os.ErrNotExist
	}

	return m.Migrations[len(m.Migrations)-1], nil
}

func (m *Migrator) Prev(currVersion uint) (migration, error) {
	i := findVersionIndex(m.Migrations, currVersion)
	if i <= 0 {
		return migration{}, os.ErrNotExist
	}

	return m.Migrations[i-1], nil
}

func (m *Migrator) Next(currVersion uint) (migration, error) {
	if currVersion == 0 {
		return m.First()
	}

	i := findVersionIndex(m.Migrations, currVersion)
	if i == len(m.Migrations)-1 || i == -1 {
		return migration{}, os.ErrNotExist
	}

	return m.Migrations[i+1], nil
}

func (m *Migrator) Curr() (migration, error) {
	cvn, err := m.Version()
	if err != nil {
		return migration{}, err
	}

	if cvn == 0 {
		return emptyMigration, nil
	}

	i := findVersionIndex(m.Migrations, cvn)
	if i == -1 {
		return migration{}, os.ErrNotExist
	}

	return m.Migrations[i], nil
}

// Bootstrap configures an migration controlled database and migrates it to the latest version
func (m *Migrator) Bootstrap() error {
	err := m.driver.AddVersionTable()
	if err != nil {
		return err
	}

	return m.Up()
}

// Up migrates a database to it's latest version
func (m *Migrator) Up() error {

	lv, err := m.Latest()
	if err != nil {
		return err
	}
	for {
		mig, err := m.StepUp()
		if err != nil {
			return err
		}

		if mig.Version == lv.Version {
			return nil
		}
	}

}

// Down migrates a database to an 'Empty Database' state
func (m *Migrator) Down() error {

	for {
		mig, err := m.StepDown()
		if err != nil {
			return err
		}
		if mig.Version == 0 {
			return nil
		}
	}

}

// StepDown executes the current version's down.sql file and returns the resulting current migration.
func (m *Migrator) StepDown() (migration, error) {
	cm, err := m.Curr()
	if err != nil {
		return migration{}, err
	}

	if cm.Version == 0 {
		return migration{}, errors.New("empty database - cannot migrate down any further")
	}

	pvm, err := m.Prev(cm.Version)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			pvm = emptyMigration
		} else {
			return migration{}, err
		}
	}

	_, sql, err := m.readDown(cm)
	if err != nil {
		return migration{}, err
	}

	err = m.driver.RunMigration(sql, pvm.Version)
	if err != nil {
		return migration{}, err
	}

	return pvm, nil
}

// StepUp execute the current version's up.sql file and returns the resulting current migration
func (m *Migrator) StepUp() (migration, error) {
	isLatest, err := m.IsLatest()
	if err != nil {
		return migration{}, err
	}

	if isLatest {
		lm, err := m.Latest()
		if err != nil {
			return migration{}, err
		}
		return lm, nil
	}

	cv, err := m.Version()
	if err != nil {
		return migration{}, nil
	}

	nm, err := m.Next(cv)
	if err != nil {
		return migration{}, err
	}

	sql, err := m.readUp(nm)
	if err != nil {
		return migration{}, err
	}

	err = m.driver.RunMigration(sql, nm.Version)
	if err != nil {
		return migration{}, err
	}

	return nm, err
}

func (m *Migrator) readUp(mig migration) (string, error) {
	path := filepath.Join(m.embeddedRoot, mig.upFile)
	data, err := embeddedMigrationsFS.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (m *Migrator) readDown(mig migration) (migration, string, error) {
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

func (m *Migrator) IsLatest() (bool, error) {
	dbv, err := m.Version()
	if err != nil {
		return false, fmt.Errorf("failed to find database version: %w", err)
	}

	lv, err := m.Latest()
	if err != nil {
		return false, fmt.Errorf("failed to find last configured migration: %w", err)
	}

	return dbv == lv.Version, nil

}

func (m *Migrator) IsEmpty() (bool, error) {
	return m.driver.IsEmpty()
}
