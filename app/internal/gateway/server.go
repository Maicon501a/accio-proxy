// Package gateway exposes an OpenAI-compatible HTTP API backed by the
// phoenix client. Endpoints: POST /v1/chat/completions (stream + tools),
// GET /v1/models, GET /health.
package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"accio-proxy/internal/account"
	"accio-proxy/internal/official"
	"accio-proxy/internal/phoenix"
)

// AccountPicker selects the access token for a request (rotation lives in
// the account store).
type AccountPicker interface {
	Pick(ctx context.Context) (email, token string, err error)
	ReportFailure(email string)
}

// SessionManager agrupa o que o gateway precisa para manter as sessões
// vivas: renovar tokens expirados e rastrear créditos por conta.
type SessionManager struct {
	store   CreditStore
	utdid   string
	appVer  string
	mu      sync.Mutex
	lastAsk map[string]time.Time
}

// CreditStore é o mínimo que o gerente de sessão precisa persistir.
type CreditStore interface {
	SetCredits(email string, remaining int, infoJSON string) error
	UpdateTokens(email, access, refresh string) error
	RefreshTokenOf(email string) (access, refresh string)
	ListForRefresh() []account.RefEntry
}

type Server struct {
	client   *phoenix.Client
	picker   AccountPicker
	official *official.Manager
	modeFn   func() string
	sess     *SessionManager
	mu       sync.Mutex
	reqLog   []map[string]any
	port     int
	server   *http.Server
	started  time.Time
	reqCount int64

	// lastServedEmail é a conta que atendeu a request mais recente; serve o
	// indicador "ATIVA AGORA" na aba Contas.
	lastServedEmail string
	servedCount     map[string]int64
}

func New(client *phoenix.Client, picker AccountPicker, port int) *Server {
	return &Server{client: client, picker: picker, port: port, started: time.Now(), servedCount: map[string]int64{}}
}

// WithSessions liga o gerente de sessões (refresh + créditos).
func (s *Server) WithSessions(store CreditStore, utdid, appVer string) *Server {
	s.sess = &SessionManager{store: store, utdid: utdid, appVer: appVer, lastAsk: map[string]time.Time{}}
	go s.sess.loop()
	return s
}

// WithOfficial liga o gerenciador do Accio Oficial e o seletor de modo ("official" ou "direct").
func (s *Server) WithOfficial(mgr *official.Manager, modeFn func() string) *Server {
	s.official = mgr
	s.modeFn = modeFn
	return s
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", s.handleChat)
	mux.HandleFunc("/v1/models", s.handleModels)
	mux.HandleFunc("/health", s.handleHealth)
	s.server = &http.Server{Addr: fmt.Sprintf("127.0.0.1:%d", s.port), Handler: mux}
	log.Printf("gateway: listening on 127.0.0.1:%d", s.port)
	return s.server.ListenAndServe()
}

func (s *Server) Stop(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	return s.server.Shutdown(ctx)
}

// ---- handlers ----

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var request map[string]any
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		httpError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	mode := "direct"
	if s.modeFn != nil {
		mode = s.modeFn()
	}

	var email, token string
	var err error
	if mode == "official" && s.official != nil {
		email, token, err = s.official.GetActiveToken(r.Context())
		if err != nil {
			httpError(w, http.StatusServiceUnavailable, "official_app_error", "Modo Oficial: "+err.Error()+". Abra o app oficial do Accio ou alterne para o Modo Direto.")
			return
		}
	} else {
		email, token, err = s.picker.Pick(r.Context())
		if err != nil {
			httpError(w, http.StatusServiceUnavailable, "no_accounts", err.Error())
			return
		}
	}

	model, _ := request["model"].(string)
	if code := resolveModel(model); code != "" {
		request["model"] = code
	} else {
		httpError(w, http.StatusBadRequest, "unknown_model", "modelo desconhecido: "+model+" (veja /v1/models)")
		return
	}
	stream, _ := request["stream"].(bool)

	s.mu.Lock()
	s.reqCount++
	s.lastServedEmail = email
	if s.servedCount == nil {
		s.servedCount = map[string]int64{}
	}
	s.servedCount[email]++
	s.reqLog = append(s.reqLog, map[string]any{
		"ts": time.Now().Format(time.RFC3339), "model": model,
		"account": email, "stream": stream, "mode": mode,
	})
	if len(s.reqLog) > 200 {
		s.reqLog = s.reqLog[len(s.reqLog)-200:]
	}
	s.mu.Unlock()
	debugLog("chat mode=%s model=%s account=%s stream=%v body=%s", mode, model, email, stream, previewJSON(request))

	if stream {
		s.streamChat(w, r, request, token, email, model)
		return
	}
	s.blockingChat(w, r, request, token, email, model)
}

