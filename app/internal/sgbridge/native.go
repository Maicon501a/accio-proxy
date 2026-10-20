package sgbridge

// Camada nativa: substitui o sidecar Node + SecurityGuard no caminho de chat.
//
// O binário proprietário (security_guard.node -> AliSafeProxy.dll /
// SecurityGuardSDK64.dll) é uma binding N-API fina que expõe só três
// operações:
//
//	initUmid(env)               inicializa o UMID do device
//	getSecurityFactorsForWeb()  devolve o mapa de headers pctb-* do request
//	urlSign / getSecurityToken  NÃO implementados no binding Windows
//
// Interceptando a saída real do addon (mesma URL, chamadas seguidas) o mapa
// tem 13 chaves, sendo 4 criptográficas (pctb-x-umt, pctb-x-mini-wua,
// pctb-x-sign, pctb-x-sgext) e o resto metadado estático de device.
//
// O que os testes contra o gateway mostraram (appKey desktop 35336201, utdid
// novo gerado na hora, 15 requests consecutivos, com tools, em DeepSeek /
// Claude / Gemini):
//
//   - request sem nenhum header pctb-*              -> 200, stream completo
//   - request com os pctb-* forjados de outra URL   -> 200, stream completo
//   - request só com os metadados (sem cripto)      -> 200, stream completo
//   - featureFlag/evaluate sem assinatura           -> 200
//
// Ou seja: o WAF do phoenix-gw.alibaba.com não valida a assinatura do
// SecurityGuard para o appKey desktop. Os campos SG são telemetria
// antifraude; quem autoriza a request é o accessToken do corpo + os headers
// de device (utdid/appKey/version).
//
// Por isso esta camada reproduz o *contrato* do módulo — o mapa de headers de
// device — sem reimplementar o SG. É o que torna o Modo Direto independente de
// binários win32-x64: no Linux o Node exige ELF e aborta ao carregar o .node
// PE, e esse era o único motivo do Modo Direto não funcionar lá.

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// bxVersion é a versão do SDK Baxia que o Accio desktop 0.33.0 reporta.
const bxVersion = "pc_1.0.70.3061_0"

// nativeSigner gera o header set de device que o desktop envia junto do chat.
type nativeSigner struct{ appKey string }

func newNativeSigner(appKey string) *nativeSigner {
	if appKey == "" {
		appKey = DesktopAppKey
	}
	return &nativeSigner{appKey: appKey}
}

// Headers devolve os headers de device para uma request. Nunca falha: o
// gateway aceita a request mesmo sem eles.
func (n *nativeSigner) Headers(_ context.Context, rawURL string) (map[string]string, error) {
	return map[string]string{
		"pctb-x-sgext":      encodeURIComponent("win_reserve"),
		"pctb-x-pv":         encodeURIComponent("6.3"),
		"pctb-x-api":        encodeURIComponent(apiPath(rawURL)),
		"pctb-x-apiver":     encodeURIComponent("1.0"),
		"pctb-x-t":          encodeURIComponent(fmt.Sprintf("%d", time.Now().Unix())),
		"pctb-x-appkey":     encodeURIComponent(n.appKey),
		"x-bx-env":          encodeURIComponent("web"),
		"pctb-x-bx-version": encodeURIComponent(bxVersion),
	}, nil
}

// apiPath reproduz o valor de pctb-x-api do desktop: host + path, sem query.
func apiPath(rawURL string) string {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Host == "" {
		return rawURL
	}
	return u.Host + u.Path
}

// encodeURIComponent replica encodeURIComponent do JS — é o que o
// sg_daemon.js aplica em cada valor antes de devolver ao lado Go, então o
// header sai byte a byte igual ao caminho Windows.
func encodeURIComponent(s string) string {
	const unreserved = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_.!~*'()"
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if strings.IndexByte(unreserved, c) >= 0 {
			b.WriteByte(c)
			continue
		}
		fmt.Fprintf(&b, "%%%02X", c)
	}
	return b.String()
}
