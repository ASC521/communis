package config

import (
	"encoding/json/jsontext"
	"os"
	"path/filepath"

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
	BusyTimeout       int    `json:"busyTimeout"`
	CacheSize         int    `json:"cacheSize"`
	ForeignKeys       bool   `json:"foreignKeys"`
	JournalMode       string `json:"journalMode"`
	Synchronous       string `json:"synchronous"`
	TempStore         string `json:"tempStore"`
	IndexDBFileName   string `json:"-"`
	IndexDBMigrations string `json:"-"`
	NotesDBMigrations string `json:"-"`
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
	Pattern string `json:"pattern"`
}

func (r *RegexPattern) MarshalerTo(enc *jsontext.Encoder) error {
	enc.WriteToken(jsontext.String("pattern"))
	enc.WriteToken(jsontext.String(r.Pattern))
	return nil
}

func (r *RegexPattern) UnmarshalerFrom(dec *jsontext.Decoder) error {
	v, err := dec.ReadValue()
	if err != nil {
		return err
	}

	r.Pattern = string(v)
	return nil
}

type Web struct {
	Host                string         `json:"host"`
	Port                uint           `json:"port"`
	LoggingIgnoredPaths []RegexPattern `json:"loggingIgnoredPaths"`
}

type Config struct {
	DataDirectory          string         `json:"dataDirectory"`
	FileLocation           string         `json:"-"`
	SQLite                 SQLite         `json:"sqlite"`
	WebHost                string         `json:"webHost"`
	WebPort                uint           `json:"webPort"`
	WebLoggingIgnoredPaths []RegexPattern `json:"webLoggingIgnoredPaths"`
	WebEnableHTTPS         bool           `json:"webEnableHttps"`
	WebCert                string         `json:"webCert"`
	WebKey                 string         `json:"webKey"`
	Debug                  bool           `json:"debug"`
}

func DefaultConfig() (*Config, error) {
	dd := DefaultDataDirectory()

	fl := DefaultFileLocation()

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
		WebHost: "0.0.0.0",
		WebPort: 6789,
		WebLoggingIgnoredPaths: []RegexPattern{
			{Pattern: `\/static\/.*`},
		},
		WebEnableHTTPS: false,
		Debug:          false,
	}, nil
}

func DefaultDataDirectory() string {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, AppName)
	}

	return filepath.Join(string(filepath.Separator), "var", "opt", AppName)
}

func DefaultFileLocation() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, ".config", AppName, "config.json")
	}

	return filepath.Join(string(filepath.Separator), "etc", "opt", AppName, "config.json")
}
