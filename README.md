# Accio Proxy

<p align="center">
  <strong>Gateway OpenAI-compatível de alta performance com arquitetura Dual-Mode para o ecossistema Accio Work / Alibaba Phoenix.</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go" alt="Go Version" />
  <img src="https://img.shields.io/badge/Wails-v2.13-DF1A5A?style=flat" alt="Wails" />
  <img src="https://img.shields.io/badge/React-19-61DAFB?style=flat&logo=react" alt="React" />
  <img src="https://img.shields.io/badge/TypeScript-5.6-3178C6?style=flat&logo=typescript" alt="TypeScript" />
  <img src="https://img.shields.io/badge/OpenAI-Compatible-412991?style=flat&logo=openai" alt="OpenAI API" />
  <img src="https://img.shields.io/badge/License-MIT-green.svg" alt="License" />
</p>

---

## 📌 Visão Geral

O **Accio Proxy** transforma contas e créditos do aplicativo **Accio Work Desktop** em um endpoint HTTP compatível com a API padrão do **OpenAI** (`/v1/chat/completions` e `/v1/models`). 

Com ele, você pode conectar clientes como **OpenCode**, **Cursor**, **Cline**, **Continue.dev** ou qualquer biblioteca padrão (`@ai-sdk/openai-compatible`, `langchain`, etc.) diretamente aos modelos mais avançados de IA (GPT-6, GPT-5.6, Claude Opus 5.5, Claude Sonnet 5, Gemini 3.8 Flash, DeepSeek V4 Flash, GLM 5.3, entre outros), usufruindo da quota das suas contas sem precisar usar a interface padrão do aplicativo desktop.

---

## ⚡ Modos de Operação (Dual-Mode Architecture)

O proxy disponibiliza um botão alternador em tempo real (na sidebar e no dashboard) entre dois modos de funcionamento:

```
┌────────────────────────────────────────────────────────────────────────┐
│                              ACCIO PROXY                               │
├──────────────────────────────────┬─────────────────────────────────────┤
│   Modo 1: App Oficial (Ativo)    │  Modo 2: Standalone / Direto        │
│   • Conexão local direta (:4097) │  • Emulador standalone sgbridge     │
│   • Sessão e assinatura oficial  │  • Pool local SQLite                │
│   • 100% Estável e seguro        │  ⚠️ INSTÁVEL: Alto risco WAF (1-3d) │
└──────────────────────────────────┴─────────────────────────────────────┘
```

### 1. Modo App Oficial (`official`) — 🟢 Recomendado & Estável
* **Como funciona:** O proxy se conecta dinamicamente ao processo do aplicativo oficial do Accio (`Accio.exe`) em execução no Windows via Hono Gateway local (descoberta automática de porta e Basic Auth via `gateway-cli.json`).
* **Captura de Contas em Tempo Real:** Lê e descriptografa nativamente no Windows (via DPAPI `CryptUnprotectData` + AES-256-GCM) os tokens de sessão (`accessToken`) das contas autenticadas.
* **Alternância de Contas com 1 Clique:** Na interface do proxy, você visualiza todas as contas salvas, seus saldos de créditos reais e pode alternar a conta ativa instantaneamente via `POST /auth/accounts/:accountId/switch`.
* **Isolamento de Webview (Privacidade & Discrição):** As requisições do seu OpenCode/Cursor são assinadas pela sessão oficial e despachadas para o gateway de inferência em nuvem (`phoenix-gw.alibaba.com`). Como não passam pelo banco local SQLite de conversas nem pelo event bus da UI do desktop, **as conversas NÃO aparecem na interface do app oficial**, gastando apenas os créditos de forma transparente.

---

### 2. Modo Endpoints Diretos / Standalone (`direct`) — ⚠️ Alerta Crítico

> [!WARNING]
> ### AVISO CRÍTICO DE INSTABILIDADE E PENALIZAÇÃO DE BLOQUEIO
> A réplica standalone sem dependência do aplicativo oficial em execução opera através de emulação reversa de assinaturas `sgbridge` e pool SQLite local.
> 
> **Este modo encontra-se instável e apresenta problemas severos:**
> 1. **Detecção WAF / Anti-Bot:** Os sistemas de segurança e WAF da Alibaba / SecurityGuard monitoram com rigor discrepâncias de ambiente e telemetria de dispositivo (`x-sg-wua`, device tokens, TLS fingerprints). Requisições diretas fora da runtime oficial possuem alta taxa de flagging.
> 2. **Risco de Bloqueio Severo:** Quando detectadas anomalias pelo WAF, a conta ou dispositivo sofre **penalizações de bloqueio imediato (hard rate-limit gate 555) variando de 1 a 3 dias** sem poder realizar chamadas ou logar.
> 
> **Recomendação:** Para desenvolvimento contínuo, estabilidade garantida e proteção contra banimentos de conta, **mantenha sempre o aplicativo oficial Accio aberto e utilize o Modo App Oficial**.

