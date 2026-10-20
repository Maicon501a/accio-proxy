package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// O Playground roda no webview (origem wails://) e chama o gateway em
// 127.0.0.1:<porta>: origem diferente, então o browser manda preflight antes do
// POST e exige os headers de CORS na resposta real.
func TestCORS(t *testing.T) {
	h := cors(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	// preflight
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/v1/chat/completions", nil)
	req.Header.Set("Origin", "http://wails.localhost")
	req.Header.Set("Access-Control-Request-Method", "POST")
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("preflight status = %d, quer %d", rec.Code, http.StatusNoContent)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("preflight Allow-Origin = %q, quer %q", got, "*")
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Error("preflight sem Access-Control-Allow-Methods")
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got == "" {
		t.Error("preflight sem Access-Control-Allow-Headers")
	}

	// resposta real também precisa anunciar a origem
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("POST status = %d, quer 200", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("POST Allow-Origin = %q, quer %q", got, "*")
	}
}
