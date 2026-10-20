package gateway

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"accio-proxy/internal/config"
)

var (
	debugMu  sync.Mutex
	debugDir = ""
)

func debugDirPath() string {
	if debugDir != "" {
		return debugDir
	}
	debugDir = config.DefaultDataDir()
	return debugDir
}

// debugLog grava linha de diagnóstico no gateway-debug.log ao lado do DB.
// Falha silenciosa — nunca quebra o request path.
func debugLog(format string, args ...any) {
	defer func() { _ = recover() }()
	line := time.Now().Format("15:04:05.000") + " " + fmt.Sprintf(format, args...) + "\n"
	debugMu.Lock()
	defer debugMu.Unlock()
	f, err := os.OpenFile(filepath.Join(debugDirPath(), "gateway-debug.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(line)
}

// previewJSON serializa o request truncado pra caber numa linha de log.
func previewJSON(v map[string]any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "<marshal error>"
	}
	s := string(b)
	s = strings.ReplaceAll(s, "\n", " ")
	const max = 4000
	if len(s) > max {
		s = s[:max] + "…"
	}
	return s
}
