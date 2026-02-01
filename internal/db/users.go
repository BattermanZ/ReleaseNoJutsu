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

func (db *DB) EnsureUser(chatID int64, isAdmin bool) error {
	adminVal := 0
	if isAdmin {
		adminVal = 1
	}
	_, err := db.Exec(`
		INSERT OR IGNORE INTO users (chat_id, is_admin, created_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
	`, chatID, adminVal)
	if err != nil {
		return err
	}
	if isAdmin {
		_, err = db.Exec("UPDATE users SET is_admin = 1 WHERE chat_id = ?", chatID)
	}
	return err
}

func (db *DB) IsUserAuthorized(chatID int64) (bool, bool, error) {
	var isAdmin int
	err := db.QueryRow("SELECT is_admin FROM users WHERE chat_id = ?", chatID).Scan(&isAdmin)
	if err == sql.ErrNoRows {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	return true, isAdmin != 0, nil
}
