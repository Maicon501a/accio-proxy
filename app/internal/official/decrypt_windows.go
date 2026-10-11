//go:build windows

package official

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

var (
	crypt32                = syscall.NewLazyDLL("crypt32.dll")
	procCryptUnprotectData = crypt32.NewProc("CryptUnprotectData")
)

type dataBlob struct {
	cbData uint32
	pbData *byte
}

func cryptUnprotectData(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("empty data for cryptUnprotectData")
	}
	inBlob := dataBlob{
		cbData: uint32(len(data)),
		pbData: &data[0],
	}
	var outBlob dataBlob
	r1, _, err := procCryptUnprotectData.Call(
		uintptr(unsafe.Pointer(&inBlob)),
		0,
		0,
		0,
		0,
		0,
		uintptr(unsafe.Pointer(&outBlob)),
	)
	if r1 == 0 {
		return nil, fmt.Errorf("CryptUnprotectData failed: %v", err)
	}
	defer syscall.LocalFree(syscall.Handle(unsafe.Pointer(outBlob.pbData)))

	out := make([]byte, outBlob.cbData)
	copy(out, unsafe.Slice(outBlob.pbData, outBlob.cbData))
	return out, nil
}

// getMasterKey reads Local State and extracts the AES-256 master key on Windows.
func getMasterKey() ([]byte, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		home, _ := os.UserHomeDir()
		appData = filepath.Join(home, "AppData", "Roaming")
	}
	localStatePath := filepath.Join(appData, "Accio", "Local State")
	b, err := os.ReadFile(localStatePath)
	if err != nil {
		return nil, fmt.Errorf("read Local State: %w", err)
	}
	var ls struct {
		OSCrypt struct {
			EncryptedKey string `json:"encrypted_key"`
		} `json:"os_crypt"`
	}
	if err := json.Unmarshal(b, &ls); err != nil {
		return nil, fmt.Errorf("parse Local State: %w", err)
	}
	rawKey, err := base64.StdEncoding.DecodeString(ls.OSCrypt.EncryptedKey)
	if err != nil {
		return nil, fmt.Errorf("decode encrypted_key: %w", err)
	}
	if len(rawKey) < 5 || string(rawKey[:5]) != "DPAPI" {
		return nil, errors.New("invalid encrypted_key DPAPI prefix")
	}
	return cryptUnprotectData(rawKey[5:])
}
