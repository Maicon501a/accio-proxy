import { useEffect, useState } from "react";

export default function Dashboard({
  health,
  tick,
  onToggleMode,
}: {
  health: Record<string, any>;
  tick: number;
  onToggleMode?: (mode: string) => void;
}) {
  const [logs, setLogs] = useState<string[]>([]);
  const currentMode = health.proxy_mode || "official";
  const isOfficialAlive = !!health.official_running;
  const officialSupported = health.official_supported !== false;

  useEffect(() => {
    (window as any).go.main.App.Logs()
      .then((l: string[]) => setLogs(l || []))
      .catch(() => {});
  }, [tick]);

  const officialCreds = health.official_credits || {};
  const activeAcc = health.official_account || {};
  const remaining = officialCreds.remaining ?? null;
  const total = officialCreds.total ?? null;

  return (
    <div>
      <h1>Dashboard</h1>
      <p className="subtitle">estado e modo operacional do proxy em tempo real</p>

      {/* Hero Switcher do Modo de Operação */}
      <div className={"mode-card mode-" + currentMode}>
        <div style={{ flex: 1 }}>
          <div className="mode-card-title">
            <span
              className="dot"
              style={{
                background: currentMode === "official" ? "var(--green)" : "#60a5fa",
              }}
            />
            {currentMode === "official"
              ? "Modo App Oficial (Accio Desktop)"
              : "Modo Endpoints Diretos (Pool Standalone)"}
          </div>
          <div className="mode-card-desc">
            {currentMode === "official"
              ? "Conecta diretamente ao aplicativo oficial Accio em execução. As requisições do OpenCode utilizam a autenticação oficial e gastam seus créditos, sem poluir a webview ou histórico de conversas do desktop."
              : "Funciona de forma independente do app oficial, alternando requisições entre as contas salvas no banco de dados SQLite local. Assina com os headers de device nativos em Go, sem depender do módulo proprietário do SecurityGuard."}
          </div>
        </div>

        {onToggleMode && (
          <div className="mode-toggle-group">
            <button
              className={
                "mode-toggle-btn" +
                (currentMode === "official" ? " active official" : "")
              }
              onClick={() => onToggleMode("official")}
              disabled={officialSupported === false}
              title={
                officialSupported === false
                  ? "Indisponível nesta plataforma: depende do Accio Desktop e do DPAPI do Windows"
                  : undefined
              }
            >
              <span
                className="dot"
                style={{
                  width: 6,
                  height: 6,
                  borderRadius: "50%",
                  background: isOfficialAlive ? "var(--green)" : "var(--amber)",
                }}
              />
              App Oficial
            </button>
            <button
              className={
                "mode-toggle-btn" +
                (currentMode === "direct" ? " active direct" : "")
              }
              onClick={() => onToggleMode("direct")}
            >
              Endpoints Direto
            </button>
          </div>
        )}
      </div>

      {/* Estatísticas baseadas no Modo */}
      {currentMode === "official" ? (
        <div className="stat-grid">
          <div className="card stat">
            <div className="label">App Oficial</div>
            <div className={"value " + (isOfficialAlive ? "ok" : "bad")}>
              {isOfficialAlive ? "online" : "offline"}
            </div>
            <div style={{ fontSize: 11, color: "var(--text-faint)", marginTop: 4 }}>
              {isOfficialAlive
                ? `porta :${health.official_port || 4097} · pid ${health.official_pid || "—"}`
                : "abra o Accio.exe"}
            </div>
          </div>

          <div className="card stat">
            <div className="label">Conta Ativa</div>
            <div
              className="value"
              style={{
                fontSize: 18,
                whiteSpace: "nowrap",
                overflow: "hidden",
                textOverflow: "ellipsis",
              }}
              title={activeAcc.email || activeAcc.name || "Nenhuma"}
            >
              {activeAcc.name || activeAcc.email || (isOfficialAlive ? "—" : "Desconectado")}
            </div>
            <div style={{ fontSize: 11, color: "var(--text-faint)", marginTop: 4 }}>
              {activeAcc.email || "autenticado no Accio"}
            </div>
          </div>

          <div className="card stat">
            <div className="label">Créditos Restantes</div>
            <div className="value ok">
              {remaining !== null ? (
                <>
                  {remaining}
                  {total !== null && (
                    <span style={{ fontSize: 15, color: "var(--text-faint)", fontWeight: 400 }}>
                      {" "}
                      / {total}
                    </span>
                  )}
                </>
              ) : (
                <span style={{ fontSize: 18, color: "var(--text-faint)" }}>—</span>
              )}
            </div>
            <div style={{ fontSize: 11, color: "var(--text-faint)", marginTop: 4 }}>
              {officialCreds.type ? `plano ${officialCreds.type}` : "atualizado em tempo real"}
            </div>
          </div>

          <div className="card stat">
            <div className="label">Porta do Gateway</div>
            <div className="value ok">:{health.gateway_port ?? 8080}</div>
            <div style={{ fontSize: 11, color: "var(--text-faint)", marginTop: 4 }}>
              endpoint p/ OpenCode
            </div>
          </div>
        </div>
      ) : (
        <div className="stat-grid">
          <div className="card stat">
            <div className="label">Contas ativas</div>
            <div className={"value " + (health.active > 0 ? "ok" : "bad")}>
              {health.active ?? 0}
            </div>
          </div>
          <div className="card stat">
            <div className="label">Total de contas</div>
            <div className="value">{health.accounts ?? 0}</div>
          </div>
          <div className="card stat">
            <div className="label">Gateway</div>
            <div className="value ok">:{health.gateway_port ?? 0}</div>
          </div>
          <div className="card stat">
            <div className="label">Emulador</div>
            {health.emulator_enabled ? (
              <div className={"value " + (health.emulator_alive ? "ok" : "bad")}>
                {health.emulator_alive ? "online" : "offline"}
              </div>
            ) : (
              <div className="value" style={{ color: "var(--text-faint)", fontSize: 18 }}>
                desativado
              </div>
            )}
          </div>
          <div className="card stat">
            <div className="label">Fábrica</div>
            <div className="value" style={{ fontSize: 18 }}>
              {health.factory_running ? (
                <span className="badge running">
                  <span className="dot" /> rodando
                </span>
              ) : (
                <span className="badge unknown">parada</span>
              )}
            </div>
          </div>
        </div>
      )}

      {/* Console de Eventos */}
      <h1 style={{ fontSize: 15 }}>Atividade</h1>
      <p className="subtitle" style={{ marginBottom: 12 }}>
        últimos eventos do gateway e assinador
      </p>
      <div className="console">
        {logs.length === 0 ? (
          <div>sem eventos ainda</div>
        ) : (
          logs.slice(-40).map((line, i) => (
            <div key={i}>
              <span className="ts">{line.slice(0, 8)}</span>
              {line.slice(9)}
            </div>
          ))
        )}
      </div>
    </div>
  );
}
