package main

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

func main() {
	db, _ := sql.Open("sqlite", `C:\Users\maicon2\AppData\Local\AccioProxy\accounts.db`)
	rows, _ := db.Query(`SELECT id, email, status, last_used_at, length(refresh_token), substr(credits_info,1,80) FROM accounts ORDER BY id`)
	for rows.Next() {
		var id int
		var email, status string
		var used sql.NullString
		var rt int
		var ci string
		rows.Scan(&id, &email, &status, &used, &rt, &ci)
		fmt.Printf("id=%d %s status=%s refresh_len=%d last_used=%v credits=%s\n", id, email, status, rt, used.String, ci)
	}
}
