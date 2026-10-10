package factory

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// ReqableHarvester extracts accessTokens from the reqable-mcp capture
// database. The phoenix WS handshake URL
// (wss://phoenix-gw.alibaba.com/ws/vps/app?connectionToken=X&accessToken=Y)
// is stored as a row in the requests table — reading it read-only with
// busy_timeout is safe while reqable keeps writing.
type ReqableHarvester struct {
	// DBPath overrides the default %APPDATA%\reqable-mcp\requests.db.
	DBPath string

	mu   sync.Mutex
	seen map[string]bool
}

var tokenPairRe = regexp.MustCompile(`connectionToken=([a-f0-9]{16,64})&accessToken=([a-f0-9]{16,64})`)

func (h *ReqableHarvester) dbPath() string {
	if h.DBPath != "" {
		return h.DBPath
	}
	return filepath.Join(os.Getenv("APPDATA"), "reqable-mcp", "requests.db")
}

// WaitForAccessToken polls the capture DB until a WS handshake carrying an
// accessToken unseen by this harvester appears (or timeout).
func (h *ReqableHarvester) WaitForAccessToken(ctx context.Context, since time.Time, timeout time.Duration) (string, string, error) {
	deadline := time.Now().Add(timeout)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return "", "", fmt.Errorf("no phoenix WS handshake captured before timeout")
		}
		pollCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		acc, conn, err := h.latest(pollCtx, since)
		cancel()
		if err == nil && acc != "" {
			return acc, conn, nil
		}
		if ctx.Err() != nil {
			return "", "", ctx.Err()
		}
		select {
		case <-ctx.Done():
			return "", "", ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

// latest returns the newest unseen accessToken captured after `since`.
func (h *ReqableHarvester) latest(ctx context.Context, since time.Time) (string, string, error) {
	path := h.dbPath()
	if _, err := os.Stat(path); err != nil {
		return "", "", fmt.Errorf("capture db not found: %s", path)
	}
	// mode=ro + immutable=0: read-only but still sees new WAL commits.
	dsn := fmt.Sprintf("file:%s?mode=ro&_pragma=busy_timeout(5000)&_pragma=query_only(true)",
		filepath.ToSlash(path))
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return "", "", err
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx, `
		SELECT url FROM requests
		WHERE host = 'phoenix-gw.alibaba.com' AND url LIKE '%accessToken%'
		ORDER BY created_at DESC LIMIT 20`)
	if err != nil {
		return "", "", err
	}
	defer rows.Close()

	for rows.Next() {
		var url string
		if err := rows.Scan(&url); err != nil {
			return "", "", err
		}
		m := tokenPairRe.FindStringSubmatch(url)
		if m == nil {
			continue
		}
		conn, acc := m[1], m[2]
		h.mu.Lock()
		if h.seen == nil {
			h.seen = map[string]bool{}
		}
		dup := h.seen[acc]
		h.seen[acc] = true
		h.mu.Unlock()
		if dup {
			continue
		}
		return acc, conn, nil
	}
	return "", "", rows.Err()
}

// SnapshotNow marks every token currently in the capture DB as seen. Call it
// right before driving a login flow so the harvest only accepts handshakes
// produced by THAT login — otherwise the first poll returns a stale token
// from a previous session.
func (h *ReqableHarvester) SnapshotNow(ctx context.Context) error {
	path := h.dbPath()
	if _, err := os.Stat(path); err != nil {
		return nil // no capture db yet: nothing to snapshot
	}
	dsn := fmt.Sprintf("file:%s?mode=ro&_pragma=busy_timeout(5000)&_pragma=query_only(true)",
		filepath.ToSlash(path))
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	rows, err := db.QueryContext(ctx, `
		SELECT url FROM requests
		WHERE host = 'phoenix-gw.alibaba.com' AND url LIKE '%accessToken%'
		ORDER BY created_at DESC LIMIT 50`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var url string
		if err := rows.Scan(&url); err != nil {
			return err
		}
		if m := tokenPairRe.FindStringSubmatch(url); m != nil {
			h.MarkSeeded(m[2])
		}
	}
	return rows.Err()
}

// MarkSeeded registers already-known tokens so the factory does not harvest
// a stale handshake from a previous login session.
func (h *ReqableHarvester) MarkSeeded(tokens ...string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.seen == nil {
		h.seen = map[string]bool{}
	}
	for _, t := range tokens {
		h.seen[t] = true
	}
}
