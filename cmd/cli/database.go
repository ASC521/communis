package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ASC521/communis/config"
	"github.com/ASC521/communis/dbx"
	"github.com/ASC521/communis/dbx/sqlitex"
)

func MigrateCMD(conf *config.Config, args []string, dbPath, migrationsDir string) error {
	migFlags := flag.NewFlagSet("migrate", flag.ExitOnError)
	migFlags.Usage = func() {
		fmt.Fprint(os.Stdout, "Usage: communis [global options] database migrate <subcommand>\n\n")
		fmt.Fprint(os.Stdout, "\nAvailable Commands:\n")
		fmt.Fprint(os.Stdout, "down        migrate database down to an empty database\n")
		fmt.Fprint(os.Stdout, "list        list all migrations\n")
		fmt.Fprint(os.Stdout, "step-down   migrate database to previous version\n")
		fmt.Fprint(os.Stdout, "step-up     migrate database to next version\n")
		fmt.Fprint(os.Stdout, "up          migrate database up to latest version\n\n")
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

	db, err := sqlitex.NewSQLiteDB(ctx, dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	mig, err := dbx.NewSQLiteMigrator(ctx, db, migrationsDir)
	if err != nil {
		return err
	}

	cmd, _ := args[0], args[1:]
	switch cmd {
	case "down":
		return mig.Down()
	case "step-down":
		pm, err := mig.StepDown()
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "database migrated to version %v - %s\n", pm.Version, pm.Name)
		return nil
	case "step-up":
		nm, err := mig.StepUp()
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "database migrated to version %v - %s\n", nm.Version, nm.Name)
		return nil
	case "up":
		return mig.Up()
	case "list":

		cv, err := mig.Version()
		if err != nil {
			return err
		}

		if cv == 0 {
			fmt.Fprint(os.Stdout, "*** 0000 - empty database ***\n")
		} else {
			fmt.Fprint(os.Stdout, "0000 - empty database\n")
		}
		for _, m := range mig.Migrations {
			if cv == m.Version {
				fmt.Fprintf(os.Stdout, "*** %04d - %s ***\n", m.Version, m.Name)
			} else {
				fmt.Fprintf(os.Stdout, "%04d - %s\n", m.Version, m.Name)
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
	notesDBName := dbFlags.String("notes-db-name", "", "file name of notes database to operate on")

	dbFlags.Usage = func() {
		fmt.Fprint(os.Stdout, "Usage: communis [global options] database [database options] <subcommand> [command options]\n\n")
		fmt.Fprint(os.Stdout, "Database Options:\n")
		dbFlags.PrintDefaults()
		fmt.Fprint(os.Stdout, "\nAvailable Commands:\n")
		fmt.Fprint(os.Stdout, "bootstrap    create and migrate up a database\n")
		fmt.Fprint(os.Stdout, "migrate      manage database schema\n\n")

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
		dbPath = filepath.Join(conf.SQLite.DBDirectory, conf.SQLite.IndexDBFileName)
		migrationsDir = conf.SQLite.IndexDBMigrations
	case "notes":

		if *notesDBName == "" {
			return fmt.Errorf("notes-db-name is required.")
		}

		dbPath = filepath.Join(conf.SQLite.DBDirectory, *notesDBName)
		migrationsDir = conf.SQLite.NotesDBMigrations
	default:
		return fmt.Errorf("database type %s is not valid.  Valid options index or notes", *dbType)
	}

	cmd, subArgs := args[0], args[1:]
	switch cmd {
	case "bootstrap":
		ctx := context.Background()
		db, err := sqlitex.NewSQLiteDB(ctx, dbPath)
		if err != nil {
			return err
		}
		defer db.Close()

		mig, err := dbx.NewSQLiteMigrator(ctx, db, migrationsDir)
		if err != nil {
			return err
		}

		return mig.Bootstrap()

	case "migrate":
		return MigrateCMD(conf, subArgs, dbPath, migrationsDir)
	default:
		return fmt.Errorf("command %s is not supported", cmd)
	}

}
