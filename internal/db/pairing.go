package db

import "time"

func (db *DB) CreatePairingCode(code string, adminChatID int64, expiresAt time.Time) error {
	_, err := db.Exec(`
		INSERT INTO pairing_codes (code, expires_at, created_by_admin, created_at)
		VALUES (?, ?, ?, ?)
	`, code, expiresAt.UTC(), adminChatID, time.Now().UTC())
	return err
}

func (db *DB) RedeemPairingCode(code string, chatID int64) (bool, error) {
	tx, err := db.Begin()
	if err != nil {
		return false, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		} else {
			_ = tx.Commit()
		}
	}()

	var usedAt *time.Time
	var expiresAt time.Time
	err = tx.QueryRow(`
		SELECT used_at, expires_at
		FROM pairing_codes
		WHERE code = ?
	`, code).Scan(&usedAt, &expiresAt)
	if err != nil {
		return false, err
	}

	if usedAt != nil {
		return false, nil
	}
	if time.Now().UTC().After(expiresAt.UTC()) {
		return false, nil
	}

	_, err = tx.Exec(`
		UPDATE pairing_codes
		SET used_by_chat_id = ?, used_at = ?
		WHERE code = ? AND used_at IS NULL
	`, chatID, time.Now().UTC(), code)
	if err != nil {
		return false, err
	}

	return true, nil
}
