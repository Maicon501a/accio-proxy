//go:build windows

package config

import (
	"os"
	"path/filepath"
)

// platformDataDir é a pasta de dados do proxy no Windows: %LOCALAPPDATA%.
func platformDataDir() string {
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		return filepath.Join(local, "AccioProxy")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "AppData", "Local", "AccioProxy")
}

// platformConfigDir é onde o config.json mora. No Windows fica junto do
// banco (mesma pasta do histórico de versões anteriores).
func platformConfigDir() string { return platformDataDir() }

// platformADBPath é o default do emulador no Windows.
func platformADBPath() string {
	return `D:\Program Files\Netease\MuMuPlayer\nx_main\adb.exe`
}