// streamWithWarmRetry runs one chat attempt; on the sentinel 555 gate it
// warms the device (featureFlag/evaluate chatter, bounded at 50s) and
// retries once. On 401/402 it renews the account session and retries.
func (s *Server) streamWithWarmRetry(ctx context.Context, request map[string]any, token, email string, emit func(text, reasoning string, call map[string]any, done bool)) error {
	err := s.client.StreamChat(ctx, request, token, emit)
	isOfficial := s.modeFn != nil && s.modeFn() == "official"

	if phoenix.IsUnauthorized(err) {
		if isOfficial && s.official != nil {
			log.Printf("gateway: token expirado no modo oficial — recarregando do vault")
			if _, newTok, rerr := s.official.GetActiveToken(ctx); rerr == nil && newTok != "" && newTok != token {
				request["token"] = newTok
				err = s.client.StreamChat(ctx, request, newTok, emit)
			}
		} else if s.sess != nil {
			log.Printf("gateway: 402 em %s — renovando sessão", email)
			if newTok, rerr := s.sess.RefreshAccount(ctx, email, token); rerr == nil && newTok != "" && newTok != token {
				request["token"] = newTok
				err = s.client.StreamChat(ctx, request, newTok, emit)
			}
		}
	}
	if !phoenix.IsRateLimited(err) {
		if isOfficial && s.official != nil {
			go func() {
				c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_, _ = s.official.RefreshCredits(c)
			}()
		} else if s.sess != nil && email != "" {
			go s.sess.UpdateCredits(email)
		}
		return err
	}
	debugLog("555 rate limit em %s — warming device (até 50s)", email)
	log.Printf("gateway: 555 rate limit — warming device and retrying")
	warmCtx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
	s.client.WarmDevice(warmCtx)
	cancel()
	err = s.client.StreamChat(ctx, request, token, emit)
	if isOfficial && s.official != nil {
		go func() {
			c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, _ = s.official.RefreshCredits(c)
		}()
	} else if s.sess != nil && email != "" {
		go s.sess.UpdateCredits(email)
	}
	return err
}

func (s *Server) streamChat(w http.ResponseWriter, r *http.Request, request map[string]any, token, email, model string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	fl, _ := w.(http.Flusher)
	id := fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())
	writeSSE(w, chunk(id, model, map[string]any{"role": "assistant"}, nil))
	if fl != nil {
		fl.Flush()
	}
	// keepalive: modelos com reasoning longo (fable/opus com effort alto) e o
	// warm do device (555) podem passar 30-50s sem bytes. Clients OpenAI
	// cancelam por idle — comentários SSE ": ping" seguram a conexão viva.
	keepAliveDone := make(chan struct{})
	defer close(keepAliveDone)
	go func() {
		t := time.NewTicker(10 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-keepAliveDone:
				return
			case <-t.C:
				s.mu.Lock()
				_, _ = fmt.Fprint(w, ": ping\n\n")
				s.mu.Unlock()
				if fl != nil {
					fl.Flush()
				}
			}
		}
	}()
	err := s.streamWithWarmRetry(r.Context(), request, token, email, func(text, reasoning string, call map[string]any, done bool) {
		delta := map[string]any{}
		if text != "" {
			delta["content"] = text
		}
		if reasoning != "" {
			delta["reasoning_content"] = reasoning
		}
		if call != nil {
			delta["tool_calls"] = []any{call}
		}
		if len(delta) > 0 {
			writeSSE(w, chunk(id, model, delta, nil))
		}
		if done {
			writeSSE(w, chunk(id, model, map[string]any{}, "stop"))
			_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
		}
		if fl != nil {
			fl.Flush()
		}
	})
	if err != nil {
		// stream already started; surface the error as a final SSE event
		debugLog("stream error: %v", err)
		writeSSE(w, map[string]any{"error": map[string]any{"message": err.Error(), "type": "gateway_error"}})
		if fl != nil {
			fl.Flush()
		}
	}
}

