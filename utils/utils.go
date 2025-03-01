package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/google/uuid"
)

// GenerateSecureToken generates a cryptographically secure random token of the specified length (in bytes)
// and returns it as a hex-encoded string.
func GenerateSecureToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate secure token: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// GenerateCID generates a 16-digit Content Identifier (CID) for simplicity.
// In production, this could be integrated with IPFS or another CID system.
func GenerateCID() (string, error) {
	bytes := make([]byte, 8) // 8 bytes = 16 hex digits
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate CID: %w", err)
	}
	return fmt.Sprintf("%016x", bytes), nil
}

// GetNodeID returns a persistent node identifier.
// It checks for a "node_id" file in the current directory; if it doesn't exist,
// it generates a new UUID, writes it to the file, and returns it.
func GetNodeID() string {
	const idFile = "node_id"
	data, err := os.ReadFile(idFile)
	if err == nil {
		return strings.TrimSpace(string(data))
	}

	// If file doesn't exist or can't be read, generate a new UUID.
	newID := uuid.New().String()
	if err := os.WriteFile(idFile, []byte(newID), 0644); err != nil {
		log.Printf("[!] Failed to write node_id file: %v", err)
	}
	return newID
}
