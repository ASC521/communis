package config

import (
	"fmt"

	"github.com/ASC521/communis/dbx/sqlitex"
)

type SQLite struct {
	DBDirectory       string `toml:"db-directory"`
	BusyTimeout       int    `toml:"busy-timeout"`
	CacheSize         int    `toml:"cache-size"`
	ForeignKeys       bool   `toml:"foreign-keys"`
	JournalMode       string `toml:"journal-mode"`
	Synchronous       string `toml:"synchronous"`
	TempStore         string `toml:"temp-store"`
	IndexDBFileName   string
	IndexDBMigrations string
	NotesDBMigrations string
}

func ValidSQLite(s SQLite) error {
	_, err := sqlitex.JournalModeFromString(s.JournalMode)
	if err != nil {
		return err
	}

	_, err = sqlitex.SynchronousFromString(s.Synchronous)
	if err != nil {
		return err
	}

	_, err = sqlitex.TempStoreFromString(s.TempStore)
	if err != nil {
		return err
	}

	if s.DBDirectory == "" {
		return fmt.Errorf("filepath is a required configuration")
	}

	return nil
}

type Web struct {
	Host                string   `toml:"host"`
	Port                uint     `toml:"port"`
	LoggingIgnoredPaths []string `toml:"logging-ignored-paths"`
}

type Config struct {
	SQLite         SQLite `toml:"sqlite"`
	Web            Web    `toml:"web"`
	VerboseLogging bool   `toml:"verbose-logging"`
}

func DefaultConfig() *Config {
	return &Config{
		SQLite: SQLite{
			DBDirectory:       "",
			BusyTimeout:       5000,
			CacheSize:         2000,
			ForeignKeys:       true,
			JournalMode:       "WAL",
			Synchronous:       "NORMAL",
			TempStore:         "MEMORY",
			IndexDBFileName:   "index.db",
			IndexDBMigrations: "sql/index-db",
			NotesDBMigrations: "sql/notes-db",
		},
		Web: Web{
			Host:                "localhost",
			Port:                6789,
			LoggingIgnoredPaths: []string{},
		},
		VerboseLogging: false,
	}
}
