// Package phoenix implements the Accio chat client (desktop transport).
//
// The wire contract was verified against the official @ali/accio-adk-ts
// transport of the Accio desktop app: POST
// `${gatewayLLM}/generateContent?sg_k=md5(requestId)` with a JSON body in
// ADK camelCase shape and SSE frames in response. Tools travel as
// name/description/parameters_json entries; model tool calls come back as
// content.parts[].functionCall {id,name,argsJson}.
package phoenix

import (
	"bufio"
	"bytes"
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	GatewayBase = "https://phoenix-gw.alibaba.com/api"
	GatewayLLM  = GatewayBase + "/adk/llm"
)

// Signer produces the SecurityGuard WAF headers for a URL (pctb-x-sign and
// friends). Implemented by sgbridge (Node sidecar) in internal/sgbridge.
type Signer interface {
	Headers(ctx context.Context, url string) (map[string]string, error)
}

// Client is a minimal chat client against the Accio gateway.
type Client struct {
	httpClient *http.Client
	signer     Signer
	utdid      string
	appVersion string
	appKey     string
}

func New(signer Signer, utdid, appVersion, appKey string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 10 * time.Minute},
		signer:     signer,
		utdid:      utdid,
		appVersion: appVersion,
		appKey:     appKey,
	}
}

// GatewayError carries an upstream failure with status and raw body.
type GatewayError struct {
	Status int
	Raw    string
	Cause  error
}

func (e *GatewayError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("accio gateway %d: %v (%s)", e.Status, e.Cause, truncate(e.Raw, 200))
	}
	return fmt.Sprintf("accio gateway %d: %s", e.Status, truncate(e.Raw, 200))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// StreamChat sends an OpenAI-shaped request and calls emit for every delta
// (text, reasoning, or tool call) as frames arrive. done=true marks the end.
func (c *Client) StreamChat(ctx context.Context, request map[string]any, token string, emit func(text, reasoning string, call map[string]any, done bool)) error {
	resp, err := c.post(ctx, request, token)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return readFrames(resp.Body, emit)
}

func (c *Client) post(ctx context.Context, request map[string]any, token string) (*http.Response, error) {
	id := firstString(request, "request_id")
	if id == "" {
		id = "req-" + randomID()
	}
	body := BuildBody(request, token, id)
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	h := md5.Sum([]byte(id))
	endpoint := GatewayLLM + "/generateContent?sg_k=" + hex.EncodeToString(h[:])
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)
	if err := c.sign(ctx, req, endpoint); err != nil {
		return nil, err
	}
	return c.do(req)
}

// setHeaders applies the verified desktop header set. Keep it minimal:
// extra Cookie/UA variants are classified as bots by the Baxia WAF.
func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if v := strings.TrimSpace(c.utdid); v != "" {
		req.Header.Set("utdid", v)
	}
	if v := strings.TrimSpace(c.appVersion); v != "" {
		req.Header.Set("version", v)
	}
	if v := strings.TrimSpace(c.appKey); v != "" {
		req.Header.Set("appKey", v)
	}
	req.Header.Set("x-deploy-target", "desktop")
	req.Header.Set("x-accio-route-region", "SG")
	req.Header.Set("x-package-region", "SG")
	req.Header.Set("User-Agent", "node")
}

// sign attaches the SecurityGuard WAF headers for the request URL.
func (c *Client) sign(ctx context.Context, req *http.Request, url string) error {
	if c.signer == nil {
		return nil
	}
	security, err := c.signer.Headers(ctx, url)
	if err != nil {
		return fmt.Errorf("security headers: %w", err)
	}
	for k, v := range security {
		req.Header.Set(k, v)
	}
	return nil
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		_ = resp.Body.Close()
		return nil, &GatewayError{Status: resp.StatusCode, Raw: string(raw)}
	}
	return resp, nil
}

// WarmDevice replays the official app's launch chatter — SecurityGuard-signed
// featureFlag/evaluate calls — so sentinel sees this utdid as an
// SG-initialized desktop device. Required before chat on fresh devices;
// skipping it earns a 555 rate-limit on the first generateContent.
func (c *Client) WarmDevice(ctx context.Context) {
	if c.signer == nil {
		return
	}
	for i := 0; i < 3; i++ {
		target := GatewayBase + "/tool/featureFlag/evaluate"
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, strings.NewReader("{}"))
		if err != nil {
			return
		}
		c.setHeaders(req)
		if err := c.sign(ctx, req, target); err != nil {
			return
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return
		}
		_, _ = io.ReadAll(io.LimitReader(resp.Body, 4096))
		_ = resp.Body.Close()
		if resp.StatusCode != 200 {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(4 * time.Second):
		}
	}
}

// IsRateLimited reports whether the error is the sentinel 555 gate.
func IsRateLimited(err error) bool {
	var ge *GatewayError
	if errors.As(err, &ge) {
		return strings.Contains(ge.Raw, "555") || strings.Contains(ge.Error(), "555")
	}
	return err != nil && strings.Contains(err.Error(), "555")
}

