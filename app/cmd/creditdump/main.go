package main

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

func main() {
	db, _ := sql.Open("sqlite", `C:\Users\maicon2\AppData\Local\AccioProxy\accounts.db`)
	rows, _ := db.Query(`SELECT email, credits, substr(credits_info,1,260) FROM accounts WHERE status='active'`)
	for rows.Next() {
		var e, ci string
		var c int
		rows.Scan(&e, &c, &ci)
		fmt.Printf("%s credits=%d info=%s\n", e, c, ci)
	}
}
