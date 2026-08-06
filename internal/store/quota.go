package store

import (
	"fmt"
	"time"
)

type QuotaSnapshot struct {
	KeyID          int64  `json:"key_id"`
	KeyName        string `json:"name"`
	DailyLimit     int64  `json:"daily_limit"`
	MonthlyLimit   int64  `json:"monthly_limit"`
	UsedToday      int64  `json:"used_tokens"`
	RemainingToday int64  `json:"remaining_tokens"`
	UsedThisMonth  int64  `json:"used_monthly"`
	RemainingMonth int64  `json:"remaining_monthly"`
	ResetAt        int64  `json:"reset_at"`
}

func dailyPeriod() string {
	return time.Now().UTC().Format("2006-01-02")
}

func monthlyPeriod() string {
	return time.Now().UTC().Format("2006-01")
}

func nextDailyReset() int64 {
	now := time.Now().UTC()
	tomorrow := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
	return tomorrow.Unix()
}

// CheckQuota reads current usage and checks against limits. Returns (allowed, used, limit).
func (s *Store) CheckQuota(keyID int64) (allowed bool, dailyUsed, dailyLimit int64, err error) {
	var monthlyLimitVal int64
	err = s.DB.QueryRow(
		`SELECT daily_limit, monthly_limit FROM api_keys WHERE id = ? AND is_active = 1`, keyID,
	).Scan(&dailyLimit, &monthlyLimitVal)
	if err != nil {
		return false, 0, 0, fmt.Errorf("check quota: %w", err)
	}

	_ = monthlyLimitVal // reserved for future monthly check

	var used sqlNullInt64
	err = s.DB.QueryRow(
		`SELECT tokens_used FROM quota_usage WHERE key_id = ? AND period = ?`, keyID, dailyPeriod(),
	).Scan(&used)
	if err != nil {
		// No row means 0 used — allowed if there's a limit.
		dailyUsed = 0
	} else {
		dailyUsed = used.Int64
	}

	if dailyLimit <= 0 {
		return true, dailyUsed, 0, nil
	}
	return dailyUsed < dailyLimit, dailyUsed, dailyLimit, nil
}

type sqlNullInt64 struct {
	Int64 int64
	Valid bool
}

func (n *sqlNullInt64) Scan(value interface{}) error {
	if value == nil {
		n.Int64 = 0
		n.Valid = true
		return nil
	}
	// SQLite returns int64 directly
	switch v := value.(type) {
	case int64:
		n.Int64 = v
		n.Valid = true
	default:
		n.Int64 = 0
		n.Valid = true
	}
	return nil
}

// RecordUsage adds tokens to the quota usage for this key (daily and monthly).
func (s *Store) RecordUsage(keyID int64, tokens int) error {
	if tokens <= 0 {
		return nil
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Upsert daily
	_, err = tx.Exec(
		`INSERT INTO quota_usage (key_id, period, tokens_used) VALUES (?, ?, ?)
		 ON CONFLICT(key_id, period) DO UPDATE SET tokens_used = tokens_used + ?`,
		keyID, dailyPeriod(), tokens, tokens,
	)
	if err != nil {
		return fmt.Errorf("record daily usage: %w", err)
	}

	// Upsert monthly
	_, err = tx.Exec(
		`INSERT INTO quota_usage (key_id, period, tokens_used) VALUES (?, ?, ?)
		 ON CONFLICT(key_id, period) DO UPDATE SET tokens_used = tokens_used + ?`,
		keyID, monthlyPeriod(), tokens, tokens,
	)
	if err != nil {
		return fmt.Errorf("record monthly usage: %w", err)
	}

	return tx.Commit()
}

// GetQuotaSnapshots returns quota usage for all keys.
func (s *Store) GetQuotaSnapshots() ([]QuotaSnapshot, error) {
	rows, err := s.DB.Query(`
		SELECT k.id, k.name, k.daily_limit, k.monthly_limit,
			COALESCE((SELECT tokens_used FROM quota_usage WHERE key_id = k.id AND period = ?), 0),
			COALESCE((SELECT tokens_used FROM quota_usage WHERE key_id = k.id AND period = ?), 0)
		FROM api_keys k
		ORDER BY k.id
	`, dailyPeriod(), monthlyPeriod())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []QuotaSnapshot
	resetAt := nextDailyReset()
	for rows.Next() {
		var q QuotaSnapshot
		if err := rows.Scan(&q.KeyID, &q.KeyName, &q.DailyLimit, &q.MonthlyLimit, &q.UsedToday, &q.UsedThisMonth); err != nil {
			return nil, err
		}
		q.ResetAt = resetAt
		if q.DailyLimit > q.UsedToday {
			q.RemainingToday = q.DailyLimit - q.UsedToday
		}
		if q.MonthlyLimit > q.UsedThisMonth {
			q.RemainingMonth = q.MonthlyLimit - q.UsedThisMonth
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

// SeedKey inserts a key from config seed data if the DB is empty.
func (s *Store) SeedKey(token, name, role, models string, dailyLimit int64) error {
	var count int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM api_keys WHERE name = ?`, name).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil // already seeded
	}
	if role == "" {
		role = "user"
	}
	hash := hashToken(token)
	_, err := s.DB.Exec(
		`INSERT INTO api_keys (name, role, models, token_hash, daily_limit) VALUES (?, ?, ?, ?, ?)`,
		name, role, models, hash, dailyLimit,
	)
	return err
}
