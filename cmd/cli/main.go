//go:generate go run . generate css -dark-theme dracula -light-theme tango
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/ASC521/communis/config"
	"github.com/BurntSushi/toml"
	"github.com/mitchellh/go-homedir"
)

var (
	version          = Version()
	containerVersion = ContainerVersion()
)

func main() {

	defaultConfLoc := config.DefaultFileLocation()

	defaultDataDir := config.DefaultDataDirectory()

	globalFlags := flag.NewFlagSet("global", flag.ExitOnError)
	cfp := globalFlags.String("config-file", defaultConfLoc, "location of configuration toml file")
	debugF := globalFlags.Bool("debug", false, "run application in debug mode (default: false)")
	dataDirF := globalFlags.String("data-directory", defaultDataDir, "location of persistent data storage")
	sqliteBT := globalFlags.Int("sqlite-busy-timeout", 0, "busy_timeout pragma setting (default 5000)")
	sqliteCS := globalFlags.Int("sqlite-cache-size", 0, "cache_size pragma setting (default 2000)")
	sqliteFK := globalFlags.Bool("sqlite-foreign-keys", false, "foreign_keys pragma setting (default true)")
	sqliteJM := globalFlags.String("sqlite-journal-mode", "", "journal_mode pragma setting - options: DELETE | TRUNCATE | PERSIST | MEMORY | WAL | OFF (default \"WAL\")")
	sqliteSync := globalFlags.String("sqlite-synchronous", "", "synchronous pragma setting - options: OFF | NORMAL | FULL | EXTRA (default\"NORMAL\")")
	sqliteTS := globalFlags.String("sqlite-temp-store", "", "temp_store pragma setting - options: DEFAULT | FILE | MEMORY (default \"MEMORY\")")

	globalFlags.Usage = func() {
		fmt.Fprint(os.Stderr, "Usage: communis [global options] <command> [command options]\n\n")
		fmt.Fprint(os.Stderr, "Global Options:\n")
		globalFlags.PrintDefaults()
		fmt.Fprint(os.Stderr, "\nAvailable Commands:\n")
		fmt.Fprint(os.Stderr, "database          create and manage database\n")
		fmt.Fprint(os.Stderr, "generate          generate files to support running application\n")
		fmt.Fprint(os.Stderr, "uninstall         uninstall application on linux\n")
		fmt.Fprint(os.Stderr, "serve             run web server\n")
		fmt.Fprint(os.Stderr, "version           print version of application\n\n")
	}

	globalFlags.Parse(os.Args[1:])

	args := globalFlags.Args()
	if len(args) == 0 {
		globalFlags.Usage()
		os.Exit(0)
	}

	resCFP, err := homedir.Expand(*cfp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to resolve home directory path for config file: %s", err.Error())
		os.Exit(1)
	}
	resCFP, err = filepath.Abs(resCFP)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to resolve relative path of the config file: %s", err.Error())
		os.Exit(1)
	}
	conf, err := config.DefaultConfig()
	if err != nil {
		fmt.Fprint(os.Stderr, err.Error())
	}
	_, err = os.Stat(resCFP)
	if errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(os.Stderr, "config file does not exist at %s, skipping loading\n", resCFP)
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "error occured finding config file: %v\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "loading config file from %s\n", resCFP)
		tomlMetaData, err := toml.DecodeFile(resCFP, conf)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to parse toml file: %v", err.Error())
			os.Exit(1)
		}

		if len(tomlMetaData.Undecoded()) > 0 {
			fmt.Fprintf(os.Stderr, "unknown configuration keys: %v", tomlMetaData.Undecoded())
			os.Exit(1)
		}
		conf.FileLocation = resCFP
	}

	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "debug":
			conf.Debug = *debugF
		case "sqlite-busy-timeout":
			conf.SQLite.BusyTimeout = *sqliteBT
		case "sqlite-cache-size":
			conf.SQLite.CacheSize = *sqliteCS
		case "data-directory":
			conf.DataDirectory = *dataDirF
		case "sqlite-foreign-keys":
			conf.SQLite.ForeignKeys = *sqliteFK
		case "sqlite-synchronous":
			conf.SQLite.Synchronous = *sqliteSync
		case "sqlite-temp-store":
			conf.SQLite.TempStore = *sqliteTS
		case "sqlite-journal-mode":
			conf.SQLite.JournalMode = *sqliteJM
		default:
			fmt.Fprintf(os.Stderr, "cli flag %s ignored", f.Name)
		}
	})

	cmd, subArgs := args[0], args[1:]
	switch cmd {
	case "database":
		err = DatabaseCMD(conf, subArgs)
	case "serve":
		err = ServeCMD(conf, subArgs)
	case "generate":
		err = GenerateCMD(conf, subArgs)
	case "uninstall":
		err = UninstallCMD(conf, subArgs)
	case "version":
		err = VersionCMD(subArgs)
	default:
		fmt.Fprintln(os.Stderr, fmt.Errorf("%s is not a valid command", cmd))
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

}

func VersionCMD(args []string) error {
	verFlags := flag.NewFlagSet("version", flag.ExitOnError)
	container := verFlags.Bool("container", false, "print version in container acceptable format")
	err := verFlags.Parse(args)
	if err != nil {
		return err
	}
	if *container {
		fmt.Fprintln(os.Stdout, containerVersion)
		return nil
	}

	fmt.Fprintln(os.Stdout, version)
	return nil
}

func Version() string {
	bi, ok := debug.ReadBuildInfo()
	if ok {
		return bi.Main.Version
	}
	return ""
}

func ContainerVersion() string {
	bi, ok := debug.ReadBuildInfo()
	if ok {
		return strings.ReplaceAll(bi.Main.Version, "+", "-")
	}
	return ""
}
