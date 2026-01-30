package main

import (
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/ASC521/communis/config"
	"github.com/BurntSushi/toml"
	"github.com/mitchellh/go-homedir"
)

func main() {

	globalFlags := flag.NewFlagSet("global", flag.ExitOnError)
	cfp := globalFlags.String("config-file", "./communis.toml", "location of configuration toml file")
	verboseLogF := globalFlags.Bool("verbose-logging", false, "enable verbose logging")
	sqliteBT := globalFlags.Int("sqlite-busy-timeout", 0, "busy_timeout pragma setting (default 5000)")
	sqliteCS := globalFlags.Int("sqlite-cache-size", 0, "cache_size pragma setting (default 2000)")
	sqliteDBDir := globalFlags.String("sqlite-directory", "", "location of directory containing sqlite database files")
	sqliteFK := globalFlags.Bool("sqlite-foreign-keys", false, "foreign_keys pragma setting (default true)")
	sqliteJM := globalFlags.String("sqlite-journal-mode", "", "journal_mode pragma setting - options: DELETE | TRUNCATE | PERSIST | MEMORY | WAL | OFF (default \"WAL\")")
	sqliteSync := globalFlags.String("sqlite-synchronous", "", "synchronous pragma setting - options: OFF | NORMAL | FULL | EXTRA (default\"NORMAL\")")
	sqliteTS := globalFlags.String("sqlite-temp-store", "", "temp_store pragma setting - options: DEFAULT | FILE | MEMORY (default \"MEMORY\")")

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

	resCFP, err := homedir.Expand(*cfp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to resolve home directory path for config file")
		os.Exit(1)
	}
	resCFP, err = filepath.Abs(resCFP)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to resolve relative path of the config file")
	}
	conf := config.DefaultConfig()
	_, err = os.Stat(resCFP)
	if errors.Is(err, os.ErrNotExist) {
		slog.Info(fmt.Sprintf("config file does not exist at %s, skipping loading", resCFP), "config-location", resCFP)
		fmt.Fprintf(os.Stdout, "config file does not exist at %s, skipping loading", resCFP)
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "error occured finding config file: %v\n", err)
	} else {
		fmt.Fprintf(os.Stdout, "loading config file from %s\n", resCFP)
		md, err := toml.DecodeFile(resCFP, conf)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to parse toml file: %v", err.Error())
			os.Exit(1)
		}

		if len(md.Undecoded()) > 0 {
			fmt.Fprintf(os.Stderr, "unknown configuration keys: %v", md.Undecoded())
			os.Exit(1)
		}
	}

	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "verbose-logging":
			conf.VerboseLogging = *verboseLogF
		case "sqlite-busy-timeout":
			conf.SQLite.BusyTimeout = *sqliteBT
		case "sqlite-cache-size":
			conf.SQLite.CacheSize = *sqliteCS
		case "sqlite-file-path":
			conf.SQLite.DBDirectory = *sqliteDBDir
		case "sqlite-foreign-keys":
			conf.SQLite.ForeignKeys = *sqliteFK
		case "sqlite-synchronous":
			conf.SQLite.Synchronous = *sqliteSync
		case "sqlite-temp-store":
			conf.SQLite.TempStore = *sqliteTS
		case "sqlite-journal-mode":
			conf.SQLite.JournalMode = *sqliteJM
		default:
			fmt.Fprintf(os.Stdout, "cli flag %s ignored", f.Name)
		}
	})

	cmd, subArgs := args[0], args[1:]
	switch cmd {
	case "database":
		err = DatabaseCMD(conf, subArgs)
	case "web":
		err = WebCMD(conf, subArgs)
	default:
		fmt.Fprintln(os.Stderr, fmt.Errorf("%s is not a valid command", cmd))
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

}
