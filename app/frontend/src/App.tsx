import { useState, useEffect } from "react";
import "./style.css";
import Dashboard from "./views/Dashboard";
import Accounts from "./views/Accounts";
import Models from "./views/Models";
import ManualLogin from "./views/ManualLogin";
import Playground from "./views/Playground";
import Settings from "./views/Settings";

type View = "dashboard" | "accounts" | "models" | "manual" | "playground" | "settings";

const NAV: { id: View; label: string; icon: string }[] = [
  { id: "dashboard", label: "Dashboard", icon: "◧" },
  { id: "accounts", label: "Contas", icon: "◎" },
  { id: "models", label: "Modelos", icon: "❖" },
  { id: "manual", label: "Login Manual", icon: "⇥" },
  { id: "playground", label: "Playground", icon: "❯" },
  { id: "settings", label: "Config", icon: "⚙" },
];

export default function App() {
  const [view, setView] = useState<View>("dashboard");
  const [health, setHealth] = useState<Record<string, any>>({});
  const [tick, setTick] = useState(0);
  const [selectedModel, setSelectedModel] = useState<string>("deepseek-v4-flash");

  const poll = async () => {
    try {
      const h = await (window as any).go.main.App.Health();
      setHealth(h || {});
    } catch {}
  };

  useEffect(() => {
    let alive = true;
    poll();
    const t = setInterval(() => {
      if (alive) {
        poll();
        setTick((x) => x + 1);
      }
    }, 4000);
    return () => {
      alive = false;
      clearInterval(t);
    };
  }, []);

  const nav = (id: View) => (e: React.MouseEvent) => {
    e.preventDefault();
    setView(id);
  };

  const currentMode = health.proxy_mode || "official";
  const officialSupported = health.official_supported !== false;

  const toggleMode = async (targetMode: string) => {
    if (targetMode === currentMode) return;
    try {
      await (window as any).go.main.App.SetProxyMode(targetMode);
      setHealth((prev) => ({ ...prev, proxy_mode: targetMode }));
      setTick((x) => x + 1);
    } catch (e: any) {
      alert("Erro ao alternar modo: " + (e?.message || e));
    }
  };

  return (
    <div className="app">
      <aside className="sidebar">
        <div className="logo">
          <span className="dot" /> Accio Proxy
        </div>

        {/* Botão de Alternância de Modos (Accio Oficial vs Endpoints Direto) */}
        <div className="mode-switch-bar" title="Alterne entre o Modo App Oficial (Accio aberto) e Modo Direto">
          <button
            className={"mode-btn" + (currentMode === "official" ? " active official" : "")}
            onClick={() => toggleMode("official")}
            disabled={!officialSupported}
            title={
              officialSupported
                ? "Usa a sessão do Accio Desktop aberto"
                : "Indisponível nesta plataforma: o Modo Oficial depende do Accio Desktop e do DPAPI do Windows"
            }
          >
            <span
              className="dot"
              style={{
                width: 6,
                height: 6,
                borderRadius: "50%",
                background: health.official_running ? "var(--green)" : "var(--amber)",
              }}
            />
            Oficial
          </button>
          <button
            className={"mode-btn" + (currentMode === "direct" ? " active" : "")}
            onClick={() => toggleMode("direct")}
          >
            Direto
          </button>
        </div>

        {NAV.map((n) => (
          <button
            key={n.id}
            className={"nav-item" + (view === n.id ? " active" : "")}
            onClick={nav(n.id)}
          >
            <span style={{ width: 16, textAlign: "center", opacity: 0.7 }}>{n.icon}</span>
            {n.label}
          </button>
        ))}

        <div className="spacer" />

        <div className="gateway-status">
          <div>
            modo:{" "}
            <strong style={{ color: currentMode === "official" ? "var(--green)" : "var(--text)" }}>
              {currentMode === "official" ? "Accio Oficial" : "Endpoints Direto"}
            </strong>
          </div>
          <div>gateway :{health.gateway_port ?? "—"}</div>
          {currentMode === "official" ? (
            <div>
              oficial:{" "}
              {health.official_running ? (
                <span style={{ color: "var(--green)" }}>conectado (:{health.official_port || 4097})</span>
              ) : (
                <span style={{ color: "var(--amber)" }}>desconectado</span>
              )}
            </div>
          ) : (
            <div>
              contas ativas {health.active ?? 0}/{health.accounts ?? 0}
            </div>
          )}
          {health.emulator_enabled ? (
            <div>emulador {health.emulator_alive ? "online" : "offline"}</div>
          ) : null}
        </div>
      </aside>

      <main className="main">
        {view === "dashboard" && (
          <Dashboard health={health} tick={tick} onToggleMode={toggleMode} />
        )}
        {view === "accounts" && (
          <Accounts health={health} tick={tick} onToggleMode={toggleMode} />
        )}
        {view === "models" && (
          <Models
            onSelectPlayground={(modelId) => {
              setSelectedModel(modelId);
              setView("playground");
            }}
          />
        )}
        {view === "manual" && <ManualLogin />}
        {view === "playground" && <Playground initialModel={selectedModel} />}
        {view === "settings" && (
          <Settings
            onSaved={() => {
              poll();
              setTick((x) => x + 1);
            }}
          />
        )}
      </main>
    </div>
  );
}
