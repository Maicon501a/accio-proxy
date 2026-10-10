# Accio Android — Protocol RE Notes

> Fonte: captura MITM real do app `com.accio.android.app` v1.42.2 (2026-09-20)
> Cadeia: iptables (uid do app) → guestpipe (emulador) → tcppipe (host) → Reqable
> CA do Reqable injetado no store do conscrypt via nsenter nos zygotes (Android 15).

## 1. Endpoints descobertos

| Host | Papel | Observações |
|---|---|---|
| `acs.m.alibaba.com` | **gateway mtop do Accio Android** | `/gw/mtop.alibaba.phoenix.gateway.<API>/1.0/` — aceita MITM (sem pinning efetivo) |
| `acs.m.taobao.com` | gateway mtop (telemetria) | `mtop.common.getTimestamp` — aceita MITM |
| `phoenix-gw.alibaba.com` | gateway direto (chat/WS) | **MITM mata o app** (anti-tamper client-side no TLS desse host); WS `/ws/vps/app` capturável quando MITMado |
| `stream.accio.com` | chat stream (desktop) | pinado; bypass no relay |
| `tls-accio.dingtalk.com` | ACCS/IMPaas LWP (canal de nuvem) | bypass no relay — conexão longa, protocolo binário server-first |
| `global-orange-dc.alibaba.com` | probe de conectividade | `/checkProbe` GET 200 |
| `amdc.alibaba.com` | HTTPDNS da Alibaba | bypass |
| `sale.alibaba.com` | fluxo ICBU (login web) | apareceu durante login |
| `47.246.x.x :443 (IP direto)` | canal binário ANet/ACCS | server-first, sem SNI — passa direto |

## 2. API mtop confirmada (capturada com response 200)

### mtop.alibaba.phoenix.gateway.meta.agent.query

```
POST https://acs.m.alibaba.com/gw/mtop.alibaba.phoenix.gateway.meta.agent.query/1.0/
Content-Type: application/x-www-form-urlencoded;charset=UTF-8
```

Headers essenciais (todos URL-encoded quando necessário):

| Header | Valor de exemplo | Origem |
|---|---|---|
| `x-appkey` | `34959200` | fixo do app Android ICBU |
| `x-ttid` | `212200@icbu_android_1.42.2` | canal/app/versão |
| `x-app-ver` | `1.42.2` | versão do app |
| `x-utdid` | `arAOlI8pLukDABrhiafD8uxh` | utdid do device (SG) |
| `x-devid` | `Aruzl7Fv...` | device id (SG) |
| `x-umt` | `nm4Cfh5L...` | token de device (SG) |
| `x-c-traceid` | `<utdid><ms><seq>` | trace |
| `firstopentime` | `1789922964321` | epoch ms do primeiro open |
| `x-sid` | `a59ea1143b85b6339ace5cfb62f2edd7` | **sessão do login** |
| `x-uid` | `4500110307598` | **uid da sessão** |
| `x-t` | `1789934005` | epoch s do request |
| `x-sign` | `azbHzv007xAAI1T...` | **assinatura SecurityGuard (por request)** |
| `x-mini-wua` | `aKAR4okC%2B6d...` | **assinatura wua (SG)** |
| `x-sgext` | `JBPMqMyDV%2B...` | **extensão SG** |
| `x-sgext`/`x-bx-version` | `6.8.260602` | versão baxia |
| `user-agent` | `MTOPSDK%2F3.1.1.7+(Android%3B15%3BSamsung%3BSM-S911B)` | |
| `x-nettype`/`x-nq` | `WIFI` | |
| `traffic` | `BR` | país |
| `f-refer` | `mtop` | |
| `x-extdata` | `openappkey%3DDEFAULT_AUTH` | |
| `x-features` | `27` | |

Body (form-urlencoded, campo único `data`):

```json
{
  "appCountry": "BR", "dpScaling": "1.5", "timeZone": "America/Sao_Paulo",
  "language": "pt", "deviceId": "<utdid>", "version": "", "versionCode": "14202",
  "addressId": "", "localeCode": "BR", "displayWidth": "720",
  "packageRegion": "GLOBAL", "countryCode": "br", "teamId": "0",
  "deviceLevel": "1", "currency": "USD", "appZipCode": ""
}
```

Response (200, `ret: ["SUCCESS::调用成功"]`):

```json
{
  "api": "mtop.alibaba.phoenix.gateway.meta.agent.query",
  "data": {
    "code": "200",
    "data": {
      "data": [ { "agentId": "MID-85004888U1789933-...", "agentType": "site-builder", ... } ],
      "hasUpdate": "true", "version": "6"
    },
    "message": "0"
  }
}
```

> userId Accio da conta logada: `1792004888`. Agentes default provisionados no primeiro login
> (site-builder, seo-geo, omni-ecommerce, seller-assistant, design_expert, sourcing).

## 3. WebSocket phoenix (sandbox do chat)

```
GET wss://phoenix-gw.alibaba.com/ws/vps/app?connectionToken=<hex32>&accessToken=<hex32>
```

- handshake 101 (o `accessToken` é o token da sessão Accio)
- frames JSON text:
  - in: `{"deviceId":1048705308,"sessionId":"s-04pomilogux5nk085","timestamp":...,"type":"connected"}`
  - in: `{"requestId":"488d92c8-...","sessionId":"s-...","timestamp":...,"type":"sandbox_active"}`
  - out: `{"type":"heartbeat"}` (a cada ~10s)
  - in/out: `{"type":4}` / `{"type":5}` (controle)

## 4. Modelo de autenticação