func (s *Server) blockingChat(w http.ResponseWriter, r *http.Request, request map[string]any, token, email, model string) {
	var text, reasoning string
	var calls []map[string]any
	err := s.streamWithWarmRetry(r.Context(), request, token, email, func(t, rs string, call map[string]any, _ bool) {
		text += t
		reasoning += rs
		if call != nil {
			calls = append(calls, call)
		}
	})
	if err != nil {
		debugLog("blocking error: %v", err)
		httpError(w, http.StatusBadGateway, "gateway_error", err.Error())
		return
	}
	msg := map[string]any{"role": "assistant", "content": text}
	if reasoning != "" {
		msg["reasoning_content"] = reasoning
	}
	if len(calls) > 0 {
		msg["tool_calls"] = calls
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id": fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()), "object": "chat.completion",
		"created": time.Now().Unix(), "model": model,
		"choices": []any{map[string]any{"index": 0, "message": msg, "finish_reason": "stop"}},
		"usage":   map[string]int{"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0},
	})
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	cat := GetCatalog()
	models := make([]map[string]any, 0, len(cat))
	for _, m := range cat {
		item := map[string]any{
			"id":             m.ID,
			"object":         "model",
			"created":        1720000000,
			"owned_by":       m.Owner,
			"display_name":   m.Name,
			"accio_code":     m.Code,
			"reasoning":      m.Reason,
			"context_window": m.ContextWin,
			"multimodal":     m.Multimodal,
			"free_use":       m.FreeUse,
			"usage_multiple": m.Multiple,
		}
		if len(m.Efforts) > 0 {
			item["reasoning_efforts"] = m.Efforts
			item["default_effort"] = m.DefEffort
		}
		models = append(models, item)
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": models})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	count := s.reqCount
	last := s.lastServedEmail
	served := map[string]int64{}
	for k, v := range s.servedCount {
		served[k] = v
	}
	mode := "direct"
	if s.modeFn != nil {
		mode = s.modeFn()
	}
	s.mu.Unlock()
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status": "ok", "mode": mode, "requests": count, "uptime_s": int(time.Since(s.started).Seconds()),
		"serving": last, "served_count": served,
	})
}

// ServingInfo devolve a conta que atendeu a última request + contadores.
func (s *Server) ServingInfo() (last string, served map[string]int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[string]int64{}
	for k, v := range s.servedCount {
		out[k] = v
	}
	return s.lastServedEmail, out
}

// EnsureCredits dispara (async) a consulta de créditos de uma conta — usado
// quando a conta entra no pool ou quando a UI nota credits_info vazio. O
// cooldown de 5min dentro do SessionManager evita martelação.
func (s *Server) EnsureCredits(email string) {
	if s == nil || s.sess == nil {
		return
	}
	go s.sess.UpdateCredits(email)
}

// ModelEntry mapeia um alias amigável (usado como id OpenAI) para o código
// interno do Accio com metadados de raciocínio, contexto e custo.
type ModelEntry struct {
	ID         string   `json:"id"`
	Code       string   `json:"code"`
	Name       string   `json:"name"`
	Owner      string   `json:"owned_by"`
	Reason     bool     `json:"reason"`
	Efforts    []string `json:"efforts,omitempty"`
	DefEffort  string   `json:"default_effort,omitempty"`
	ContextWin int      `json:"context_window"`
	Multimodal bool     `json:"multimodal"`
	FreeUse    bool     `json:"free_use"`
	Multiple   float64  `json:"usage_multiple"`
}

