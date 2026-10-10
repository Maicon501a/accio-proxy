//go:build windows

package sgbridge

import (
	"os/exec"
	"syscall"
)

// CREATE_NO_WINDOW (0x08000000) impede que o Windows crie uma janela de
// console para o processo filho. HideWindow sozinho ainda deixa o console
// existir (e piscar); a flag é o que realmente suprime a janela.
const createNoWindow = 0x08000000

// hideWindow keeps the Node sidecar fully headless (no console window at all).
func hideWindow(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
}
