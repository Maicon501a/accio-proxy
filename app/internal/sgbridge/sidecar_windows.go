//go:build windows

package sgbridge

// sidecarDefault mantém no Windows o caminho já validado: o bundle nativo está
// embutido no binário e o addon devolve a assinatura SG real. A camada Go pura
// fica como fallback se o bundle não existir ou o Node faltar.
func sidecarDefault() bool { return true }
