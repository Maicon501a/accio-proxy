package main

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"accio-proxy/internal/phoenix"

	_ "modernc.org/sqlite"
)

func main() {
	db, err := sql.Open("sqlite", `C:\Users\maicon2\AppData\Local\AccioProxy\accounts.db`)
	if err != nil {
		panic(err)
	}
	rows, err := db.Query(`SELECT email, access_token FROM accounts WHERE status='active'`)
	if err != nil {
		panic(err)
	}
	type acc struct{ email, token string }
	var list []acc
	for rows.Next() {
		var a acc
		rows.Scan(&a.email, &a.token)
		list = append(list, a)
	}
	rows.Close()
	for _, a := range list {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		info, err := phoenix.QueryCredits(ctx, "P1gA0TtxF-WABzMXTyZp6uScVVv_3QRpyqQxXaeXWhwqUX5O7hY6Zc0q920sv-pi2DluMZyB9QTFmFKcALlsFT54", "0.33.0", a.token)
		if err != nil {
			fmt.Printf("%s -> ERRO: %v\n", a.email, err)
		} else {
			fmt.Printf("%s -> remaining=%v total=%v plan=%v\n", a.email, info["remaining"], info["total"], info["planType"])
		}
		cancel()
	}
}
