package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"accio-proxy/internal/account"
	"accio-proxy/internal/config"
	"accio-proxy/internal/emulator"
	"accio-proxy/internal/factory"
	"accio-proxy/internal/gateway"
	"accio-proxy/internal/nativeauth"
	"accio-proxy/internal/official"
	"accio-proxy/internal/phoenix"
	"accio-proxy/internal/sgbridge"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the Wails binding surface for the frontend.
type App struct {
	cfg      *config.Config
	store    *account.Store
	driver   *emulator.Driver
	harvest  *factory.ReqableHarvester
	official *official.Manager
	ctx      context.Context

	nativeFlow *nativeauth.Flow

	gwOnce sync.Once
	gwErr  error
	gw     *gateway.Server

	logMu    sync.Mutex
	logLines []string

	factoryRunning bool
	factoryCancel  context.CancelFunc
}

func NewApp() *App {
	a := &App{}
	cfg, err := config.Load()
	if err != nil {
		log.Printf("config: %v", err)
	}
	a.cfg = cfg
	_ = a.initStore()
	a.driver = emulator.New(cfg.ADBPath, cfg.DeviceSerial)
	a.driver.SetEnabled(cfg.EmulatorEnabled)
	a.harvest = &factory.ReqableHarvester{}
	a.official = official.NewManager()
	a.log("accio-proxy ready (modo: %s)", a.cfg.ProxyMode)
	return a
}

func (a *App) initStore() error {
	store, err := account.Open(a.cfg.AccountsDB())
	if err != nil {
		return err
	}
	a.store = store
	return nil
}

func (a *App) log(format string, args ...any) {
	a.logMu.Lock()
	defer a.logMu.Unlock()
	a.logLines = append(a.logLines, time.Now().Format("15:04:05")+" "+fmt.Sprintf(format, args...))
	if len(a.logLines) > 500 {
		a.logLines = a.logLines[len(a.logLines)-500:]
	}
}

// ---- frontend bindings ----

// Health reports overall status for the dashboard.
func (a *App) Health() map[string]any {
	accounts := 0
	active := 0
	if a.store != nil {
		if list, err := a.store.List(); err == nil {
			accounts = len(list)
			for _, x := range list {
				if x.Status == "active" {
					active++
				}
			}
		}
	}
	emulatorAlive := false
	if a.driver.Enabled {
		emulatorAlive = a.driver.IsAlive(context.Background())
	}
	serving, servedCount := "", map[string]int64{}
	if a.gw != nil {
		serving, servedCount = a.gw.ServingInfo()
	}

	offStat := official.OfficialStatus{}
	if a.official != nil {
		offStat = a.official.GetStatus()
	}

	return map[string]any{
		"gateway_port":       a.cfg.GatewayPort,
		"proxy_mode":         a.cfg.ProxyMode,
		"official_supported": config.OfficialSupported(),
		"accounts":           accounts,
		"active":           active,
		"emulator_enabled": a.driver.Enabled,
		"emulator_alive":   emulatorAlive,
		"factory_running":  a.factoryRunning,
		"serving":          serving,
		"served_count":     servedCount,
		"official_running": offStat.Running,
		"official_port":    offStat.Port,
		"official_pid":     offStat.PID,
		"official_account": offStat.ActiveAccount,
		"official_credits": offStat.Credits,
		"official_error":   offStat.Error,
	}
}

// GetProxyMode retorna o modo atual: "official" ou "direct".
func (a *App) GetProxyMode() string {
	if a.cfg.ProxyMode == "" {
		return "official"
	}
	return a.cfg.ProxyMode
}

// SetProxyMode alterna entre "official" e "direct".
func (a *App) SetProxyMode(mode string) string {
	if mode == "official" && !config.OfficialSupported() {
		return "o Modo Oficial depende do Accio Desktop e do DPAPI do Windows — nesta plataforma use o Modo Direto"
	}
	if err := a.cfg.SetProxyMode(mode); err != nil {
		return err.Error()
	}
	a.log("modo alterado para: %s", a.cfg.ProxyMode)
	return ""
}

// GetOfficialStatus retorna o status da conexão com o app Accio Oficial.
func (a *App) GetOfficialStatus() map[string]any {
	if a.official == nil {
		return map[string]any{"running": false, "error": "gerenciador não inicializado"}
	}
	st := a.official.GetStatus()
	return map[string]any{
		"running":        st.Running,
		"url":            st.URL,
		"port":           st.Port,
		"pid":            st.PID,
		"active_account": st.ActiveAccount,
		"credits":        st.Credits,
		"error":          st.Error,
	}
}

