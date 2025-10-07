package mailwatch

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	_ "embed"
	"fmt"
	"io"
)

//go:embed key
var _key []byte

// Encrypt function
func encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(_key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt function
func decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(_key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// The nonce is the first nonceSize bytes of the ciphertext
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Decrypt the data
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}
