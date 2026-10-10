package official

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// GatewayCLI represents the connection credentials written by the official Accio desktop app.
type GatewayCLI struct {
	SchemaVersion int    `json:"schemaVersion"`
	URL           string `json:"url"`
	WsURL         string `json:"wsUrl"`
	AuthMode      string `json:"authMode"`
	Username      string `json:"username"`
	Password      string `json:"password"`
	RelayPort     int    `json:"relayPort"`
	PID           int    `json:"pid"`
	CreatedAt     string `json:"createdAt"`
}

type OfficialStatus struct {
	Running       bool           `json:"running"`
	URL           string         `json:"url"`
	Port          int            `json:"port"`
	PID           int            `json:"pid"`
	ActiveAccount map[string]any `json:"active_account"`
	Credits       map[string]any `json:"credits"`
	Error         string         `json:"error,omitempty"`
}

type OfficialAccount struct {
	AccountID   string         `json:"account_id"`
	AccioID     string         `json:"accio_id"`
	Email       string         `json:"email"`
	Name        string         `json:"name"`
	UserLevel   string         `json:"user_level"`
	IsActive    bool           `json:"is_active"`
	AccessToken string         `json:"access_token"`
	Credits     map[string]any `json:"credits,omitempty"`
	LoginAt     int64          `json:"login_at"`
}

type Manager struct {
	mu           sync.RWMutex
	client       *http.Client
	cliInfo      *GatewayCLI
	cliPath      string
	lastActiveID string
	cachedTokens map[string]string // accountId -> accessToken
	cachedCreds  map[string]any
	cachedStatus OfficialStatus
	lastCheck    time.Time
}

func NewManager() *Manager {
	return &Manager{
		client:       &http.Client{Timeout: 6 * time.Second},
		cachedTokens: make(map[string]string),
	}
}

// FindGatewayCLI finds the most recent gateway-cli.json in ~/.accio/accounts/*/.accio/runtime/gateway-cli.json
func (m *Manager) FindGatewayCLI() (*GatewayCLI, string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, "", err
	}
	accDir := filepath.Join(home, ".accio", "accounts")
	entries, err := os.ReadDir(accDir)
	if err != nil {
		return nil, "", err
	}

	type match struct {
		info *GatewayCLI
		path string
		mod  time.Time
	}
	var matches []match

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(accDir, e.Name(), ".accio", "runtime", "gateway-cli.json")
		st, err := os.Stat(p)
		if err != nil {
			continue
		}
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var g GatewayCLI
		if err := json.Unmarshal(data, &g); err != nil {
			continue
		}
		if g.URL != "" && g.Password != "" {
			matches = append(matches, match{info: &g, path: p, mod: st.ModTime()})
		}
	}

	if len(matches) == 0 {
		return nil, "", errors.New("nenhuma credencial do Accio oficial encontrada (o app está aberto?)")
	}

	// Sort by modification time descending
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].mod.After(matches[j].mod)
	})

	return matches[0].info, matches[0].path, nil
}

