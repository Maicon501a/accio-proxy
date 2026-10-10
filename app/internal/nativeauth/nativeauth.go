// Package nativeauth implementa o login nativo Accio (PKCE OAuth) direto no
// proxy, sem emulador e sem SecurityGuard. Wire contract extraído do app
// desktop 0.33.0 (app.asar):
//
//  1. gera PKCE (verifier/challenge) + state
//  2. abre no browser: https://www.accio.com/login?return_url=<callback>
//     &state=..&code_challenge=..&code_challenge_method=S256&client_id=accio-work
//  3. usuário faz email+OTP no browser; o passport redireciona para
//     http://127.0.0.1:8791/auth/callback?code=..&state=..
//  4. troca o code em POST {GatewayBase}/oauth/token
//     {code, codeVerifier, clientId:"accio-work", redirectUri}
//     → {accessToken, refreshToken, expiresAt}
package nativeauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	CallbackAddr = "127.0.0.1:8791"
	LoginBaseURL = "https://www.accio.com/login"
	ClientID     = "accio-work"
)

// Store é o mínimo do account.Store que o fluxo precisa.
type Store interface {
	UpsertTokens(email, access, refresh string) error
}

// Flow conduz um login por vez.
type Flow struct {
	GatewayBase string
	Store       Store

	mu       sync.Mutex
	verifier string
	state    string
	redirect string
	email    string
	srv      *http.Server
	ln       net.Listener
	done     chan error
	status   string
}

func b64u(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

// Start inicia o fluxo: sobe o listener de callback e devolve a URL de login
// para abrir no browser.
func (f *Flow) Start(email string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.srv != nil {
		return "", fmt.Errorf("já existe um login em andamento")
	}
	vb := make([]byte, 32)
	if _, err := rand.Read(vb); err != nil {
		return "", err
	}
	sb := make([]byte, 32)
	if _, err := rand.Read(sb); err != nil {
		return "", err
	}
	f.verifier = b64u(vb)
	f.state = hex.EncodeToString(sb)
	f.email = email
	chal := b64u(sha256Sum([]byte(f.verifier)))
	trace := fmt.Sprintf("ttid_%x", sb[:16])
	f.redirect = fmt.Sprintf("http://%s/auth/callback?login_trace_id=%s", CallbackAddr, trace)
	f.done = make(chan error, 1)
	f.status = "waiting"

	ln, err := net.Listen("tcp", CallbackAddr)
	if err != nil {
		return "", fmt.Errorf("callback listener: %w", err)
	}
	f.ln = ln
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/callback", f.handleCallback)
	f.srv = &http.Server{Handler: mux, ReadHeaderTimeout: 15 * time.Second}
	go func() { _ = f.srv.Serve(ln) }()
	// timeout de 10 min
	go func() {
		select {
		case <-time.After(10 * time.Minute):
			f.finish(fmt.Errorf("tempo esgotado — login não completado em 10 min"))
		case <-f.done:
		}
	}()

	loginURL := fmt.Sprintf("%s?return_url=%s&state=%s&code_challenge=%s&code_challenge_method=S256&client_id=%s",
		LoginBaseURL,
		encodeQuery(f.redirect),
		encodeQuery(f.state),
		encodeQuery(chal),
		ClientID)
	return loginURL, nil
}

func sha256Sum(b []byte) []byte {
	h := sha256.Sum256(b)
	return h[:]
}

func encodeQuery(s string) string {
	r := strings.NewReplacer(
		"%", "%25", " ", "%20", "!", "%21", "\"", "%22", "#", "%23", "$", "%24",
		"&", "%26", "'", "%27", "(", "%28", ")", "%29", "*", "%2A", "+", "%2B",
		",", "%2C", "/", "%2F", ":", "%3A", ";", "%3B", "=", "%3D", "?", "%3F",
		"@", "%40", "[", "%5B", "]", "%5D",
	)
	return r.Replace(s)
}

func (f *Flow) handleCallback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if st := q.Get("state"); st != f.state {
		f.finish(fmt.Errorf("state inválido no callback (esperado troca de estado?)"))
		http.Redirect(w, r, "https://www.accio.com", http.StatusFound)
		return
	}
	code := q.Get("code")
	if code == "" {
		f.finish(fmt.Errorf("callback sem code: %s", r.URL.RawQuery))
		fmt.Fprint(w, callbackPage("Falha", "Callback sem código. Veja o log do proxy."))
		return
	}
	email, err := f.exchange(code)
	if err != nil {
		f.finish(err)
		fmt.Fprint(w, callbackPage("Falha", err.Error()))
		return
	}
	f.mu.Lock()
	f.status = "ok:" + email
	f.mu.Unlock()
	f.finish(nil)
	fmt.Fprint(w, callbackPage("Conta conectada", email+" salva no pool do proxy. Pode fechar esta aba."))
}