---

## 🧠 Esforços de Raciocínio (Reasoning Efforts)

Uma análise aprofundada dos modelos e da API oficial revela diferenças importantes em relação ao suporte de raciocínio (*Thinking / Reasoning Content*):

### Modelos Sem Suporte a Esforço de Raciocínio (`reasoning: false`)
Modelos como **DeepSeek V4 Flash** e **GLM 5.3** **NÃO aceitam parâmetros de esforço de raciocínio** (`reasoningEfforts: null` na API oficial). Eles operam em modo direto / streaming contínuo. 

> [!IMPORTANT]
> Se o cliente enviar parâmetros como `reasoning_effort` ou `thinking_budget` para o DeepSeek ou GLM, a chamada pode ser rejeitada pela API. O Accio Proxy sanitiza automaticamente as requisições para esses modelos, e a configuração recomendada para o OpenCode é explicitamente `"reasoning": false`.

### Modelos Com Suporte a Esforço de Raciocínio (`reasoning: true`)
Estes modelos suportam raciocínio configurável (`low`, `medium`, `high`, `xhigh`, `max`):
* **GPT-6 Astra:** `low`, `medium`, `high`, `xhigh`, `max` (padrão: `high`)
* **GPT-5.6 (Luna, Terra, Sol) & GPT-6 (Luna, Sol):** `low`, `medium`, `high` (padrão: `high`)
* **Claude (Opus 5, Opus 5.5, Opus 4.8, Opus 4.7, Sonnet 5, Sonnet 4.6, Fable 5, Fable 5.1):** `low`, `medium`, `high`, `max`
* **Claude Haiku 4.5:** `low`, `medium`, `high`
* **Gemini (3.8 Flash, 3.7 Flash, 3.5 Flash, 3 Flash, 3.1 Pro):** `low`, `high`

---

## 🗂️ Catálogo de Modelos

O Accio Proxy expõe automaticamente todos os modelos disponíveis via endpoint `/v1/models` e na tela dedicada **Modelos (`❖`)**:

| ID no Proxy (OpenCode / API) | Nome Oficial | Provedor | Raciocínio | Contexto | Modalidades | Custo |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| `deepseek-v4-flash` | DeepSeek V4 Flash | DeepSeek | ❌ Direto | 1.000.000 | Texto + Visão | Grátis |
| `deepseek-v4-pro` | DeepSeek V4 Pro | DeepSeek | ❌ Direto | 1.000.000 | Apenas Texto | 0.34x |
| `glm-5.3` | GLM 5.3 | Zhipu | ❌ Direto | 1.000.000 | Apenas Texto | 0.16x |
| `glm-5.3-flash` | GLM 5.3 Flash | Zhipu | ❌ Direto | 1.000.000 | Texto + Visão | Grátis |
| `gpt-6-astra` | GPT 6 Astra | OpenAI | ✅ low..max | 1.050.000 | Texto + Visão | 3.50x |
| `gpt-6-luna` | GPT 6 Luna | OpenAI | ✅ low..high | 1.050.000 | Texto + Visão | 0.04x |
| `gpt-6-sol` | GPT 6 Sol | OpenAI | ✅ low..high | 1.050.000 | Texto + Visão | 0.70x |
| `gpt-5.6-luna` | GPT 5.6 Luna | OpenAI | ✅ low..high | 1.050.000 | Texto + Visão | Grátis |
| `gpt-5.6-terra` | GPT 5.6 Terra | OpenAI | ✅ low..high | 1.050.000 | Texto + Visão | Grátis |
| `gpt-5.6-sol` | GPT 5.6 Sol | OpenAI | ✅ low..high | 1.050.000 | Texto + Visão | Grátis |
| `claude-opus-5.5` | Claude Opus 5.5 | Anthropic | ✅ low..max | 1.000.000 | Texto + Visão | 1.33x |
| `claude-opus-5` | Claude Opus 5 | Anthropic | ✅ low..max | 1.000.000 | Texto + Visão | 2.80x |
| `claude-sonnet-5` | Claude Sonnet 5 | Anthropic | ✅ low..max | 1.000.000 | Texto + Visão | Grátis |
| `claude-sonnet-4.6` | Claude Sonnet 4.6 | Anthropic | ✅ low..max | 1.000.000 | Texto + Visão | Grátis |
| `gemini-3.8-flash` | Gemini 3.8 Flash | Google | ✅ low, high | 1.000.000 | Texto + Visão | Grátis |
| `gemini-3.7-flash` | Gemini 3.7 Flash | Google | ✅ low, high | 1.000.000 | Texto + Visão | Grátis |
| `qwen-3.8-max` | Qwen 3.8 Max | Alibaba | ❌ Direto | 1.000.000 | Texto + Visão | 0.44x |
| `qwen-3.8-flash` | Qwen 3.8 Flash | Alibaba | ❌ Direto | 1.000.000 | Texto + Visão | Grátis |
| `kimi-k2.6` | Kimi K2.6 | Moonshot | ❌ Direto | 256.000 | Texto + Visão | 0.14x |
| `minimax-m3` | MiniMax M3 | MiniMax | ❌ Direto | 1.000.000 | Texto + Visão | 0.08x |

