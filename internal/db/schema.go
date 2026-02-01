package db

// CreateTables creates the necessary tables in the database.
func (db *DB) CreateTables() error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS manga (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			mangadex_id TEXT NOT NULL,
			title TEXT NOT NULL,
			is_manga_plus INTEGER NOT NULL DEFAULT 0,
			last_checked TIMESTAMP,
			last_seen_at TIMESTAMP,
			last_read_number REAL,
			unread_count INTEGER DEFAULT 0,
			FOREIGN KEY (user_id) REFERENCES users (chat_id)
		);

		CREATE TABLE IF NOT EXISTS chapters (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			manga_id INTEGER,
			chapter_number TEXT NOT NULL,
			title TEXT,
			published_at TIMESTAMP,
			readable_at TIMESTAMP,
			created_at TIMESTAMP,
			updated_at TIMESTAMP,
			FOREIGN KEY (manga_id) REFERENCES manga (id)
		);

		CREATE TABLE IF NOT EXISTS users (
			chat_id INTEGER PRIMARY KEY,
			is_admin INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS pairing_codes (
			code TEXT PRIMARY KEY,
			expires_at TIMESTAMP NOT NULL,
			used_by_chat_id INTEGER,
			used_at TIMESTAMP,
			created_by_admin INTEGER NOT NULL,
			created_at TIMESTAMP NOT NULL
		);

		CREATE TABLE IF NOT EXISTS system_status (
			key TEXT PRIMARY KEY,
			last_update TIMESTAMP
		);
	`)
	return err
}