// exchange troca o authorization code pelos tokens e grava a conta no pool.
func (f *Flow) exchange(code string) (string, error) {
	body, _ := json.Marshal(map[string]string{
		"code":         code,
		"codeVerifier": f.verifier,
		"clientId":     ClientID,
		"redirectUri":  f.redirect,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.GatewayBase+"/oauth/token", strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return "", fmt.Errorf("exchange: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("exchange HTTP %d: %.300s", resp.StatusCode, raw)
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return "", fmt.Errorf("exchange JSON: %w (%.200s)", err, raw)
	}
	data, _ := root["data"].(map[string]any)
	if data == nil {
		data = root
	}
	access, _ := data["accessToken"].(string)
	refresh, _ := data["refreshToken"].(string)
	expiresAt, _ := data["expiresAt"].(float64)
	if access == "" || refresh == "" || expiresAt <= 0 {
		return "", fmt.Errorf("exchange incompleto: %.300s", raw)
	}
	// nome da conta: usa o digitado; se vazio, tenta achar identidade no
	// próprio response do /oauth/token (email/loginId/userId costumam vir);
	// último recurso: nome sintético estável pelo hash do token.
	email := strings.TrimSpace(f.email)
	if email == "" {
		email = identityFromResponse(root)
	}
	if email == "" {
		email = "oauth-" + hex.EncodeToString(sha256Sum([]byte(access))[:6])
	}
	if err := f.Store.UpsertTokens(email, access, refresh); err != nil {
		return "", fmt.Errorf("salvar conta: %w", err)
	}
	return email, nil
}

// identityFromResponse vasculha o JSON do /oauth/token atrás de qualquer
// identificador de conta (email, loginId, userId...). Aceita "a@b.c" direto
// ou id numérico (vira "<id>@accio.local", padrão das contas do pool).
func identityFromResponse(root map[string]any) string {
	var found string
	var walk func(m map[string]any)
	walk = func(m map[string]any) {
		if found != "" {
			return
		}
		for k, v := range m {
			lk := strings.ToLower(k)
			switch {
			case strings.Contains(lk, "email") || lk == "loginid" || lk == "loginname" || lk == "account":
				if s, ok := v.(string); ok && s != "" {
					found = s
					return
				}
			case lk == "userid" || lk == "uid" || lk == "user_id":
				switch n := v.(type) {
				case string:
					if n != "" {
						found = n
					}
				case float64:
					found = fmt.Sprintf("%.0f", n)
				}
			}
			if sub, ok := v.(map[string]any); ok {
				walk(sub)
			}
			if arr, ok := v.([]any); ok {
				for _, it := range arr {
					if sub, ok := it.(map[string]any); ok {
						walk(sub)
					}
				}
			}
		}
	}
	walk(root)
	if found == "" {
		return ""
	}
	if strings.Contains(found, "@") {
		return found
	}
	// id numérico puro → nomeia no padrão do pool
	numeric := found != ""
	for _, r := range found {
		if r < '0' || r > '9' {
			numeric = false
			break
		}
	}
	if numeric {
		return found + "@accio.local"
	}
	return found
}

func (f *Flow) finish(err error) {
	f.mu.Lock()
	srv, ln := f.srv, f.ln
	f.srv, f.ln = nil, nil
	done := f.done
	if err != nil && f.status == "waiting" {
		f.status = "erro: " + err.Error()
	}
	f.mu.Unlock()
	if ln != nil {
		_ = srv.Close()
	}
	if done != nil {
		select {
		case done <- err:
		default:
		}
	}
}

// Cancel aborta o fluxo em andamento.
func (f *Flow) Cancel() {
	f.finish(fmt.Errorf("cancelado"))
}

// Status devolve "waiting" | "ok:<email>" | "erro:<msg>" | "idle".
func (f *Flow) Status() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.srv != nil && f.status == "" {
		return "waiting"
	}
	if f.status == "" {
		return "idle"
	}
	return f.status
}

func callbackPage(title, msg string) string {
	return fmt.Sprintf(`<!doctype html><html><head><meta charset="utf-8"><title>Accio Proxy</title>
<style>body{background:#18181b;color:#e4e4e7;font-family:system-ui;display:flex;align-items:center;justify-content:center;height:100vh;margin:0}
div{max-width:420px;text-align:center}h1{font-size:22px}p{color:#a1a1aa}</style></head>
<body><div><h1>%s</h1><p>%s</p></div></body></html>`, title, msg)
}
