package main

import (
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/ASC521/communis/config"
	"github.com/BurntSushi/toml"
)

func main() {

	globalFlags := flag.NewFlagSet("global", flag.ExitOnError)
	cfp := globalFlags.String("config-file", "./communis.toml", "location of configuration toml file")
	verboseLogF := globalFlags.Bool("verbose-logging", false, "enable verbose logging")
	sqliteBT := globalFlags.Int("sqlite-busy-timeout", 5000, "busy_timeout pragma setting")
	sqliteCS := globalFlags.Int("sqlite-cache-size", 2000, "cache_size pragma setting")
	sqliteFP := globalFlags.String("sqlite-file-path", "", "location of database file")
	sqliteFK := globalFlags.Bool("sqlite-foreign-keys", true, "foreign_keys pragma setting")
	sqliteJM := globalFlags.String("sqlite-journal-mode", "WAL", "journal_mode pragma setting - options: DELETE | TRUNCATE | PERSIST | MEMORY | WAL | OFF")
	sqliteSync := globalFlags.String("sqlite-synchronous", "NORMAL", "synchronous pragma setting - options: OFF | NORMAL | FULL | EXTRA")
	sqliteTS := globalFlags.String("sqlite-temp-store", "MEMORY", "temp_store pragma setting - options: DEFAULT | FILE | MEMORY")

	globalFlags.Usage = func() {
		fmt.Fprint(os.Stdout, "Usage: communis [global options] <command> [command options]\n\n")
		fmt.Fprint(os.Stdout, "Global Options:\n")
		globalFlags.PrintDefaults()
		fmt.Fprint(os.Stdout, "\nAvailable Commands:\n")
		fmt.Fprint(os.Stdout, "database    create and manage database\n")
		fmt.Fprint(os.Stdout, "web         run web server\n\n")
	}

	globalFlags.Parse(os.Args[1:])

	args := globalFlags.Args()
	if len(args) == 0 {
		globalFlags.Usage()
		os.Exit(0)
	}

	var conf config.Config
	_, err := os.Stat(*cfp)
	if errors.Is(err, os.ErrNotExist) {
		slog.Debug(fmt.Sprintf("config file does not exist at %s, skipping loading", *cfp), "config-location", *cfp)
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "error occured finding config file: %v\n", err)
	} else {
		md, err := toml.DecodeFile(*cfp, &conf)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to parse toml file: %v", err.Error())
			os.Exit(1)
		}

		if len(md.Undecoded()) > 0 {
			fmt.Fprintf(os.Stderr, "unknown configuration keys: %v", md.Undecoded())
			os.Exit(1)
		}
	}

	if verboseLogF != nil {
		conf.VerboseLogging = *verboseLogF
	}

	if sqliteBT != nil {
		conf.SQLite.BusyTimeout = *sqliteBT
	}
	if sqliteCS != nil {
		conf.SQLite.CacheSize = *sqliteCS
	}
	if sqliteFP != nil {
		conf.SQLite.FilePath = *sqliteFP
	}
	if sqliteFK != nil {
		conf.SQLite.ForeignKeys = *sqliteFK
	}
	if sqliteJM != nil {
		conf.SQLite.JournalMode = *sqliteJM
	}
	if sqliteSync != nil {
		conf.SQLite.Synchronous = *sqliteSync
	}
	if sqliteTS != nil {
		conf.SQLite.TempStore = *sqliteTS
	}

	cmd, subArgs := args[0], args[1:]
	switch cmd {
	case "database":
		err = DatabaseCMD(&conf, subArgs)
	case "web":
		err = WebCMD(&conf, subArgs)
	default:
		fmt.Fprintln(os.Stderr, fmt.Errorf("%s is not a valid command", cmd))
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

}