// CheckAlive verifies if the official Accio local server responds.
func (m *Manager) CheckAlive(ctx context.Context) (OfficialStatus, error) {
	cli, cliPath, err := m.FindGatewayCLI()
	if err != nil {
		return OfficialStatus{Running: false, Error: err.Error()}, err
	}
	m.mu.Lock()
	m.cliInfo = cli
	m.cliPath = cliPath
	m.mu.Unlock()

	statusURL := strings.TrimRight(cli.URL, "/") + "/auth/status"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, statusURL, nil)
	if err != nil {
		return OfficialStatus{Running: false, Error: err.Error()}, err
	}
	req.SetBasicAuth(cli.Username, cli.Password)

	resp, err := m.client.Do(req)
	if err != nil {
		return OfficialStatus{Running: false, URL: cli.URL, PID: cli.PID, Error: "Accio oficial não está respondendo"}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return OfficialStatus{Running: false, URL: cli.URL, PID: cli.PID, Error: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, raw)}, errors.New("auth status failed")
	}

	var root struct {
		Authenticated bool           `json:"authenticated"`
		User          map[string]any `json:"user"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&root); err != nil {
		return OfficialStatus{Running: false, URL: cli.URL, PID: cli.PID, Error: "resposta inválida"}, err
	}

	u, _ := url.Parse(cli.URL)
	port := 4097
	if u != nil && u.Port() != "" {
		if p, err := strconv.Atoi(u.Port()); err == nil {
			port = p
		}
	}

	stat := OfficialStatus{
		Running:       root.Authenticated,
		URL:           cli.URL,
		Port:          port,
		PID:           cli.PID,
		ActiveAccount: root.User,
	}

	// Fetch current credits
	if credits, err := m.fetchCredits(ctx, cli); err == nil {
		stat.Credits = credits
	}

	m.mu.Lock()
	if activeID, ok := root.User["id"].(string); ok {
		m.lastActiveID = activeID
	}
	m.cachedCreds = stat.Credits
	m.cachedStatus = stat
	m.lastCheck = time.Now()
	m.mu.Unlock()

	return stat, nil
}

func (m *Manager) fetchCredits(ctx context.Context, cli *GatewayCLI) (map[string]any, error) {
	credURL := strings.TrimRight(cli.URL, "/") + "/subscription/current"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, credURL, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(cli.Username, cli.Password)
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var res struct {
		Success bool           `json:"success"`
		Data    map[string]any `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	if !res.Success || res.Data == nil {
		return nil, errors.New("subscription data unavailable")
	}

	total := toInt(res.Data["total"])
	remaining := toInt(res.Data["remaining"])
	used := total - remaining
	if used < 0 {
		used = 0
	}

	out := map[string]any{
		"total":     total,
		"remaining": remaining,
		"used":      used,
		"type":      res.Data["type"],
		"checked":   time.Now().Format("15:04:05"),
	}
	if ent, ok := res.Data["entitlement"].(map[string]any); ok {
		out["entitlement"] = ent
		if daily, ok := ent["daily"].(map[string]any); ok {
			out["daily_remaining"] = toInt(daily["remaining"])
			out["daily_total"] = toInt(daily["total"])
		}
	}
	return out, nil
}

func toInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	}
	return 0
}

// GetStatus returns the cached or fresh status of the official app.
func (m *Manager) GetStatus() OfficialStatus {
	m.mu.RLock()
	if time.Since(m.lastCheck) < 3*time.Second && m.cachedStatus.Running {
		defer m.mu.RUnlock()
		return m.cachedStatus
	}
	m.mu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	st, _ := m.CheckAlive(ctx)
	return st
}

// loadTokensFromDisk decrypts remembered-accounts.enc to get access tokens for all accounts.
func (m *Manager) loadTokensFromDisk() (map[string]string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		home, _ := os.UserHomeDir()
		appData = filepath.Join(home, "AppData", "Roaming")
	}
	encPath := filepath.Join(appData, "Accio", "remembered-accounts.enc")
	dec, err := DecryptEncryptedFile(encPath)
	if err != nil {
		return nil, err
	}
	var accs map[string]struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
	}
	if err := json.Unmarshal(dec, &accs); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(accs))
	for id, acc := range accs {
		if acc.AccessToken != "" {
			out[id] = acc.AccessToken
		}
	}

	// Also check credentials.enc for current active token
	credPath := filepath.Join(appData, "Accio", "credentials.enc")
	if decCred, err := DecryptEncryptedFile(credPath); err == nil {
		var activeCred struct {
			AccessToken string `json:"accessToken"`
			User        struct {
				ID string `json:"id"`
			} `json:"user"`
		}
		if err := json.Unmarshal(decCred, &activeCred); err == nil && activeCred.AccessToken != "" {
			if activeCred.User.ID != "" {
				out[activeCred.User.ID] = activeCred.AccessToken
			}
			out["__active__"] = activeCred.AccessToken
		}
	}

	return out, nil
}

