package store

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

type KeyInfo struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Role         string `json:"role"`
	DailyLimit   int64  `json:"daily_limit"`
	MonthlyLimit int64  `json:"monthly_limit"`
	Models       string `json:"models"`
	IsActive     bool   `json:"is_active"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type KeyIdentity struct {
	ID         int64
	Name       string
	Role       string
	DailyLimit int64
	Models     string // comma-separated, empty = all allowed
}

// AllowedModel returns true if the given model is in the allowlist.
// An empty Models field means all models are allowed.
func (id *KeyIdentity) AllowedModel(model string) bool {
	if id == nil || id.Models == "" {
		return true
	}
	for _, m := range strings.Split(id.Models, ",") {
		if strings.TrimSpace(m) == model {
			return true
		}
	}
	return false
}

// IsAdmin returns true if the identity has admin role.
func (id *KeyIdentity) IsAdmin() bool {
	return id != nil && id.Role == "admin"
}

func GenerateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return "sk-" + hex.EncodeToString(b)
}

func hashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

func (s *Store) CreateKey(name, role, models string, dailyLimit, monthlyLimit int64) (token string, err error) {
	if role == "" {
		role = "user"
	}
	token = GenerateToken()
	hash := hashToken(token)
	_, err = s.DB.Exec(
		`INSERT INTO api_keys (name, role, models, token_hash, daily_limit, monthly_limit) VALUES (?, ?, ?, ?, ?, ?)`,
		name, role, models, hash, dailyLimit, monthlyLimit,
	)
	if err != nil {
		return "", fmt.Errorf("create key: %w", err)
	}
	return token, nil
}

func (s *Store) ListKeys() ([]KeyInfo, error) {
	rows, err := s.DB.Query(
		`SELECT id, name, role, daily_limit, monthly_limit, models, is_active, created_at, updated_at FROM api_keys ORDER BY id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []KeyInfo
	for rows.Next() {
		var k KeyInfo
		var active int
		if err := rows.Scan(&k.ID, &k.Name, &k.Role, &k.DailyLimit, &k.MonthlyLimit, &k.Models, &active, &k.CreatedAt, &k.UpdatedAt); err != nil {
			return nil, err
		}
		k.IsActive = active != 0
		out = append(out, k)
	}
	return out, rows.Err()
}

func (s *Store) GetKeyByID(id int64) (*KeyInfo, error) {
	var k KeyInfo
	var active int
	err := s.DB.QueryRow(
		`SELECT id, name, role, daily_limit, monthly_limit, models, is_active, created_at, updated_at FROM api_keys WHERE id = ?`, id,
	).Scan(&k.ID, &k.Name, &k.Role, &k.DailyLimit, &k.MonthlyLimit, &k.Models, &active, &k.CreatedAt, &k.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	k.IsActive = active != 0
	return &k, nil
}

// LookupIdentity finds the key identity by token. Returns nil if not found or inactive.
func (s *Store) LookupIdentity(token string) (*KeyIdentity, error) {
	h := hashToken(token)
	rows, err := s.DB.Query(`SELECT id, name, role, token_hash, daily_limit, models FROM api_keys WHERE is_active = 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id, dailyLimit int64
		var name, role, models string
		var storedHash []byte
		if err := rows.Scan(&id, &name, &role, &storedHash, &dailyLimit, &models); err != nil {
			return nil, err
		}
		if subtle.ConstantTimeCompare(h, storedHash) == 1 {
			return &KeyIdentity{ID: id, Name: name, Role: role, DailyLimit: dailyLimit, Models: models}, nil
		}
	}
	return nil, rows.Err()
}

func (s *Store) UpdateKey(id int64, name, role, models string, dailyLimit, monthlyLimit int64) error {
	_, err := s.DB.Exec(
		`UPDATE api_keys SET name = ?, role = ?, models = ?, daily_limit = ?, monthly_limit = ?, updated_at = ? WHERE id = ?`,
		name, role, models, dailyLimit, monthlyLimit, time.Now().UTC().Format(time.RFC3339), id,
	)
	return err
}

func (s *Store) SetKeyActive(id int64, active bool) error {
	v := 0
	if active {
		v = 1
	}
	_, err := s.DB.Exec(
		`UPDATE api_keys SET is_active = ?, updated_at = ? WHERE id = ?`,
		v, time.Now().UTC().Format(time.RFC3339), id,
	)
	return err
}

func (s *Store) DeleteKey(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM api_keys WHERE id = ?`, id)
	return err
}
