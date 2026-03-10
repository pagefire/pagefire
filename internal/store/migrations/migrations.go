package migrations

import "embed"

//go:embed sqlite/*.sql
var SQLiteMigrations embed.FS

//go:embed postgres/*.sql
var PostgresMigrations embed.FS
