package store

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	// This dashboard is a single process; force a single connection so SQLite's
	// file-level locking never has to arbitrate between pooled connections.
	db.SetMaxOpenConns(1)

	if err := migrate(db); err != nil {
		return nil, err
	}

	return &Store{db: db}, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS devices (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL UNIQUE,
	address TEXT NOT NULL,
	kind TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT 'online',
	failed_polls INTEGER NOT NULL DEFAULT 0,
	first_seen TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	last_seen TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS services (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	device_id INTEGER NOT NULL REFERENCES devices(id),
	name TEXT NOT NULL,
	type TEXT NOT NULL,
	version TEXT NOT NULL DEFAULT '',
	watched INTEGER NOT NULL DEFAULT 0,
	last_state TEXT NOT NULL DEFAULT '',
	last_polled_at TIMESTAMP,
	UNIQUE(device_id, name)
);

CREATE TABLE IF NOT EXISTS host_stats (
	device_id INTEGER PRIMARY KEY REFERENCES devices(id),
	cpu_user REAL NOT NULL DEFAULT 0,
	cpu_system REAL NOT NULL DEFAULT 0,
	cpu_idle REAL NOT NULL DEFAULT 0,
	mem_used INTEGER NOT NULL DEFAULT 0,
	mem_free INTEGER NOT NULL DEFAULT 0,
	mem_total INTEGER NOT NULL DEFAULT 0,
	polled_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS host_temperatures (
	device_id INTEGER NOT NULL REFERENCES devices(id),
	sensor TEXT NOT NULL,
	celsius REAL NOT NULL DEFAULT 0,
	UNIQUE(device_id, sensor)
);
`)
	return err
}

func (s *Store) Close() error {
	return s.db.Close()
}
