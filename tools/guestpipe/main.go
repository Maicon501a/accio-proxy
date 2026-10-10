package main

// guestpipe v5: transparent relay with capture-list MITM.
//
// iptables REDIRECTs the Accio app's TCP flows (uid 10066, non-loopback)
// to 127.0.0.1:3130 (this process). For each accepted connection:
//
//  1. The ORIGINAL destination is recovered with getsockopt(SO_ORIGINAL_DST)
//     and dialed IMMEDIATELY — before any byte sniffing. Some app protocols
//     (IMPaas LWP / ACCS) are server-first: the client sends nothing until
//     the server speaks, so waiting for client bytes before dialing
//     deadlocks the channel ("Ligação à nuvem perdida").
//
//  2. We peek the client's first bytes with a short deadline (300ms):
//     - TLS ClientHello whose SNI is on the capture list → the direct conn
//       is dropped and the flow is re-sent through the host bridge
//       (10.0.2.2:3129 → Reqable 127.0.0.1:9000) as HTTP CONNECT. Reqable
//       MITMs it with its CA (installed in the emulator's conscrypt store).
//     - Anything else (pinned TLS, plain HTTP, binary, silence) → pure
//       direct pipe. Unknown traffic always works; only known capture
//       hosts pay the MITM price.
//
// The capture list exists to record the login/signup flow (m.alibaba.com
// SSO, login.accio.com) for the account-creator RE. The chat endpoints
// (phoenix-gw, stream.accio.com) are SecurityGuard-pinned and pass
// through; their shapes are already known from the desktop client RE.
//
// The bridge dial uses an IP literal (no DNS: a pure-Go binary on Android
// has no working system resolver — netd is Binder-only).

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const bridge = "10.0.2.2:3129"

// sniffWait is how long we wait for the client's first bytes when deciding
// whether a flow should be MITM'd. TLS clients send the ClientHello
// immediately; silence after this window means a server-first protocol,
// which is piped direct.
const sniffWait = 300 * time.Millisecond

// captureHosts are SNI names that MUST be MITM'd by Reqable (the flows we
// are reverse-engineering). Exact match for now; add subdomains as found.
var captureHosts = map[string]bool{
	"m.alibaba.com":      true, // SSO sign-in (webview)
	"login.accio.com":    true, // newlogin/alibaba_sign.htm
	"acs.m.alibaba.com":  true, // mtop API gateway (native Alibaba login SDK)
	"acs.m.taobao.com":   true, // mtop API gateway (taobao flavor)
	"account.alibaba.com": true,
	"login.alibaba.com":  true,
	"passport.alibaba.com": true,
	"www.accio.com":      true,
	"accio.com":          true,
	"sale.alibaba.com":   true, // ICBU login flow host (seen during mobile login)
	"global-orange-dc.alibaba.com": true,
	"login0.taobao.com":  true,
	"login.m.taobao.com": true,
	// phoenix-gw carries the WS handshake with connectionToken/accessToken in
	// the URL — the token harvest source. Anti-tamper note: with the
	// conscrypt CA injection (patch-ca.sh) the app tolerates this MITM;
	// verified 2026-09-20 (login + chat functional through capture).
	"phoenix-gw.alibaba.com": true,
}

// blockRaw kills non-TLS/non-HTTP connections (the ACCS/ANet binary
// channel). The login flow on this app version rides the binary channel
// and NEVER falls back to plain HTTPS — blocking it only stalls the login
// (app retries forever, then dies). Keep it OFF; the login hosts above
// are captured directly instead.
const blockRaw = false

// SO_ORIGINAL_DST (linux/netfilter).
const soOriginalDst = 80

func main() {
	log.SetFlags(log.Ltime | log.Lmicroseconds)
	ln, err := net.Listen("tcp", "127.0.0.1:3130")
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("guestpipe v5 :3130 (capture=%v)", capKeys())
	for {
		c, err := ln.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go handle(c)
	}
}

func capKeys() []string {
	ks := make([]string, 0, len(captureHosts))
	for k := range captureHosts {
		ks = append(ks, k)
	}
	return ks
}

