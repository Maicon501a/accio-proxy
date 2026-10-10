import { useEffect, useRef, useState } from "react";

type Step = "choose" | "waiting" | "done" | "error" | "emu-email" | "emu-code";

export default function ManualLogin() {
  const [step, setStep] = useState<Step>("choose");
  const [email, setEmail] = useState("");
  const [code, code_set] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [doneEmail, setDoneEmail] = useState("");
  const pollRef = useRef<number | null>(null);

  useEffect(() => () => {
    if (pollRef.current) window.clearInterval(pollRef.current);
  }, []);

  const stopPoll = () => {
    if (pollRef.current) {
      window.clearInterval(pollRef.current);
      pollRef.current = null;
    }
  };

  // ---- login nativo (PKCE no browser, sem emulador) ----
  const startNative = async () => {
    setBusy(true);
    setError("");
    try {
      const err = await (window as any).go.main.App.NativeLoginStart(email.trim());
      if (err) {
        setError(err);
        setStep("error");
        return;
      }
      setStep("waiting");
      stopPoll();
      pollRef.current = window.setInterval(async () => {
        try {
          const st = await (window as any).go.main.App.NativeLoginStatus();
          if (st.startsWith("ok:")) {
            stopPoll();
            setDoneEmail(st.slice(3));
            setStep("done");
          } else if (st.startsWith("erro:")) {
            stopPoll();
            setError(st.slice(5));
            setStep("error");
          }
        } catch {}
      }, 1500);
    } catch (e: any) {
      setError(String(e?.message || e));
      setStep("error");
    } finally {
      setBusy(false);
    }
  };

  const cancelNative = async () => {
    stopPoll();
    try { await (window as any).go.main.App.NativeLoginCancel(); } catch {}
    setStep("choose");
  };

  // ---- login via emulador (legado) ----
  const sendCode = async () => {
    if (!email.trim() || busy) return;
    setBusy(true);
    setError("");
    try {
      const err = await (window as any).go.main.App.ManualLoginStart(email.trim());
      if (err) {
        setError(err);
        setStep("error");
      } else {
        setStep("emu-code");
      }
    } catch (e: any) {
      setError(String(e?.message || e));
      setStep("error");
    } finally {
      setBusy(false);
    }
  };

  const finishEmu = async () => {
    if (!code.trim() || busy) return;
    setBusy(true);
    setError("");
    try {
      const err = await (window as any).go.main.App.ManualLoginFinish(email.trim(), code.trim());
      if (err) {
        setError(err);
        setStep("error");
      } else {
        setDoneEmail(email.trim());
        setStep("done");
      }
    } catch (e: any) {
      setError(String(e?.message || e));
      setStep("error");
    } finally {
      setBusy(false);
    }
  };

  const reset = () => {
    stopPoll();
    setStep("choose");
    setEmail("");
    code_set("");
    setError("");
  };

  return (
    <div style={{ maxWidth: 480 }}>
      <h1>Login</h1>
      <p className="subtitle">conecta uma conta Accio no pool do proxy</p>

      {step === "choose" && (
        <>
          <div className="card" style={{ marginBottom: 14 }}>
            <div style={{ fontWeight: 600, marginBottom: 4 }}>login nativo <span className="badge" style={{ background: "rgba(74,222,128,0.12)", color: "var(--green)", border: "1px solid rgba(74,222,128,0.35)" }}>recomendado</span></div>
            <p style={{ fontSize: 12.5, color: "var(--text-dim)", marginBottom: 12, lineHeight: 1.6 }}>
              abre o login oficial do Accio no teu browser — tu faz email + código lá, e o proxy
              recebe o token sozinho. sem emulador, sem captcha quebrado.
            </p>
            <label className="field">
              <span className="label">nome no pool <span style={{ color: "var(--text-faint)", fontWeight: 400 }}>(opcional — vazio = automático)</span></span>
              <input
                className="input"
                placeholder="vazio: o proxy puxa do próprio login"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && startNative()}
                disabled={busy}
              />
            </label>
            <div style={{ display: "flex", justifyContent: "flex-end" }}>
              <button className="btn" onClick={startNative} disabled={busy}>
                {busy ? "abrindo browser…" : "abrir login no browser"}
              </button>
            </div>
          </div>

          <details>
            <summary style={{ fontSize: 12.5, color: "var(--text-faint)", cursor: "pointer", userSelect: "none" }}>
              login via emulador (legado — requer emulador ligado na aba Config)
            </summary>
            <div className="card" style={{ marginTop: 10 }}>
              <label className="field">
                <span className="label">email</span>
                <input
                  className="input"
                  placeholder="ex: usuario@emailnox.live"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  disabled={busy}
                />
              </label>
              <div style={{ display: "flex", justifyContent: "flex-end" }}>
                <button className="btn ghost" onClick={sendCode} disabled={busy || !email.trim()}>
                  {busy ? "enviando…" : "enviar código pelo emulador"}
                </button>
              </div>
            </div>
          </details>
        </>
      )}

      {step === "waiting" && (
        <div className="card" style={{ textAlign: "center", padding: 28 }}>
          <div className="spin" style={{ margin: "0 auto 14px" }} />
          <div style={{ fontWeight: 600, marginBottom: 6 }}>aguardando login no browser…</div>
          <div style={{ color: "var(--text-dim)", fontSize: 13, lineHeight: 1.6 }}>
            termina o login (email + código) na aba que abriu.<br />
            quando o Accio redirecionar de volta, a conta entra no pool sozinha.
          </div>
          <div style={{ marginTop: 18 }}>
            <button className="btn ghost" onClick={cancelNative}>cancelar</button>
          </div>
        </div>
      )}

      {(step === "done") && (
        <div className="card" style={{ textAlign: "center", padding: 28 }}>
          <div style={{ fontSize: 28, color: "var(--green)", marginBottom: 10 }}>✓</div>
          <div style={{ fontWeight: 600, marginBottom: 6 }}>conta salva no pool</div>
          <div style={{ color: "var(--text-dim)", fontSize: 13 }}>{doneEmail}</div>
          <div style={{ marginTop: 18 }}>
            <button className="btn" onClick={reset}>logar outra conta</button>
          </div>
        </div>
      )}

      {step === "error" && (
        <div className="card" style={{ borderColor: "rgba(248,113,113,0.4)" }}>
          <div style={{ color: "var(--red)", fontSize: 13, marginBottom: 14, whiteSpace: "pre-wrap" }}>
            {error}
          </div>
          <div style={{ display: "flex", justifyContent: "flex-end" }}>
            <button className="btn ghost" onClick={reset}>tentar de novo</button>
          </div>
        </div>
      )}

      {step === "emu-code" && (
        <div className="card">
          <p style={{ fontSize: 13, color: "var(--text-dim)", marginBottom: 14 }}>
            código enviado para <strong style={{ color: "var(--text)" }}>{email}</strong> pelo
            emulador. cola o código de 6 dígitos aqui.
          </p>
          <label className="field">
            <span className="label">código</span>
            <input
              className="input mono"
              placeholder="000000"
              maxLength={6}
              value={code}
              onChange={(e) => code_set(e.target.value.replace(/\D/g, ""))}
              onKeyDown={(e) => e.key === "Enter" && finishEmu()}
              disabled={busy}
              style={{ letterSpacing: 4, fontSize: 16, textAlign: "center" }}
            />
          </label>
          <div style={{ display: "flex", justifyContent: "flex-end", gap: 10 }}>
            <button className="btn ghost" onClick={reset} disabled={busy}>voltar</button>
            <button className="btn" onClick={finishEmu} disabled={busy || code.length < 6}>
              {busy ? "logando…" : "entrar"}
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
