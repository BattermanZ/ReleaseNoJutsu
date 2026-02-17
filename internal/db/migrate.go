package db

type migrationFlags struct {
	hasMangaLastReadAt bool
	hasChaptersIsRead  bool
}

func (db *DB) Migrate(adminUserID int64) error {
	return db.withForeignKeysDisabled(func() error {
		flags, err := db.migrateSchema(adminUserID)
		if err != nil {
			return err
		}
		return db.migrateData(flags)
	})
}

func (db *DB) withForeignKeysDisabled(run func() error) error {
	// Disable FK checks for the duration of migration to avoid legacy data conflicts.
	if _, err := db.Exec("PRAGMA foreign_keys = OFF"); err != nil {
		return err
	}
	defer func() {
		_, _ = db.Exec("PRAGMA foreign_keys = ON")
	}()
	return run()
}
