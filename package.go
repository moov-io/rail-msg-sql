package railmsgsql

import (
	"embed"
)

//go:embed migrations/*.sql
var SqliteMigrations embed.FS
