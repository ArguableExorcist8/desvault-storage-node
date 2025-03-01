package storage

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

// EncryptData encrypts plaintext using AES-GCM with the provided key.
// It returns a byte slice with the nonce prepended to the ciphertext.
// The key must be 16, 24, or 32 bytes long corresponding to AES-128, AES-192, or AES-256 respectively.
// In a production system, manage and rotate your encryption keys securely.
func EncryptData(plaintext, key []byte) ([]byte, error) {
	keyLen := len(key)
	if keyLen != 16 && keyLen != 24 && keyLen != 32 {
		return nil, fmt.Errorf("invalid key length: %d bytes (must be 16, 24, or 32)", keyLen)
	}

	// Create a new AES cipher block.
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	// Create a GCM instance.
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate a secure random nonce.
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Seal encrypts and authenticates the plaintext.
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	// Prepend the nonce to the ciphertext.
	return append(nonce, ciphertext...), nil
}

// DecryptData decrypts data that was encrypted with EncryptData.
// It expects the nonce to be prepended to the ciphertext.
// The key must be the same length as used during encryption.
func DecryptData(data, key []byte) ([]byte, error) {
	keyLen := len(key)
	if keyLen != 16 && keyLen != 24 && keyLen != 32 {
		return nil, fmt.Errorf("invalid key length: %d bytes (must be 16, 24, or 32)", keyLen)
	}

	// Create a new AES cipher block.
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	// Create a GCM instance.
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("encrypted data too short")
	}

	// Extract nonce and ciphertext.
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt data: %w", err)
	}
	return plaintext, nil
}
