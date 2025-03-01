package setup

import (
    "fmt"
    "log"
    "os"
    "path/filepath"
    "strconv"
    "strings"
    "syscall"
    "time"
)

// Config holds the configuration data.
type Config struct {
    StorageGB     int
    PointsPerHour int
    WalletConfig  string // Optional: Add if wallet config is needed.
}

// configPath is the configuration file path.
var configPath = expandPath("~/.desvault/storage.conf")

// startTimeFile is the file where the node's start time is stored.
var startTimeFile = expandPath("~/.desvault/start_time")

// LoadConfig loads the configuration from the config file.
func LoadConfig() (*Config, error) {
    data, err := os.ReadFile(configPath)
    if err != nil {
        return nil, fmt.Errorf("failed to read config: %w", err)
    }
    lines := strings.Split(string(data), "\n")
    if len(lines) < 2 {
        return nil, fmt.Errorf("invalid config file format")
    }
    storageParts := strings.Split(lines[0], "=")
    pointsParts := strings.Split(lines[1], "=")
    if len(storageParts) < 2 || len(pointsParts) < 2 {
        return nil, fmt.Errorf("invalid config file format")
    }
    storageGB, err := strconv.Atoi(strings.TrimSpace(storageParts[1]))
    if err != nil {
        return nil, fmt.Errorf("failed to parse storageGB: %w", err)
    }
    pointsPerHour, err := strconv.Atoi(strings.TrimSpace(pointsParts[1]))
    if err != nil {
        return nil, fmt.Errorf("failed to parse pointsPerHour: %w", err)
    }
    return &Config{
        StorageGB:     storageGB,
        PointsPerHour: pointsPerHour,
        WalletConfig:  "", // Add actual wallet config parsing if needed.
    }, nil
}

// FirstTimeSetup performs initial configuration if the config file does not exist.
func FirstTimeSetup() {
    if _, err := os.Stat(configPath); err == nil {
        return // Config exists; no setup needed.
    }

    fmt.Println(`
██████╗ ███████╗███████╗██╗   ██╗ █████╗ ██╗   ██╗██╗  ████████╗
██╔══██╗██╔════╝██╔════╝██║   ██║██╔══██╗██║   ██║██║  ╚══██╔══╝
██║  ██║█████╗  ███████╗██║   ██║███████║██║   ██║██║     ██║   
██║  ██║██╔══╝  ╚════██║╚██╗ ██╔╝██╔══██║██║   ██║██║     ██║   
██████╔╝███████╗███████║ ╚████╔╝ ██║  ██║╚██████╔╝███████╗██║   
╚═════╝ ╚══════╝╚══════╝  ╚═══╝  ╚═╝  ╚═╝ ╚═════╝ ╚══════╝╚═╝`)
    fmt.Println("[!] First-time setup detected")

    var storageGB int
    fmt.Print("[?] Enter storage allocation in GB (between 5 and your maximum allowed): ")
    fmt.Scan(&storageGB)

    // Check free disk space and enforce that allocation cannot exceed half of free space.
    home, err := os.UserHomeDir()
    if err != nil {
        log.Fatalf("[!] Could not determine home directory: %v", err)
    }
    freeBytes, err := GetFreeDiskSpace(home)
    if err != nil {
        log.Fatalf("[!] Could not determine free disk space: %v", err)
    }
    freeGB := int(freeBytes / (1024 * 1024 * 1024))
    maxAllowed := freeGB / 2
    if storageGB > maxAllowed {
        log.Fatalf("[!] You cannot allocate more than %d GB (half of your free space of %d GB).", maxAllowed, freeGB)
    }

    estimatedPoints := CalculatePoints(storageGB)
    configData := fmt.Sprintf("storageGB=%d\npointsPerHour=%d\n", storageGB, estimatedPoints)
    if err := os.WriteFile(configPath, []byte(configData), 0644); err != nil {
        log.Fatalf("[!] Failed to save configuration: %v", err)
    }

    fmt.Printf("[+] Config saved to %s\n", configPath)
    fmt.Printf("Region: %s\nAllocated Storage: %d GB\nEstimated Rewards: %d pts/hour\n",
        GetRegion(), storageGB, estimatedPoints)
}

// CalculatePoints returns estimated reward points based on storage.
func CalculatePoints(storageGB int) int {
    return storageGB * 100 // Example: 100 points per GB.
}

// GetFreeDiskSpace returns the free disk space (in bytes) for the given path.
func GetFreeDiskSpace(path string) (uint64, error) {
    var stat syscall.Statfs_t
    err := syscall.Statfs(path, &stat)
    if err != nil {
        return 0, err
    }
    // Available blocks * size per block = available space in bytes.
    return stat.Bavail * uint64(stat.Bsize), nil
}

// SetStorageAllocation updates the storage allocation in the configuration file.
// It enforces that the new allocation does not exceed half of the free disk space.
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

    configData := fmt.Sprintf("storageGB=%d\npointsPerHour=%d\n", newStorage, CalculatePoints(newStorage))
    err = os.WriteFile(configPath, []byte(configData), 0644)
    if err != nil {
        fmt.Println("[!] Failed to update storage allocation.")
    } else {
        fmt.Println("[+] Storage allocation updated successfully.")
    }
}

// ReadStorageAllocation reads the storage allocation and points per hour from the configuration file.
func ReadStorageAllocation() (int, int) {
    data, err := os.ReadFile(configPath)
    if err != nil {
        log.Fatalf("[!] Failed to read config: %v", err)
    }
    lines := strings.Split(string(data), "\n")
    if len(lines) < 2 {
        log.Fatalf("[!] Invalid config file format.")
    }
    storageParts := strings.Split(lines[0], "=")
    pointsParts := strings.Split(lines[1], "=")
    if len(storageParts) < 2 || len(pointsParts) < 2 {
        log.Fatalf("[!] Invalid config file format.")
    }
    storageGB, err := strconv.Atoi(strings.TrimSpace(storageParts[1]))
    if err != nil {
        log.Fatalf("[!] Failed to parse storageGB: %v", err)
    }
    pointsPerHour, err := strconv.Atoi(strings.TrimSpace(pointsParts[1]))
    if err != nil {
        log.Fatalf("[!] Failed to parse pointsPerHour: %v", err)
    }
    return storageGB, pointsPerHour
}

// SetStartTime writes the current time to the startTimeFile.
func SetStartTime() error {
    t := time.Now()
    return os.WriteFile(startTimeFile, []byte(t.Format(time.RFC3339)), 0644)
}

// GetStartTimeOrNow returns the stored start time or the current time if not found.
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

// GetUptime returns the total uptime of the node based on the stored start time.
func GetUptime() string {
    start := GetStartTimeOrNow()
    duration := time.Since(start)
    hours := int(duration.Hours())
    minutes := int(duration.Minutes()) % 60
    return fmt.Sprintf("%dh %dm", hours, minutes)
}

// GetRegion returns the node's region (hardcoded for now).
func GetRegion() string {
    return "ASIA"
}

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