// ListAccounts queries /auth/accounts and enriches with decrypted tokens and credits.
func (m *Manager) ListAccounts(ctx context.Context) ([]OfficialAccount, error) {
	cli := m.getCli()
	if cli == nil {
		var err error
		cli, _, err = m.FindGatewayCLI()
		if err != nil {
			return nil, err
		}
	}

	tokens, err := m.loadTokensFromDisk()
	if err != nil {
		log.Printf("official: erro ao ler tokens do disco: %v", err)
		tokens = make(map[string]string)
	}
	m.mu.Lock()
	m.cachedTokens = tokens
	m.mu.Unlock()

	reqURL := strings.TrimRight(cli.URL, "/") + "/auth/accounts"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(cli.Username, cli.Password)

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d ao listar contas oficiais", resp.StatusCode)
	}

	var rawResp struct {
		Accounts []struct {
			AccountID string `json:"accountId"`
			Auth      struct {
				AccioID           string `json:"accioId"`
				Name              string `json:"name"`
				DesensitizedEmail string `json:"desensitizedEmail"`
				UserLevel         string `json:"userLevel"`
			} `json:"auth"`
			LoginAt int64 `json:"loginAt"`
		} `json:"accounts"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rawResp); err != nil {
		return nil, err
	}

	m.mu.RLock()
	activeID := m.lastActiveID
	cachedCreds := m.cachedCreds
	m.mu.RUnlock()

	var result []OfficialAccount
	for _, a := range rawResp.Accounts {
		isActive := a.AccountID == activeID
		token := tokens[a.AccountID]
		if token == "" && isActive {
			token = tokens["__active__"]
		}

		maskedToken := token
		if len(token) > 12 {
			maskedToken = token[:6] + "…" + token[len(token)-4:]
		}

		acc := OfficialAccount{
			AccountID:   a.AccountID,
			AccioID:     a.Auth.AccioID,
			Email:       a.Auth.DesensitizedEmail,
			Name:        a.Auth.Name,
			UserLevel:   a.Auth.UserLevel,
			IsActive:    isActive,
			AccessToken: maskedToken,
			LoginAt:     a.LoginAt,
		}
		if isActive && cachedCreds != nil {
			acc.Credits = cachedCreds
		}
		result = append(result, acc)
	}

	return result, nil
}

// SwitchAccount tells the official Accio app to switch to the given account ID.
func (m *Manager) SwitchAccount(ctx context.Context, accountID string) error {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return errors.New("accountId é obrigatório")
	}
	cli := m.getCli()
	if cli == nil {
		var err error
		cli, _, err = m.FindGatewayCLI()
		if err != nil {
			return err
		}
	}

	switchURL := fmt.Sprintf("%s/auth/accounts/%s/switch", strings.TrimRight(cli.URL, "/"), accountID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, switchURL, bytes.NewReader([]byte("{}")))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(cli.Username, cli.Password)

	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("erro ao chamar switch no Accio oficial: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("switch retornou HTTP %d: %s", resp.StatusCode, raw)
	}

	// Update active account and credits
	m.mu.Lock()
	m.lastActiveID = accountID
	m.mu.Unlock()

	// Recheck status
	_, _ = m.CheckAlive(ctx)
	return nil
}

// RefreshCredits explicitly updates credits of the active account.
func (m *Manager) RefreshCredits(ctx context.Context) (map[string]any, error) {
	cli := m.getCli()
	if cli == nil {
		var err error
		cli, _, err = m.FindGatewayCLI()
		if err != nil {
			return nil, err
		}
	}
	creds, err := m.fetchCredits(ctx, cli)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	m.cachedCreds = creds
	m.mu.Unlock()
	return creds, nil
}

// GetActiveToken returns (email, accessToken, error) for the active account in the official Accio app.
func (m *Manager) GetActiveToken(ctx context.Context) (email, token string, err error) {
	st := m.GetStatus()
	if !st.Running {
		return "", "", errors.New("o app oficial do Accio não está aberto ou autenticado")
	}

	activeEmail, _ := st.ActiveAccount["desensitizedEmail"].(string)
	activeID, _ := st.ActiveAccount["id"].(string)
	if activeEmail == "" && activeID != "" {
		activeEmail = "accio-" + activeID
	}

	m.mu.RLock()
	cachedTok := m.cachedTokens[activeID]
	if cachedTok == "" {
		cachedTok = m.cachedTokens["__active__"]
	}
	m.mu.RUnlock()

	if cachedTok != "" {
		return activeEmail, cachedTok, nil
	}

	// Reload tokens from disk
	tokens, err := m.loadTokensFromDisk()
	if err != nil {
		return "", "", fmt.Errorf("ler token da conta oficial: %w", err)
	}
	m.mu.Lock()
	m.cachedTokens = tokens
	m.mu.Unlock()

	tok := tokens[activeID]
	if tok == "" {
		tok = tokens["__active__"]
	}
	if tok == "" {
		return "", "", fmt.Errorf("token da conta %s não encontrado no vault local", activeID)
	}
	return activeEmail, tok, nil
}

func (m *Manager) getCli() *GatewayCLI {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cliInfo
}
