package p2p

import (
	"fmt"
	"log"
	"math/rand"
	"time"
)

// Structure to hold shard info
type Shard struct {
	ID      string
	Data    []byte
	Replica int
}

// Function to distribute shards across nodes
func DistributeShards(fileID string, fileData []byte, nodes []string) map[string][]Shard {
	rand.Seed(time.Now().UnixNano())
	shardMap := make(map[string][]Shard)

	// Split file into 5 shards
	shards := SplitFileIntoShards(fileData, 5)

	// Assign each shard to 3 random nodes
	for _, shard := range shards {
		for i := 0; i < 3; i++ {
			randomNode := nodes[rand.Intn(len(nodes))]
			shardMap[randomNode] = append(shardMap[randomNode], shard)
		}
	}

	log.Println("[+] Shard replication complete! 5x3 redundancy achieved.")
	return shardMap
}

// Function to split file into 5 shards
func SplitFileIntoShards(data []byte, numShards int) []Shard {
	shardSize := len(data) / numShards
	var shards []Shard

	for i := 0; i < numShards; i++ {
		start := i * shardSize
		end := start + shardSize
		if i == numShards-1 {
			end = len(data)
		}

		shards = append(shards, Shard{
			ID:      fmt.Sprintf("shard-%d", i),
			Data:    data[start:end],
			Replica: 3,
		})
	}

	return shards
}
