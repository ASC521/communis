package migrations

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"slices"
	"strconv"
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

type Migration struct {
	Version  uint
	upFile   string
	downFile string
	Name     string
}

var emptyMigration = Migration{Version: 0, Name: "Empty Database"}

func FindVersionIndex(migrations []Migration, version uint) int {
	for i, m := range migrations {
		if m.Version == version {
			return i
		}
	}
	return -1
}

func Load(migPath string) ([]Migration, error) {

	migrations := []Migration{}
	err := fs.WalkDir(embeddedMigrationsFS, migPath, func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}
		fileName := d.Name()
		matches := migrationRegex.FindStringSubmatch(fileName)
		if len(matches) != 4 {
			return nil
		}

		version, err := strconv.Atoi(matches[1])
		if err != nil {
			return err
		}

		index := FindVersionIndex(migrations, uint(version))
		if index == -1 {
			migration := Migration{Version: uint(version), Name: matches[2]}
			migrations = append(migrations, migration)
			index = FindVersionIndex(migrations, uint(version))
		}

		migration := &migrations[index]
		switch matches[3] {
		case "up":
			migration.upFile = path
		case "down":
			migration.downFile = path
		default:
			return fmt.Errorf("unsupported direction %s", matches[3])
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	slices.SortFunc(migrations, func(a, b Migration) int {
		return int(a.Version) - int(b.Version)
	})

	return migrations, nil
}

func First(migrations []Migration) (Migration, error) {
	if len(migrations) == 0 {
		return Migration{}, os.ErrNotExist
	}

	return migrations[0], nil
}

func Latest(migrations []Migration) (Migration, error) {
	if len(migrations) == 0 {
		return Migration{}, os.ErrNotExist
	}

	return migrations[len(migrations)-1], nil
}

func Prev(migrations []Migration, currVersion uint) (Migration, error) {
	i := FindVersionIndex(migrations, currVersion)
	if i <= 0 {
		return Migration{}, os.ErrNotExist
	}

	return migrations[i-1], nil
}

func Next(migrations []Migration, currVersion uint) (Migration, error) {
	if currVersion == 0 {
		return First(migrations)
	}

	i := FindVersionIndex(migrations, currVersion)
	if i == len(migrations)-1 || i == -1 {
		return Migration{}, os.ErrNotExist
	}

	return migrations[i+1], nil
}

func Curr(migrations []Migration, driver MigrationDriver) (Migration, error) {
	cvn, err := driver.Version()
	if err != nil {
		return Migration{}, err
	}

	if cvn == 0 {
		return emptyMigration, nil
	}

	i := FindVersionIndex(migrations, cvn)
	if i == -1 {
		return Migration{}, os.ErrNotExist
	}

	return migrations[i], nil
}

// Bootstrap configures an migration controlled database and migrates it to the latest version
func Bootstrap(migrations []Migration, driver MigrationDriver) (int, error) {
	err := driver.AddVersionTable()
	if err != nil {
		return -1, err
	}

	return Up(migrations, driver)
}

// Up migrates a database to it's latest version
func Up(migrations []Migration, driver MigrationDriver) (int, error) {

	lv, err := Latest(migrations)
	if err != nil {
		return -1, err
	}
	for {
		mig, err := StepUp(migrations, driver)
		if err != nil {
			return -1, err
		}

		if mig.Version == lv.Version {
			return int(mig.Version), nil
		}
	}

}

// StepUp execute the current version's up.sql file and returns the resulting current migration
func StepUp(migrations []Migration, driver MigrationDriver) (Migration, error) {
	isLatest, err := IsLatest(migrations, driver)
	if err != nil {
		return Migration{}, err
	}

	if isLatest {
		lm, err := Latest(migrations)
		if err != nil {
			return Migration{}, err
		}
		return lm, nil
	}

	cv, err := driver.Version()
	if err != nil {
		return Migration{}, nil
	}

	nm, err := Next(migrations, cv)
	if err != nil {
		return Migration{}, err
	}

	sql, err := readUp(nm)
	if err != nil {
		return Migration{}, err
	}

	err = driver.RunMigration(sql, nm.Version)
	if err != nil {
		return Migration{}, err
	}

	return nm, err
}

func readUp(migration Migration) (string, error) {
	data, err := embeddedMigrationsFS.ReadFile(migration.upFile)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Down migrates a database to an 'Empty Database' state
func Down(migrations []Migration, driver MigrationDriver) error {

	for {
		mig, err := StepDown(migrations, driver)
		if err != nil {
			return err
		}
		if mig.Version == 0 {
			return nil
		}
	}

}

// StepDown executes the current version's down.sql file and returns the resulting current migration.
func StepDown(migrations []Migration, driver MigrationDriver) (Migration, error) {
	currMigration, err := Curr(migrations, driver)
	if err != nil {
		return Migration{}, err
	}

	if currMigration.Version == 0 {
		return Migration{}, errors.New("empty database - cannot migrate down any further")
	}

	prevMigration, err := Prev(migrations, currMigration.Version)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			prevMigration = emptyMigration
		} else {
			return Migration{}, err
		}
	}

	_, sql, err := readDown(currMigration)
	if err != nil {
		return Migration{}, err
	}

	err = driver.RunMigration(sql, prevMigration.Version)
	if err != nil {
		return Migration{}, err
	}

	return prevMigration, nil
}

func readDown(migration Migration) (Migration, string, error) {
	data, err := embeddedMigrationsFS.ReadFile(migration.downFile)
	if err != nil {
		return Migration{}, "", err
	}
	return migration, string(data), nil
}

func IsLatest(migrations []Migration, driver MigrationDriver) (bool, error) {
	dbv, err := driver.Version()
	if err != nil {
		return false, fmt.Errorf("failed to find database version: %w", err)
	}

	lv, err := Latest(migrations)
	if err != nil {
		return false, fmt.Errorf("failed to find last configured migration: %w", err)
	}

	return dbv == lv.Version, nil

}