var baseCatalog = []ModelEntry{
	{"auto", "auto", "Auto", "openai", false, nil, "", 1050000, true, true, 0},
	{"gpt-6-astra", "1Helix-G6aS8tR2qN7m", "GPT 6 Astra", "openai", true, []string{"low", "medium", "high", "xhigh", "max"}, "high", 1050000, true, false, 3.5},
	{"gpt-6-luna", "1Helix-G6Lu8nK4qR2m", "GPT 6 Luna", "openai", true, []string{"low", "medium", "high"}, "high", 1050000, true, false, 0.04},
	{"gpt-6-sol", "1Helix-G6So7pW3xN9q", "GPT 6 Sol", "openai", true, []string{"low", "medium", "high"}, "high", 1050000, true, false, 0.7},
	{"gpt-5.6-luna", "1Helix-3EjR3WN1dY6s", "GPT 5.6 Luna", "openai", true, []string{"low", "medium", "high"}, "high", 1050000, true, true, 0.09},
	{"gpt-5.6-terra", "1Helix-B33T3WN1dY6s", "GPT 5.6 Terra", "openai", true, []string{"low", "medium", "high"}, "high", 1050000, true, true, 0.88},
	{"gpt-5.6-sol", "1Helix-B3jR3WN1dY6s", "GPT 5.6 Sol", "openai", true, []string{"low", "medium", "high"}, "high", 1050000, true, true, 2.2},
	{"gpt-5.5", "1Helix-B3jR8wN1dY6s", "GPT 5.5", "openai", true, []string{"low", "high"}, "low", 1050000, true, true, 1.8},
	{"grok-4.6", "1Helix-G46W8nP2qR6m", "Grok 4.6", "openai", false, nil, "", 500000, true, true, 0.5},
	{"claude-opus-5.5", "1Orbit-Q55pN8xR6cP2m", "Claude Opus 5.5", "claude", true, []string{"low", "medium", "high", "xhigh", "max"}, "medium", 1000000, true, false, 1.33},
	{"claude-opus-5", "1Orbit-Q5pN8xR6cP2m", "Claude Opus 5", "claude", true, []string{"low", "medium", "high", "xhigh", "max"}, "high", 1000000, true, false, 2.8},
	{"claude-opus-4.8", "1Orbit-Q2XN3xR6cP2m", "Claude Opus 4.8", "claude", true, []string{"low", "medium", "high", "max"}, "low", 1000000, true, true, 1.7},
	{"claude-opus-4.7", "1Orbit-Q7hN2R6cP2m", "Claude Opus 4.7", "claude", true, []string{"low", "medium", "high", "max"}, "low", 1000000, true, true, 1.65},
	{"claude-sonnet-5", "1Orbit-I93F4tK8bW1f", "Claude Sonnet 5", "claude", true, []string{"low", "medium", "high", "xhigh", "max"}, "high", 1000000, true, true, 0.7},
	{"claude-sonnet-4.6", "1Orbit-I9eY7YK8bW1f", "Claude Sonnet 4.6", "claude", true, []string{"low", "medium", "high", "max"}, "low", 1000000, true, true, 1.0},
	{"claude-haiku-4.5", "1Orbit-I93V4tK8bW1f", "Claude Haiku 4.5", "claude", true, []string{"low", "medium", "high", "xhigh", "max"}, "medium", 200000, true, true, 0.35},
	{"claude-fable-5.1", "1Orbit-F51W8nP2qR6m", "Claude Fable 5.1", "claude", true, []string{"low", "medium", "high", "max"}, "high", 1000000, true, false, 3.5},
	{"claude-fable-5", "1Orbit-Q2XN3dX6cP2m", "Claude Fable 5", "claude", true, []string{"low", "medium", "high", "max"}, "high", 1000000, true, false, 3.5},
	{"gemini-3.8-flash", "1Nexus-R8HF9qK7we1s", "Gemini 3.8 Flash", "gemini", true, []string{"low", "high"}, "high", 1000000, true, true, 0.25},
	{"gemini-3.7-flash", "1Nexus-R7GF8qJ6vd8p", "Gemini 3.7 Flash", "gemini", true, []string{"low", "high"}, "high", 1000000, true, true, 0.25},
	{"gemini-3.5-flash", "1Nexus-R3wF8qJ5vc7o", "Gemini 3.5 Flash", "gemini", true, []string{"low", "high"}, "high", 1000000, true, true, 0.4},
	{"gemini-3-flash", "1Nexus-R36W8qJ5vB6h", "Gemini 3 Flash", "gemini", true, []string{"low", "high"}, "low", 1000000, true, true, 0.13},
	{"gemini-3.1-pro", "1Nexus-W5rQ9fG1iC7o", "Gemini 3.1 Pro", "gemini", true, []string{"low", "high"}, "low", 1000000, true, true, 0.55},
	{"deepseek-v4-flash", "1Nova-Q3xM8vJ1rH6z", "DeepSeek V4 Flash", "deepseek", false, nil, "", 1000000, true, true, 0.09},
	{"deepseek-v4-pro", "1Nova-bW7yT4kL9pN2", "DeepSeek V4 Pro", "deepseek", false, nil, "", 1000000, false, true, 0.34},
	{"glm-5.3", "1Chronos-L53W8nP2qR6m", "GLM 5.3", "zhipu", false, nil, "", 1000000, false, true, 0.16},
	{"glm-5.3-flash", "1Chronos-F53X9nQ3rT8w", "GLM 5.3 Flash", "zhipu", false, nil, "", 1000000, true, true, 0.03},
	{"glm-5.2", "1Chronos-G5p2V9sA2mK7", "GLM 5.2", "zhipu", false, nil, "", 1000000, false, true, 0.16},
	{"glm-5.1", "1Chronos-dX3cV9sA2mK7", "GLM 5.1", "zhipu", false, nil, "", 200000, false, true, 0.12},
	{"qwen-3.8-flash", "1Drift-F8hS3mK7pL2n", "Qwen 3.8 Flash", "qwen", false, nil, "", 1000000, true, true, 0.03},
	{"qwen-3.8-flash-plus", "1Drift-F8hS9mK7pL2n", "Qwen 3.8 Flash+", "qwen", false, nil, "", 1000000, true, true, 0.04},
	{"qwen-3.8-max", "1Drift-T3TB6WS9a23w", "Qwen 3.8 Max", "qwen", false, nil, "", 1000000, true, true, 0.44},
	{"qwen-3.8-max-plus", "1Drift-T3T9WWS9a23w", "Qwen 3.8 Max+", "qwen", false, nil, "", 1000000, true, true, 0.56},
	{"qwen-3.8-27b", "1Drift-N8kP2Q7wR5mX", "Qwen 3.8 27B", "qwen", false, nil, "", 512000, true, true, 0.13},
	{"qwen-3.5-a3b", "1Drift-A3bM5nK8pQ2w", "Qwen 3.5 A3B", "qwen", false, nil, "", 256000, false, true, 0.13},
	{"kimi-k2.6", "1Synapse-G5aJ8wL1dV4s", "Kimi K2.6", "moonshot", false, nil, "", 256000, true, true, 0.14},
	{"minimax-m3", "1Zenith-M27hS2SR5xQ2p", "MiniMax M3", "minimax", false, nil, "", 1000000, true, true, 0.08},
}