func handle(c net.Conn) {
	defer c.Close()

	origIP, origPort, err := originalDst(c)
	if err != nil {
		log.Printf("origdst: %v", err)
		return
	}
	origAddr := fmt.Sprintf("%s:%d", origIP, origPort)

	// Dial the original destination right away (server-first protocols).
	direct, err := net.DialTimeout("tcp", origAddr, 10*time.Second)
	if err != nil {
		log.Printf("dial %s: %v", origAddr, err)
		return
	}

	// Peek the client's opening bytes with a short deadline.
	br := bufio.NewReader(c)
	_ = c.SetReadDeadline(time.Now().Add(sniffWait))
	_, sni, kind := peekFlow(br)
	_ = c.SetReadDeadline(time.Time{})

	if blockRaw && kind == "raw" && origPort == 443 {
		// drop the binary ACCS/ANet channel so mtop falls back to HTTPS
		log.Printf("block raw %s", origAddr)
		return
	}

	var up net.Conn
	switch {
	case kind == "tls" && captureHosts[sni]:
		direct.Close()
		up, err = dialViaBridge(sni, 443)
		if err != nil {
			log.Printf("bridge %s: %v", sni, err)
			return
		}
		log.Printf("capture %s (%s)", sni, origAddr)
	default:
		up = direct
		if kind == "tls" {
			log.Printf("direct tls %s (%s)", sni, origAddr)
		} else if kind == "http" {
			log.Printf("direct http (%s)", origAddr)
		} else {
			log.Printf("direct raw (%s)", origAddr)
		}
	}
	defer up.Close()

	// NOTE: peeked bytes are NOT consumed from br — io.Copy(up, br) sends
	// them exactly once. Do NOT replay head here (that would duplicate the
	// ClientHello and the server would reset the connection).
	done := make(chan struct{}, 2)
	go func() { io.Copy(up, br); done <- struct{}{} }()
	urb := bufio.NewReader(up)
	go func() { io.Copy(c, urb); done <- struct{}{} }()
	<-done
}

// peekFlow peeks and classifies the client's first bytes. Returns the
// peeked bytes (unconsumed from br), the SNI when TLS, and the kind:
// "tls", "http", or "raw" (binary / nothing arrived within the deadline).
func peekFlow(br *bufio.Reader) (head []byte, sni, kind string) {
	pk, err := br.Peek(5)
	if err != nil {
		// timeout or EOF: nothing useful from the client yet — pipe direct
		return nil, "", "raw"
	}
	if pk[0] == 0x16 {
		head, sni, err = sniffTLS(br)
		if err == nil && sni != "" {
			return head, sni, "tls"
		}
		return head, "", "raw" // TLS without a parseable SNI — pipe direct
	}
	head, err = peekHTTPHost(br)
	if err == nil {
		return head, "", "http"
	}
	return head, "", "raw"
}

// originalDst asks netfilter for the destination the socket was redirected
// from. Works because the connection arrived via iptables REDIRECT.
func originalDst(c net.Conn) (net.IP, int, error) {
	tc, ok := c.(*net.TCPConn)
	if !ok {
		return nil, 0, errors.New("not a TCPConn")
	}
	raw, err := tc.SyscallConn()
	if err != nil {
		return nil, 0, err
	}
	var ip net.IP
	var port int
	var opErr error
	err = raw.Control(func(fd uintptr) {
		// struct sockaddr_in { sa_family_t; in_port_t; struct in_addr; pad[8] }
		var sa [16]byte
		saLen := uintptr(len(sa))
		_, _, errno := syscall.Syscall6(
			syscall.SYS_GETSOCKOPT,
			fd,
			syscall.SOL_IP,
			soOriginalDst,
			uintptr(unsafe.Pointer(&sa[0])),
			uintptr(unsafe.Pointer(&saLen)),
			0,
		)
		if errno != 0 {
			opErr = errno
			return
		}
		fam := binary.LittleEndian.Uint16(sa[0:2]) // host byte order
		if fam != syscall.AF_INET {
			opErr = fmt.Errorf("unexpected family %d", fam)
			return
		}
		port = int(binary.BigEndian.Uint16(sa[2:4]))
		ip = net.IPv4(sa[4], sa[5], sa[6], sa[7])
	})
	if err != nil {
		return nil, 0, err
	}
	if opErr != nil {
		return nil, 0, opErr
	}
	return ip, port, nil
}

