package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/ASC521/communis/config"
	"github.com/ASC521/communis/dbx"
	"github.com/ASC521/communis/dbx/sqlitex"
)

func MigrateCMD(conf *config.Config, args []string) error {
	migFlags := flag.NewFlagSet("migrate", flag.ExitOnError)
	migFlags.Usage = func() {
		fmt.Fprint(os.Stdout, "Usage: communis [global options] database migrate <subcommand>\n\n")
		fmt.Fprint(os.Stdout, "\nAvailable Commands:\n")
		fmt.Fprint(os.Stdout, "down    migrate database down to an empty database\n")
		fmt.Fprint(os.Stdout, "list    list all migrations\n")
		fmt.Fprint(os.Stdout, "up      migrate database up to latest version\n\n")
	}
	err := migFlags.Parse(args)
	if err != nil {
		return err
	}

	if len(args) == 0 {
		migFlags.Usage()
		return nil
	}

	cmd, _ := args[0], args[1:]
	switch cmd {
	case "down":
		ctx := context.Background()
		db, err := sqlitex.NewSQLiteDB(ctx, conf.SQLite.FilePath)
		if err != nil {
			return err
		}
		defer db.Close()

		mig, err := dbx.NewSQLiteMigrator(ctx, db)
		if err != nil {
			return err
		}

		return mig.Down()
	case "up":
		ctx := context.Background()
		db, err := sqlitex.NewSQLiteDB(ctx, conf.SQLite.FilePath)
		if err != nil {
			return err
		}
		defer db.Close()

		mig, err := dbx.NewSQLiteMigrator(ctx, db)
		if err != nil {
			return err
		}

		return mig.Up()
	case "list":
		ctx := context.Background()
		db, err := sqlitex.NewSQLiteDB(ctx, conf.SQLite.FilePath)
		if err != nil {
			return err
		}
		defer db.Close()

		mig, err := dbx.NewSQLiteMigrator(ctx, db)
		if err != nil {
			return err
		}

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

	dbFlags.Usage = func() {
		fmt.Fprint(os.Stdout, "Usage: communis [global options] database <subcommand> [command options]\n\n")
		fmt.Fprint(os.Stdout, "\nAvailable Commands:\n")
		fmt.Fprint(os.Stdout, "bootstrap    create and migrate up a database\n")
		fmt.Fprint(os.Stdout, "migrate      manage database schema\n\n")

	}
	err := dbFlags.Parse(args)
	if err != nil {
		return err
	}

	if len(args) == 0 {
		dbFlags.Usage()
		return nil
	}

	cmd, subArgs := args[0], args[1:]
	switch cmd {
	case "bootstrap":
		ctx := context.Background()
		db, err := sqlitex.NewSQLiteDB(ctx, conf.SQLite.FilePath)
		if err != nil {
			return err
		}
		defer db.Close()

		mig, err := dbx.NewSQLiteMigrator(ctx, db)
		if err != nil {
			return err
		}

		return mig.Bootstrap()

	case "migrate":
		MigrateCMD(conf, subArgs)
	default:
		return fmt.Errorf("command %s is not supported", cmd)
	}

	return nil
}
