package main

import (
	"flag"
	"fmt"
	"os"
)

func migrateUsage() {
	fmt.Fprintln(os.Stdout, "communis database migrate:")
	fmt.Fprintln(os.Stdout, "    up     migrate database up to latest version")
	fmt.Fprintln(os.Stdout, "    down   migrate database down to previous version")
	fmt.Fprintln(os.Stdout, "    list   list migrations")
}

func runMigrate(args []string) error {
	return nil
}

func databaseUsage() {
	fmt.Fprintln(os.Stdout, "communis database:")
	fmt.Fprintln(os.Stdout, "    migrate  manage migrations of database schema")
}

func runDatabase(args []string) error {

	flag := flag.NewFlagSet("database", flag.ExitOnError)
	flag.Usage = usage
	err := flag.Parse(args)
	if err != nil {
		return err
	}

	if len(args) == 0 {
		databaseUsage()
		return nil
	}

	cmd, subArgs := args[0], args[1:]
	switch cmd {
	case "migrate":
		runMigrate(subArgs)
	default:

	}

	return nil
}