var (
	cachedCatalog   []ModelEntry
	lastCatalogSync time.Time
	catalogMu       sync.RWMutex
)

// GetCatalog devolve a lista completa de modelos disponíveis, enriquecida
// dinamicamente a partir do model_cache.json do app oficial quando disponível.
func GetCatalog() []ModelEntry {
	catalogMu.RLock()
	if len(cachedCatalog) > 0 && time.Since(lastCatalogSync) < 30*time.Second {
		defer catalogMu.RUnlock()
		return cachedCatalog
	}
	catalogMu.RUnlock()

	catalogMu.Lock()
	defer catalogMu.Unlock()

	// Recarrega base
	result := make([]ModelEntry, len(baseCatalog))
	copy(result, baseCatalog)

	codeMap := make(map[string]int, len(result))
	for idx, m := range result {
		codeMap[m.Code] = idx
	}

	// Tenta carregar do disco do Accio oficial
	home, _ := os.UserHomeDir()
	cachePath := filepath.Join(home, ".accio", "model_cache.json")
	if data, err := os.ReadFile(cachePath); err == nil {
		var root struct {
			Snapshots map[string]struct {
				Data []struct {
					Provider            string `json:"provider"`
					ProviderDisplayName string `json:"providerDisplayName"`
					ModelList           []struct {
						ModelCode              string   `json:"modelCode"`
						ModelDisplayName       string   `json:"modelDisplayName"`
						ModelName              string   `json:"modelName"`
						Visible                bool     `json:"visible"`
						Multimodal             bool     `json:"multimodal"`
						ReasoningEfforts       []string `json:"reasoningEfforts"`
						DefaultReasoningEffort string   `json:"defaultReasoningEffort"`
						ContextWindow          int      `json:"contextWindow"`
						FreeUse                bool     `json:"freeUse"`
						UsageMultiple          *float64 `json:"usageMultiple"`
					} `json:"modelList"`
				} `json:"data"`
			} `json:"snapshots"`
		}

		if err := json.Unmarshal(data, &root); err == nil {
			var snapshotData []struct {
				Provider            string `json:"provider"`
				ProviderDisplayName string `json:"providerDisplayName"`
				ModelList           []struct {
					ModelCode              string   `json:"modelCode"`
					ModelDisplayName       string   `json:"modelDisplayName"`
					ModelName              string   `json:"modelName"`
					Visible                bool     `json:"visible"`
					Multimodal             bool     `json:"multimodal"`
					ReasoningEfforts       []string `json:"reasoningEfforts"`
					DefaultReasoningEffort string   `json:"defaultReasoningEffort"`
					ContextWindow          int      `json:"contextWindow"`
					FreeUse                bool     `json:"freeUse"`
					UsageMultiple          *float64 `json:"usageMultiple"`
				} `json:"modelList"`
			}

			if snap, ok := root.Snapshots["GLOBAL::pt"]; ok && len(snap.Data) > 0 {
				snapshotData = snap.Data
			} else {
				for _, snap := range root.Snapshots {
					if len(snap.Data) > 0 {
						snapshotData = snap.Data
						break
					}
				}
			}

			for _, p := range snapshotData {
				for _, m := range p.ModelList {
					if !m.Visible {
						continue
					}
					mult := 0.0
					if m.UsageMultiple != nil {
						mult = *m.UsageMultiple
					}
					hasReason := len(m.ReasoningEfforts) > 0

					if idx, exists := codeMap[m.ModelCode]; exists {
						// Atualiza dados oficiais
						result[idx].Reason = hasReason
						result[idx].Efforts = m.ReasoningEfforts
						result[idx].DefEffort = m.DefaultReasoningEffort
						result[idx].ContextWin = m.ContextWindow
						result[idx].Multimodal = m.Multimodal
						result[idx].FreeUse = m.FreeUse
						result[idx].Multiple = mult
					} else {
						// Modelo novo não presente no catálogo base: adiciona dinamicamente
						slug := strings.ToLower(m.ModelDisplayName)
						slug = strings.ReplaceAll(slug, " ", "-")
						slug = strings.ReplaceAll(slug, "+", "-plus")
						newEntry := ModelEntry{
							ID:         slug,
							Code:       m.ModelCode,
							Name:       m.ModelDisplayName,
							Owner:      p.Provider,
							Reason:     hasReason,
							Efforts:    m.ReasoningEfforts,
							DefEffort:  m.DefaultReasoningEffort,
							ContextWin: m.ContextWindow,
							Multimodal: m.Multimodal,
							FreeUse:    m.FreeUse,
							Multiple:   mult,
						}
						result = append(result, newEntry)
						codeMap[m.ModelCode] = len(result) - 1
					}
				}
			}
		}
	}

	cachedCatalog = result
	lastCatalogSync = time.Now()
	return result
}

