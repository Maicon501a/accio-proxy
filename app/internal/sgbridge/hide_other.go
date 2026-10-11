//go:build !windows

package sgbridge

import "os/exec"

func hideWindow(cmd *exec.Cmd) {}