*(Consulte a lista completa com os 37 modelos na aba **Modelos** do aplicativo).*

---

## 🛠️ Configuração no OpenCode

Adicione ou atualize o provedor `accio` no seu arquivo `~/.config/opencode/opencode.json`:

```json
{
  "provider": {
    "accio": {
      "name": "Accio Proxy",
      "npm": "@ai-sdk/openai-compatible",
      "options": {
        "baseURL": "http://127.0.0.1:8790/v1",
        "apiKey": "accio-local",
        "timeout": 600000,
        "headerTimeout": 60000
      },
      "models": {
        "deepseek-v4-flash": {
          "name": "DeepSeek V4 Flash",
          "attachment": true,
          "reasoning": false,
          "supportsImages": true,
          "tool_call": true,
          "limit": { "context": 1000000, "output": 64000 }
        },
        "glm-5.3": {
          "name": "GLM 5.3",
          "attachment": true,
          "reasoning": false,
          "supportsImages": false,
          "tool_call": true,
          "limit": { "context": 1000000, "output": 64000 }
        },
        "gpt-6-astra": {
          "name": "GPT-6 Astra",
          "attachment": true,
          "reasoning": true,
          "supportsImages": true,
          "tool_call": true,
          "limit": { "context": 1050000, "output": 64000 }
        },
        "claude-opus-5.5": {
          "name": "Claude Opus 5.5",
          "attachment": true,
          "reasoning": true,
          "supportsImages": true,
          "tool_call": true,
          "limit": { "context": 1000000, "output": 64000 }
        },
        "claude-sonnet-5": {
          "name": "Claude Sonnet 5",
          "attachment": true,
          "reasoning": true,
          "supportsImages": true,
          "tool_call": true,
          "limit": { "context": 1000000, "output": 64000 }
        },
        "gemini-3.8-flash": {
          "name": "Gemini 3.8 Flash",
          "attachment": true,
          "reasoning": true,
          "supportsImages": true,
          "tool_call": true,
          "limit": { "context": 1000000, "output": 64000 }
        }
      }
    }
  }
}
```

---

## 🚀 Como Compilar e Executar

### Pré-requisitos
* **Go** 1.23+
* **Node.js** 20+ e **npm**
* **Wails CLI v2**:
  ```bash
  go install github.com/wailsapp/wails/v2/cmd/wails@latest
  ```

### Build do Executável
No diretório `app`:

```bash
cd app

# Instalar dependências do frontend
cd frontend
npm install
cd ..

# Gerar o executável nativo do Windows
wails build
```

O binário final será gerado em:
```
app/build/bin/app.exe
```

---

## 🔒 Segurança e Privacidade

* **Local First:** Nenhuma credencial, cookie ou token de sessão é enviado para servidores de terceiros.
* **Criptografia DPAPI:** O acesso aos tokens no Windows utiliza a API nativa do sistema (`CryptUnprotectData`), sem ferramentas de terceiros.
* **Isolamento de Conversas:** Suas conversas no OpenCode/Cursor continuam 100% isoladas da interface do aplicativo Accio oficial.

---

## 📄 Licença

Distribuído sob a licença MIT. Consulte `LICENSE` para mais informações.