// GetCatalog expõe o catálogo para outros pacotes do app.
func (s *Server) GetCatalog() []ModelEntry {
	return GetCatalog()
}

// resolveModel traduz o model pedido (alias, alias com prefixo accio/, ou o
// próprio modelCode) para o código interno. Desconhecido devolve "".
func resolveModel(m string) string {
	m = strings.TrimPrefix(strings.TrimSpace(m), "accio/")
	if m == "" {
		return ""
	}
	cat := GetCatalog()
	for _, e := range cat {
		if strings.EqualFold(m, e.ID) || strings.EqualFold(m, e.Code) {
			return e.Code
		}
	}
	// código cru do Accio passa direto (modelo novo fora do catálogo)
	if len(m) >= 16 {
		return m
	}
	return ""
}


// ---- helpers ----

func chunk(id, model string, delta map[string]any, finish any) map[string]any {
	return map[string]any{
		"id": id, "object": "chat.completion.chunk", "created": time.Now().Unix(), "model": model,
		"choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": finish}},
	}
}

func writeSSE(w http.ResponseWriter, v any) {
	b, _ := json.Marshal(v)
	_, _ = fmt.Fprintf(w, "data: %s\n\n", b)
}

func httpError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{"message": message, "type": code, "code": code},
	})
}

// unused import guard
var _ = strings.TrimSpace