// ListOfficialAccounts lista as contas capturadas do Accio Oficial em tempo real.
func (a *App) ListOfficialAccounts() []map[string]any {
	if a.official == nil {
		return []map[string]any{}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	accs, err := a.official.ListAccounts(ctx)
	if err != nil {
		a.log("erro ao listar contas oficiais: %v", err)
		return []map[string]any{}
	}
	out := make([]map[string]any, 0, len(accs))
	for _, acc := range accs {
		out = append(out, map[string]any{
			"account_id":   acc.AccountID,
			"accio_id":     acc.AccioID,
			"email":        acc.Email,
			"name":         acc.Name,
			"user_level":   acc.UserLevel,
			"is_active":    acc.IsActive,
			"access_token": acc.AccessToken,
			"credits":      acc.Credits,
			"login_at":     acc.LoginAt,
		})
	}
	return out
}

// SwitchOfficialAccount troca a conta ativa no Accio Oficial.
func (a *App) SwitchOfficialAccount(accountID string) string {
	if a.official == nil {
		return "gerenciador oficial indisponível"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := a.official.SwitchAccount(ctx, accountID); err != nil {
		a.log("erro no switch da conta oficial %s: %v", accountID, err)
		return err.Error()
	}
	a.log("conta oficial alternada com sucesso: %s", accountID)
	return ""
}

// RefreshOfficialCredits atualiza os créditos da conta ativa no app oficial.
func (a *App) RefreshOfficialCredits() map[string]any {
	if a.official == nil {
		return map[string]any{"error": "gerenciador indisponível"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	creds, err := a.official.RefreshCredits(ctx)
	if err != nil {
		a.log("erro ao atualizar créditos oficiais: %v", err)
		return map[string]any{"error": err.Error()}
	}
	a.log("créditos oficiais atualizados: %v", creds["remaining"])
	return creds
}

// GetModels devolve a lista completa de modelos disponíveis através do proxy.
func (a *App) GetModels() []gateway.ModelEntry {
	return gateway.GetCatalog()
}

// SetEmulatorEnabled liga/desliga o emulador (persiste no config.json).
func (a *App) SetEmulatorEnabled(on bool) string {
	if err := a.cfg.SetEmulatorEnabled(on); err != nil {
		return err.Error()
	}
	a.driver.SetEnabled(on)
	a.log("emulador: %v", map[bool]string{true: "ativado", false: "desativado"}[on])
	return ""
}

// StartGateway boots the OpenAI-compatible server (idempotent).
func (a *App) StartGateway() string {
	a.gwOnce.Do(func() {
		picker := &storePicker{store: a.store}
		signer := sgbridge.New(a.cfg.DataDir)
		client := phoenix.New(signer, a.cfg.Utdid, "0.33.0", sgbridge.DesktopAppKey)
		srv := gateway.New(client, picker, a.cfg.GatewayPort).
			WithSessions(a.store, a.cfg.Utdid, "0.33.0").
			WithOfficial(a.official, func() string { return a.cfg.ProxyMode })
		go func() {
			if err := srv.Start(); err != nil {
				a.gwErr = err
				a.log("gateway: %v", err)
			}
		}()
		a.gw = srv
		a.log("gateway started on :%d (modo: %s)", a.cfg.GatewayPort, a.cfg.ProxyMode)
	})
	if a.gwErr != nil {
		return a.gwErr.Error()
	}
	return ""
}

// ListAccounts returns the account pool for the UI.
func (a *App) ListAccounts() []map[string]any {
	if a.store == nil {
		return []map[string]any{}
	}
	list, err := a.store.List()
	if err != nil {
		return []map[string]any{}
	}
	out := make([]map[string]any, 0, len(list))
	for _, x := range list {
		token := x.AccessToken
		masked := token
		if len(token) > 12 {
			masked = token[:6] + "…" + token[len(token)-4:]
		}
		credits := map[string]any{}
		if x.CreditsInfo != "" {
			_ = json.Unmarshal([]byte(x.CreditsInfo), &credits)
		} else if x.Status == "active" {
			// conta ativa ainda sem snapshot — cobre o buraco da varredura
			// (cooldown de 5min dentro do SessionManager evita martelação).
			a.creditsSoon(x.Email)
		}
		out = append(out, map[string]any{
			"id": x.ID, "email": x.Email, "token": masked, "status": x.Status,
			"credits": x.Credits, "credits_info": credits, "note": x.Note,
			"created": x.CreatedAt, "last_used": x.LastUsedAt, "pinned": x.Pinned,
		})
	}
	return out
}

// creditsSoon dispara a consulta de créditos de uma conta via gateway.
func (a *App) creditsSoon(email string) {
	if a.gw == nil {
		return
	}
	a.gw.EnsureCredits(email)
}

// RefreshCredits consulta o saldo de todas as contas ativas agora (botão da UI).
func (a *App) RefreshCredits() string {
	if a.store == nil {
		return "store indisponível"
	}
	type entry struct{ Email, Access string }
	var entries []entry
	for _, e := range a.store.ListForRefresh() {
		entries = append(entries, entry{e.Email, e.Access})
	}
	if len(entries) == 0 {
		return "nenhuma conta ativa com sessão"
	}
	ok := 0
	for _, e := range entries {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		info, err := phoenix.QueryCredits(ctx, a.cfg.Utdid, "0.33.0", e.Access)
		cancel()
		if err != nil {
			a.log("créditos %s: %v", e.Email, err)
			continue
		}
		b, _ := json.Marshal(info)
		remaining := 0
		if r, ok2 := info["remaining"].(int); ok2 {
			remaining = r
		}
		_ = a.store.SetCredits(e.Email, remaining, string(b))
		ok++
	}
	return fmt.Sprintf("%d/%d contas atualizadas", ok, len(entries))
}

// DeleteAccount removes an account from the pool.
func (a *App) DeleteAccount(email string) string {
	if err := a.store.Delete(email); err != nil {
		return err.Error()
	}
	a.log("account deleted: %s", email)
	return ""
}

// AddAccount inserts an account with a known token (manual paste — useful
// for seeding from a capture while the factory is not wired yet).
func (a *App) AddAccount(email, token string) string {
	email = strings.TrimSpace(email)
	token = strings.TrimSpace(token)
	if email == "" || token == "" {
		return "email and token required"
	}
	if _, err := a.store.Insert(account.Account{
		Email: email, AccessToken: token, Status: "active", Note: "pasted",
	}); err != nil {
		return err.Error()
	}
	a.log("account added: %s", email)
	return ""
}

// SetAccountStatus toggles active/dead.
func (a *App) SetAccountStatus(email, status string) string {
	if err := a.store.SetStatus(email, status); err != nil {
		return err.Error()
	}
	return ""
}

// PinAccount fixa (ou desfixa, se já for a fixada) a conta por onde todas as
// requests do gateway passam. Com email "" apenas limpa o pin.
func (a *App) PinAccount(email string) string {
	if email != "" && a.store.PinnedEmail() == email {
		email = ""
	}
	if err := a.store.SetPinned(email); err != nil {
		return err.Error()
	}
	if email == "" {
		a.log("pin removido — rotação automática de volta")
	} else {
		a.log("conta fixada: %s (todas as requests passam por ela)", email)
	}
	return ""
}

// ManualLoginStart begins the manual login flow with the given email.
func (a *App) ManualLoginStart(email string) string {
	if !a.driver.Enabled {
		return "emulador desativado — ative na aba Config"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := a.driver.LoginEmailFlow(ctx, email); err != nil {
		a.log("manual login: %v", err)
		return err.Error()
	}
	a.log("manual login: code sent to %s", email)
	return ""
}

// ManualLoginFinish enters the code and harvests the token.
func (a *App) ManualLoginFinish(email, code string) string {
	if !a.driver.Enabled {
		return "emulador desativado — ative na aba Config"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	ml := factory.ManualLogin{Driver: a.driver, Harvest: a.harvest, Store: a.store}
	if err := ml.FinishCode(ctx, email, code); err != nil {
		a.log("manual login finish: %v", err)
		return err.Error()
	}
	a.log("manual login complete: %s", email)
	return ""
}

// CreateAccounts starts the on-demand factory for N accounts.
func (a *App) CreateAccounts(n int) string {
	if a.factoryRunning {
		return "factory already running"
	}
	if !a.driver.Enabled {
		return "emulador desativado — ative na aba Config"
	}
	if n <= 0 {
		return "count must be positive"
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.factoryRunning = true
	a.factoryCancel = cancel
	go func() {
		defer func() {
			a.factoryRunning = false
			cancel()
		}()
		mail := &stubMail{} // replaced by the real provider after site RE
		f := factory.Factory{
			Store:   a.store,
			Driver:  a.driver,
			Mail:    mail,
			Harvest: a.harvest,
			Log:     func(f string, args ...any) { a.log(f, args...) },
		}
		created, err := f.CreateN(ctx, n)
		if err != nil {
			a.log("factory: %v", err)
		}
		a.log("factory done: %d/%d created", created, n)
	}()
	return ""
}

// StopFactory cancels a running factory.
func (a *App) StopFactory() string {
	if a.factoryCancel != nil {
		a.factoryCancel()
		a.log("factory stopped by user")
	}
	return ""
}

// Logs returns the recent activity log lines.
func (a *App) Logs() []string {
	a.logMu.Lock()
	defer a.logMu.Unlock()
	out := make([]string, len(a.logLines))
	copy(out, a.logLines)
	return out
}

// GetConfig / SetConfig manage settings.
func (a *App) GetConfig() map[string]any {
	return map[string]any{
		"gateway_port":     a.cfg.GatewayPort,
		"proxy_mode":       a.cfg.ProxyMode,
		"adb_path":         a.cfg.ADBPath,
		"serial":           a.cfg.DeviceSerial,
		"emulator_enabled": a.cfg.EmulatorEnabled,
	}
}

func (a *App) SetConfig(port int, adb, serial string) string {
	if err := a.cfg.Set(port, adb, serial); err != nil {
		return err.Error()
	}
	a.driver = emulator.New(adb, serial)
	a.driver.SetEnabled(a.cfg.EmulatorEnabled)
	return ""
}

// storePicker adapts the account store to the gateway.
type storePicker struct{ store *account.Store }

func (p *storePicker) Pick(ctx context.Context) (string, string, error) {
	return p.store.Pick(ctx)
}
func (p *storePicker) ReportFailure(email string) {
	p.store.ReportFailure(email)
}

// compile-time: o store precisa satisfazer o CreditStore do gateway.
var _ gateway.CreditStore = (*account.Store)(nil)

// stubMail is a placeholder until the temp-mail RE lands.
type stubMail struct{}

func (s *stubMail) NewInbox(ctx context.Context) (string, error) {
	return "", fmt.Errorf("temp mail provider not configured yet")
}
func (s *stubMail) WaitForCode(ctx context.Context, email string, timeout time.Duration) (string, error) {
	return "", fmt.Errorf("temp mail provider not configured yet")
}

var _ = strings.TrimSpace

// nativeStoreAdapter adapta o account.Store ao nativeauth.Store.
type nativeStoreAdapter struct {
	s   *account.Store
	app *App
}

func (n nativeStoreAdapter) UpsertTokens(email, access, refresh string) error {
	if list, err := n.s.List(); err == nil {
		for _, acc := range list {
			if acc.Email == email {
				if err := n.s.UpdateTokens(email, access, refresh); err != nil {
					return err
				}
				n.app.creditsSoon(email)
				return nil
			}
		}
	}
	_, err := n.s.Insert(account.Account{
		Email: email, AccessToken: access, RefreshToken: refresh,
		Status: "active", Note: "native-oauth",
	})
	if err == nil {
		n.app.creditsSoon(email)
	}
	return err
}

// NativeLoginStart inicia o login nativo (PKCE via browser) e abre o browser.
func (a *App) NativeLoginStart(email string) string {
	if a.nativeFlow == nil {
		a.nativeFlow = &nativeauth.Flow{
			GatewayBase: phoenix.GatewayBase,
			Store:       nativeStoreAdapter{s: a.store, app: a},
		}
	}
	loginURL, err := a.nativeFlow.Start(strings.TrimSpace(email))
	if err != nil {
		a.log("native login: %v", err)
		return err.Error()
	}
	a.log("native login: browser aberto, aguardando callback")
	if a.ctx != nil {
		runtime.BrowserOpenURL(a.ctx, loginURL)
	}
	return ""
}

// NativeLoginStatus devolve "waiting" | "ok:<email>" | "erro:<msg>" | "idle".
func (a *App) NativeLoginStatus() string {
	if a.nativeFlow == nil {
		return "idle"
	}
	return a.nativeFlow.Status()
}

// NativeLoginCancel aborta o login nativo em andamento.
func (a *App) NativeLoginCancel() string {
	if a.nativeFlow != nil {
		a.nativeFlow.Cancel()
	}
	return ""
}

// startup is the Wails lifecycle hook.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.log("window started")
	_ = a.StartGateway()
}
