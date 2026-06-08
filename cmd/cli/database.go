package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ASC521/communis/config"
	"github.com/ASC521/communis/dbx/migrations"
	"github.com/ASC521/communis/dbx/sqlitex"
)

func MigrateCMD(conf *config.Config, args []string, dbPath, migrationsDir string) error {
	migFlags := flag.NewFlagSet("migrate", flag.ExitOnError)
	migFlags.Usage = func() {
		fmt.Fprint(os.Stderr, "Usage: communis [global options] database migrate <subcommand>\n\n")
		fmt.Fprint(os.Stderr, "\nAvailable Commands:\n")
		fmt.Fprint(os.Stderr, "down        migrate database down to an empty database\n")
		fmt.Fprint(os.Stderr, "list        list all migrations\n")
		fmt.Fprint(os.Stderr, "step-down   migrate database to previous version\n")
		fmt.Fprint(os.Stderr, "step-up     migrate database to next version\n")
		fmt.Fprint(os.Stderr, "up          migrate database up to latest version\n\n")
	}
	err := migFlags.Parse(args)
	if err != nil {
		return err
	}

	if len(args) == 0 {
		migFlags.Usage()
		return nil
	}

	ctx := context.Background()

	db, err := sqlitex.NewSQLiteDB(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	migs, err := migrations.Load(migrationsDir)
	if err != nil {
		return err
	}

	migDriver := sqlitex.NewMigrationDriver(db)

	cmd, _ := args[0], args[1:]
	switch cmd {
	case "down":
		return migrations.Down(ctx, migs, migDriver)
	case "step-down":
		pm, err := migrations.StepDown(ctx, migs, migDriver)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "database migrated to version %v - %s\n", pm.Version, pm.Name)
		return nil
	case "step-up":
		nm, err := migrations.StepUp(ctx, migs, migDriver)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "database migrated to version %v - %s\n", nm.Version, nm.Name)
		return nil
	case "up":
		_, err = migrations.Up(ctx, migs, migDriver)
		return err
	case "list":

		cv, err := migDriver.Version(ctx)
		if err != nil {
			return err
		}

		if cv == 0 {
			fmt.Fprint(os.Stderr, "*** 0000 - empty database ***\n")
		} else {
			fmt.Fprint(os.Stderr, "0000 - empty database\n")
		}
		for _, m := range migs {
			if cv == m.Version {
				fmt.Fprintf(os.Stderr, "*** %04d - %s ***\n", m.Version, m.Name)
			} else {
				fmt.Fprintf(os.Stderr, "%04d - %s\n", m.Version, m.Name)
			}
		}

		return nil
	default:
		return fmt.Errorf("command %s is not support", cmd)
	}
}

func DatabaseCMD(conf *config.Config, args []string) error {

	dbFlags := flag.NewFlagSet("database", flag.ExitOnError)
	dbType := dbFlags.String("db-type", "", "database type to migrate - options: INDEX | NOTES")
	notesUserNumber := dbFlags.String("notes-user-number", "", "user number of notes database to operate on")

	dbFlags.Usage = func() {
		fmt.Fprint(os.Stderr, "Usage: communis [global options] database [database options] <subcommand> [command options]\n\n")
		fmt.Fprint(os.Stderr, "Database Options:\n")
		dbFlags.PrintDefaults()
		fmt.Fprint(os.Stderr, "\nAvailable Commands:\n")
		fmt.Fprint(os.Stderr, "bootstrap    create and migrate up a database\n")
		fmt.Fprint(os.Stderr, "migrate      manage database schema\n\n")

	}

	err := dbFlags.Parse(args)
	if err != nil {
		return err
	}

	args = dbFlags.Args()

	if len(args) == 0 {
		dbFlags.Usage()
		return nil
	}

	dbPath := ""
	migrationsDir := ""
	switch strings.ToLower(*dbType) {
	case "index":
		dbPath = filepath.Join(conf.DataDirectory, conf.SQLite.IndexDBFileName)
		migrationsDir = conf.SQLite.IndexDBMigrations
	case "notes":

		if *notesUserNumber == "" {
			return fmt.Errorf("notes-db-name is required")
		}

		dbPath = filepath.Join(conf.DataDirectory, "user-databases", fmt.Sprintf("%s.db", *notesUserNumber))
		migrationsDir = conf.SQLite.NotesDBMigrations
	default:
		return fmt.Errorf("database type %s is not valid.  Valid options index or notes", *dbType)
	}

	cmd, subArgs := args[0], args[1:]
	switch cmd {
	case "bootstrap":
		ctx := context.Background()
		db, err := sqlitex.NewSQLiteDB(dbPath)
		if err != nil {
			return err
		}
		defer db.Close()

		migs, err := migrations.Load(migrationsDir)
		if err != nil {
			return err
		}

		driver := sqlitex.NewMigrationDriver(db)
		_, err = migrations.Bootstrap(ctx, migs, driver)
		return err

	case "migrate":
		return MigrateCMD(conf, subArgs, dbPath, migrationsDir)
	default:
		return fmt.Errorf("command %s is not supported", cmd)
	}

}
