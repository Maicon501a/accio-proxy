import { useEffect, useState } from "react";

type Msg = { who: "user" | "assistant"; body: string; reasoning?: string };

type ModelItem = {
  id: string;
  name: string;
  code: string;
  owned_by: string;
  reason: boolean;
  efforts?: string[];
  default_effort?: string;
};

export default function Playground({ initialModel }: { initialModel?: string }) {
  const [msgs, setMsgs] = useState<Msg[]>([]);
  const [input, setInput] = useState("");
  const [busy, setBusy] = useState(false);
  const [model, setModel] = useState(initialModel || "deepseek-v4-flash");
  const [models, setModels] = useState<ModelItem[]>([]);
  const [effort, setEffort] = useState("high");

  useEffect(() => {
    (window as any).go.main.App.GetModels()
      .then((res: any[]) => {
        if (Array.isArray(res) && res.length > 0) {
          setModels(res);
          if (initialModel) {
            setModel(initialModel);
          }
        }
      })
      .catch(() => {});
  }, [initialModel]);

  const currentModelObj = models.find((m) => m.id === model);

  // Sync default effort when model changes
  useEffect(() => {
    if (currentModelObj?.default_effort) {
      setEffort(currentModelObj.default_effort);
    }
  }, [model, currentModelObj]);

  const send = async () => {
    const text = input.trim();
    if (!text || busy) return;
    setInput("");
    setBusy(true);
    const next: Msg[] = [...msgs, { who: "user", body: text }];
    setMsgs([...next, { who: "assistant", body: "", reasoning: "" }]);

    try {
      const port = await (window as any).go.main.App.GetConfig().then((c: any) => c.gateway_port);
      const payload: Record<string, any> = {
        model,
        stream: true,
        messages: next.map((m) => ({ role: m.who, content: m.body })),
      };

      if (currentModelObj?.reason && effort) {
        payload.reasoning_effort = effort;
      }

      const res = await fetch(`http://127.0.0.1:${port}/v1/chat/completions`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      });
      if (!res.body) throw new Error("sem corpo de resposta");

      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      let accContent = "";
      let accReasoning = "";
      let buf = "";

      for (;;) {
        const { done, value } = await reader.read();
        if (done) break;
        buf += decoder.decode(value, { stream: true });
        const lines = buf.split("\n");
        buf = lines.pop() ?? "";
        for (const line of lines) {
          const t = line.trim();
          if (!t.startsWith("data:")) continue;
          const jsonPayload = t.slice(5).trim();
          if (jsonPayload === "[DONE]") continue;
          try {
            const j = JSON.parse(jsonPayload);
            const delta = j.choices?.[0]?.delta;
            if (delta?.content) accContent += delta.content;
            if (delta?.reasoning_content) accReasoning += delta.reasoning_content;
            setMsgs([...next, { who: "assistant", body: accContent, reasoning: accReasoning }]);
          } catch {}
        }
      }
      if (!accContent && !accReasoning) {
        setMsgs([...next, { who: "assistant", body: "(sem resposta — confira contas/gateway)" }]);
      }
    } catch (e: any) {
      setMsgs([...next, { who: "assistant", body: "erro: " + e.message }]);
    } finally {
      setBusy(false);
    }
  };

  // Group models by provider
  const grouped: Record<string, ModelItem[]> = {};
  for (const m of models) {
    const p = m.owned_by || "outros";
    if (!grouped[p]) grouped[p] = [];
    grouped[p].push(m);
  }

  return (
    <div className="playground">
      <h1>Playground</h1>
      <p className="subtitle">teste o gateway OpenAI-compatible local com qualquer modelo do catálogo</p>

      <div style={{ display: "flex", gap: 16, alignItems: "center", flexWrap: "wrap", marginBottom: 12 }}>
        <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
          <span style={{ fontSize: 12, color: "var(--text-faint)" }}>modelo</span>
          <select
            className="input"
            style={{ minWidth: 240 }}
            value={model}
            onChange={(e) => setModel(e.target.value)}
          >
            {Object.entries(grouped).map(([provider, list]) => (
              <optgroup key={provider} label={provider.toUpperCase()}>
                {list.map((m) => (
                  <option key={m.id} value={m.id}>
                    {m.name} ({m.id})
                  </option>
                ))}
              </optgroup>
            ))}
            {models.length === 0 && <option value="deepseek-v4-flash">DeepSeek V4 Flash</option>}
          </select>
        </div>

        {currentModelObj?.reason ? (
          <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
            <span style={{ fontSize: 12, color: "var(--text-faint)" }}>raciocínio (effort)</span>
            <select
              className="input"
              style={{ width: 110 }}
              value={effort}
              onChange={(e) => setEffort(e.target.value)}
            >
              {(currentModelObj.efforts || ["low", "medium", "high"]).map((ef) => (
                <option key={ef} value={ef}>
                  {ef}
                </option>
              ))}
            </select>
          </div>
        ) : (
          <span
            className="badge unknown"
            style={{ fontSize: 11, background: "rgba(255,255,255,0.04)" }}
          >
            Direto (sem esforço de raciocínio)
          </span>
        )}
      </div>

      <div className="chat-box">
        {msgs.length === 0 ? (
          <div className="empty">envie uma mensagem para testar o modelo selecionado</div>
        ) : (
          msgs.map((m, i) => (
            <div key={i} className={"msg " + m.who}>
              <div className="who">{m.who === "user" ? "você" : "assistente"}</div>
              {m.reasoning && (
                <details
                  style={{
                    background: "rgba(255,255,255,0.03)",
                    border: "1px solid var(--border)",
                    borderRadius: 6,
                    padding: "6px 10px",
                    marginBottom: 8,
                    fontSize: 12.5,
                  }}
                  open
                >
                  <summary style={{ cursor: "pointer", color: "var(--text-faint)", fontWeight: 500 }}>
                    Pensamento / Raciocínio ({m.reasoning.length} caracteres)
                  </summary>
                  <div
                    style={{
                      marginTop: 6,
                      color: "var(--text-dim)",
                      whiteSpace: "pre-wrap",
                      lineHeight: 1.5,
                    }}
                  >
                    {m.reasoning}
                  </div>
                </details>
              )}
              <div className="body" style={{ whiteSpace: "pre-wrap" }}>
                {m.body}
              </div>
            </div>
          ))
        )}
      </div>

      <div className="send-row">
        <input
          className="input"
          placeholder="mensagem…"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && send()}
          disabled={busy}
        />
        <button className="btn" onClick={send} disabled={busy || !input.trim()}>
          {busy ? "…" : "enviar"}
        </button>
      </div>
    </div>
  );
}
