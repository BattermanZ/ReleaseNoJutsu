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

func (db *DB) SetUserPendingState(chatID int64, state, payload string) error {
	_, err := db.Exec("UPDATE users SET pending_state = ?, pending_payload = ? WHERE chat_id = ?", state, payload, chatID)
	return err
}

func (db *DB) ClearUserPendingState(chatID int64) error {
	_, err := db.Exec("UPDATE users SET pending_state = NULL, pending_payload = NULL WHERE chat_id = ?", chatID)
	return err
}

func (db *DB) GetUserPendingState(chatID int64) (state string, payload string, hasState bool, err error) {
	var stateVal sql.NullString
	var payloadVal sql.NullString
	err = db.QueryRow("SELECT pending_state, pending_payload FROM users WHERE chat_id = ?", chatID).Scan(&stateVal, &payloadVal)
	if err == sql.ErrNoRows {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, err
	}
	if !stateVal.Valid || stateVal.String == "" {
		return "", "", false, nil
	}
	if payloadVal.Valid {
		return stateVal.String, payloadVal.String, true, nil
	}
	return stateVal.String, "", true, nil
}
