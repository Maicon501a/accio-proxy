// probeprofile testa endpoints candidatos de perfil no gateway Accio com um
// token real do accounts.db, procurando um que devolva o email da conta.
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"time"

	_ "modernc.org/sqlite"
)

const base = "https://phoenix-gw.alibaba.com/api"

var candidates = []string{
	"/user/profile",
	"/user/current",
	"/user/info",
	"/user/me",
	"/account/profile",
	"/passport/user",
	"/user/getUserInfo",
	"/user/basicInfo",
	"/member/profile",
}

func main() {
	db, err := sql.Open("sqlite", `C:\Users\maicon2\AppData\Local\AccioProxy\accounts.db`)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	var email, token string
	if err := db.QueryRow(`SELECT email, access_token FROM accounts WHERE status='active' AND access_token != '' ORDER BY id LIMIT 1`).Scan(&email, &token); err != nil {
		log.Fatal(err)
	}
	fmt.Println("conta:", email, "token:", token[:8]+"…")

	hc := &http.Client{Timeout: 20 * time.Second}
	for _, path := range candidates {
		req, _ := http.NewRequest("GET", base+path, nil)
		q := req.URL.Query()
		q.Set("accessToken", token)
		q.Set("utdid", "probe")
		q.Set("version", "0.33.0")
		req.URL.RawQuery = q.Encode()
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-language", "pt")
		req.Header.Set("x-locale", "pt-BR")
		req.Header.Set("x-platform", "pc-win")
		req.Header.Set("user-agent", "AccioWork/0.33.0 (Windows NT 10.0; Win64; x64)")
		resp, err := hc.Do(req)
		if err != nil {
			fmt.Printf("%-22s ERRO %v\n", path, err)
			continue
		}
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		dumped, _ := httputil.DumpResponse(resp, false)
		_ = dumped
		var pretty string
		var m map[string]any
		if json.Unmarshal(raw, &m) == nil {
			b, _ := json.MarshalIndent(m, "", "  ")
			pretty = string(b)
		} else {
			pretty = string(raw)
		}
		if len(pretty) > 600 {
			pretty = pretty[:600]
		}
		fmt.Printf("%-22s HTTP %d\n%s\n\n", path, resp.StatusCode, pretty)
	}
}
