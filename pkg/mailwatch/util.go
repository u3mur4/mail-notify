package mailwatch

import (
	_ "embed"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"os"
)

func copyFile(src string, dst string) error {
	// Open the source file
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	// Create the destination file
	destinationFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destinationFile.Close()

	// Copy the file content
	_, err = io.Copy(destinationFile, sourceFile)
	if err != nil {
		return err
	}

	// Optionally, you can also copy the file permissions from the source file
	srcInfo, err := sourceFile.Stat()
	if err != nil {
		return err
	}
	err = os.Chmod(dst, srcInfo.Mode())
	if err != nil {
		return err
	}

	return nil
}

//go:embed key
var	_key []byte

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
