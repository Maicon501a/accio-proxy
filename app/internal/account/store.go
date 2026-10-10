// Package account implements the persistent account pool.
package account

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Account is one Accio identity with its harvested session artifacts.
type Account struct {
	ID           int64  `json:"id"`
	Email        string `json:"email"`
	AccessToken  string `json:"access_token"`
	UserID       string `json:"user_id"`
	Utdid        string `json:"utdid"`
	Status       string `json:"status"` // active | dead | unknown
	Credits      int    `json:"credits"`
	Pinned       bool   `json:"pinned"`
	Note         string `json:"note"`
	CreatedAt    string `json:"created_at"`
	LastUsedAt   string `json:"last_used_at"`
	CreditsInfo  string `json:"credits_info"`
	RefreshToken string `json:"-"`
}

var ErrEmpty = errors.New("no accounts available")

// RefEntry é uma conta elegível à renovação periódica de token.
type RefEntry struct {
	Email, Access, Refresh string
}

// Store is a sqlite-backed pool with round-robin rotation.
type Store struct {
	db *sql.DB
	mu sync.Mutex
	rr int
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS accounts (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	email TEXT NOT NULL UNIQUE,
	access_token TEXT NOT NULL DEFAULT '',
	user_id TEXT NOT NULL DEFAULT '',
	utdid TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'unknown',
	credits INTEGER NOT NULL DEFAULT 0,
	note TEXT NOT NULL DEFAULT '',
	refresh_token TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL DEFAULT (datetime('now')),
	last_used_at TEXT NOT NULL DEFAULT ''
);`)
	if err != nil {
		return err
	}
	// migração: detalhe de créditos (JSON) — ignora se a coluna já existe
	_, _ = s.db.Exec(`ALTER TABLE accounts ADD COLUMN credits_info TEXT NOT NULL DEFAULT ''`)
	// migração: conta fixada (pin) — ignora se a coluna já existe
	_, _ = s.db.Exec(`ALTER TABLE accounts ADD COLUMN pinned INTEGER NOT NULL DEFAULT 0`)
	return nil
}

// Insert adds (or updates) an account by email.
func (s *Store) Insert(a Account) (int64, error) {
	res, err := s.db.Exec(`
INSERT INTO accounts (email, access_token, user_id, utdid, status, credits, note, refresh_token)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(email) DO UPDATE SET
	access_token=excluded.access_token,
	user_id=excluded.user_id,
	utdid=excluded.utdid,
	status=excluded.status,
	credits=excluded.credits,
	refresh_token=excluded.refresh_token,
	last_used_at=''`,
		a.Email, a.AccessToken, a.UserID, a.Utdid, a.Status, a.Credits, a.Note, a.RefreshToken)
	if err != nil {
		return 0, err
}
	id, _ := res.LastInsertId()
	return id, nil
}

// Pick returns the account for a chat request: a conta fixada (pinned) tem
// prioridade absoluta; sem pin, cai no round-robin pulando contas zeradas
// de créditos (só pula quem já tem snapshot de créditos com remaining=0 —
// conta nunca consultada continua elegível).
func (s *Store) Pick(ctx context.Context) (string, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// 1) conta fixada pelo usuário — sempre vence
	var pid int64
	var pemail, ptoken string
	if err := s.db.QueryRow(`SELECT id, email, access_token FROM accounts WHERE pinned=1 AND status='active' AND access_token != ''`).Scan(&pid, &pemail, &ptoken); err == nil {
		_, _ = s.db.Exec(`UPDATE accounts SET last_used_at=datetime('now') WHERE id=?`, pid)
		return pemail, ptoken, nil
	}
	// 2) round-robin padrão, ignorando contas com saldo zerado confirmado
	rows, err := s.db.Query(`SELECT id, email, access_token FROM accounts WHERE status='active' AND access_token != '' AND (credits > 0 OR credits_info = '') ORDER BY id`)
	if err != nil {
		return "", "", err
	}
	defer rows.Close()
	type entry struct {
		id    int64
		email string
		token string
	}
	var list []entry
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.id, &e.email, &e.token); err != nil {
			return "", "", err
		}
		list = append(list, e)
	}
	if len(list) == 0 {
		return "", "", ErrEmpty
	}
	if s.rr >= len(list) {
		s.rr = 0
	}
	e := list[s.rr]
	s.rr++
	_, _ = s.db.Exec(`UPDATE accounts SET last_used_at=datetime('now') WHERE id=?`, e.id)
	return e.email, e.token, nil
}

// SetCredits persiste o snapshot de créditos (remaining no inteiro, JSON completo em credits_info).
func (s *Store) SetCredits(email string, remaining int, infoJSON string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`UPDATE accounts SET credits=?, credits_info=? WHERE email=?`, remaining, infoJSON, email)
	return err
}

// UpdateTokens substitui o par access/refresh de uma conta (pós-refresh).
func (s *Store) UpdateTokens(email, access, refresh string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`UPDATE accounts SET access_token=?, refresh_token=? WHERE email=?`, access, refresh, email)
	return err
}

// RefreshTokenOf devolve o par access/refresh atual de uma conta.
func (s *Store) RefreshTokenOf(email string) (string, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var access, refresh string
	_ = s.db.QueryRow(`SELECT access_token, refresh_token FROM accounts WHERE email=?`, email).Scan(&access, &refresh)
	return access, refresh
}

// ListForRefresh lista todas as contas ativas com refreshToken para a
// varredura periódica de renovação.
func (s *Store) ListForRefresh() []RefEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`SELECT email, access_token, refresh_token FROM accounts WHERE status='active' AND refresh_token != ''`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []RefEntry
	for rows.Next() {
		var e RefEntry
		if err := rows.Scan(&e.Email, &e.Access, &e.Refresh); err == nil {
			out = append(out, e)
		}
	}
	return out
}

// ReportFailure marks an account as dead after repeated gateway failures.
func (s *Store) ReportFailure(email string) {
	_, _ = s.db.Exec(`UPDATE accounts SET status='dead' WHERE email=?`, email)
}

// List returns all accounts (newest first) for the UI.
func (s *Store) List() ([]Account, error) {
	rows, err := s.db.Query(`SELECT id, email, access_token, user_id, utdid, status, credits, note, created_at, last_used_at, credits_info, pinned FROM accounts ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Account
	for rows.Next() {
		var a Account
		var pinned int
		if err := rows.Scan(&a.ID, &a.Email, &a.AccessToken, &a.UserID, &a.Utdid, &a.Status, &a.Credits, &a.Note, &a.CreatedAt, &a.LastUsedAt, &a.CreditsInfo, &pinned); err != nil {
			return nil, err
		}
		a.Pinned = pinned != 0
		out = append(out, a)
	}
	return out, nil
}

