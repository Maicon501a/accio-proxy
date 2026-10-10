package main

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

func main() {
	db, err := sql.Open("sqlite", `C:\Users\maicon2\AppData\Local\AccioProxy\accounts.db`)
	if err != nil {
		panic(err)
	}
	defer db.Close()
	rows, err := db.Query(`SELECT id,email,substr(access_token,1,16),length(refresh_token),status FROM accounts ORDER BY id`)
	if err != nil {
		panic(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, rl int
		var e, a, st string
		rows.Scan(&id, &e, &a, &rl, &st)
		fmt.Printf("id=%d %s access=%s... refreshlen=%d status=%s\n", id, e, a, rl, st)
	}
}
