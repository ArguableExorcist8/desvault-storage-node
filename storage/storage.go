package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"

	shell "github.com/ipfs/go-ipfs-api"
)

// Shard represents a single fragment of a file.
type Shard struct {
	ID     string   // Unique shard identifier (hash)
	Data   []byte   // The raw (or encrypted) shard data
	CID    string   // IPFS CID after uploading the shard
	Copies []string // Node IDs where the shard is stored
}

// FileMetadata defines the metadata for a file split into shards.
type FileMetadata struct {
	FileName string
	FileSize int64
	CID      string   // Global CID computed from the shards
	Shards   []Shard  // The shards that make up the file
}

var (
	// ShardMap is a global map for tracking storage (if needed)
	ShardMap = make(map[string]bool)
	mu       sync.Mutex // Mutex for thread-safe operations
)

// encryptionKey is a 32-byte key for AES-256 encryption.
// generate and manage this key securely.
var encryptionKey = []byte("0123456789abcdef0123456789abcdef") // 32 bytes

// GetStorageDir returns the dedicated storage folder path.
func GetStorageDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Failed to determine home directory: %v", err)
	}
	return filepath.Join(home, ".desvault", "storage")
}

// InitializeStorage ensures the storage directory exists.
func InitializeStorage() {
	dir := GetStorageDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Fatalf("Failed to create storage directory: %v", err)
	}
	log.Printf("[+] Storage directory initialized: %s", dir)
}

// ConnectToIPFS connects to the local IPFS daemon.
func ConnectToIPFS() *shell.Shell {
	return shell.NewShell("localhost:5001")
}

// SplitFileIntoShards splits a file into 5 shards and handles any remainder.
func SplitFileIntoShards(filename string) ([]Shard, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}
	fileSize := fileInfo.Size()
	numShards := 5

	baseSize := fileSize / int64(numShards)
	remainder := fileSize % int64(numShards)

	shards := make([]Shard, 0, numShards)
	for i := 0; i < numShards; i++ {
		currentSize := baseSize
		if i == numShards-1 {
			currentSize += remainder
		}
		data := make([]byte, currentSize)
		n, err := io.ReadFull(file, data)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return nil, err
		}
		data = data[:n]
		hash := sha256.Sum256(data)
		shardID := hex.EncodeToString(hash[:])
		log.Printf("Shard %d raw data length: %d", i, len(data))
		shard := Shard{
			ID:   shardID,
			Data: data,
		}
		shards = append(shards, shard)
	}
	return shards, nil
}

// UploadShardToIPFS encrypts a shard's data, uploads it to IPFS,
// and writes a permanent copy in the dedicated storage folder.
func UploadShardToIPFS(shard *Shard) error {
	sh := ConnectToIPFS()

	encryptedData, err := EncryptData(shard.Data, encryptionKey)
	if err != nil {
		return fmt.Errorf("failed to encrypt shard data: %w", err)
	}
	shard.Data = encryptedData
	log.Printf("Encrypted shard %s length: %d", shard.ID, len(shard.Data))

	// Write to a temporary file.
	tempFile, err := os.CreateTemp("", "shard_")
	if err != nil {
		return err
	}
	defer os.Remove(tempFile.Name())
	if _, err := tempFile.Write(shard.Data); err != nil {
		return err
	}
	// Reset file pointer so that IPFS can read from the beginning.
	if _, err := tempFile.Seek(0, io.SeekStart); err != nil {
		return err
	}

	cid, err := sh.Add(tempFile)
	if err != nil {
		return fmt.Errorf("failed to add shard to IPFS: %w", err)
	}
	shard.CID = cid
	log.Printf("Shard %s uploaded with CID: %s", shard.ID, cid)

	// Write the encrypted shard data to a permanent file.
	permanentPath := filepath.Join(GetStorageDir(), shard.ID+".bin")
	if err := os.WriteFile(permanentPath, shard.Data, 0644); err != nil {
		log.Printf("[Warning] Failed to store permanent copy of shard %s: %v", shard.ID, err)
	} else {
		log.Printf("[+] Permanent copy stored: %s", permanentPath)
	}

	return nil
}

// UploadFileWithMetadata splits a file into shards, uploads them, computes a global CID, and returns metadata.
func UploadFileWithMetadata(filePath string) (FileMetadata, error) {
	shards, err := SplitFileIntoShards(filePath)
	if err != nil {
		return FileMetadata{}, err
	}

	for i := range shards {
		if err := UploadShardToIPFS(&shards[i]); err != nil {
			return FileMetadata{}, err
		}
	}

	var concatenated string
	for _, shard := range shards {
		concatenated += shard.CID
	}
	globalHash := sha256.Sum256([]byte(concatenated))
	globalCID := hex.EncodeToString(globalHash[:16])

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return FileMetadata{}, err
	}

	metadata := FileMetadata{
		FileName: fileInfo.Name(),
		FileSize: fileInfo.Size(),
		CID:      globalCID,
		Shards:   shards,
	}
	return metadata, nil
}

// DownloadShardFromIPFS downloads a shard from IPFS using Cat() and decrypts it.
// If IPFS returns empty data, it will error.
func DownloadShardFromIPFS(cid string) ([]byte, error) {
	sh := ConnectToIPFS()

	reader, err := sh.Cat(cid)
	if err != nil {
		return nil, fmt.Errorf("failed to cat CID %s: %w", cid, err)
	}
	defer reader.Close()

	encryptedData, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read data for CID %s: %w", cid, err)
	}

	log.Printf("Downloaded encrypted data length for shard %s: %d", cid, len(encryptedData))
	if len(encryptedData) == 0 {
		return nil, fmt.Errorf("downloaded shard data is empty for CID %s", cid)
	}

	plainData, err := DecryptData(encryptedData, encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt shard data (length: %d): %w", len(encryptedData), err)
	}
	return plainData, nil
}

// DownloadFile reconstructs the original file from its shards.
func DownloadFile(shards []Shard, outputPath string) error {
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer outputFile.Close()

	for _, shard := range shards {
		data, err := DownloadShardFromIPFS(shard.CID)
		if err != nil {
			return err
		}
		_, err = outputFile.Write(data)
		if err != nil {
			return err
		}
	}
	return nil
}

// ListFiles retrieves pinned files from IPFS (example implementation).
func ListFiles() ([]map[string]interface{}, error) {
	sh := ConnectToIPFS()
	ctx := context.Background()

	var result struct {
		Keys map[string]struct {
			Type string `json:"Type"`
		} `json:"Keys"`
	}
	if err := sh.Request("pin/ls").Exec(ctx, &result); err != nil {
		return nil, err
	}

	var fileList []map[string]interface{}
	for cid, info := range result.Keys {
		fileList = append(fileList, map[string]interface{}{
			"cid":  cid,
			"type": info.Type,
		})
	}

	return fileList, nil
}

// GetFilePath returns a local path for a given CID (stub implementation).
func GetFilePath(cid string) (string, error) {
	return "/path/to/file", nil
}

// GetShardCount returns the number of shards stored.
func GetShardCount() int {
	mu.Lock()
	defer mu.Unlock()
	return len(ShardMap)
}

// StartStorageService initializes the storage service.
func StartStorageService() {
	InitializeStorage()
	log.Println("Storage service started")
}
