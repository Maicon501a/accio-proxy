# Accio Proxy — Architecture

> OpenAI-compatible proxy + account factory for Accio (Android path).
> Stack: Go (backend) + Wails v2 + React/TS (frontend).

```
┌───────────────────────────── D:\accio proxy ─────────────────────────────┐
│                                                                          │
│  app/                          ← Wails project root                      │
│  ├── main.go                   ← entrypoint: wires backend + wails       │
│  ├── app.go                    ← Wails bindings (frontend ↔ Go)          │
│  ├── internal/                                                            │
│  │   ├── gateway/              ← OpenAI-compatible HTTP server           │
│  │   │   ├── server.go         /v1/chat/completions (stream+tools)       │
│  │   │   │                     /v1/models, /health                       │
│  │   │   └── convert.go        OpenAI ⇄ phoenix protocol translation     │
│  │   ├── phoenix/              ← Accio protocol client (Android path)    │
│  │   │   ├── client.go         mtop requests + sign injection            │
│  │   │   ├── ws.go             websocket /ws/vps/app client              │
│  │   │   ├── mtop.go           API names, header set, body builders      │
│  │   │   └── types.go          protocol types (functionCall etc.)        │
│  │   ├── account/              ← account store + lifecycle               │
│  │   │   ├── store.go          sqlite-backed account pool                │
│  │   │   │                     (email, tokens, credits, status)          │
│  │   │   └── health.go         credit check / token refresh probing      │
│  │   ├── factory/              ← on-demand account creation              │
│  │   │   ├── factory.go        orchestrates: N accounts requested →      │
│  │   │   │                     loop until N healthy accounts exist       │
│  │   │   ├── emulator.go       adb driver: launch app, tap flows,        │
│  │   │   │                     harvest session tokens                    │
│  │   │   └── mitm.go           reqable MCP bridge: watch captured        │
│  │   │                     WS handshakes → extract accessToken           │
│  │   ├── tempmail/             ← pluggable temp email provider           │
│  │   │   └── provider.go       interface (RE of the site comes later)    │
│  │   └── config/               ← settings persistence                    │
│  │       └── config.go                                                     │
│  ├── frontend/                 ← React + TS (Wails)                      │
│  │   └── src/                                                            │
│  │       ├── App.tsx           shell + routing                           │
│  │       ├── views/            Dashboard / Accounts / Playground /       │
│  │   │   │                     ManualLogin / Settings                    │
│  │   │   └── ...                                                          │
│  │       └── components/       shared UI primitives                      │
│  └── build/                    wails build assets                        │
│                                                                          │
│  docs/                         ← RE notes, protocol, this file           │
│  tools/                        ← capture chain (tcppipe, guestpipe,      │
│                                 udpdns) used during RE, kept for ops     │
└──────────────────────────────────────────────────────────────────────────┘
```

## Data flow

### Chat (opencode → proxy → Accio)

```
opencode ──POST /v1/chat/completions──▶ gateway
   │   (OpenAI format, tools=[...], stream=true)
   │
   gateway ──translate──▶ phoenix client ──▶ Accio (mtop / WS)
   │                                          │
   │   ◀── SSE chunks ◀── stream translate ◀──┘
   │       (delta.tool_calls = phoenix functionCall)
   opencode executes tools, sends results back (functionResponse)
```

### Account factory (on-demand)

```
UI: "create N accounts"
   │
   factory loop:
   │   1. tempmail.provide() ──▶ fresh inbox
   │   2. emulator.runSignupFlow(email)   (adb-driven UI automation)
   │   3. app opens WS post-login ──▶ mitm watcher harvests accessToken
   │   4. account.health.verify() ──▶ credits? store.save()
   │   └─ repeat until N healthy accounts
   │
   store ◀── gateway picks account per request (rotation)
```

### Manual login (from the UI)

```
UI: paste email → send code → app receives code (user reads inbox)
    → user types code in UI → emulator.enterCode(code)
    → app logs in → WS handshake → mitm harvest → account saved
```

## Security headers (mtop)

Requests need per-request SecurityGuard signatures (x-sign/x-mini-wua/x-sgext)
for appKey 34959200. Options (in order of preference):

1. **Emulator sign daemon**: a small app on the emulator signs requests on
   demand (the app's own SG library does the work) — build later.
2. **WS-only mode**: chat rides the websocket (token in URL, no sign) —
   primary path for the proxy.

## Status (2026-09-20)

- [x] RE: protocol mapped (docs/RE-PROTOCOL.md)
- [x] Wails project scaffolded
- [ ] phoenix client (port from old proxy + WS)
- [ ] gateway (OpenAI-compatible)
- [ ] account store + factory + manual login
- [ ] frontend views
