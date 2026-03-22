package storage

// RecordSignupInterest stores a signup interest record for analytics.
// No foreign keys -- this is a standalone analytics table.
func (db *DB) RecordSignupInterest(email, usecase, usecaseOther, source, sourceOther, ip string) error {
	_, err := db.Exec(`
		INSERT INTO signup_interest (email, usecase, usecase_other, source, source_other, ip, created_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'))`,
		email, usecase, usecaseOther, source, sourceOther, ip)
	return err
}
