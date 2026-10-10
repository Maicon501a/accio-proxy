package main

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

func main() {
	db, err := sql.Open("sqlite", `C:\Users\maicon2\AppData\Roaming\reqable-mcp\requests.db`)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("=== hosts accio ===")
	rows, _ := db.Query(`SELECT DISTINCT host, COUNT(*) FROM requests WHERE host LIKE '%accio%' GROUP BY host ORDER BY 2 DESC`)
	for rows.Next() {
		var h string
		var n int
		rows.Scan(&h, &n)
		fmt.Printf("%6d  %s\n", n, h)
	}
	rows.Close()

	fmt.Println("=== urls login/code/auth em hosts accio/alibaba gateway ===")
	q := `SELECT DISTINCT host, method, substr(url,1,160) FROM requests
	      WHERE (host LIKE '%accio%' OR host LIKE 'acs%alibaba%' OR host LIKE 'acs%taobao%')
	        AND (lower(url) LIKE '%code%' OR lower(url) LIKE '%login%' OR lower(url) LIKE '%auth%' OR lower(url) LIKE '%session%' OR lower(url) LIKE '%token%')
	      ORDER BY host, url LIMIT 60`
	rows2, _ := db.Query(q)
	for rows2.Next() {
		var h, m, u string
		rows2.Scan(&h, &m, &u)
		fmt.Printf("%-30s %-5s %s\n", h, m, u)
	}
	rows2.Close()
}
