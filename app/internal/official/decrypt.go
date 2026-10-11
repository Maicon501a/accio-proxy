package official

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"fmt"
	"os"
)

// DecryptEncryptedFile decrypts an Electron safeStorage encrypted file (v10 + IV12 + Ciphertext + Tag16).
func DecryptEncryptedFile(path string) ([]byte, error) {
	enc, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(enc) < 3+12+16 {
		return nil, errors.New("file too short for safeStorage encryption")
	}
	if string(enc[:3]) != "v10" {
		return nil, errors.New("invalid safeStorage v10 prefix")
	}
	masterKey, err := getMasterKey()
	if err != nil {
		return nil, err
	}
	iv := enc[3:15]
	ciphertextAndTag := enc[15:]

	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, fmt.Errorf("aes.NewCipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cipher.NewGCM: %w", err)
	}
	plaintext, err := gcm.Open(nil, iv, ciphertextAndTag, nil)
	if err != nil {
		return nil, fmt.Errorf("gcm.Open: %w", err)
	}
	return plaintext, nil
}
