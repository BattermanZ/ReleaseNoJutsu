package db

import "database/sql"

func (db *DB) GetAllUsers() (*sql.Rows, error) {
	return db.Query("SELECT chat_id FROM users")
}

func (db *DB) ListUsers() ([]int64, error) {
	rows, err := db.GetAllUsers()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var chatIDs []int64
	for rows.Next() {
		var chatID int64
		if err := rows.Scan(&chatID); err != nil {
			return nil, err
		}
		chatIDs = append(chatIDs, chatID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return chatIDs, nil
}

func (db *DB) EnsureUser(chatID int64) error {
	dbMutex.Lock()
	defer dbMutex.Unlock()

	_, err := db.Exec("INSERT OR IGNORE INTO users (chat_id) VALUES (?)", chatID)
	return err
}
