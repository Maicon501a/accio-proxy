package main

// tcppipe: minimal TCP pipe for the MuMu emulator transparent-proxy chain.
//
// The emulator's Go redsocks resolves 10.0.2.2 to a Tailscale IP the guest
// cannot route. This bridge listens on 0.0.0.0:3129 (reachable from the
// emulator as 10.0.2.2:3129) and forwards every connection to the Reqable
// proxy on 127.0.0.1:9000, which only the host can reach.
//
// Usage: tcppipe.exe [listen_port] [target_host:target_port]

import (
	"io"
	"log"
	"net"
	"os"
)

func main() {
	listen := ":3129"
	target := "127.0.0.1:9000"
	if len(os.Args) > 1 {
		listen = ":" + os.Args[1]
	}
	if len(os.Args) > 2 {
		target = os.Args[2]
	}
	ln, err := net.Listen("tcp", listen)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("tcppipe %s -> %s", listen, target)
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go func(c net.Conn) {
			defer c.Close()
			up, err := net.Dial("tcp", target)
			if err != nil {
				return
			}
			defer up.Close()
			done := make(chan struct{}, 2)
			go func() { io.Copy(up, c); done <- struct{}{} }()
			go func() { io.Copy(c, up); done <- struct{}{} }()
			<-done
		}(conn)
	}
}
