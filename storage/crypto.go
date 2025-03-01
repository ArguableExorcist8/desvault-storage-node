package storage

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ============================================================================
// KeyManager: Manages Encryption Keys and Supports Rotation
// ============================================================================

// KeyManager manages encryption keys and persists them in a JSON file.
type KeyManager struct {
	Active string            `json:"active"` // Active key version identifier.
	Keys   map[string]string `json:"keys"`   // Mapping of key version to hex-encoded key.
	File   string            `json:"-"`      // File where keys are stored.
	mu     sync.Mutex        `json:"-"`
}

// NewKeyManager returns a new KeyManager instance for the given file.
// If the file exists, it loads the keys; otherwise, it creates a new key file with an initial key.
func NewKeyManager(file string) (*KeyManager, error) {
	km := &KeyManager{
		Keys: make(map[string]string),
		File: file,
	}
	// Check if the key file exists.
	if _, err := os.Stat(file); os.IsNotExist(err) {
		// File does not exist; create a new key.
		newKey := make([]byte, 32) // 32 bytes for AES-256.
		if _, err := rand.Read(newKey); err != nil {
			return nil, fmt.Errorf("failed to generate new key: %w", err)
		}
		km.Keys["v1"] = hex.EncodeToString(newKey)
		km.Active = "v1"
		if err := km.Save(); err != nil {
			return nil, fmt.Errorf("failed to save new key file: %w", err)
		}
		log.Printf("[INFO] New key file created with active key version 'v1'")
		return km, nil
	}

	// File exists; load the keys.
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}
	if err := json.Unmarshal(data, km); err != nil {
		return nil, fmt.Errorf("failed to unmarshal key file: %w", err)
	}
	log.Printf("[INFO] Key file loaded with active key version '%s'", km.Active)
	return km, nil
}

// Save persists the KeyManager state to disk.
func (km *KeyManager) Save() error {
	km.mu.Lock()
	defer km.mu.Unlock()
	data, err := json.MarshalIndent(km, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal key manager: %w", err)
	}
	return ioutil.WriteFile(km.File, data, 0644)
}

// GetActiveKey returns the active key version and its raw bytes.
func (km *KeyManager) GetActiveKey() (string, []byte, error) {
	km.mu.Lock()
	defer km.mu.Unlock()
	if km.Active == "" {
		return "", nil, fmt.Errorf("no active key found")
	}
	keyHex, ok := km.Keys[km.Active]
	if !ok {
		return "", nil, fmt.Errorf("active key version %s not found", km.Active)
	}
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return "", nil, fmt.Errorf("failed to decode active key: %w", err)
	}
	return km.Active, key, nil
}

// GetKey returns the raw bytes for the key associated with the provided version.
func (km *KeyManager) GetKey(version string) ([]byte, error) {
	km.mu.Lock()
	defer km.mu.Unlock()
	keyHex, ok := km.Keys[version]
	if !ok {
		return nil, fmt.Errorf("key version %s not found", version)
	}
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, fmt.Errorf("failed to decode key for version %s: %w", version, err)
	}
	return key, nil
}

// RotateKey adds a new key under the specified version and marks it as active.
func (km *KeyManager) RotateKey(version string, newKey []byte) error {
	km.mu.Lock()
	defer km.mu.Unlock()
	l := len(newKey)
	if l != 16 && l != 24 && l != 32 {
		return fmt.Errorf("invalid key length: %d bytes (must be 16, 24, or 32)", l)
	}
	km.Keys[version] = hex.EncodeToString(newKey)
	km.Active = version
	return km.Save()
}

// GetDefaultKeyManager returns a KeyManager loaded from a default file in the storage directory.
func GetDefaultKeyManager() (*KeyManager, error) {
	storageDir := GetStorageDir()
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}
	keyFile := filepath.Join(storageDir, "keys.json")
	return NewKeyManager(keyFile)
}

// ============================================================================
// Encryption/Decryption Functions Using KeyManager
// ============================================================================

// EncryptDataWithKeyManager encrypts plaintext using the active key from the KeyManager.
// The returned ciphertext is prefixed with the active key version and a colon.
// Example format: "v1:<hex-ciphertext>"
func EncryptDataWithKeyManager(km *KeyManager, plaintext []byte) ([]byte, error) {
	version, key, err := km.GetActiveKey()
	if err != nil {
		return nil, err
	}
	ciphertext, err := encrypt(plaintext, key)
	if err != nil {
		return nil, err
	}
	// Prepend version and colon.
	return []byte(fmt.Sprintf("%s:%s", version, hex.EncodeToString(ciphertext))), nil
}

// DecryptDataWithKeyManager decrypts data that was encrypted using EncryptDataWithKeyManager.
// It expects the ciphertext to be prefixed with the key version and a colon.
func DecryptDataWithKeyManager(km *KeyManager, data []byte) ([]byte, error) {
	parts := strings.SplitN(string(data), ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid encrypted data format")
	}
	version := parts[0]
	ciphertextHex := parts[1]
	ciphertext, err := hex.DecodeString(ciphertextHex)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ciphertext: %w", err)
	}
	key, err := km.GetKey(version)
	if err != nil {
		return nil, err
	}
	return decrypt(ciphertext, key)
}

// ============================================================================
// Core AES-GCM Encryption/Decryption Functions
// ============================================================================

// encrypt performs AES-GCM encryption on plaintext using the provided key.
// It returns the nonce concatenated with the ciphertext.
func encrypt(plaintext, key []byte) ([]byte, error) {
	if l := len(key); l != 16 && l != 24 && l != 32 {
		return nil, fmt.Errorf("invalid key length: %d bytes", l)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM mode: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// decrypt performs AES-GCM decryption on data using the provided key.
// It expects the nonce to be prepended to the ciphertext.
func decrypt(data, key []byte) ([]byte, error) {
	if l := len(key); l != 16 && l != 24 && l != 32 {
		return nil, fmt.Errorf("invalid key length: %d bytes", l)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM mode: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("encrypted data too short")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %w", err)
	}
	return plaintext, nil
}
