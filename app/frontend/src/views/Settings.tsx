import { useEffect, useState } from "react";

export default function Settings({ onSaved }: { onSaved: () => void }) {
  const [port, setPort] = useState(8790);
  const [adb, setAdb] = useState("");
  const [serial, setSerial] = useState("");
  const [saved, setSaved] = useState(false);
  const [emulatorOn, setEmulatorOn] = useState(false);

  useEffect(() => {
    (window as any).go.main.App.GetConfig().then((c: any) => {
      setPort(c.gateway_port);
      setAdb(c.adb_path);
      setSerial(c.serial);
      setEmulatorOn(!!c.emulator_enabled);
    }).catch(() => {});
  }, []);

  const save = async () => {
    const err = await (window as any).go.main.App.SetConfig(port, adb, serial);
    if (!err) {
      setSaved(true);
      setTimeout(() => setSaved(false), 1600);
      onSaved();
    } else {
      alert(err);
    }
  };

  const toggleEmulator = async () => {
    const next = !emulatorOn;
    const err = await (window as any).go.main.App.SetEmulatorEnabled(next);
    if (!err) {
      setEmulatorOn(next);
      onSaved();
    } else {
      alert(err);
    }
  };

  return (
    <div style={{ maxWidth: 480 }}>
      <h1>Config</h1>
      <p className="subtitle">ajustes do proxy e do emulador</p>

      <div className="card">
        <div className="field" style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 14 }}>
          <div>
            <div className="label" style={{ marginBottom: 2 }}>emulador (adb)</div>
            <div style={{ fontSize: 12, color: "var(--text-faint)" }}>
              {emulatorOn
                ? "ligado — usado só pela fábrica/login manual"
                : "desligado — nenhum adb executado (padrão)"}
            </div>
          </div>
          <button
            className={"btn sm " + (emulatorOn ? "" : "ghost")}
            onClick={toggleEmulator}
            style={emulatorOn ? { background: "var(--green)", borderColor: "var(--green)" } : {}}
          >
            {emulatorOn ? "desligar" : "ligar"}
          </button>
        </div>

        <label className="field">
          <span className="label">porta do gateway (OpenAI-compatible)</span>
          <input
            className="input"
            type="number"
            value={port}
            onChange={(e) => setPort(parseInt(e.target.value) || 8790)}
          />
        </label>
        <label className="field">
          <span className="label">caminho do adb</span>
          <input className="input mono" value={adb} onChange={(e) => setAdb(e.target.value)} disabled={!emulatorOn} style={emulatorOn ? {} : { opacity: .45 }} />
        </label>
        <label className="field">
          <span className="label">serial do device</span>
          <input className="input mono" value={serial} onChange={(e) => setSerial(e.target.value)} disabled={!emulatorOn} style={emulatorOn ? {} : { opacity: .45 }} />
        </label>
        <div style={{ display: "flex", justifyContent: "flex-end", alignItems: "center", gap: 12 }}>
          {saved && <span style={{ color: "var(--green)", fontSize: 12.5 }}>salvo</span>}
          <button className="btn" onClick={save}>salvar</button>
        </div>
      </div>

      <p style={{ marginTop: 18, fontSize: 12, color: "var(--text-faint)", lineHeight: 1.7 }}>
        use no opencode:{" "}
        <span className="mono" style={{ color: "var(--text-dim)" }}>
          provider base URL http://127.0.0.1:{port}/v1
        </span>
        <br />
        api key: qualquer valor (não validado)
      </p>
    </div>
  );
}
