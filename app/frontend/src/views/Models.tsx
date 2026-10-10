import { useEffect, useState } from "react";

type ModelItem = {
  id: string;
  code: string;
  name: string;
  owned_by: string;
  reason: boolean;
  efforts?: string[];
  default_effort?: string;
  context_window: number;
  multimodal: boolean;
  free_use: boolean;
  usage_multiple: number;
};

export default function Models({ onSelectPlayground }: { onSelectPlayground?: (modelId: string) => void }) {
  const [models, setModels] = useState<ModelItem[]>([]);
  const [filter, setFilter] = useState("all");
  const [search, setSearch] = useState("");
  const [copiedId, setCopiedId] = useState<string | null>(null);

  useEffect(() => {
    (window as any).go.main.App.GetModels()
      .then((res: any[]) => {
        if (Array.isArray(res) && res.length > 0) {
          setModels(res);
        }
      })
      .catch(() => {});
  }, []);

  const copyToClipboard = (id: string) => {
    navigator.clipboard.writeText(id);
    setCopiedId(id);
    setTimeout(() => setCopiedId(null), 2000);
  };

  const filtered = models.filter((m) => {
    const matchesSearch =
      m.id.toLowerCase().includes(search.toLowerCase()) ||
      m.name.toLowerCase().includes(search.toLowerCase()) ||
      m.code.toLowerCase().includes(search.toLowerCase()) ||
      m.owned_by.toLowerCase().includes(search.toLowerCase());

    if (!matchesSearch) return false;

    if (filter === "all") return true;
    if (filter === "reasoning") return m.reason;
    if (filter === "no-reasoning") return !m.reason;
    if (filter === "free") return m.free_use;
    return m.owned_by === filter;
  });

  const providers = Array.from(new Set(models.map((m) => m.owned_by))).filter(Boolean);

  const formatTokens = (tokens: number) => {
    if (!tokens) return "—";
    if (tokens >= 1000000) return `${(tokens / 1000000).toFixed(tokens % 1000000 === 0 ? 0 : 1)}M`;
    if (tokens >= 1000) return `${Math.round(tokens / 1000)}k`;
    return `${tokens}`;
  };

  return (
    <div>
      <div style={{ display: "flex", alignItems: "center", gap: 16, marginBottom: 4 }}>
        <h1>Modelos Disponíveis</h1>
        <div style={{ flex: 1 }} />
        <span className="badge" style={{ background: "rgba(255,255,255,0.06)", color: "var(--text-dim)" }}>
          {models.length} modelos detectados
        </span>
      </div>
      <p className="subtitle">
        catálogo completo do Accio Proxy — use estes IDs no OpenCode, Cline ou Cursor (OpenAI-compatible)
      </p>

      {/* Banner Informativo sobre Raciocínio (DeepSeek & GLM) */}
      <div className="official-banner" style={{ marginBottom: 20 }}>
        <div className="info-col">
          <div className="title">
            <span className="dot" style={{ background: "var(--green)" }} />
            Esforços de Raciocínio (Reasoning Efforts)
          </div>
          <div className="desc">
            Modelos como <strong>DeepSeek V4 Flash</strong> e <strong>GLM 5.3</strong> operam em modo direto (sem esforço de raciocínio configurável na API do Accio). Modelos como <strong>GPT-6 Astra</strong>, <strong>GPT-5.6</strong>, <strong>Claude Opus/Sonnet</strong> e <strong>Gemini</strong> aceitam parâmetros de raciocínio (<span className="mono">low, medium, high, max</span>).
          </div>
        </div>
      </div>

      {/* Barra de Filtros e Busca */}
      <div style={{ display: "flex", gap: 12, alignItems: "center", marginBottom: 16, flexWrap: "wrap" }}>
        <input
          className="input"
          placeholder="buscar por nome, id ou provedor…"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          style={{ width: 280 }}
        />

        <div className="subtabs">
          <button className={"subtab-btn" + (filter === "all" ? " active" : "")} onClick={() => setFilter("all")}>
            Todos ({models.length})
          </button>
          <button className={"subtab-btn" + (filter === "reasoning" ? " active" : "")} onClick={() => setFilter("reasoning")}>
            Com Raciocínio ({models.filter((m) => m.reason).length})
          </button>
          <button className={"subtab-btn" + (filter === "no-reasoning" ? " active" : "")} onClick={() => setFilter("no-reasoning")}>
            Direto / Sem Raciocínio ({models.filter((m) => !m.reason).length})
          </button>
          {providers.map((p) => (
            <button
              key={p}
              className={"subtab-btn" + (filter === p ? " active" : "")}
              onClick={() => setFilter(p)}
              style={{ textTransform: "capitalize" }}
            >
              {p}
            </button>
          ))}
        </div>
      </div>

      {/* Tabela de Modelos */}
      {filtered.length === 0 ? (
        <div className="empty">nenhum modelo encontrado com os filtros atuais</div>
      ) : (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>ID p/ OpenCode</th>
                <th>Nome Oficial</th>
                <th>Provedor</th>
                <th>Raciocínio</th>
                <th>Contexto</th>
                <th>Capacidades</th>
                <th>Custo</th>
                <th style={{ textAlign: "right" }}>Ações</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((m) => {
                const isDeepSeekOrGLM = m.id.includes("deepseek") || m.id.includes("glm");
                return (
                  <tr key={m.id} className="acct-row">
                    <td>
                      <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                        <span className="mono" style={{ fontWeight: 600, color: "var(--accent)" }}>
                          {m.id}
                        </span>
                        <button
                          className="btn ghost sm"
                          style={{ padding: "2px 6px", fontSize: 11 }}
                          onClick={() => copyToClipboard(m.id)}
                          title="Copiar ID para usar no OpenCode"
                        >
                          {copiedId === m.id ? "✓ copiado" : "copiar"}
                        </button>
                      </div>
                    </td>
                    <td>
                      <div style={{ fontWeight: 500 }}>{m.name}</div>
                      <div className="mono" style={{ fontSize: 11, color: "var(--text-faint)" }}>
                        {m.code}
                      </div>
                    </td>
                    <td>
                      <span
                        className="badge"
                        style={{
                          textTransform: "uppercase",
                          fontSize: 10,
                          background: "rgba(255,255,255,0.05)",
                          color: "var(--text-dim)",
                        }}
                      >
                        {m.owned_by}
                      </span>
                    </td>
                    <td>
                      {m.reason ? (
                        <div>
                          <span
                            className="badge"
                            style={{
                              background: "rgba(52,211,153,0.12)",
                              border: "1px solid rgba(52,211,153,0.3)",
                              color: "var(--green)",
                            }}
                          >
                            <span className="dot" /> Suportado
                          </span>
                          {m.efforts && m.efforts.length > 0 && (
                            <div style={{ fontSize: 11, color: "var(--text-faint)", marginTop: 3 }}>
                              {m.efforts.join(", ")}
                            </div>
                          )}
                        </div>
                      ) : (
                        <div>
                          <span
                            className="badge unknown"
                            style={{
                              background: isDeepSeekOrGLM ? "rgba(255,255,255,0.04)" : undefined,
                            }}
                          >
                            Direto (Sem Raciocínio)
                          </span>
                        </div>
                      )}
                    </td>
                    <td>
                      <span style={{ fontWeight: 500 }}>{formatTokens(m.context_window)}</span>
                    </td>
                    <td>
                      <div style={{ display: "flex", gap: 4, flexWrap: "wrap" }}>
                        <span className="badge unknown" style={{ fontSize: 10 }}>texto</span>
                        {m.multimodal && (
                          <span
                            className="badge"
                            style={{
                              fontSize: 10,
                              background: "rgba(96,165,250,0.15)",
                              color: "#60a5fa",
                              border: "1px solid rgba(96,165,250,0.3)",
                            }}
                          >
                            visão
                          </span>
                        )}
                      </div>
                    </td>
                    <td>
                      {m.free_use ? (
                        <span style={{ color: "var(--green)", fontSize: 12, fontWeight: 500 }}>
                          Grátis
                        </span>
                      ) : (
                        <span style={{ color: "var(--text-dim)", fontSize: 12 }}>
                          {m.usage_multiple > 0 ? `${m.usage_multiple}x` : "pago"}
                        </span>
                      )}
                    </td>
                    <td style={{ textAlign: "right", whiteSpace: "nowrap" }}>
                      {onSelectPlayground && (
                        <button
                          className="btn ghost sm"
                          onClick={() => onSelectPlayground(m.id)}
                          title="Abrir no Playground"
                        >
                          testar ❯
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
    </div>
  );
}
