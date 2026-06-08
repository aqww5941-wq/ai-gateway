package store

func (s *Store) migrate() error {
	_, err := s.DB.Exec(`
		CREATE TABLE IF NOT EXISTS api_keys (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			name           TEXT NOT NULL UNIQUE,
			token_hash     BLOB NOT NULL,
			daily_limit    INTEGER NOT NULL DEFAULT 0,
			monthly_limit  INTEGER NOT NULL DEFAULT 0,
			role           TEXT NOT NULL DEFAULT 'user',
			models         TEXT NOT NULL DEFAULT '',
			is_active      INTEGER NOT NULL DEFAULT 1,
			created_at     TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at     TEXT NOT NULL DEFAULT (datetime('now'))
		);

		CREATE TABLE IF NOT EXISTS quota_usage (
			key_id      INTEGER NOT NULL,
			period      TEXT NOT NULL,
			tokens_used INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (key_id, period),
			FOREIGN KEY (key_id) REFERENCES api_keys(id) ON DELETE CASCADE
		);
	`)
	if err != nil {
		return err
	}

	// Migrate existing databases that lack the role / models columns.
	s.DB.Exec(`ALTER TABLE api_keys ADD COLUMN role TEXT NOT NULL DEFAULT 'user'`)
	s.DB.Exec(`ALTER TABLE api_keys ADD COLUMN models TEXT NOT NULL DEFAULT ''`)
	s.DB.Exec(`UPDATE api_keys SET role = 'admin' WHERE name = 'admin'`)

	_, err = s.DB.Exec(`
		CREATE TABLE IF NOT EXISTS audit_logs (
			id                INTEGER PRIMARY KEY AUTOINCREMENT,
			key_name          TEXT NOT NULL,
			model             TEXT NOT NULL,
			provider          TEXT NOT NULL,
			prompt_tokens     INTEGER NOT NULL DEFAULT 0,
			completion_tokens INTEGER NOT NULL DEFAULT 0,
			total_tokens      INTEGER NOT NULL DEFAULT 0,
			status_code       INTEGER NOT NULL,
			latency_ms        INTEGER NOT NULL DEFAULT 0,
			stream            INTEGER NOT NULL DEFAULT 0,
			error_message     TEXT NOT NULL DEFAULT '',
			created_at        TEXT NOT NULL DEFAULT (datetime('now'))
		);

		CREATE INDEX IF NOT EXISTS idx_audit_key  ON audit_logs(key_name);
		CREATE INDEX IF NOT EXISTS idx_audit_time ON audit_logs(created_at);
	`)
	return err
}
