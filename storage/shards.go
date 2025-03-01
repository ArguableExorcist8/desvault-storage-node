package storage

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"desvault/storage"
)

// GetShardCount scans the local storage directory and returns the number of shard files.
// It assumes that shard files are stored with a ".bin" extension.
func GetShardCount() int {
	storageDir := storage.GetStorageDir()
	files, err := ioutil.ReadDir(storageDir)
	if err != nil {
		log.Printf("[ERROR] Unable to read storage directory %s: %v", storageDir, err)
		return 0
	}
	count := 0
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".bin" {
			count++
		}
	}
	return count
}

// ListShards returns a slice of shard file names found in the local storage directory.
func ListShards() ([]string, error) {
	storageDir := storage.GetStorageDir()
	files, err := ioutil.ReadDir(storageDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read storage directory: %w", err)
	}
	var shardFiles []string
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".bin" {
			shardFiles = append(shardFiles, file.Name())
		}
	}
	return shardFiles, nil
}

// GetShardByID returns the full file path for a given shard identifier.
// It looks for a file named "<shardID>.bin" in the storage directory.
func GetShardByID(shardID string) (string, error) {
	storageDir := storage.GetStorageDir()
	filePath := filepath.Join(storageDir, shardID+".bin")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", fmt.Errorf("shard %s not found", shardID)
	}
	return filePath, nil
}
