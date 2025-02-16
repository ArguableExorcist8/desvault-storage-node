package reputation

// Reputation formula based on uptime and storage
func calculateReputation(uptimeHours, storageGB int) float64 {
	baseReputation := float64(uptimeHours) / 100.0
	storageBonus := float64(storageGB) / 50.0
	return baseReputation + storageBonus
}
