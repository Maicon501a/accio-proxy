package sgbridge

import (
	"context"
	"testing"
)

// O contrato do signer nativo: devolver o header set de device sem depender de
// binário proprietário e sem falhar.
func TestNativeHeaders(t *testing.T) {
	s := newNativeSigner(DesktopAppKey)
	headers, err := s.Headers(context.Background(), "https://phoenix-gw.alibaba.com/api/adk/llm/generateContent?sg_k=deadbeef")
	if err != nil {
		t.Fatalf("Headers() error = %v", err)
	}

	want := []string{
		"pctb-x-sgext", "pctb-x-pv", "pctb-x-api", "pctb-x-apiver",
		"pctb-x-t", "pctb-x-appkey", "x-bx-env", "pctb-x-bx-version",
	}
	for _, k := range want {
		if headers[k] == "" {
			t.Errorf("header %q ausente ou vazio", k)
		}
	}

	// pctb-x-api é host + path, sem query — igual ao desktop.
	if got, want := headers["pctb-x-api"], "phoenix-gw.alibaba.com%2Fapi%2Fadk%2Fllm%2FgenerateContent"; got != want {
		t.Errorf("pctb-x-api = %q, quer %q", got, want)
	}
	if got, want := headers["pctb-x-appkey"], DesktopAppKey; got != want {
		t.Errorf("pctb-x-appkey = %q, quer %q", got, want)
	}
	if got, want := headers["pctb-x-bx-version"], "pc_1.0.70.3061_0"; got != want {
		t.Errorf("pctb-x-bx-version = %q, quer %q", got, want)
	}
}

// Valores precisam sair url-encoded como o lado JS fazia (encodeURIComponent),
// senão o header chega diferente do caminho Windows.
func TestEncodeURIComponent(t *testing.T) {
	cases := map[string]string{
		"win_reserve": "win_reserve",
		"6.3":         "6.3",
		"pc_1.0.70.3061_0": "pc_1.0.70.3061_0",
		"a/b+c":       "a%2Fb%2Bc",
		"época":       "%C3%A9poca",
		"a b":         "a%20b",
	}
	for in, want := range cases {
		if got := encodeURIComponent(in); got != want {
			t.Errorf("encodeURIComponent(%q) = %q, quer %q", in, got, want)
		}
	}
}

// O signer público não pode exigir sidecar para responder.
func TestSignerFallsBackToNative(t *testing.T) {
	t.Setenv("ACCIO_SG_SIDECAR", "0")
	t.Setenv("ACCIO_SG_SPOOF_SEED", "")

	s := New(t.TempDir())
	defer s.Close()

	headers, err := s.Headers(context.Background(), "https://phoenix-gw.alibaba.com/api/tool/featureFlag/evaluate")
	if err != nil {
		t.Fatalf("Headers() error = %v", err)
	}
	if len(headers) == 0 {
		t.Fatal("esperava headers de device, veio vazio")
	}
	if s.useSidecar {
		t.Fatal("sidecar deveria estar desligado com ACCIO_SG_SIDECAR=0")
	}
}