// IsUnauthorized reports auth failures (401/402) — sinal de access token
// expirado; nesse caso o proxy deve renovar a sessão e tentar de novo.
func IsUnauthorized(err error) bool {
	var ge *GatewayError
	if errors.As(err, &ge) {
		return ge.Status == 401 || ge.Status == 402 ||
			strings.Contains(ge.Raw, `"error_code":"402"`) ||
			strings.Contains(ge.Raw, `"error_code":"401"`) ||
			strings.Contains(strings.ToLower(ge.Raw), "unauthorized")
	}
	return false
}

// RefreshTokens renova a sessão de uma conta via POST /auth/refresh_token
// (wire contract verificado contra o repo legado). Devolve o novo par
// access/refresh; se o servidor não mandar refresh novo, reutiliza o antigo.
func RefreshTokens(ctx context.Context, utdid, appVersion, accessToken, refreshToken string) (string, string, error) {
	if refreshToken == "" {
		return "", "", errors.New("refresh token vazio")
	}
	body, _ := json.Marshal(map[string]string{
		"utdid":        utdid,
		"version":      appVersion,
		"accessToken":  accessToken,
		"refreshToken": refreshToken,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, GatewayBase+"/auth/refresh_token", bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	setDesktopHeaders(req, utdid, appVersion)
	hc := &http.Client{Timeout: 30 * time.Second}
	resp, err := hc.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("refresh HTTP %d: %.200s", resp.StatusCode, raw)
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return "", "", fmt.Errorf("refresh JSON: %w (%.200s)", err, raw)
	}
	if ok, _ := root["success"].(bool); !ok {
		return "", "", fmt.Errorf("refresh rejeitado: %.200s", raw)
	}
	data, _ := root["data"].(map[string]any)
	if data == nil {
		data = root
	}
	access, _ := data["accessToken"].(string)
	refresh, _ := data["refreshToken"].(string)
	if access == "" {
		return "", "", errors.New("refresh devolveu access vazio")
	}
	if refresh == "" {
		refresh = refreshToken
	}
	return access, refresh, nil
}

// QueryCredits lê o saldo de créditos de uma conta no endpoint de
// entitlement (wire contract capturado do app 0.29.1: token vai na query,
// headers desktop). Retorna total/remaining/used/planType + entitlements por
// pool (daily/monthly/referral) quando presentes.
func QueryCredits(ctx context.Context, utdid, appVersion, token string) (map[string]any, error) {
	endpoint := GatewayBase + "/entitlement/currentSubscription"
	result, status, err := creditsCall(ctx, utdid, appVersion, token, endpoint)
	if err != nil && status != http.StatusUnauthorized && status != http.StatusForbidden {
		result, _, err = creditsCall(ctx, utdid, appVersion, token, GatewayBase+"/entitlement/quota")
	}
	return result, err
}

func creditsCall(ctx context.Context, utdid, appVersion, token, endpoint string) (map[string]any, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, 0, err
	}
	q := req.URL.Query()
	q.Set("accessToken", token)
	q.Set("subscripType", "INDIVIDUAL")
	q.Set("utdid", utdid)
	q.Set("version", appVersion)
	req.URL.RawQuery = q.Encode()
	setDesktopHeaders(req, utdid, appVersion)
	hc := &http.Client{Timeout: 30 * time.Second}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, resp.StatusCode, fmt.Errorf("credits rejeitado: HTTP %d: %.200s", resp.StatusCode, raw)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, resp.StatusCode, fmt.Errorf("credits HTTP %d", resp.StatusCode)
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, resp.StatusCode, err
	}
	if ok, present := root["success"].(bool); present && !ok {
		code, _ := root["code"].(string)
		return nil, http.StatusUnauthorized, fmt.Errorf("credits rejeitado: code=%s", code)
	}
	data := root
	if d, ok := root["data"].(map[string]any); ok {
		data = d
	}
	total := toInt(firstValue(data, "total", "totalCredits", "creditTotal"))
	remaining := toInt(firstValue(data, "remaining", "remainingCredits", "creditRemaining"))
	used := toInt(firstValue(data, "used", "usedCredits"))
	if used == 0 && total > 0 {
		used = total - remaining
	}
	pools := map[string]any{}
	aggRemaining, aggTotal := 0, 0
	if ent, ok := data["entitlement"].(map[string]any); ok {
		for _, name := range []string{"daily", "monthly", "referral"} {
			if pool, ok := ent[name].(map[string]any); ok {
				pt := toInt(firstValue(pool, "total"))
				pr := toInt(firstValue(pool, "remaining"))
				pools[name] = map[string]int{"total": pt, "remaining": pr}
				aggTotal += pt
				aggRemaining += pr
			}
		}
	}
	if aggRemaining > 0 {
		remaining = aggRemaining
	}
	if aggTotal > 0 {
		total = aggTotal
	}
	planType := firstStringOf(data, "type", "planType", "subscriptionType")
	return map[string]any{
		"total":      total,
		"remaining":  remaining,
		"used":       used,
		"planType":   planType,
		"pools":      pools,
		"checked_at": time.Now().Format(time.RFC3339),
	}, http.StatusOK, nil
}

