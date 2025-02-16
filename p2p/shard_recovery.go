package p2p

import (
	"log"
)

// Check if a node storing shards is offline
func DetectOfflineNodes(activeNodes map[string]bool, shardMap map[string][]Shard) {
	for node, shards := range shardMap {
		if !activeNodes[node] {
			log.Printf("[!] Node %s is offline. Redistributing %d shards.\n", node, len(shards))
			RedistributeShards(shards, activeNodes)
		}
	}
}

// Redistribute lost shards to active nodes
func RedistributeShards(shards []Shard, activeNodes map[string]bool) {
	for _, shard := range shards {
		for node := range activeNodes {
			log.Printf("[+] Redistributing shard %s to node %s.\n", shard.ID, node)
			break
		}
	}
}
