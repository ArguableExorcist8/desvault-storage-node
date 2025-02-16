package rewards

import (
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"
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

// SaveRewards saves the rewards map to disk.
func SaveRewards(rewardsMap map[string]Reward) error {
	mu.Lock()
	defer mu.Unlock()

	data, err := json.MarshalIndent(rewardsMap, "", "  ")
	if err != nil {
		return err
	}

	err = os.WriteFile(rewardFile, data, 0644)
	if err != nil {
		return err
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
			return map[string]Reward{}, nil
		}
		return nil, err
	}

	var rewardsMap map[string]Reward
	err = json.Unmarshal(data, &rewardsMap)
	if err != nil {
		return nil, err
	}

	return rewardsMap, nil
}

// LoadNodeRewards returns the rewards map; if an error occurs (other than file not existing),
// it logs the error and returns an empty map.
func LoadNodeRewards() map[string]Reward {
	rewardsMap, err := LoadRewards()
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("[!] Error loading node rewards: %v", err)
		}
		return map[string]Reward{}
	}
	return rewardsMap
}
