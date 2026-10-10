package main

// udpdns: UDP DNS relay for the MuMu emulator guest.
//
// The guest cannot resolve names with a pure-Go binary (Android resolves
// through netd/Binder, not a resolv.conf file). This relay listens on
// 0.0.0.0:5353 (reachable from the guest as 10.0.2.2:5353) and forwards
// each datagram to an upstream resolver on the host, then relays the
// answer back. One upstream socket per client address is kept for the
// lifetime of a "session" (idle timeout 30s).
//
// Usage: udpdns.exe [listen_port] [upstream_host:port]

import (
	"log"
	"net"
	"os"
	"sync"
	"time"
)

func main() {
	listen := ":5353"
	upstream := "8.8.8.8:53"
	if len(os.Args) > 1 {
		listen = ":" + os.Args[1]
	}
	if len(os.Args) > 2 {
		upstream = os.Args[2]
	}
	addr, err := net.ResolveUDPAddr("udp", listen)
	if err != nil {
		log.Fatal(err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("udpdns %s -> %s", listen, upstream)

	sessions := map[string]*session{}
	var mu sync.Mutex

	buf := make([]byte, 4096)
	for {
		n, client, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("read: %v", err)
			continue
		}
		mu.Lock()
		s, ok := sessions[client.String()]
		if !ok {
			up, err := net.DialTimeout("udp", upstream, 5*time.Second)
			if err != nil {
				mu.Unlock()
				log.Printf("dial upstream: %v", err)
				continue
			}
			s = &session{up: up.(*net.UDPConn), client: client, done: make(chan struct{})}
			sessions[client.String()] = s
			go func(key string, s *session) {
				b := make([]byte, 4096)
				for {
					_ = s.up.SetReadDeadline(time.Now().Add(30 * time.Second))
					n, err := s.up.Read(b)
					if err != nil {
						break
					}
					conn.WriteToUDP(b[:n], s.client)
				}
				mu.Lock()
				delete(sessions, key)
				mu.Unlock()
				s.up.Close()
			}(client.String(), s)
		}
		mu.Unlock()
		s.up.Write(buf[:n])
	}
}

type session struct {
	up     *net.UDPConn
	client *net.UDPAddr
	done   chan struct{}
}
