package points

import (
	"log"
	"sync"
	"time"
)

// Total accumulated points (thread-safe)
var (
	totalPoints       int
	currentStorageGB  int
	mu                sync.Mutex
)

// Storage rewards table based on allocated storage (GB)
var storageRewards = []struct {
	minGB, maxGB, points int
}{
	{5, 20, 1000},
	{20, 40, 1300},
	{40, 50, 1500},
	{51, 75, 2000},
	{75, 100, 2500},
	{101, 200, 3000},
	{201, 400, 4000},
	{401, 99999, 5000}, // 401+ GB
}

// GetTotalPoints returns the total accumulated points (thread-safe)
func GetTotalPoints() int {
	mu.Lock()
	defer mu.Unlock()
	return totalPoints
}

// AddPoints safely updates the total points
func AddPoints(points int) {
	mu.Lock()
	defer mu.Unlock()
	totalPoints += points
}

// CalculatePoints determines the reward points based on storage usage (GB)
func CalculatePoints(storageGB int) int {
	for _, reward := range storageRewards {
		if storageGB >= reward.minGB && storageGB < reward.maxGB {
			return reward.points
		}
	}
	return 0
}

// UpdateRewards recalculates and updates the points based on the stored usage.
func UpdateRewards() {
	mu.Lock()
	defer mu.Unlock()

	points := CalculatePoints(currentStorageGB)
	AddPoints(points)

	log.Printf("[Rewards] Updated rewards: %d points for %dGB storage (Total: %d)\n", points, currentStorageGB, totalPoints)
}


// GetStorageUsage returns the current storage usage
func GetStorageUsage() int {
	mu.Lock()
	defer mu.Unlock()
	return currentStorageGB
}

// StartRewardSystem periodically distributes points based on storage usage
func StartRewardSystem(stopChan chan struct{}) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			storageGB := GetStorageUsage()
			points := CalculatePoints(storageGB)
			AddPoints(points)
			log.Printf("[Rewards] Allocated %d points for %dGB storage (Total: %d)\n", points, storageGB, GetTotalPoints())
		case <-stopChan:
			log.Println("[Rewards] Reward system stopped.")
			return
		}
	}
}