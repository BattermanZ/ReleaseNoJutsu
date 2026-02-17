package db

import "database/sql"

func (db *DB) hasColumn(tableName, columnName string) (bool, error) {
	rows, err := db.Query("PRAGMA table_info(" + tableName + ")")
	if err != nil {
		return false, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var cid int
		var name string
		var ctype string
		var notNull int
		var dfltValue any
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dfltValue, &pk); err != nil {
			return false, err
		}
		if name == columnName {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return false, nil
}

func (db *DB) hasTable(tableName string) (bool, error) {
	row := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name = ?", tableName)
	var name string
	if err := row.Scan(&name); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return name == tableName, nil
}
