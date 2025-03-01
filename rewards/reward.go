package rewards

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Reward represents the rewards earned by a node.
type Reward struct {
	NodeID      string    `json:"nodeID"`
	NodeType    string    `json:"nodeType"`
	BasePoints  int       `json:"basePoints"`
	Multiplier  float64   `json:"multiplier"`
	TotalPoints float64   `json:"totalPoints"`
	LastUpdated time.Time `json:"last_updated"`
}

// CalculateRewards calculates the rewards for a node based on its type and base points.
// It uses a multiplier specific to the node type, logs the result, and returns the Reward.
func CalculateRewards(nodeID string, nodeType string, basePoints int) Reward {
	multiplier := GetMultiplier(nodeType)
	totalPoints := float64(basePoints) * multiplier

	reward := Reward{
		NodeID:      nodeID,
		NodeType:    nodeType,
		BasePoints:  basePoints,
		Multiplier:  multiplier,
		TotalPoints: totalPoints,
		LastUpdated: time.Now(),
	}
	log.Printf("[+] Rewards updated: %s earned %.2f points", nodeID, totalPoints)
	return reward
}

// CalculatePoints calculates reward points based on storage contributed.
// Example: 100 points per GB per hour.
func CalculatePoints(storageGB int) int {
	return storageGB * 100
}

// Reward multipliers for different node types.
const (
	CloudOnlyMultiplier = 1.0
	LocalOnlyMultiplier = 2.0
	HybridMultiplier    = 2.5
)

// GetMultiplier returns the reward multiplier based on the node type.
func GetMultiplier(nodeType string) float64 {
	switch nodeType {
	case "cloud":
		return CloudOnlyMultiplier
	case "local":
		return LocalOnlyMultiplier
	case "hybrid":
		return HybridMultiplier
	default:
		return 1.0
	}
}

var mu sync.Mutex
var rewardFile = "rewards.json"

// SaveRewards saves the rewards map (nodeID -> Reward) to disk as a JSON file.
func SaveRewards(rewardsMap map[string]Reward) error {
	mu.Lock()
	defer mu.Unlock()

	data, err := json.MarshalIndent(rewardsMap, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal rewards: %w", err)
	}

	err = os.WriteFile(rewardFile, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write rewards file: %w", err)
	}

	log.Println("[+] Rewards saved to disk.")
	return nil
}

// LoadRewards loads the rewards map from disk.
// If the file does not exist, it returns an empty map.
func LoadRewards() (map[string]Reward, error) {
	mu.Lock()
	defer mu.Unlock()

	data, err := os.ReadFile(rewardFile)
	if err != nil {
		if os.IsNotExist(err) {
			// No rewards file yet; return an empty map.
			return map[string]Reward{}, nil
		}
		return nil, fmt.Errorf("failed to read rewards file: %w", err)
	}

	var rewardsMap map[string]Reward
	if err := json.Unmarshal(data, &rewardsMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal rewards: %w", err)
	}

	return rewardsMap, nil
}

// LoadNodeRewards returns the rewards map; if an error occurs (other than file not existing),
// it logs the error and returns an empty map.
func LoadNodeRewards() map[string]Reward {
	rewardsMap, err := LoadRewards()
	if err != nil {
		log.Printf("[!] Error loading node rewards: %v", err)
		return map[string]Reward{}
	}
	return rewardsMap
}

// GenerateRewardID generates a unique reward identifier. In production, you might use this to track reward entries.
func GenerateRewardID() string {
	return uuid.New().String()
}
