package main

import (
	"database/sql"
	"log"
)

const schema = `
CREATE TABLE IF NOT EXISTS reports (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    lat          REAL    NOT NULL,
    lon          REAL    NOT NULL,
    diagnosis    TEXT    NOT NULL,
    carrier      TEXT    NOT NULL DEFAULT '',
    app_version  TEXT    NOT NULL DEFAULT '',
    network_type TEXT    NOT NULL DEFAULT 'cellular',
    created_at   INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_reports_latlon    ON reports(lat, lon);
CREATE INDEX IF NOT EXISTS idx_reports_createdat ON reports(created_at);
CREATE INDEX IF NOT EXISTS idx_reports_diagnosis ON reports(diagnosis);
`

func initDB(path string) *sql.DB {
	db, err := sql.Open("sqlite3", path+"?_journal=WAL&_busy_timeout=5000&_synchronous=NORMAL")
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	db.SetMaxOpenConns(1) // SQLite: single writer
	db.SetMaxIdleConns(1)

	if _, err := db.Exec(schema); err != nil {
		log.Fatalf("init schema: %v", err)
	}
	log.Println("DB ready:", path)
	return db
}
