// Package config persists application settings next to the executable.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type Config struct {
	mu   sync.Mutex
	path string

	GatewayPort  int    `json:"gateway_port"` // OpenAI-compatible port
	ADBPath      string `json:"adb_path"`
	DeviceSerial string `json:"device_serial"`
	DataDir      string `json:"data_dir"`
	// Utdid is the desktop device id sent on every gateway request. Generated
	// once and persisted so the WAF sees a stable device identity.
	Utdid string `json:"utdid"`
	// EmulatorEnabled liga/desliga tudo que envolve adb/emulador. Desligado
	// por padrão: sem spawn de adb.exe, sem terminal piscando, sem Health
	// consultando o emulador a cada poll.
	EmulatorEnabled bool `json:"emulator_enabled"`
	// ProxyMode seleciona a origem das contas e do roteamento:
	// "official": usa o app do Accio oficial aberto (contas em tempo real, switch, créditos ao vivo)
	// "direct": usa o pool local do banco de dados (modo standalone / endpoints diretos)
	ProxyMode string `json:"proxy_mode"`
}

func Default() *Config {
	return &Config{
		GatewayPort:  8790,
		ADBPath:      platformADBPath(),
		DeviceSerial: "127.0.0.1:16384",
		DataDir:      platformDataDir(),
		ProxyMode:    platformDefaultMode(),
	}
}

// Load reads the config file or creates it with defaults.
func Load() (*Config, error) {
	c := Default()
	path, err := c.filePath()
	if err != nil {
		return c, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			c.ensureUtdid()
			_ = c.Save()
			return c, nil
		}
		return c, err
	}
	if err := json.Unmarshal(data, c); err != nil {
		return Default(), nil
	}
	if c.ProxyMode == "" {
		c.ProxyMode = platformDefaultMode()
	}
	if c.ProxyMode == "official" && !platformSupportsOfficial() {
		// config herdado/copiado do Windows: o modo oficial não tem como
		// funcionar aqui (sem Accio Desktop, sem DPAPI). Cai no direto.
		c.ProxyMode = "direct"
	}
	c.path = path
	c.ensureUtdid()
	return c, nil
}

// ensureUtdid generates and persists a desktop device id on first run.
func (c *Config) ensureUtdid() {
	if c.Utdid != "" {
		return
	}
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	c.Utdid = hex.EncodeToString(b)
	_ = c.Save()
}

func (c *Config) filePath() (string, error) {
	if c.path != "" {
		return c.path, nil
	}
	c.path = filepath.Join(platformConfigDir(), "config.json")
	return c.path, nil
}

func (c *Config) Save() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	path, err := c.filePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (c *Config) Set(port int, adb, serial string) error {
	c.mu.Lock()
	c.GatewayPort = port
	c.ADBPath = adb
	c.DeviceSerial = serial
	c.mu.Unlock()
	return c.Save()
}

// SetEmulatorEnabled persiste o interruptor do emulador.
func (c *Config) SetEmulatorEnabled(on bool) error {
	c.mu.Lock()
	c.EmulatorEnabled = on
	c.mu.Unlock()
	return c.Save()
}

// SetProxyMode alterna entre "official" e "direct".
func (c *Config) SetProxyMode(mode string) error {
	c.mu.Lock()
	if mode != "direct" && mode != "official" {
		mode = platformDefaultMode()
	}
	c.ProxyMode = mode
	c.mu.Unlock()
	return c.Save()
}

// AccountsDB returns the sqlite path inside the data dir.
func (c *Config) AccountsDB() string {
	return filepath.Join(c.DataDir, "accounts.db")
}

// DefaultDataDir devolve a pasta de dados da plataforma, sem depender de um
// Config carregado. Usado por logs e utilitários.
func DefaultDataDir() string { return platformDataDir() }

// OfficialSupported informa se o modo oficial pode funcionar nesta plataforma
// (precisa do Accio Desktop e do DPAPI, ambos Windows-only).
func OfficialSupported() bool { return platformSupportsOfficial() }
