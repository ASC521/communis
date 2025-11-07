package config

type SQLite struct {
	FilePath    string `toml:"filepath"`
	BusyTimeout int    `toml:"busy-timeout"`
	CacheSize   int    `toml:"cache-size"`
	ForeignKeys bool   `toml:"foreign-keys"`
	JournalMode string `toml:"journal-mode"`
	Synchronous string `toml:"synchronous"`
	TempStore   string `toml:"temp-store"`
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
