package db

import (
	"database/sql"
	_ "modernc.org/sqlite"
)

func Open() (*sql.DB, error) {
	return sql.Open("sqlite", "./mscale.db")
}

