// updatetoken troca o access_token de uma conta no pool do proxy.
// uso: updatetoken <email|id> <novoAccessToken>
package main

import (
	"database/sql"
	"log"
	"os"

	_ "modernc.org/sqlite"
)

func main() {
	if len(os.Args) < 3 {
		log.Fatal("uso: updatetoken <email|id> <accessToken>")
	}
	key, token := os.Args[1], os.Args[2]
	db, err := sql.Open("sqlite", `C:\Users\maicon2\AppData\Local\AccioProxy\accounts.db`)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	res, err := db.Exec(`UPDATE accounts SET access_token=?, status='active' WHERE email LIKE ?`, token, "%"+key+"%")
	if err != nil {
		log.Fatal(err)
	}
	n, _ := res.RowsAffected()
	log.Printf("rows afetadas: %d", n)
}
