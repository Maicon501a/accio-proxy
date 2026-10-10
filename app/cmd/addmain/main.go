// addmain insere a conta principal (1790559875) no pool do proxy com o
// accessToken valido (renovado via refresh_token) e marca o throwaway morto.
package main

import (
	"database/sql"
	"log"

	_ "modernc.org/sqlite"
)

func main() {
	db, err := sql.Open("sqlite", `C:\Users\maicon2\AppData\Local\AccioProxy\accounts.db`)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	// throwaway: token rejeitado pelo servidor
	if _, err := db.Exec(`UPDATE accounts SET status='dead' WHERE email LIKE '%1790888909%'`); err != nil {
		log.Fatal(err)
	}
	// conta principal: upsert
	_, err = db.Exec(`INSERT INTO accounts (email, access_token, user_id, utdid, status, credits, note, refresh_token)
		VALUES (?,?,?,?, 'active','','main-gmail',?)
		ON CONFLICT(email) DO UPDATE SET access_token=excluded.access_token, status='active', refresh_token=excluded.refresh_token`,
		"1790559875@accio.local",
		"a493ff11237c2d53927ea42454b8b1e9",
		"1790559875",
		"52120c4be05c828503917ec8357c2bf7",
		"393ae3be335620f4c8a8e3396c9bc8a09c0e889dc3139d350425c2840f134b4a6d399f1fbdc64717edde5467bfc02409a70f4278dba7ab9114f0b34312e0f29d")
	if err != nil {
		log.Fatal(err)
	}
	log.Println("ok")
}