// sniffTLS parses the SNI out of a TLS ClientHello.
func sniffTLS(br *bufio.Reader) ([]byte, string, error) {
	hdr, err := br.Peek(5)
	if err != nil {
		return nil, "", err
	}
	recLen := int(binary.BigEndian.Uint16(hdr[3:5]))
	rec, err := br.Peek(5 + recLen)
	if err != nil {
		return nil, "", err
	}
	body := rec[5:]
	if len(body) < 4 || body[0] != 0x01 {
		return nil, "", errors.New("not a client hello")
	}
	hsLen := int(body[1])<<16 | int(body[2])<<8 | int(body[3])
	if len(body) < 4+hsLen {
		return nil, "", errors.New("short handshake")
	}
	p := body[4:]
	if len(p) < 34+1+2+1 {
		return nil, "", errors.New("short fields")
	}
	p = p[34:]
	sidLen := int(p[0])
	p = p[1+sidLen:]
	if len(p) < 2 {
		return nil, "", errors.New("short ciphers")
	}
	ciphLen := int(binary.BigEndian.Uint16(p)) &^ 1
	p = p[2+ciphLen:]
	if len(p) < 1 {
		return nil, "", errors.New("short comp")
	}
	compLen := int(p[0])
	p = p[1+compLen:]
	if len(p) < 2 {
		return nil, "", errors.New("no extensions")
	}
	extLen := int(binary.BigEndian.Uint16(p))
	p = p[2:]
	if len(p) < extLen {
		return nil, "", errors.New("short extensions")
	}
	for len(p) >= 4 {
		et := binary.BigEndian.Uint16(p)
		el := int(binary.BigEndian.Uint16(p[2:]))
		if len(p) < 4+el {
			break
		}
		if et == 0 {
			e := p[4 : 4+el]
			if len(e) >= 5 && e[2] == 0 {
				nl := int(binary.BigEndian.Uint16(e[3:5]))
				if len(e) >= 5+nl {
					return rec, string(e[5 : 5+nl]), nil
				}
			}
		}
		p = p[4+el:]
	}
	return nil, "", errors.New("no SNI")
}

// peekHTTPHost peeks until the Host header of a plain HTTP request
// (without consuming; the returned slice holds all peeked bytes).
func peekHTTPHost(br *bufio.Reader) ([]byte, error) {
	var buf bytes.Buffer
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return nil, err
		}
		buf.WriteString(line)
		t := strings.TrimSpace(line)
		if t == "" {
			break
		}
		l := strings.ToLower(t)
		if strings.HasPrefix(l, "host:") {
			return buf.Bytes(), nil
		}
		if buf.Len() > 16*1024 {
			break
		}
	}
	return nil, errors.New("no host header")
}

// dialViaBridge opens a CONNECT tunnel through the host bridge to Reqable.
func dialViaBridge(host string, port int) (net.Conn, error) {
	d := net.Dialer{Timeout: 10 * time.Second}
	up, err := d.Dial("tcp", bridge)
	if err != nil {
		return nil, err
	}
	req := fmt.Sprintf("CONNECT %s:%d HTTP/1.1\r\nHost: %s:%d\r\n\r\n", host, port, host, port)
	if _, err := up.Write([]byte(req)); err != nil {
		up.Close()
		return nil, err
	}
	urb := bufio.NewReader(up)
	line, err := urb.ReadString('\n')
	if err != nil || !strings.Contains(line, "200") {
		up.Close()
		return nil, fmt.Errorf("connect rejected: %q err=%v", strings.TrimSpace(line), err)
	}
	for {
		h, err := urb.ReadString('\n')
		if err != nil {
			up.Close()
			return nil, err
		}
		if strings.TrimSpace(h) == "" {
			break
		}
	}
	return &bufferedConn{Conn: up, r: urb}, nil
}

// bufferedConn prepends buffered bytes ahead of the underlying conn.
type bufferedConn struct {
	net.Conn
	r *bufio.Reader
}

func (b *bufferedConn) Read(p []byte) (int, error) {
	if b.r != nil && b.r.Buffered() > 0 {
		return b.r.Read(p)
	}
	return b.Conn.Read(p)
}

// keep context import used (dialViaBridge may switch to DialContext later)
var _ = context.Background
