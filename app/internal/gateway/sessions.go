// sessions: renovação automática de tokens e rastreamento de créditos.
package gateway

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"accio-proxy/internal/phoenix"
)

// creditCooldown evita martelar o endpoint de entitlement — consulta no
// máximo uma vez por conta a cada 5 minutos.
const creditCooldown = 5 * time.Minute

// refreshInterval: varredura preventiva que renova qualquer conta ativa
// com refreshToken (mantém o pool sempre com token fresco).
const refreshInterval = 6 * time.Hour

// RefreshAccount renova a sessão de uma conta agora. Devolve o novo access
// token, ou "" se não deu.
func (m *SessionManager) RefreshAccount(ctx context.Context, email, currentAccess string) (string, error) {
	if m == nil || m.store == nil {
		return "", nil
	}
	access, refresh := m.store.RefreshTokenOf(email)
	if access == "" {
		access = currentAccess
	}
	if refresh == "" {
		log.Printf("sessions: %s sem refresh_token — não dá pra renovar", email)
		return "", nil
	}
	newAccess, newRefresh, err := phoenix.RefreshTokens(ctx, m.utdid, m.appVer, access, refresh)
	if err != nil {
		log.Printf("sessions: refresh %s falhou: %v", email, err)
		return "", err
	}
	if err := m.store.UpdateTokens(email, newAccess, newRefresh); err != nil {
		log.Printf("sessions: salvar tokens de %s: %v", email, err)
	}
	log.Printf("sessions: %s renovado (access novo salvo)", email)
	return newAccess, nil
}

// UpdateCredits consulta o saldo e persiste no store (com cooldown).
// Em caso de erro grava o motivo em credits_info (UI mostra em vez de
// ficar eternamente em "carregando…").
func (m *SessionManager) UpdateCredits(email string) {
	if m == nil || m.store == nil {
		return
	}
	m.mu.Lock()
	if t, ok := m.lastAsk[email]; ok && time.Since(t) < creditCooldown {
		m.mu.Unlock()
		return
	}
	m.lastAsk[email] = time.Now()
	m.mu.Unlock()

	access, _ := m.store.RefreshTokenOf(email)
	if access == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	info, err := phoenix.QueryCredits(ctx, m.utdid, m.appVer, access)
	if err != nil {
		log.Printf("sessions: créditos %s: %v", email, err)
		errJSON, _ := json.Marshal(map[string]any{
			"checked_at": time.Now().Format(time.RFC3339),
			"error":      err.Error(),
		})
		_ = m.store.SetCredits(email, -1, string(errJSON))
		return
	}
	b, _ := json.Marshal(info)
	remaining := 0
	if r, ok := info["remaining"].(int); ok {
		remaining = r
	}
	if err := m.store.SetCredits(email, remaining, string(b)); err != nil {
		log.Printf("sessions: salvar créditos %s: %v", email, err)
	}
}

// loop roda a varredura preventiva de refresh em intervalo fixo.
func (m *SessionManager) loop() {
	// primeira passada 1 minuto após o boot
	m.sweep()
	t := time.NewTicker(refreshInterval)
	for range t.C {
		m.sweep()
	}
}

// sweep renova todas as contas com refresh_token e atualiza créditos.
func (m *SessionManager) sweep() {
	if m == nil || m.store == nil {
		return
	}
	for _, a := range m.store.ListForRefresh() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if _, err := m.RefreshAccount(ctx, a.Email, a.Access); err == nil {
			m.UpdateCredits(a.Email)
		}
		cancel()
	}
}
