package setup

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// WalletConfig holds configuration for wallet authentication.
type WalletConfig struct {
	APIKey string `json:"apiKey"`
}

// Config holds basic configuration settings.
type Config struct {
	Region            string       `json:"region"`
	WalletConfig      WalletConfig `json:"walletConfig"`
	StorageAllocation int          `json:"storageAllocation"` // in GB
}

// File paths for configuration and start time.
var (
	configPath    = expandPath("~/.desvault/config.json")
	startTimeFile = expandPath("~/.desvault/start_time")
)

// expandPath expands a "~" to the user's home directory.
func expandPath(path string) string {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("Failed to get user home directory: %v", err)
		}
		return filepath.Join(home, path[1:])
	}
	return path
}

// LoadConfig loads configuration from the JSON config file.
// If the config file does not exist, it triggers FirstTimeSetup.
func LoadConfig() (*Config, error) {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		FirstTimeSetup()
	}
	f, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer f.Close()

	decoder := json.NewDecoder(f)
	var config Config
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("error decoding config file: %w", err)
	}
	return &config, nil
}

// FirstTimeSetup performs initial configuration if the config file is missing.
func FirstTimeSetup() {
	// Ensure the configuration directory exists.
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		log.Fatalf("Failed to create config directory: %v", err)
	}

	// Display a banner and prompt the user.
	fmt.Println(`
██████╗ ███████╗███████╗██╗   ██╗ █████╗ ██╗   ██╗██╗  ████████╗
██╔══██╗██╔════╝██╔════╝██║   ██║██╔══██╗██║   ██║██║  ╚══██╔══╝
██║  ██║█████╗  ███████╗██║   ██║███████║██║   ██║██║     ██║   
██║  ██║██╔══╝  ╚════██║╚██╗ ██╔╝██╔══██║██║   ██║██║     ██║   
██████╔╝███████╗███████║ ╚████╔╝ ██║  ██║╚██████╔╝███████╗██║   
╚═════╝ ╚══════╝╚══════╝  ╚═══╝  ╚═╝  ╚═╝ ╚═════╝ ╚══════╝╚═╝`)
	fmt.Println("[!] First-time setup detected")

	var storageGB int
	fmt.Print("[?] Enter storage allocation in GB (minimum 5 GB and no more than half your free space): ")
	fmt.Scan(&storageGB)

	// Enforce a minimum allocation.
	if storageGB < 5 {
		log.Fatalf("[!] Storage allocation must be at least 5 GB.")
	}

	// Validate free disk space.
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Could not determine home directory: %v", err)
	}
	freeBytes, err := GetFreeDiskSpace(home)
	if err != nil {
		log.Fatalf("Could not determine free disk space: %v", err)
	}
	freeGB := int(freeBytes / (1024 * 1024 * 1024))
	maxAllowed := freeGB / 2
	if storageGB > maxAllowed {
		log.Fatalf("You cannot allocate more than %d GB (half of your free space of %d GB).", maxAllowed, freeGB)
	}

	estimatedPoints := CalculatePoints(storageGB)
	// Create a default configuration.
	config := Config{
		Region:            "us-east-1", // default region; adjust as needed
		WalletConfig:      WalletConfig{APIKey: "default-api-key"},
		StorageAllocation: storageGB,
	}

	// Save the configuration to file in JSON format.
	configFile, err := os.Create(configPath)
	if err != nil {
		log.Fatalf("Failed to create config file: %v", err)
	}
	defer configFile.Close()
	encoder := json.NewEncoder(configFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(&config); err != nil {
		log.Fatalf("Failed to encode config to file: %v", err)
	}

	fmt.Printf("[+] Config saved to %s\n", configPath)
	fmt.Printf("Region: %s\nAllocated Storage: %d GB\nEstimated Rewards: %d pts/hour\n",
		config.Region, storageGB, estimatedPoints)
}

// CalculatePoints returns estimated reward points based on the allocated storage.
func CalculatePoints(storageGB int) int {
	return storageGB * 100 // Example: 100 points per GB.
}

// GetFreeDiskSpace returns the available disk space (in bytes) for the given path.
func GetFreeDiskSpace(path string) (uint64, error) {
	var stat syscall.Statfs_t
	err := syscall.Statfs(path, &stat)
	if err != nil {
		return 0, err
	}
	// Available blocks * size per block equals free space in bytes.
	return stat.Bavail * uint64(stat.Bsize), nil
}

// SetStorageAllocation updates the storage allocation in the config file,
// enforcing that the new allocation does not exceed half of the free disk space.
func SetStorageAllocation(newStorage int) {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("[!] Could not determine home directory:", err)
		return
	}
	freeBytes, err := GetFreeDiskSpace(home)
	if err != nil {
		fmt.Println("[!] Could not determine free disk space:", err)
		return
	}
	freeGB := int(freeBytes / (1024 * 1024 * 1024))
	maxAllowed := freeGB / 2
	if newStorage > maxAllowed {
		fmt.Printf("[!] You cannot allocate more than %d GB (half of your free space of %d GB).\n", maxAllowed, freeGB)
		return
	}

	config, err := LoadConfig()
	if err != nil {
		fmt.Println("[!] Failed to load config:", err)
		return
	}
	config.StorageAllocation = newStorage

	f, err := os.Create(configPath)
	if err != nil {
		fmt.Println("[!] Failed to update config:", err)
		return
	}
	defer f.Close()
	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(config); err != nil {
		fmt.Println("[!] Failed to encode updated config:", err)
	} else {
		fmt.Println("[+] Storage allocation updated successfully.")
	}
}

// ReadStorageAllocation returns the storage allocation (in GB) from the config file.
func ReadStorageAllocation() (int, error) {
	config, err := LoadConfig()
	if err != nil {
		return 0, err
	}
	return config.StorageAllocation, nil
}

// SetStartTime writes the current time to the start time file for uptime tracking.
func SetStartTime() error {
	t := time.Now()
	dir := filepath.Dir(startTimeFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory for start time file: %v", err)
	}
	return os.WriteFile(startTimeFile, []byte(t.Format(time.RFC3339)), 0644)
}

// GetStartTimeOrNow returns the stored start time, or the current time if unavailable.
func GetStartTimeOrNow() time.Time {
	data, err := os.ReadFile(startTimeFile)
	if err != nil {
		return time.Now()
	}
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(string(data)))
	if err != nil {
		return time.Now()
	}
	return t
}

// GetUptime calculates and returns the node's uptime in hours and minutes.
func GetUptime() string {
	start := GetStartTimeOrNow()
	duration := time.Since(start)
	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60
	return fmt.Sprintf("%dh %dm", hours, minutes)
}

// GetRegion returns the node's region as defined in the configuration.
func GetRegion() string {
	config, err := LoadConfig()
	if err != nil {
		return "us-east-1"
	}
	return config.Region
}
