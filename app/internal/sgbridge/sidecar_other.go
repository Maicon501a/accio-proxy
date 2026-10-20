//go:build !windows

package sgbridge

// sidecarDefault: fora do Windows o sidecar nunca é o default. O addon
// security_guard.node é um PE win32-x64; o Node no Linux exige ELF e aborta no
// dlopen. Como o gateway não valida a assinatura SG do appKey desktop (ver
// native.go), o caminho nativo em Go é o correto — e é o que faz o Modo Direto
// funcionar no Linux. Quem quiser forçar o sidecar usa ACCIO_SG_SIDECAR=1.
func sidecarDefault() bool { return false }
