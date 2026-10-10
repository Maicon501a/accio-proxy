//go:build !windows

package emulator

import "os/exec"

func hideCmd(cmd *exec.Cmd) {}
