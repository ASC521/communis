package config

import (
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/ASC521/communis/dbx/sqlitex"
)

const (
	LinuxOS        = "linux"
	MacOS          = "darwin"
	WindowsOS      = "windows"
	ConfigFileName = "config.toml"
	AppName        = "communis"
)

type SQLite struct {
	BusyTimeout       int    `toml:"busy-timeout"`
	CacheSize         int    `toml:"cache-size"`
	ForeignKeys       bool   `toml:"foreign-keys"`
	JournalMode       string `toml:"journal-mode"`
	Synchronous       string `toml:"synchronous"`
	TempStore         string `toml:"temp-store"`
	IndexDBFileName   string `toml:"-"`
	IndexDBMigrations string `toml:"-"`
	NotesDBMigrations string `toml:"-"`
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

	return nil
}

type RegexPattern struct {
	Pattern string
}

func (r *RegexPattern) MarshalTOML() ([]byte, error) {
	var b []byte
	return fmt.Appendf(b, "'%s'", r.Pattern), nil
}

func (r *RegexPattern) UnmarshalText(text []byte) error {
	r.Pattern = string(text)
	return nil
}

type Web struct {
	Host                string         `toml:"host"`
	Port                uint           `toml:"port"`
	Debug               bool           `toml:"debug"`
	LoggingIgnoredPaths []RegexPattern `toml:"logging-ignored-paths"`
}

type Config struct {
	DataDirectory  string `toml:"data-directory"`
	FileLocation   string `toml:"-"`
	SQLite         SQLite `toml:"sqlite"`
	Web            Web    `toml:"web"`
	VerboseLogging bool   `toml:"verbose-logging"`
}

func DefaultConfig() (*Config, error) {

	dd, err := DefaultDataDirectory()
	if err != nil {
		return nil, err
	}

	fl, err := DefaultFileLocation()
	if err != nil {
		return nil, err
	}

	return &Config{
		DataDirectory: dd,
		FileLocation:  fl,
		SQLite: SQLite{
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
			Host: "localhost",
			Port: 6789,
			LoggingIgnoredPaths: []RegexPattern{
				{Pattern: `\/static\/.*`},
			},
			Debug: false,
		},
		VerboseLogging: false,
	}, nil
}

func DefaultDataDirectory() (string, error) {
	switch runtime.GOOS {
	case LinuxOS:
		return filepath.Join(string(filepath.Separator), "var", "lib", AppName), nil
	case WindowsOS:
		return filepath.Join(string(filepath.Separator), "%PROGRAMDATA%", AppName), nil
	case MacOS:
		return filepath.Join(string(filepath.Separator), "var", "db", AppName), nil
	default:
		return "", fmt.Errorf("%s operating system is not supported", runtime.GOOS)
	}

}

func DefaultFileLocation() (string, error) {
	switch runtime.GOOS {
	case LinuxOS, MacOS:
		return filepath.Join(string(filepath.Separator), "etc", AppName, ConfigFileName), nil
	case WindowsOS:
		return filepath.Join(string(filepath.Separator), "%PROGRAMDATA%", AppName, ConfigFileName), nil
	default:
		return "", fmt.Errorf("%s operating system is not supported", runtime.GOOS)
	}

}
