import { useEffect, useState } from "react";

type Pool = { total: number; remaining: number };

type CreditsInfo = {
  total?: number;
  remaining?: number;
  used?: number;
  planType?: string;
  checked_at?: string;
  error?: string;
  pools?: Record<string, Pool>;
};

type Account = {
  id: number;
  email: string;
  token: string;
  status: string;
  credits: number;
  pinned?: boolean;
  credits_info?: CreditsInfo;
  note: string;
  origin?: string;
  created: string;
  last_used: string;
};

// OriginVault marca as contas importadas do Accio Desktop.
const ORIGIN_VAULT = "vault";

type OfficialAccount = {
  account_id: string;
  accio_id?: string;
  email: string;
  name?: string;
  user_level?: string;
  is_active: boolean;
  access_token: string;
  login_at: number;
  credits?: {
    total?: number;
    remaining?: number;
    used?: number;
    type?: string;
    checked?: string;
    daily_remaining?: number;
    daily_total?: number;
  };
};

export default function Accounts({
  tick,
  health,
  onToggleMode,
}: {
  tick: number;
  health?: Record<string, any>;
  onToggleMode?: (mode: string) => void;
}) {
  const currentMode = health?.proxy_mode || "official";
  const officialSupported = health?.official_supported !== false;
  const [tab, setTab] = useState<"official" | "direct">(
    (currentMode as "official" | "direct") === "official" && !officialSupported ? "direct" : (currentMode as "official" | "direct")
  );

  // Keep tab synced when mode changes externally unless user manually switched tab
  useEffect(() => {
    const next = (currentMode as "official" | "direct") === "official" && !officialSupported ? "direct" : (currentMode as "official" | "direct");
    setTab(next);
  }, [currentMode]);

  // Direct Mode State
  const [list, setList] = useState<Account[]>([]);
  const [count, setCount] = useState(1);
  const [running, setRunning] = useState(false);
  const [showAdd, setShowAdd] = useState(false);
  const [addEmail, setAddEmail] = useState("");
  const [addToken, setAddToken] = useState("");
  const [refreshing, setRefreshing] = useState(false);

  // Official Mode State
  const [officialAccounts, setOfficialAccounts] = useState<OfficialAccount[]>([]);
  const [switchingId, setSwitchingId] = useState<string | null>(null);
  const [refreshingOfficial, setRefreshingOfficial] = useState(false);

  const refreshDirect = () => {
    (window as any).go.main.App.ListAccounts()
      .then((l: Account[]) => setList(l || []))
      .catch(() => {});
    (window as any).go.main.App.Health()
      .then((h: any) => setRunning(!!h.factory_running))
      .catch(() => {});
  };

  const refreshOfficial = () => {
    (window as any).go.main.App.ListOfficialAccounts()
      .then((accs: OfficialAccount[]) => setOfficialAccounts(accs || []))
      .catch(() => {});
  };

  useEffect(() => {
    refreshDirect();
    refreshOfficial();
  }, [tick]);

  // Actions for Direct Mode
  const createN = async () => {
    const err = await (window as any).go.main.App.CreateAccounts(count);
    if (err) alert(err);
    refreshDirect();
  };

  const stop = async () => {
    await (window as any).go.main.App.StopFactory();
    refreshDirect();
  };

  const del = async (email: string) => {
    if (!confirm(`remover ${email}?`)) return;
    await (window as any).go.main.App.DeleteAccount(email);
    refreshDirect();
  };

  const refreshCreditsDirect = async () => {
    setRefreshing(true);
    try {
      const msg = await (window as any).go.main.App.RefreshCredits();
      if (msg) alert(msg);
    } finally {
      setRefreshing(false);
      refreshDirect();
    }
  };

  const add = async () => {
    const err = await (window as any).go.main.App.AddAccount(addEmail, addToken);
    if (err) alert(err);
    setShowAdd(false);
    setAddEmail("");
    setAddToken("");
    refreshDirect();
  };

  const pin = async (a: Account) => {
    await (window as any).go.main.App.PinAccount(a.pinned ? "" : a.email);
    refreshDirect();
  };

  // Importa as contas do vault do Accio Desktop para o pool local.
  const [importing, setImporting] = useState(false);
  const importFromVault = async () => {
    if (importing) return;
    setImporting(true);
    try {
      const res = await (window as any).go.main.App.ImportAccountsFromVault();
      if (res?.error) {
        alert("Não deu para importar: " + res.error);
      } else {
        const parts = [`${res.imported} conta(s) nova(s)`];
        if (res.updated) parts.push(`${res.updated} atualizada(s)`);
        alert(`Importado do Accio Desktop: ${parts.join(", ")}.`);
      }
    } catch (e: any) {
      alert("Erro ao importar: " + (e?.message || e));
    } finally {
      setImporting(false);
      refreshDirect();
    }
  };

  const toggle = async (a: Account) => {
    const next = a.status === "active" ? "dead" : "active";
    await (window as any).go.main.App.SetAccountStatus(a.email, next);
    refreshDirect();
  };

  // Actions for Official Mode
  const switchAccount = async (account: OfficialAccount) => {
    if (account.is_active) return;
    setSwitchingId(account.account_id);
    try {
      const err = await (window as any).go.main.App.SwitchOfficialAccount(account.account_id);
      if (err) {
        alert("Erro ao alternar conta oficial: " + err);
      } else {
        refreshOfficial();
      }
    } catch (e: any) {
      alert("Erro ao alternar conta oficial: " + (e?.message || e));
    } finally {
      setSwitchingId(null);
    }
  };

  const refreshCreditsOfficial = async () => {
    setRefreshingOfficial(true);
    try {
      await (window as any).go.main.App.RefreshOfficialCredits();
      refreshOfficial();
    } catch (e: any) {
      alert("Erro ao atualizar créditos oficiais: " + (e?.message || e));
    } finally {
      setRefreshingOfficial(false);
    }
  };

  const isOfficialAlive = !!health?.official_running;

  return (
    <div>
      {/* Top Header */}
      <div style={{ display: "flex", alignItems: "center", gap: 16, marginBottom: 8 }}>
        <h1>Contas</h1>
        <div style={{ flex: 1 }} />

        {/* Tab switch between Official & Direct Pool */}
        <div className="subtabs">
          <button
            className={"subtab-btn" + (tab === "official" ? " active" : "")}
            onClick={() => setTab("official")}
            disabled={!officialSupported}
            title={
              officialSupported
                ? undefined
                : "Indisponível aqui: o App Oficial depende do Accio Desktop e do DPAPI do Windows"
            }
          >
            App Oficial {isOfficialAlive ? "(Online)" : ""}
          </button>
          <button
            className={"subtab-btn" + (tab === "direct" ? " active" : "")}
            onClick={() => setTab("direct")}
          >
            Pool Local (Direto)
          </button>
        </div>

        {tab === "direct" && (
          <>
            {running ? (
              <span className="badge running">
                <span className="dot" /> fábrica rodando
              </span>
            ) : null}
            {running ? (
              <button className="btn ghost sm" onClick={stop}>
                parar fábrica
              </button>
            ) : (
              <>
                <button
                  className="btn ghost sm"
                  onClick={refreshCreditsDirect}
                  disabled={refreshing}
                >
                  {refreshing ? "atualizando…" : "atualizar créditos"}
                </button>
                <button className="btn ghost sm" onClick={() => setShowAdd(true)}>
                  colar conta
                </button>
                <button
                  className="btn ghost sm"
                  onClick={importFromVault}
                  disabled={importing}
                  title="Lê as contas já logadas no Accio Desktop (accessToken + refreshToken do vault local) e adiciona ao pool do Modo Direto"
                >
                  {importing ? "importando…" : "extrair contas do Accio Desktop"}
                </button>
                <input
                  className="input"
                  style={{ width: 90 }}
                  type="number"
                  min={1}
                  value={count}
                  onChange={(e) => setCount(parseInt(e.target.value) || 1)}
                />
                <button className="btn" onClick={createN}>
                  criar {count} conta{count > 1 ? "s" : ""}
                </button>
              </>
            )}
          </>
        )}

        {tab === "official" && (
          <button
            className="btn ghost sm"
            onClick={refreshCreditsOfficial}
            disabled={refreshingOfficial || !isOfficialAlive}
          >
            {refreshingOfficial ? "atualizando…" : "atualizar créditos"}
          </button>
        )}
      </div>

      <p className="subtitle">
        {tab === "official"
          ? "contas capturadas do app oficial Accio — alterne com 1 clique e veja créditos em tempo real"
          : "pool de contas local SQLite — rotação automática e criação via fábrica"}
      </p>

      {/* --- OFFICIAL TAB --- */}
      {tab === "official" && (
        <>
          <div className={"official-banner" + (isOfficialAlive ? "" : " offline")}>
            <div className="info-col">
              <div className="title">
                <span
                  className="dot"
                  style={{
                    background: isOfficialAlive ? "var(--green)" : "var(--red)",
                  }}
                />
                {isOfficialAlive ? (
                  <>
                    App Accio Oficial Detectado
                    <span style={{ fontSize: 12, fontWeight: 400, color: "var(--text-faint)" }}>
                      (Porta :{health?.official_port || 4097} · PID {health?.official_pid || "—"})
                    </span>
                  </>
                ) : (
                  <>App Accio Oficial Desconectado</>
                )}
              </div>
              <div className="desc">
                {isOfficialAlive
                  ? "As requisições no proxy utilizam as contas e assinaturas oficiais do Accio, consumindo créditos sem exibir as conversas na interface do desktop."
                  : "Inicie o aplicativo oficial Accio no Windows para que o proxy se conecte ao processo e carregue suas contas autenticadas."}
              </div>
            </div>

            {currentMode !== "official" && onToggleMode && (
              <button
                className="btn sm"
                onClick={() => onToggleMode("official")}
                title="Ativar o Modo Oficial como o modo padrão do proxy"
              >
                Ativar Modo Oficial
              </button>
            )}
          </div>

          {officialAccounts.length === 0 ? (
            <div className="empty">
              {isOfficialAlive
                ? "Nenhuma conta autenticada encontrada no app oficial."
                : "Abra o aplicativo oficial Accio no seu computador para sincronizar as contas."}
            </div>
          ) : (
            <div className="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>Conta / Nome</th>
                    <th>Status no App</th>
                    <th>Créditos Atuais</th>
                    <th>ID Oficial</th>
                    <th>Token Local</th>
                    <th style={{ textAlign: "right" }}>Ação</th>
                  </tr>
                </thead>
                <tbody>
                  {officialAccounts.map((a) => {
                    const isSwitching = switchingId === a.account_id;
                    const creds = a.credits || (a.is_active ? health?.official_credits : null);
                    const remaining = creds?.remaining ?? 0;
                    const total = creds?.total ?? 0;
                    const pct = total > 0 ? Math.round((remaining / total) * 100) : null;

                    return (
                      <tr
                        key={a.account_id}
                        className={"acct-row" + (a.is_active ? " pinned" : "")}
                      >
                        <td>
                          <div style={{ fontWeight: 600 }}>{a.name || a.email}</div>
                          {a.name && (
                            <div style={{ fontSize: 12, color: "var(--text-faint)" }}>
                              {a.email}
                            </div>
                          )}
                        </td>
                        <td>
                          {a.is_active ? (
                            <span
                              className="badge"
                              style={{
                                background: "rgba(74,222,128,0.15)",
                                border: "1px solid rgba(74,222,128,0.45)",
                                color: "#4ade80",
                              }}
                            >
                              <span className="dot" /> ATIVA NO ACCIO
                            </span>
                          ) : (
                            <span className="badge unknown">salva</span>
                          )}
                          {a.user_level && (
                            <span
                              style={{
                                marginLeft: 6,
                                fontSize: 11,
                                color: "var(--text-dim)",
                                textTransform: "uppercase",
                              }}
                            >
                              {a.user_level}
                            </span>
                          )}
                        </td>
                        <td>
                          {a.is_active && creds ? (
                            <div style={{ minWidth: 150 }}>
                              <div
                                style={{
                                  display: "flex",
                                  alignItems: "baseline",
                                  gap: 6,
                                }}
                              >
                                <strong style={{ fontSize: 15 }}>{remaining}</strong>
                                {total > 0 && (
                                  <span style={{ color: "var(--text-faint)" }}>/ {total}</span>
                                )}
                                {pct !== null && (
                                  <span
                                    style={{
                                      color:
                                        pct > 30 ? "#4ade80" : pct > 10 ? "#fbbf24" : "#f87171",
                                      fontSize: 12,
                                    }}
                                  >
                                    {pct}%
                                  </span>
                                )}
                                {creds.type && (
                                  <span style={{ fontSize: 11, color: "var(--text-faint)" }}>
                                    ({creds.type})
                                  </span>
                                )}
                              </div>
                              {total > 0 && (
                                <div
                                  style={{
                                    background: "rgba(255,255,255,.08)",
                                    borderRadius: 99,
                                    height: 4,
                                    marginTop: 4,
                                    overflow: "hidden",
                                  }}
                                >
                                  <div
                                    style={{
                                      width: `${Math.max(0, Math.min(100, pct ?? 0))}%`,
                                      height: "100%",
                                      background:
                                        (pct ?? 0) > 30
                                          ? "#4ade80"
                                          : (pct ?? 0) > 10
                                          ? "#fbbf24"
                                          : "#f87171",
                                      transition: "width .4s",
                                    }}
                                  />
                                </div>
                              )}
                              {creds.daily_total ? (
                                <div
                                  style={{
                                    fontSize: 11,
                                    color: "var(--text-faint)",
                                    marginTop: 3,
                                  }}
                                >
                                  diário: {creds.daily_remaining}/{creds.daily_total}
                                </div>
                              ) : null}
                            </div>
                          ) : a.is_active ? (
                            <span style={{ color: "var(--text-faint)" }}>carregando…</span>
                          ) : (
                            <span style={{ color: "var(--text-faint)" }}>
                              (ative para ver saldo)
                            </span>
                          )}
                        </td>
                        <td className="mono" style={{ fontSize: 12, color: "var(--text-dim)" }}>
                          {a.account_id}
                        </td>
                        <td className="mono">{a.access_token || "—"}</td>
                        <td style={{ whiteSpace: "nowrap", textAlign: "right" }}>
                          {a.is_active ? (
                            <span
                              style={{
                                fontSize: 12,
                                color: "var(--green)",
                                fontWeight: 500,
                              }}
                            >
                              ✓ Em uso pelo Proxy
                            </span>
                          ) : (
                            <button
                              className="btn sm"
                              onClick={() => switchAccount(a)}
                              disabled={isSwitching || !isOfficialAlive}
                            >
                              {isSwitching ? "alternando…" : "alternar conta"}
                            </button>
                          )}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}
        </>
      )}

      {/* --- DIRECT TAB --- */}
      {tab === "direct" && (
        <>
          {currentMode !== "direct" && onToggleMode && (
            <div className="official-banner" style={{ borderColor: "rgba(96, 165, 250, 0.3)" }}>
              <div className="info-col">
                <div className="title">
                  <span className="dot" style={{ background: "#60a5fa" }} />
                  Visualizando Pool Local (Modo Direto)
                </div>
                <div className="desc">
                  O proxy está configurado no momento para o <strong>Modo App Oficial</strong>.
                  Para que o gateway passe a responder usando este pool local, ative o Modo Direto.
                </div>
              </div>
              <button className="btn sm" onClick={() => onToggleMode("direct")}>
                Ativar Modo Direto
              </button>
            </div>
          )}

          {list.length === 0 ? (
            <div className="empty">nenhuma conta ainda — use a fábrica ou o login manual</div>
          ) : (
            <div className="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>Email</th>
                    <th>Status</th>
                    <th>Créditos</th>
                    <th>Token</th>
                    <th>Origem</th>
                    <th>Criada</th>
                    <th>Último uso</th>
                    <th></th>
                  </tr>
                </thead>
                <tbody>
                  {list.map((a) => {
                    const ci = a.credits_info || {};
                    const pools = Object.entries(ci.pools || {}) as [string, Pool][];
                    const remaining = ci.remaining ?? a.credits;
                    const total = ci.total ?? 0;
                    const pct = total > 0 ? Math.round((remaining / total) * 100) : null;
                    const isServing = health?.serving === a.email && a.status === "active";
                    return (
                      <tr
                        key={a.id}
                        className={"acct-row" + (a.pinned ? " pinned" : "")}
                        title={
                          a.pinned ? "clique para soltar o pin" : "clique para fixar esta conta"
                        }
                        onClick={() => pin(a)}
                      >
                        <td>
                          {a.email}
                          {a.origin === ORIGIN_VAULT && (
                            <span
                              className="badge"
                              style={{
                                marginLeft: 8,
                                background: "rgba(96,165,250,0.15)",
                                border: "1px solid rgba(96,165,250,0.45)",
                                color: "#60a5fa",
                              }}
                              title="Importada do vault do Accio Desktop"
                            >
                              <span className="dot" /> ACCIO DESKTOP
                            </span>
                          )}
                          {a.pinned && (
                            <span className="badge pinned-badge">
                              <span className="dot" /> FIXADA
                            </span>
                          )}
                          {isServing && !a.pinned && (
                            <span
                              className="badge"
                              style={{
                                marginLeft: 8,
                                background: "rgba(74,222,128,0.15)",
                                border: "1px solid rgba(74,222,128,0.45)",
                                color: "#4ade80",
                              }}
                            >
                              <span className="dot" /> ATIVA AGORA
                            </span>
                          )}
                        </td>
                        <td>
                          <span className={"badge " + a.status}>
                            <span className="dot" /> {a.status}
                          </span>
                        </td>
                        <td>
                          {ci.error ? (
                            <span style={{ color: "var(--red)", fontSize: 12 }} title={ci.error}>
                              erro:{" "}
                              {ci.error.length > 42 ? ci.error.slice(0, 42) + "…" : ci.error}
                            </span>
                          ) : a.status === "dead" && !ci.checked_at ? (
                            <span style={{ color: "var(--text-faint)" }}>—</span>
                          ) : ci.checked_at ? (
                            <div style={{ minWidth: 150 }}>
                              <div
                                style={{ display: "flex", alignItems: "baseline", gap: 6 }}
                              >
                                <strong style={{ fontSize: 15 }}>{remaining}</strong>
                                {total > 0 && (
                                  <span style={{ color: "var(--text-faint)" }}>/ {total}</span>
                                )}
                                {pct !== null && (
                                  <span
                                    style={{
                                      color:
                                        pct > 30 ? "#4ade80" : pct > 10 ? "#fbbf24" : "#f87171",
                                      fontSize: 12,
                                    }}
                                  >
                                    {pct}%
                                  </span>
                                )}
                              </div>
                              {total > 0 && (
                                <div
                                  style={{
                                    background: "rgba(255,255,255,.08)",
                                    borderRadius: 99,
                                    height: 4,
                                    marginTop: 4,
                                    overflow: "hidden",
                                  }}
                                >
                                  <div
                                    style={{
                                      width: `${Math.max(0, Math.min(100, pct ?? 0))}%`,
                                      height: "100%",
                                      background:
                                        (pct ?? 0) > 30
                                          ? "#4ade80"
                                          : (pct ?? 0) > 10
                                          ? "#fbbf24"
                                          : "#f87171",
                                      transition: "width .4s",
                                    }}
                                  />
                                </div>
                              )}
                              {pools.length > 0 && (
                                <div
                                  style={{
                                    fontSize: 11,
                                    color: "var(--text-faint)",
                                    marginTop: 3,
                                  }}
                                >
                                  {pools
                                    .map(([name, p]) => `${name} ${p.remaining}/${p.total}`)
                                    .join(" · ")}
                                </div>
                              )}
                            </div>
                          ) : (
                            <span style={{ color: "var(--text-faint)" }}>carregando…</span>
                          )}
                        </td>
                        <td className="mono">{a.token}</td>
                        <td>{a.note}</td>
                        <td className="mono" style={{ color: "var(--text-faint)" }}>
                          {a.created}
                        </td>
                        <td className="mono" style={{ color: "var(--text-faint)" }}>
                          {a.last_used || "—"}
                          {health?.served_count?.[a.email] ? (
                            <span style={{ color: "var(--text-dim)" }}>
                              {" "}
                              · {health.served_count[a.email]} req
                            </span>
                          ) : null}
                        </td>
                        <td
                          style={{ whiteSpace: "nowrap", textAlign: "right" }}
                          onClick={(e) => e.stopPropagation()}
                        >
                          <button className="btn ghost sm" onClick={() => toggle(a)}>
                            {a.status === "active" ? "desativar" : "ativar"}
                          </button>{" "}
                          <button className="btn danger sm" onClick={() => del(a.email)}>
                            excluir
                          </button>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}

          {showAdd && (
            <div className="modal-overlay" onClick={() => setShowAdd(false)}>
              <div className="modal" onClick={(e) => e.stopPropagation()}>
                <h3>colar conta</h3>
                <label className="field">
                  <span className="label">email</span>
                  <input
                    className="input"
                    value={addEmail}
                    onChange={(e) => setAddEmail(e.target.value)}
                    placeholder="usuario@emailnox.live"
                  />
                </label>
                <label className="field">
                  <span className="label">access token (32 hex)</span>
                  <input
                    className="input mono"
                    value={addToken}
                    onChange={(e) => setAddToken(e.target.value)}
                    placeholder="ab9f9756…"
                  />
                </label>
                <div className="actions">
                  <button className="btn ghost" onClick={() => setShowAdd(false)}>
                    cancelar
                  </button>
                  <button
                    className="btn"
                    onClick={add}
                    disabled={!addEmail.trim() || !addToken.trim()}
                  >
                    salvar
                  </button>
                </div>
              </div>
            </div>
          )}
        </>
      )}
    </div>
  );
}