// setDesktopHeaders é o header set do app desktop para endpoints de auth/
// entitlement (diferente do set mínimo do chat, que o WAF exige enxuto).
func setDesktopHeaders(req *http.Request, utdid, appVersion string) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-language", "pt")
	req.Header.Set("x-locale", "pt-BR")
	req.Header.Set("x-platform", "pc-win")
	req.Header.Set("x-utdid", utdid)
	req.Header.Set("x-app-version", appVersion)
	req.Header.Set("x-os", "win32")
	req.Header.Set("x-deploy-target", "desktop")
	req.Header.Set("x-source", "ACCIO_DESKTOP")
	req.Header.Set("x-cna", "")
	req.Header.Set("x-package-region", "GLOBAL")
}

func toInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case string:
		var i int
		fmt.Sscanf(n, "%d", &i)
		return i
	}
	return 0
}

func firstValue(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			return v
		}
	}
	return nil
}

func firstStringOf(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s, ok := m[k].(string); ok && s != "" {
			return s
		}
	}
	return ""
}

func randomID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func firstString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// frame is one SSE data payload of the chat stream.
type frame struct {
	Content           *frameContent  `json:"content"`
	Candidate         *frameEnvelope `json:"candidate"`
	Payload           *frameEnvelope `json:"payload"`
	TurnComplete      bool           `json:"turnComplete"`
	TurnCompleteSnake bool           `json:"turn_complete"`
	ErrorCode         string         `json:"error_code"`
	ErrorCodeCamel    string         `json:"errorCode"`
	ErrorMessage      string         `json:"error_message"`
	ErrorMessageCamel string         `json:"errorMessage"`
}

type framePart struct {
	Text              string `json:"text"`
	Thought           bool   `json:"thought"`
	FunctionCall      *struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		ArgsJSON string `json:"argsJson"`
	} `json:"functionCall"`
	FunctionCallSnake *struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		ArgsJSON string `json:"args_json"`
	} `json:"function_call"`
}

type frameContent struct {
	Parts []framePart `json:"parts"`
}

type frameEnvelope struct {
	Content *frameContent `json:"content"`
}

// readFrames parses the SSE stream and emits deltas. Terminates on
// turnComplete, [DONE], or stream error.
func readFrames(r io.Reader, emit func(text, reasoning string, call map[string]any, done bool)) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), 8<<20)
	completed := false
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		raw := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if raw == "" {
			continue
		}
		lower := strings.ToLower(raw)
		if strings.HasPrefix(lower, "<!doctype html") || strings.HasPrefix(lower, "<html") {
			return &GatewayError{Status: http.StatusBadGateway, Raw: "gateway returned an HTML challenge page instead of SSE"}
		}
		if raw == "[DONE]" {
			if !completed {
				completed = true
				emit("", "", nil, true)
			}
			continue
		}
		var f frame
		if err := json.Unmarshal([]byte(raw), &f); err != nil {
			continue
		}
		if err := frameError(f); err != nil {
			return &GatewayError{Status: http.StatusBadGateway, Raw: raw, Cause: err}
		}
		content := f.Content
		if content == nil && f.Candidate != nil {
			content = f.Candidate.Content
		}
		if content == nil && f.Payload != nil {
			content = f.Payload.Content
		}
		if content != nil {
			for _, p := range content.Parts {
				callID, callName, callArgs := "", "", ""
				if p.FunctionCall != nil {
					callID, callName, callArgs = p.FunctionCall.ID, p.FunctionCall.Name, p.FunctionCall.ArgsJSON
				} else if p.FunctionCallSnake != nil {
					callID, callName, callArgs = p.FunctionCallSnake.ID, p.FunctionCallSnake.Name, p.FunctionCallSnake.ArgsJSON
				}
				if callName != "" {
					emit("", "", map[string]any{
						"index": 0, "id": callID, "type": "function",
						"function": map[string]any{"name": callName, "arguments": callArgs},
					}, false)
				} else if p.Thought {
					emit("", p.Text, nil, false)
				} else {
					emit(p.Text, "", nil, false)
				}
			}
		}
		if f.TurnComplete || f.TurnCompleteSnake {
			if !completed {
				completed = true
				emit("", "", nil, true)
			}
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	if !completed {
		return errors.New("stream ended without completion")
	}
	return nil
}

func frameError(f frame) error {
	code := f.ErrorCode
	if code == "" {
		code = f.ErrorCodeCamel
	}
	msg := f.ErrorMessage
	if msg == "" {
		msg = f.ErrorMessageCamel
	}
	if code != "" {
		return fmt.Errorf("%s: %s", code, msg)
	}
	return nil
}
