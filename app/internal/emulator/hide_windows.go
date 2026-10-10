//go:build windows

package emulator

import (
	"os/exec"
	"syscall"
)

// CREATE_NO_WINDOW (0x08000000) impede a criação da janela de console do
// adb.exe — sem isso, cada chamada do Health() pisca um terminal na tela.
const createNoWindow = 0x08000000

func hideCmd(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
}
