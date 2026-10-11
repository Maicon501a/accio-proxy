//go:build !windows

package official

import "errors"

func getMasterKey() ([]byte, error) {
	return nil, errors.New("safeStorage vault decryption is only supported on Windows")
}
