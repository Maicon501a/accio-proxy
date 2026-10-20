//go:build windows

package config

// platformDefaultMode: no Windows o app oficial do Accio existe, então o modo
// oficial (sessão dele + Basic Auth local) é o caminho recomendado.
func platformDefaultMode() string { return "official" }

// platformSupportsOfficial indica se o modo oficial tem como funcionar aqui
// (depende do Accio Desktop + DPAPI para o vault .enc).
func platformSupportsOfficial() bool { return true }
