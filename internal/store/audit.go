package store

import (
	"fmt"
	"log/slog"
	"time"
)

type AuditEntry struct {
	ID               int64  `json:"id"`
	KeyName          string `json:"key_name"`
	Model            string `json:"model"`
	Provider         string `json:"provider"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	TotalTokens      int    `json:"total_tokens"`
	StatusCode       int    `json:"status_code"`
	LatencyMs        int64  `json:"latency_ms"`
	Stream           bool   `json:"stream"`
	ErrorMessage     string `json:"error_message,omitempty"`
	CreatedAt        string `json:"created_at"`
}

func (s *Store) InsertAudit(entry *AuditEntry) error {
	stream := 0
	if entry.Stream {
		stream = 1
	}
	_, err := s.DB.Exec(
		`INSERT INTO audit_logs (key_name, model, provider, prompt_tokens, completion_tokens, total_tokens, status_code, latency_ms, stream, error_message)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.KeyName, entry.Model, entry.Provider,
		entry.PromptTokens, entry.CompletionTokens, entry.TotalTokens,
		entry.StatusCode, entry.LatencyMs, stream, entry.ErrorMessage,
	)
	return err
}

func (s *Store) QueryAudit(keyName string, limit, offset int) ([]AuditEntry, int64, error) {
	var total int64
	countSQL := `SELECT COUNT(*) FROM audit_logs`
	args := []interface{}{}
	if keyName != "" {
		countSQL += ` WHERE key_name = ?`
		args = append(args, keyName)
	}
	if err := s.DB.QueryRow(countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	querySQL := `SELECT id, key_name, model, provider, prompt_tokens, completion_tokens, total_tokens, status_code, latency_ms, stream, error_message, created_at
		 FROM audit_logs`
	qArgs := []interface{}{}
	if keyName != "" {
		querySQL += ` WHERE key_name = ?`
		qArgs = append(qArgs, keyName)
	}
	querySQL += ` ORDER BY id DESC LIMIT ? OFFSET ?`
	qArgs = append(qArgs, limit, offset)

	rows, err := s.DB.Query(querySQL, qArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []AuditEntry
	for rows.Next() {
		var e AuditEntry
		var stream int
		if err := rows.Scan(&e.ID, &e.KeyName, &e.Model, &e.Provider,
			&e.PromptTokens, &e.CompletionTokens, &e.TotalTokens,
			&e.StatusCode, &e.LatencyMs, &stream, &e.ErrorMessage, &e.CreatedAt); err != nil {
			return nil, 0, err
		}
		e.Stream = stream != 0
		out = append(out, e)
	}
	return out, total, rows.Err()
}

// StartAuditCleanup runs a background goroutine that deletes audit logs older
// than retentionDays, once every 24 hours.
func (s *Store) StartAuditCleanup(logger *slog.Logger, retentionDays int) {
	go func() {
		for {
			time.Sleep(24 * time.Hour)
			sql := fmt.Sprintf(`DELETE FROM audit_logs WHERE created_at < datetime('now', '-%d days')`, retentionDays)
			result, err := s.DB.Exec(sql)
			if err != nil {
				logger.Error("audit cleanup failed", "error", err)
				continue
			}
			n, _ := result.RowsAffected()
			if n > 0 {
				logger.Info("audit logs cleaned up", "deleted", n, "retention_days", retentionDays)
			}
		}
	}()
}
