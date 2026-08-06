package store

import (
	"bytes"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func openIsolatedStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gateway.db")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open(%q): %v", path, err)
	}
	t.Cleanup(func() {
		if err := st.Close(); err != nil && !errors.Is(err, sql.ErrConnDone) {
			t.Errorf("Close(): %v", err)
		}
	})
	return st
}

func TestOpenCreatesSchemaAndMigrationIsIdempotent(t *testing.T) {
	st := openIsolatedStore(t)
	wantObjects := []string{"api_keys", "quota_usage", "audit_logs", "idx_audit_key", "idx_audit_time"}
	for _, name := range wantObjects {
		var count int
		if err := st.DB.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name = ?`, name).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("sqlite object %q count = %d, want 1", name, count)
		}
	}

	if _, err := st.CreateKey("before-repeat", "user", "", 10, 20); err != nil {
		t.Fatal(err)
	}
	if err := st.migrate(); err != nil {
		t.Fatalf("second migrate(): %v", err)
	}
	keys, err := st.ListKeys()
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 || keys[0].Name != "before-repeat" {
		t.Fatalf("keys after repeated migration = %#v", keys)
	}

	// Current migration baseline is intentionally recorded as unversioned.
	// Task 46 owns the versioned migration framework.
	var userVersion int
	if err := st.DB.QueryRow(`PRAGMA user_version`).Scan(&userVersion); err != nil {
		t.Fatal(err)
	}
	if userVersion != 0 {
		t.Fatalf("PRAGMA user_version = %d, want current unversioned baseline 0", userVersion)
	}
}

func TestOpenMigratesLegacyKeyColumnsWithoutLosingRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	legacyHash := hashToken("invalid-legacy-token")
	if _, err := db.Exec(`
		CREATE TABLE api_keys (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			token_hash BLOB NOT NULL,
			daily_limit INTEGER NOT NULL DEFAULT 0,
			monthly_limit INTEGER NOT NULL DEFAULT 0,
			is_active INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now'))
		);
		INSERT INTO api_keys (name, token_hash, daily_limit) VALUES ('admin', ?, 100);
	`, legacyHash); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := st.Close(); err != nil {
			t.Errorf("Close(): %v", err)
		}
	})
	key, err := st.GetKeyByID(1)
	if err != nil {
		t.Fatal(err)
	}
	if key == nil || key.Name != "admin" || key.Role != "admin" || key.Models != "" || key.DailyLimit != 100 {
		t.Fatalf("migrated key = %#v", key)
	}
}

func TestKeyLifecycleStoresOnlyHashAndCascadesUsage(t *testing.T) {
	st := openIsolatedStore(t)
	token, err := st.CreateKey("developer", "", "model-a,model-b", 100, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(token, "sk-") || len(token) != 67 {
		t.Fatalf("generated token format length/prefix invalid")
	}

	var id int64
	var storedHash []byte
	if err := st.DB.QueryRow(`SELECT id, token_hash FROM api_keys WHERE name = 'developer'`).Scan(&id, &storedHash); err != nil {
		t.Fatal(err)
	}
	if len(storedHash) != 32 || bytes.Equal(storedHash, []byte(token)) {
		t.Fatalf("stored credential is not a SHA-256 digest: length=%d", len(storedHash))
	}
	var plaintextMatches int
	if err := st.DB.QueryRow(`SELECT COUNT(*) FROM api_keys WHERE CAST(token_hash AS TEXT) = ?`, token).Scan(&plaintextMatches); err != nil {
		t.Fatal(err)
	}
	if plaintextMatches != 0 {
		t.Fatal("plaintext token found in api_keys")
	}

	identity, err := st.LookupIdentity(token)
	if err != nil {
		t.Fatal(err)
	}
	if identity == nil || identity.ID != id || identity.Role != "user" || !identity.AllowedModel("model-b") || identity.AllowedModel("model-c") {
		t.Fatalf("created identity = %#v", identity)
	}
	if wrong, err := st.LookupIdentity("invalid-wrong-token"); err != nil || wrong != nil {
		t.Fatalf("wrong-token lookup = %#v, %v", wrong, err)
	}

	if err := st.UpdateKey(id, "renamed", "admin", "model-c", 200, 2000); err != nil {
		t.Fatal(err)
	}
	updated, err := st.GetKeyByID(id)
	if err != nil {
		t.Fatal(err)
	}
	if updated == nil || updated.Name != "renamed" || updated.Role != "admin" || updated.Models != "model-c" || updated.DailyLimit != 200 || updated.MonthlyLimit != 2000 {
		t.Fatalf("updated key = %#v", updated)
	}

	if err := st.SetKeyActive(id, false); err != nil {
		t.Fatal(err)
	}
	if inactive, err := st.LookupIdentity(token); err != nil || inactive != nil {
		t.Fatalf("inactive lookup = %#v, %v", inactive, err)
	}
	if err := st.SetKeyActive(id, true); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordUsage(id, 7); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteKey(id); err != nil {
		t.Fatal(err)
	}
	if deleted, err := st.GetKeyByID(id); err != nil || deleted != nil {
		t.Fatalf("deleted key = %#v, %v", deleted, err)
	}
	var usageRows int
	if err := st.DB.QueryRow(`SELECT COUNT(*) FROM quota_usage WHERE key_id = ?`, id).Scan(&usageRows); err != nil {
		t.Fatal(err)
	}
	if usageRows != 0 {
		t.Fatalf("quota rows after key deletion = %d, want 0", usageRows)
	}
}

func TestStoreOperationsReturnErrorsAfterClose(t *testing.T) {
	st := openIsolatedStore(t)
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	checks := []struct {
		name string
		run  func() error
	}{
		{name: "create key", run: func() error { _, err := st.CreateKey("closed", "user", "", 0, 0); return err }},
		{name: "list keys", run: func() error { _, err := st.ListKeys(); return err }},
		{name: "lookup identity", run: func() error { _, err := st.LookupIdentity("invalid-token"); return err }},
		{name: "check quota", run: func() error { _, _, _, err := st.CheckQuota(1); return err }},
		{name: "record usage", run: func() error { return st.RecordUsage(1, 1) }},
		{name: "insert audit", run: func() error { return st.InsertAudit(&AuditEntry{}) }},
		{name: "query audit", run: func() error { _, _, err := st.QueryAudit("", 1, 0); return err }},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if err := check.run(); err == nil {
				t.Fatal("operation error = nil after Store.Close()")
			}
		})
	}
}
