// tokenrefresh2 renova contas usando refreshTokens extraidos do
// remembered-accounts.enc (Electron safeStorage, ja descriptografado).
package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	_ "modernc.org/sqlite"
)

const (
	gateway = "https://phoenix-gw.alibaba.com/api"
	appVer  = "0.33.0"
	utdid   = "52120c4be05c828503917ec8357c2bf7"
	dbPath  = `C:\Users\maicon2\AppData\Local\AccioProxy\accounts.db`
)

// refreshTokens extraidos do remembered-accounts.enc (2026-09-21)
var accounts = []struct {
	id      string
	access  string
	refresh string
	sgcookie string
}{
	{"1790888909", "a560ac7258be40823b2a22280da50856", "20015468bbe2a06d8766d31355ae583c9c0e889dc3139d350425c2840f134b4a0d5e46cc5c2ea8c459025fe4f277e02b2b8de77ba51effdcffa195c4d5ca9f56", "E1002tYLeBOIjvRKwweGUxpiUtwR+Axq89ix7PE/shUU2o9R7srgUU3tYJZFh/BMuAHyL4ZB5MLn+fA+3NwvNraDG0EzA1U6NH16HI1fN1oBKL0="},
	{"1790559875", "a493ff11237c2d53927ea42454b8b1e9", "393ae3be335620f4c8a8e3396c9bc8a09c0e889dc3139d350425c2840f134b4a6d399f1fbdc64717edde5467bfc02409a70f4278dba7ab9114f0b34312e0f29d", "E100FQY6WjHI7rYWeI3T9n+Ju45SGcx2tG4gQxRNFKpR3OrItID9eWjVGNPQS6wWGZV12cQNnJXXd+FE/Me0XGEoFQSNHVed1Cb1BSMQlbHFAIs="},
}

func main() {
	hc := &http.Client{Timeout: 30 * time.Second}
	type result struct{ access, refresh string }
	results := map[string]result{}
	for _, a := range accounts {
		body, _ := json.Marshal(map[string]string{
			"utdid": utdid, "version": appVer,
			"accessToken": a.access, "refreshToken": a.refresh,
		})
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, gateway+"/auth/refresh_token", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-language", "pt")
		req.Header.Set("x-locale", "pt-BR")
		req.Header.Set("x-platform", "pc-win")
		req.Header.Set("x-utdid", utdid)
		req.Header.Set("x-app-version", appVer)
		req.Header.Set("x-os", "win32")
		req.Header.Set("x-deploy-target", "desktop")
		req.Header.Set("x-source", "ACCIO_DESKTOP")
		req.Header.Set("x-cna", "")
		req.Header.Set("x-package-region", "GLOBAL")
		resp, err := hc.Do(req)
		if err != nil {
			fmt.Printf("%s: transport: %v\n", a.id, err)
			continue
		}
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			fmt.Printf("%s: HTTP %d: %.220s\n", a.id, resp.StatusCode, raw)
			continue
		}
		var root map[string]any
		json.Unmarshal(raw, &root)
		data, _ := root["data"].(map[string]any)
		if data == nil {
			data = root
		}
		naccess, _ := data["accessToken"].(string)
		nrefresh, _ := data["refreshToken"].(string)
		fmt.Printf("%s: success=%v novoAccess=%q novoRefreshLen=%d raw=%.200s\n", a.id, root["success"], naccess, len(nrefresh), raw)
		if naccess != "" {
			if nrefresh == "" {
				nrefresh = a.refresh
			}
			results[a.id] = result{naccess, nrefresh}
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	for id, r := range results {
		res, err := db.Exec(`UPDATE accounts SET access_token=?, refresh_token=?, status='active' WHERE email LIKE ?`, r.access, r.refresh, "%"+id+"%")
		if err != nil {
			fmt.Printf("db %s: %v\n", id, err)
			continue
		}
		n, _ := res.RowsAffected()
		fmt.Printf("db %s: %d rows\n", id, n)
	}
}
