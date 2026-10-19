// Package sgbridge manages the SecurityGuard Node sidecar that generates
// the Baxia WAF headers (pctb-x-sign, x-mini-wua, sgext, …) the phoenix
// gateway requires on chat requests.
//
// The sidecar speaks JSON lines on stdin/stdout: {"id","url"} in,
// {"id","headers"} out, plus a one-time {"ready":true} after UMID init.
// The native addon (security_guard.node) only signs for the desktop appKey
// 35336201 — Android mtop signing is a different family and out of scope.
package sgbridge

import (
	"bufio"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

//go:embed sg_daemon.js
var defaultSGDaemonJS []byte

const DesktopAppKey = "35336201"

// Signer implements phoenix.Signer backed by the daemon.
type Signer struct {
	mu      sync.Mutex
	daemon  *daemon
	dataDir string
}

func New(dataDir string) *Signer { return &Signer{dataDir: dataDir} }

// Headers returns the WAF headers for one request URL.
func (s *Signer) Headers(ctx context.Context, url string) (map[string]string, error) {
	callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	d, err := s.get(callCtx)
	if err != nil {
		return nil, err
	}
	return d.headers(callCtx, url)
}

// Close stops the sidecar.
func (s *Signer) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.daemon != nil {
		s.daemon.shutdown()
		s.daemon = nil
	}
}

// get returns the live daemon, (re)starting it when dead. A failed start is
// not cached — the next call retries.
func (s *Signer) get(ctx context.Context) (*daemon, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.daemon != nil && !s.daemon.dead() {
		return s.daemon, nil
	}
	d := &daemon{pending: map[string]chan sgResponse{}, done: make(chan struct{})}
	if err := d.start(ctx, s.dataDir); err != nil {
		return nil, err
	}
	s.daemon = d
	return d, nil
}

// ---- daemon process ----

type daemon struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	scanner *bufio.Scanner
	ready   bool
	reqID   int64
	pending map[string]chan sgResponse
	done    chan struct{}
	doneOnce sync.Once
}

func (d *daemon) signalDone() {
	d.doneOnce.Do(func() { close(d.done) })
}

type sgRequest struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

