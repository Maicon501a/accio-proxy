// Package official — importação de contas do Accio Desktop para o pool local.
//
// O Modo Direto não usa o app oficial em runtime, mas o vault dele é uma fonte
// pronta de credenciais: o Accio Desktop guarda, por conta lembrada, o par
// accessToken + refreshToken cifrado com a chave-mestra do Electron
// (safeStorage: AES-256-GCM, chave protegida por DPAPI).
//
// remembered-accounts.enc tem, por id de conta:
//
//	{accountId, accessToken, refreshToken, expiresAt}
//
// credentials.enc tem a conta ativa no momento, com o bloco `user` (email
// desensitizado, nome, accioId).
//
// Este arquivo lê esse vault e converte em contas do pool. Nada é enviado para
// fora: é leitura local de arquivo + decifra com a chave da própria máquina.
package official

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// VaultAccount é uma conta encontrada no vault do Accio Desktop.
type VaultAccount struct {
	AccountID    string `json:"account_id"`
	AccioID      string `json:"accio_id"`
	Name         string `json:"name"`
	Email        string `json:"email"`         // desensitizado pelo Accio (ex: m***1@gmail.com)
	UserLevel    string `json:"user_level"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"`
	Source       string `json:"source"` // "remembered" | "active"
}

// vaultAccount é o formato cru dentro do .enc.
type vaultAccount struct {
	AccountID    string `json:"accountId"`
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresAt    int64  `json:"expiresAt"`
}

type vaultUser struct {
	ID                string `json:"id"`
	AccioID           string `json:"accioId"`
	Name              string `json:"name"`
	DesensitizedEmail string `json:"desensitizedEmail"`
	UserLevel         string `json:"userLevel"`
}

// vaultBaseDir devolve %APPDATA%\Accio (Windows) ou o diretório equivalente.
// O Accio Desktop só existe distribuiído para Windows e macOS; no Linux não há
// vault para importar e a função devolve um caminho inexistente.
func vaultBaseDir() string {
	if appData := os.Getenv("APPDATA"); appData != "" {
		return filepath.Join(appData, "Accio")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	// macOS guarda em ~/Library/Application Support/Accio
	return filepath.Join(home, "AppData", "Roaming", "Accio")
}

// VaultAvailable informa se existe vault do Accio Desktop nesta máquina.
func VaultAvailable() bool {
	base := vaultBaseDir()
	if base == "" {
		return false
	}
	for _, name := range []string{"remembered-accounts.enc", "credentials.enc"} {
		if _, err := os.Stat(filepath.Join(base, name)); err == nil {
			return true
		}
	}
	return false
}

// ReadVault lê e decifra as contas do vault do Accio Desktop.
func ReadVault() ([]VaultAccount, error) {
	base := vaultBaseDir()
	if base == "" {
		return nil, fmt.Errorf("não consegui resolver a pasta do Accio Desktop")
	}

	// metadata legível (sem cifra): nome/email/level por accountId
	meta := readVaultMetadata(base)

	byID := map[string]*VaultAccount{}

	encPath := filepath.Join(base, "remembered-accounts.enc")
	if raw, err := DecryptEncryptedFile(encPath); err == nil {
		var accs map[string]vaultAccount
		if err := json.Unmarshal(raw, &accs); err == nil {
			for id, a := range accs {
				if a.AccessToken == "" {
					continue
				}
				accID := a.AccountID
				if accID == "" {
					accID = id
				}
				acc := &VaultAccount{
					AccountID:    accID,
					AccessToken:  a.AccessToken,
					RefreshToken: a.RefreshToken,
					ExpiresAt:    a.ExpiresAt,
					Source:       "remembered",
				}
				applyMeta(acc, meta)
				byID[accID] = acc
			}
		}
	} else if !os.IsNotExist(err) {
		// arquivo existe mas não decifrou: o chamador decide o que mostrar
		if _, statErr := os.Stat(encPath); statErr == nil {
			return nil, fmt.Errorf("decifrar remembered-accounts.enc: %w", err)
		}
	}

	// conta ativa no app (pode ser mais recente que a lista lembrada)
	credPath := filepath.Join(base, "credentials.enc")
	if raw, err := DecryptEncryptedFile(credPath); err == nil {
		var cred struct {
			AccessToken  string    `json:"accessToken"`
			RefreshToken string    `json:"refreshToken"`
			ExpiresAt    int64     `json:"expiresAt"`
			User         vaultUser `json:"user"`
		}
		if err := json.Unmarshal(raw, &cred); err == nil && cred.AccessToken != "" {
			id := cred.User.ID
			if id == "" {
				id = cred.User.AccioID
			}
			acc := byID[id]
			if acc == nil {
				acc = &VaultAccount{AccountID: id, Source: "active"}
				byID[id] = acc
			}
			acc.AccessToken = cred.AccessToken
			acc.Source = "active"
			if cred.RefreshToken != "" {
				acc.RefreshToken = cred.RefreshToken
			}
			if cred.ExpiresAt > 0 {
				acc.ExpiresAt = cred.ExpiresAt
			}
			if cred.User.Name != "" {
				acc.Name = cred.User.Name
			}
			if cred.User.DesensitizedEmail != "" {
				acc.Email = cred.User.DesensitizedEmail
			}
			if cred.User.AccioID != "" {
				acc.AccioID = cred.User.AccioID
			}
			if cred.User.UserLevel != "" {
				acc.UserLevel = cred.User.UserLevel
			}
		}
	}

	out := make([]VaultAccount, 0, len(byID))
	for _, acc := range byID {
		out = append(out, *acc)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AccountID < out[j].AccountID })
	return out, nil
}

// readVaultMetadata lê remembered-accounts.json (texto puro) para nome/email.
func readVaultMetadata(base string) map[string]vaultUser {
	out := map[string]vaultUser{}
	raw, err := os.ReadFile(filepath.Join(base, "remembered-accounts.json"))
	if err != nil {
		return out
	}
	var root map[string]struct {
		AccountID string    `json:"accountId"`
		Auth      vaultUser `json:"auth"`
	}
	if err := json.Unmarshal(raw, &root); err != nil {
		return out
	}
	for id, entry := range root {
		key := entry.AccountID
		if key == "" {
			key = id
		}
		out[key] = entry.Auth
	}
	return out
}

func applyMeta(acc *VaultAccount, meta map[string]vaultUser) {
	u, ok := meta[acc.AccountID]
	if !ok {
		return
	}
	acc.Name = u.Name
	acc.Email = u.DesensitizedEmail
	acc.AccioID = u.AccioID
	acc.UserLevel = u.UserLevel
}

// VaultLabel monta um nome estável para a conta no pool. O email vem
// desensitizado do Accio, então o rótulo usa o id da conta para não colidir.
func (v VaultAccount) VaultLabel() string {
	base := strings.TrimSpace(v.AccountID)
	if base == "" {
		base = strings.TrimSpace(v.AccioID)
	}
	if base == "" {
		base = fmt.Sprintf("vault-%d", time.Now().UnixNano())
	}
	return base + "@accio.desktop"
}
