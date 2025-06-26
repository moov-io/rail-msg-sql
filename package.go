package railmsgsql

import (
	"embed"
)

//go:embed configs/config.default.yml
var ConfigDefaults embed.FS

//go:embed migrations/*.sql
var SqliteMigrations embed.FS
