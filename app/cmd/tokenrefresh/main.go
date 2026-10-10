// tokenrefresh renova o access_token das contas ativas do pool via
// POST /api/auth/refresh_token (wire contract verificado no repo legado).
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
	gateway   = "https://phoenix-gw.alibaba.com/api"
	appVer    = "0.33.0"
	accountsDB = `C:\Users\maicon2\AppData\Local\AccioProxy\accounts.db`
)

func main() {
	db, err := sql.Open("sqlite", accountsDB)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	rows, err := db.Query(`SELECT id, email, access_token, refresh_token FROM accounts WHERE status='active' AND refresh_token != '' ORDER BY id`)
	if err != nil {
		log.Fatalf("query: %v", err)
	}
	type acct struct {
		id      int64
		email   string
		access  string
		refresh string
	}
	var list []acct
	for rows.Next() {
		var a acct
		if err := rows.Scan(&a.id, &a.email, &a.access, &a.refresh); err != nil {
			log.Fatalf("scan: %v", err)
		}
		list = append(list, a)
	}
	rows.Close()
	if len(list) == 0 {
		log.Fatal("nenhuma conta com refresh_token")
	}

	hc := &http.Client{Timeout: 30 * time.Second}
	for _, a := range list {
		body, _ := json.Marshal(map[string]string{
			"utdid":       "52120c4be05c828503917ec8357c2bf7",
			"version":     appVer,
			"accessToken": a.access,
			"refreshToken": a.refresh,
		})
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, gateway+"/auth/refresh_token", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-language", "pt")
		req.Header.Set("x-locale", "pt-BR")
		req.Header.Set("x-platform", "pc-win")
		req.Header.Set("x-utdid", "52120c4be05c828503917ec8357c2bf7")
		req.Header.Set("x-app-version", appVer)
		req.Header.Set("x-os", "win32")
		req.Header.Set("x-deploy-target", "desktop")
		req.Header.Set("x-source", "ACCIO_DESKTOP")
		req.Header.Set("x-cna", "")
		req.Header.Set("x-package-region", "GLOBAL")
		resp, err := hc.Do(req)
		if err != nil {
			fmt.Printf("%s: transport: %v\n", a.email, err)
			continue
		}
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			fmt.Printf("%s: HTTP %d: %.200s\n", a.email, resp.StatusCode, raw)
			continue
		}
		var root map[string]any
		if err := json.Unmarshal(raw, &root); err != nil {
			fmt.Printf("%s: json invalido: %v (%.200s)\n", a.email, err, raw)
			continue
		}
		data, _ := root["data"].(map[string]any)
		if data == nil {
			data = root
		}
		naccess, _ := data["accessToken"].(string)
		nrefresh, _ := data["refreshToken"].(string)
		if naccess == "" {
			fmt.Printf("%s: sem accessToken no body: %.200s\n", a.email, raw)
			continue
		}
		if nrefresh == "" {
			nrefresh = a.refresh
		}
		if _, err := db.Exec(`UPDATE accounts SET access_token=?, refresh_token=?, last_used_at=datetime('now') WHERE id=?`, naccess, nrefresh, a.id); err != nil {
			fmt.Printf("%s: update db: %v\n", a.email, err)
			continue
		}
		fmt.Printf("%s: REFRESH OK (novo access salvo)\n", a.email)
	}
}