1. **Login** (email code) → sessão mtop: `x-sid` + `x-uid` + token Accio (`accessToken` usado no WS).
2. Cada request mtop precisa `x-sign`/`x-mini-wua`/`x-sgext` gerados pelo SecurityGuard
   (biblioteca nativa) sobre: appKey + API + body + timestamp + utdid.
3. O desktop proxy antigo já resolvia assinatura parecida via security bridge — mesma família.

## 5. Anti-tamper observado

- MITM em `phoenix-gw.alibaba.com` (HTTPS non-WS) → o **app se fecha** (crash silencioso).
  Provável: validação de cert/pinning no client via SG para esse host específico.
- MITM em `acs.m.*` (mtop): **tolerado** (requests passam e respondem 200).
- Bloquear o canal binário ACCS (IPs 47.246.x sem SNI): o login **trava e o app morre** —
  o login DEPENDE desse canal nesta versão; não há fallback HTTPS pra login.
- Frida: attach quebra ("closed") em qualquer processo no MuMu (kernel bloqueia ptrace
  de binários traduzidos houdini/arm). Server x86_64 nativo também falha.

## 6. Replay direto do mtop (validado do PC, fora do emulador)

- replay **verbatim** (mesmo x-sign/x-t) → `200 FAIL_SYS_REQUEST_EXPIRED` (sign cobre o timestamp; janela curta)
- sem x-sign → `FAIL_SYS_PROTOPARAM_MISSED`; sign falso → `FAIL_SYS_ILEGEL_SIGN`
- sign clássico HMAC com o secret do yw_1222 → `ILEGEL_SIGN` (o x-sign Android é SG nativo, não HMAC)
- **conclusão**: mtop aceita qualquer IP de origem; só exige sign SG válido (por request)
- addon Windows do desktop (`security_guard.node`) só assina appKey 35336201 (desktop);
  appKey 34959200 (Android) → errorCode 17174. NÃO dá pra assinar mtop Android no Windows hoje.

## 7. appSecret extraído do APK

`res/drawable/yw_1222.jpg` (2 primeiros bytes trocados d8ff→ffd8), após o EOI do JPEG:
pares `appKey → secret` em ASCII. Ex.: `34959200 → 6mycpzudspvor` (não serve pro x-sign,
que é SG nativo; pode servir p/ outras chamadas que usem HMAC clássico).

## 8. ARQUITETURA ESCOLHIDA (2026-09-20)

O WS do phoenix aceita MITM e **externa o accessToken na query string**:

```
wss://phoenix-gw.alibaba.com/ws/vps/app?connectionToken=<hex32>&accessToken=<hex32>
```

- Proxy = cliente WS phoenix-gw com token colhido (sem SG, sem sign)
- Criador de contas = UI automation no emulador (adb input) + colheita do
  accessToken/	connectionToken pela captura MITM quando o app abre o WS pós-login
- Créditos/conta: schema em `AccioMember.xml` (`remainingMultiStepPoints`,
  `trialMaxAgentMultiStepPoints`, `userType`), userId Accio (ex. 1792004888),
  sessão mtop `x-sid`/`x-uid` (blob encriptado em `files/aliusersdk_session_lists`)

## 9. Inventário completo de APIs mtop (extraído do dex do APK v1.42.2)

```
mtop.accio.phoenix.gateway.share.create

# metadados
mtop.alibaba.phoenix.gateway.meta.agent.create/delete/query/update
mtop.alibaba.phoenix.gateway.meta.plugin.query
mtop.alibaba.phoenix.gateway.meta.session.create/delete/query/update
mtop.alibaba.phoenix.gateway.meta.skill.query          # TOOL CALLS / skills

# sandbox VPS (onde as tools executam)
mtop.alibaba.phoenix.gateway.vps.session.prepare.us
mtop.alibaba.phoenix.gateway.vps.session.status.us
mtop.alibaba.phoenix.gateway.vps.session.keepalive.us
mtop.alibaba.phoenix.gateway.vps.session.destroy.us
mtop.alibaba.phoenix.gateway.vps.upload.signature.us

# remote control (PAREAMENTO DESKTOP <-> ANDROID)
mtop.alibaba.phoenix.gateway.rc.command.us
mtop.alibaba.phoenix.gateway.rc.getConnectionToken      # token do WS /ws/vps/app
mtop.alibaba.phoenix.gateway.rc.queryPcDeviceList
mtop.alibaba.phoenix.gateway.rc.queryTrustPairList
mtop.alibaba.phoenix.gateway.rc.requestAuthorization
mtop.alibaba.phoenix.gateway.rc.revokeTrustPair
```

Hosts internos no dex: `phoenix-gw.alibaba.com`, `pre-phoenix-gw.alibaba-inc.com`.

**Conclusão de arquitetura:** o Android expõe tool calls nativas (skills/plugins) rodando
num sandbox VPS, controláveis pelo WS `/ws/vps/app` com `connectionToken` da família `rc.*`.
O fluxo desktop→android (pareamento) usa exatamente essas APIs — o proxy novo replica isso.

## 10. Pendências

- [ ] Confirmar se o CHAT em si cavalga o WS `/ws/vps/app` (frames de resposta) ou se é
      POST phoenix-gw (invisível). Teste: MITM phoenix-gw ativo + mandar mensagem rápida
      e capturar frames antes do anti-tamper matar o app.
- [ ] Medir vida útil do accessToken (se dura dias como o desktop, colheita única por conta).
- [ ] Verificar se o WS sozinho sustenta um chat completo (subscribe/publish de turnos).
