//go:build windows

// decrypt-accio descriptografa os .enc do app Accio (Electron safeStorage):
// Local State os_crypt.encrypted_key (DPAPI) -> AES-256-GCM -> tokens.
package main

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

var (
	dllCrypt32   = syscall.NewLazyDLL("crypt32.dll")
	dllKernel32   = syscall.NewLazyDLL("kernel32.dll")
	procUnprotect = dllCrypt32.NewProc("CryptUnprotectData")
	procLocalFree = dllKernel32.NewProc("LocalFree")
)

type dataBlob struct {
	cbData uint32
	pbData *byte
}

func dpapiUnprotect(data []byte) ([]byte, error) {
	var in, out dataBlob
	if len(data) > 0 {
		in = dataBlob{cbData: uint32(len(data)), pbData: &data[0]}
	}
	r, _, err := procUnprotect.Call(
		uintptr(unsafe.Pointer(&in)), 0, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(&out)),
	)
	if r == 0 {
		return nil, err
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))
	return unsafe.Slice(out.pbData, out.cbData), nil
}

func decryptGCM(key, data []byte) ([]byte, error) {
	// formato safeStorage v10: "v10" || nonce(12) || ciphertext+tag
	if len(data) < 3+12+16 {
		return nil, fmt.Errorf("curto demais: %d", len(data))
	}
	nonce := data[3:15]
	ct := data[15:]
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, nonce, ct, nil)
}

func main() {
	fmt.Fprintln(os.Stderr, "[*] iniciando")
	localStatePath := os.Getenv("APPDATA") + `\accio\Local State`
	raw, err := os.ReadFile(localStatePath)
	if err != nil {
		panic(err)
	}
	fmt.Fprintln(os.Stderr, "[*] local state lido")
	var ls struct {
		OsCrypt struct {
			EncryptedKey string `json:"encrypted_key"`
		} `json:"os_crypt"`
	}
	if err := json.Unmarshal(raw, &ls); err != nil {
		panic(err)
	}
	ekRaw, err := base64.StdEncoding.DecodeString(ls.OsCrypt.EncryptedKey)
	if err != nil {
		panic(err)
	}
	key, err := dpapiUnprotect(ekRaw[5:]) // remove "DPAPI"
	if err != nil {
		panic(fmt.Errorf("dpapi: %w", err))
	}
	fmt.Printf("chave AES: %d bytes\n", len(key))

	for _, f := range []string{"remembered-accounts.enc", "credentials.enc"} {
		p := os.Getenv("APPDATA") + `\accio\` + f
		data, err := os.ReadFile(p)
		if err != nil {
			fmt.Printf("%s: %v\n", f, err)
			continue
		}
		plain, err := decryptGCM(key, data)
		if err != nil {
			fmt.Printf("%s: gcm: %v\n", f, err)
			continue
		}
		fmt.Printf("=== %s ===\n%s\n", f, plain)
	}
}
