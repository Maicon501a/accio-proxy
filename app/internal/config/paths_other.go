//go:build !windows

package config

import (
	"os"
	"path/filepath"
)

// platformDataDir segue o XDG base dir spec: $XDG_DATA_HOME/accio-proxy,
// caindo para ~/.local/share/accio-proxy. É onde ficam accounts.db e afins.
func platformDataDir() string {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "accio-proxy")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "accio-proxy")
	}
	return filepath.Join(home, ".local", "share", "accio-proxy")
}

// platformConfigDir segue o XDG config dir: $XDG_CONFIG_HOME/accio-proxy,
// caindo para ~/.config/accio-proxy.
func platformConfigDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "accio-proxy")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "accio-proxy")
	}
	return filepath.Join(home, ".config", "accio-proxy")
}

// platformADBPath: sem MuMu por padrão fora do Windows, usa o adb do PATH
// quando existir (o emulador é opcional e vem desligado).
func platformADBPath() string {
	return ""
}
