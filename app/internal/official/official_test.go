package official

import (
	"context"
	"testing"
	"time"
)

func TestOfficialIntegration(t *testing.T) {
	mgr := NewManager()
	cli, cliPath, err := mgr.FindGatewayCLI()
	if err != nil {
		t.Logf("Aviso: app oficial não localizado: %v", err)
		return
	}
	t.Logf("Gateway CLI encontrado em: %s (PID: %d, URL: %s)", cliPath, cli.PID, cli.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stat, err := mgr.CheckAlive(ctx)
	if err != nil {
		t.Fatalf("Erro ao testar CheckAlive: %v", err)
	}
	t.Logf("Status do App Oficial: Running=%v, Port=%d, User=%+v", stat.Running, stat.Port, stat.ActiveAccount)
	if stat.Credits != nil {
		t.Logf("Créditos: Total=%v, Remaining=%v, Type=%v", stat.Credits["total"], stat.Credits["remaining"], stat.Credits["type"])
	}

	accs, err := mgr.ListAccounts(ctx)
	if err != nil {
		t.Fatalf("Erro ao listar contas oficiais: %v", err)
	}
	t.Logf("Total de contas capturadas: %d", len(accs))
	for _, a := range accs {
		t.Logf("  - [%s] %s (%s) - Ativa: %v - Token: %s", a.AccountID, a.Name, a.Email, a.IsActive, a.AccessToken)
	}

	token, accId, err := mgr.GetActiveToken(ctx)
	if err != nil {
		t.Fatalf("Erro ao obter token ativo: %v", err)
	}
	t.Logf("Token ativo obtido para account %s: %s...", accId, token[:min(10, len(token))])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
