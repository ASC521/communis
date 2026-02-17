package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ASC521/communis/config"
	"github.com/ASC521/communis/dbx/sqlitex"
	"github.com/ASC521/communis/repository/sqlite"
)

func initAdmin(conf *config.Config, args []string) error {
	adminFlags := flag.NewFlagSet("admin", flag.ExitOnError)
	userNameF := adminFlags.String("username", "", "admin user name")
	passwordF := adminFlags.String("password", "", "admin password")
	adminFlags.Usage = func() {
		fmt.Fprint(os.Stdout, "Usage: communis [global options] user init-admin [subcommand options]\n\n")
		fmt.Fprint(os.Stdout, "Options:\n")
		adminFlags.PrintDefaults()
		fmt.Fprint(os.Stdout, "\n\n")
	}

	err := adminFlags.Parse(args)
	if err != nil {
		return err
	}

	if *userNameF == "" {
		return fmt.Errorf("username cannot be empty")
	}

	if *passwordF == "" {
		return fmt.Errorf("password cannot be empty")
	}

	indexDBFP := filepath.Join(conf.SQLite.DBDirectory, conf.SQLite.IndexDBFileName)
	indexDB, err := sqlitex.NewSQLiteDB(indexDBFP,
		sqlitex.WithBusyTimeout(conf.SQLite.BusyTimeout),
		sqlitex.WithCacheSize(conf.SQLite.CacheSize),
		sqlitex.WithForeignKeys(conf.SQLite.ForeignKeys),
		sqlitex.WithJournalMode(conf.SQLite.JournalMode),
		sqlitex.WithSynchronous(conf.SQLite.Synchronous),
		sqlitex.WithTempStore(conf.SQLite.TempStore),
	)
	if err != nil {
		return err
	}
	defer indexDB.Close()

	indexRepository := sqlite.NewIndexDBRepository(indexDB)

	_, err = indexRepository.CreateAdminUser(context.Background(), *userNameF, *passwordF)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "Admin user %s successfully created\n", *userNameF)
	return nil

}

func UserCMD(conf *config.Config, args []string) error {

	userFlags := flag.NewFlagSet("user", flag.ExitOnError)
	userFlags.Usage = func() {
		fmt.Fprint(os.Stdout, "Usage: communis [global options] user <subcommand>\n\n")
		fmt.Fprint(os.Stdout, "Available Commands:\n")
		fmt.Fprint(os.Stdout, "init-admin   adds admin user on new database\n\n")
	}

	err := userFlags.Parse(args)
	if err != nil {
		return err
	}

	if len(args) == 0 {
		userFlags.Usage()
		return nil
	}

	cmd, subArgs := args[0], args[1:]
	switch cmd {
	case "init-admin":
		return initAdmin(conf, subArgs)
	default:
		fmt.Fprintf(os.Stderr, "command %s is not supported\n", cmd)
		os.Exit(1)
	}

	return nil
}
