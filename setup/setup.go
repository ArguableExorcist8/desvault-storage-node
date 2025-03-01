package setup

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Config holds basic configuration settings.
type Config struct {
	Region            string       `json:"region"`
	WalletConfig      WalletConfig `json:"walletConfig"`
	StorageAllocation int          `json:"storageAllocation"`
	// You could add endpoints, database settings, etc.
}

// WalletConfig holds configuration for wallet authentication.
type WalletConfig struct {
	APIKey string `json:"apiKey"`
}

// LoadConfig loads configuration from "config.json" if available,
// otherwise falls back to defaults or environment variables.
func LoadConfig() (*Config, error) {
	f, err := os.Open("config.json")
	if err != nil {
		// Fallback to defaults if config file is not found.
		return &Config{
			Region:            "us-east-1",
			WalletConfig:      WalletConfig{APIKey: os.Getenv("WALLET_API_KEY")},
			StorageAllocation: 100,
		}, nil
	}
	defer f.Close()
	decoder := json.NewDecoder(f)
	var config Config
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("error decoding config: %v", err)
	}
	return &config, nil
}

var startTime time.Time

// SetStartTime records the node's start time.
func SetStartTime() error {
	startTime = time.Now()
	return nil
}

// GetStartTimeOrNow returns the start time or, if not set, the current time.
func GetStartTimeOrNow() time.Time {
	if startTime.IsZero() {
		return time.Now()
	}
	return startTime
}

// GetRegion returns the node region.
func GetRegion() string {
	return "us-east-1"
}

// GetUptime returns the elapsed time since the node started.
func GetUptime() string {
	if startTime.IsZero() {
		return "0s"
	}
	return time.Since(startTime).String()
}

// ReadStorageAllocation returns the allocated storage (in GB).
func ReadStorageAllocation() (int, error) {
	// In production, read from a persistent store or environment variable.
	return 100, nil
}

// SetStorageAllocation updates the storage allocation.
func SetStorageAllocation(newAlloc int) error {
	// In production, persist this change.
	fmt.Printf("[INFO] Storage allocation updated to %d GB\n", newAlloc)
	return nil
}

// FirstTimeSetup performs any initial setup tasks.
func FirstTimeSetup() {
	fmt.Println("[INFO] Performing first-time setup...")
	// For example, create necessary directories or default config files.
}
