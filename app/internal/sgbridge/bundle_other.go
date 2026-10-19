//go:build !windows

package sgbridge

import "errors"

func extractEmbeddedBundle(dstDir string) error {
	return errors.New("embedded security-guard bundle is only available on Windows")
}

func hasEmbeddedBundle() bool {
	return false
}
