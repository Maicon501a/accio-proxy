//go:build !windows

package config

// platformDefaultMode: fora do Windows não existe Accio Desktop nem DPAPI, e o
// único caminho viável é o direto (pool local + login OAuth nativo no browser).
// Sem isso o app abriria em "official" e toda request falharia.
func platformDefaultMode() string { return "direct" }

// platformSupportsOfficial indica se o modo oficial tem como funcionar aqui.
func platformSupportsOfficial() bool { return false }