// CountActive returns the number of active accounts.
func (s *Store) CountActive() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM accounts WHERE status='active'`).Scan(&n)
	return n, err
}

// SetStatus updates an account status by email.
func (s *Store) SetStatus(email, status string) error {
	_, err := s.db.Exec(`UPDATE accounts SET status=? WHERE email=?`, status, email)
	return err
}

// SetPinned fixa uma conta como destino de todas as requests (apaga o pin
// anterior). Email vazio só limpa o pin.
func (s *Store) SetPinned(email string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.db.Exec(`UPDATE accounts SET pinned=0 WHERE pinned=1`); err != nil {
		return err
	}
	if email == "" {
		return nil
	}
	_, err := s.db.Exec(`UPDATE accounts SET pinned=1 WHERE email=?`, email)
	return err
}

// PinnedEmail devolve a conta fixada atual ("" se nenhuma).
func (s *Store) PinnedEmail() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var email string
	_ = s.db.QueryRow(`SELECT email FROM accounts WHERE pinned=1`).Scan(&email)
	return email
}

// Delete removes an account.
func (s *Store) Delete(email string) error {
	_, err := s.db.Exec(`DELETE FROM accounts WHERE email=?`, email)
	return err
}

// Export dumps the pool as JSON (backup).
func (s *Store) Export() (string, error) {
	list, err := s.List()
	if err != nil {
		return "", err
	}
	b, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

var _ = fmt.Sprintf
var _ = strings.TrimSpace
var _ = time.Now