type sgResponse struct {
	ID      string            `json:"id"`
	Ready   bool              `json:"ready,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Error   string            `json:"error,omitempty"`
}

func (d *daemon) dead() bool {
	select {
	case <-d.done:
		return true
	default:
		return false
	}
}

func (d *daemon) start(ctx context.Context, dataDir string) error {
	nodePath, err := nodePath()
	if err != nil {
		return err
	}
	bundleDir, err := ensureBundle(dataDir)
	if err != nil {
		return err
	}
	cmdScript := filepath.Join(bundleDir, "sg_daemon.js")
	cmdEnv := []string{"ACCIO_SG_APPKEY=" + DesktopAppKey}
	if seed := strings.TrimSpace(os.Getenv("ACCIO_SG_SPOOF_SEED")); seed != "" {
		if err := materializeHook(bundleDir); err != nil {
			log.Printf("sgbridge: hook materialize failed, usando daemon sem spoof: %v", err)
		} else {
			cmdScript = filepath.Join(bundleDir, "sg_daemon_hooked.js")
			cmdEnv = append(cmdEnv,
				"SGTRACE_SPOOF_SEED="+seed,
				"SGTRACE_LOG="+filepath.Join(dataDir, "sgtrace.log"),
				"NODE_PATH="+os.Getenv("ACCIO_SGTRACE_NODE_MODULES"),
			)
		}
	}
	cmd := exec.Command(nodePath, cmdScript)
	cmd.Dir = bundleDir
	cmd.Env = append(os.Environ(), cmdEnv...)
	cmd.Stderr = os.Stderr
	hideWindow(cmd)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start sg daemon: %w", err)
	}
	d.cmd, d.stdin = cmd, stdin
	d.scanner = bufio.NewScanner(stdout)
	d.scanner.Buffer(make([]byte, 1<<20), 1<<20)

	go func() {
		_ = cmd.Wait()
		d.signalDone()
	}()
	go d.readLoop()

	readyCh := make(chan sgResponse, 1)
	d.mu.Lock()
	d.pending["__ready__"] = readyCh
	d.mu.Unlock()

	timer := time.NewTimer(15 * time.Second)
	defer timer.Stop()
	select {
	case resp := <-readyCh:
		if resp.Error != "" {
			return fmt.Errorf("sg daemon init failed: %s", resp.Error)
		}
		d.ready = true
		return nil
	case <-ctx.Done():
		_ = cmd.Process.Kill()
		return fmt.Errorf("sg daemon start interrupted: %w", ctx.Err())
	case <-timer.C:
		_ = cmd.Process.Kill()
		return errors.New("sg daemon ready timeout (15s)")
	case <-d.done:
		return errors.New("sg daemon exited before ready")
	}
}

func (d *daemon) readLoop() {
	defer d.signalDone()
	for d.scanner.Scan() {
		line := d.scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var resp sgResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			continue
		}
		d.mu.Lock()
		key := resp.ID
		if resp.Ready {
			key = "__ready__"
		}
		if ch, ok := d.pending[key]; ok {
			ch <- resp
			delete(d.pending, key)
		}
		d.mu.Unlock()
	}
	d.mu.Lock()
	for id, ch := range d.pending {
		ch <- sgResponse{ID: id, Error: "sg daemon process exited"}
		delete(d.pending, id)
	}
	d.mu.Unlock()
}

func (d *daemon) headers(ctx context.Context, url string) (map[string]string, error) {
	if !d.ready {
		return nil, errors.New("sg daemon not ready")
	}
	d.mu.Lock()
	d.reqID++
	id := fmt.Sprintf("sg-%d-%d", time.Now().UnixMilli(), d.reqID)
	ch := make(chan sgResponse, 1)
	d.pending[id] = ch
	d.mu.Unlock()

	data, _ := json.Marshal(sgRequest{ID: id, URL: url})
	data = append(data, '\n')
	if _, err := d.stdin.Write(data); err != nil {
		d.mu.Lock()
		delete(d.pending, id)
		d.mu.Unlock()
		return nil, fmt.Errorf("write to sg daemon: %w", err)
	}

	select {
	case resp := <-ch:
		if resp.Error != "" {
			return nil, errors.New("security guard: " + resp.Error)
		}
		if len(resp.Headers) == 0 {
			return nil, errors.New("security guard returned no headers")
		}
		return resp.Headers, nil
	case <-ctx.Done():
		d.mu.Lock()
		delete(d.pending, id)
		d.mu.Unlock()
		return nil, fmt.Errorf("security guard timeout: %w", ctx.Err())
	case <-d.done:
		return nil, errors.New("security guard daemon crashed")
	}
}

func (d *daemon) shutdown() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.stdin != nil {
		_ = d.stdin.Close()
	}
	if d.cmd != nil && d.cmd.Process != nil {
		select {
		case <-d.done:
		case <-time.After(2 * time.Second):
			_ = d.cmd.Process.Kill()
		}
	}
}

// ---- bundle management ----

func hasNativeFiles(dir string) bool {
	if dir == "" {
		return false
	}
	for _, rel := range []string{
		filepath.Join("prebuild", "win32-x64", "security_guard.node"),
		filepath.Join("resources", "x64", "AliSafeProxy.dll"),
	} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			return false
		}
	}
	return true
}

func bundleComplete(dir string) bool {
	if !hasNativeFiles(dir) {
		return false
	}
	daemonPath := filepath.Join(dir, "sg_daemon.js")
	if _, err := os.Stat(daemonPath); err != nil {
		return false
	}
	return true
}

func ensureBundle(dataDir string) (string, error) {
	target := filepath.Join(dataDir, "security-guard")

	// 1. Se já está completo no target, retorna na hora
	if bundleComplete(target) {
		return target, nil
	}

	// 2. Se temos o bundle embutido no binário, extrai direto para o target
	if hasEmbeddedBundle() {
		log.Printf("sgbridge: extraindo bundle nativo embutido para %s", target)
		if err := extractEmbeddedBundle(target); err != nil {
			log.Printf("sgbridge: falha ao extrair bundle embutido: %v (tentando fontes externas)", err)
		} else if bundleComplete(target) {
			log.Printf("sgbridge: bundle nativo extraído com sucesso")
			return target, nil
		}
	}

	// 3. Fallback: Se o target ainda não tem os binários nativos do SecurityGuard, copia de fontes instaladas do Accio
	if !hasNativeFiles(target) {
		for _, src := range bundleSources() {
			if !hasNativeFiles(src) {
				continue
			}
			if err := copyTree(src, target); err != nil {
				log.Printf("sgbridge: bundle copy from %s failed: %v", src, err)
				continue
			}
			if hasNativeFiles(target) {
				break
			}
		}
	}

	// Sempre assegura que o sg_daemon.js está gravado no target a partir do código Go embutido
	if len(defaultSGDaemonJS) > 0 {
		daemonFile := filepath.Join(target, "sg_daemon.js")
		if _, err := os.Stat(daemonFile); err != nil {
			_ = os.MkdirAll(target, 0o755)
			_ = os.WriteFile(daemonFile, defaultSGDaemonJS, 0o644)
		}
	}

	if bundleComplete(target) {
		return target, nil
	}

	return "", errors.New("security-guard bundle not found (verifique se o aplicativo Accio Desktop oficial está instalado na máquina)")
}

func bundleSources() []string {
	var out []string
	if v := strings.TrimSpace(os.Getenv("ACCIO_SG_DIR")); v != "" {
		out = append(out, v)
	}
	if home, err := os.UserHomeDir(); err == nil {
		out = append(out, filepath.Join(home, "AppData", "Local", "GrokDesktop", "security-guard"))
		out = append(out, filepath.Join(home, "AppData", "Local", "AccioProxy", "security-guard"))
	}
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		out = append(out, filepath.Join(local, "Programs", "Accio", "resources", "app.asar.unpacked", "node_modules", "@phoenix-common", "security-guard"))
	}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		out = append(out, filepath.Join(exeDir, "security-guard"))
		out = append(out, filepath.Join(exeDir, "..", "security-guard"))
	}
	return out
}

// materializeHook escreve o wrapper com hooks (sgtrace) no bundle.
// Requer ACCIO_SGTRACE_DLL (caminho do sgtrace.dll) e, opcionalmente,
// ACCIO_SGTRACE_NODE_MODULES (pasta com node_modules do koffi, ex.: ...\Temp\opencode\node_modules).
func materializeHook(bundleDir string) error {
	dllSrc := strings.TrimSpace(os.Getenv("ACCIO_SGTRACE_DLL"))
	if dllSrc == "" {
		return errors.New("ACCIO_SGTRACE_DLL nao definido")
	}
	data, err := os.ReadFile(dllSrc)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(bundleDir, "sgtrace.dll"), data, 0644); err != nil {
		return err
	}
	wrapper := `// gerado pelo sgbridge: preloads sgtrace (spoof) e roda sg_daemon
const path = require("path");
const koffi = require("koffi");
const trace = koffi.load(path.join(__dirname, "sgtrace.dll"));
const rc = trace.func("Install", "int", ["str"])(process.env.SGTRACE_LOG || path.join(__dirname, "sgtrace.log"));
process.stderr.write("[sg-hooked] Install -> " + rc + " seed=" + process.env.SGTRACE_SPOOF_SEED + "\n");
require(path.join(__dirname, "sg_daemon.js"));
`
	return os.WriteFile(filepath.Join(bundleDir, "sg_daemon_hooked.js"), []byte(wrapper), 0644)
}

func copyTree(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		b, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
			return err
		}
		return os.WriteFile(dst, b, 0o600)
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := copyTree(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

func nodePath() (string, error) {
	if v := strings.TrimSpace(os.Getenv("ACCIO_NODE_PATH")); v != "" {
		return v, nil
	}
	if v, err := exec.LookPath("node"); err == nil {
		return v, nil
	}
	// Accio ships its own Node under pre-install.
	appData := os.Getenv("APPDATA")
	if appData == "" {
		home, _ := os.UserHomeDir()
		appData = filepath.Join(home, "AppData", "Roaming")
	}
	root := filepath.Join(appData, "Accio", "pre-install")
	var found string
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() && strings.EqualFold(info.Name(), "node.exe") {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	if found != "" {
		return found, nil
	}
	for _, c := range []string{
		`C:\Program Files\nodejs\node.exe`,
		`C:\Program Files (x86)\nodejs\node.exe`,
	} {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return "", errors.New("node.js not found; install Node or set ACCIO_NODE_PATH")
}
