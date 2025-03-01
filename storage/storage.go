package storage

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
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

// -----------------------------------------------------------------------------
// Types and Global Variables
// -----------------------------------------------------------------------------

// Shard represents a single fragment of a file.
type Shard struct {
	ID     string   // Unique shard identifier (hash)
	Data   []byte   // The raw (or encrypted) shard data
	CID    string   // IPFS CID after uploading the shard
	Copies []string // Node IDs where the shard is stored (if applicable)
}

// FileMetadata defines metadata for a file split into shards.
type FileMetadata struct {
	FileName string
	FileSize int64
	CID      string   // Global CID computed from the shards
	Shards   []Shard  // The shards that make up the file
}

var (
	// ShardMap tracks stored shards (for production, consider a persistent store).
	ShardMap = make(map[string]bool)
	mu       sync.Mutex // Mutex for thread-safe operations
)

// encryptionKey is a 32-byte key for AES-256 encryption.
// In production, manage this key securely (e.g., in an HSM or secure vault).
var encryptionKey = []byte("0123456789abcdef0123456789abcdef") // Example key (32 bytes)

// -----------------------------------------------------------------------------
// Directory & IPFS Helpers
// -----------------------------------------------------------------------------

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
	log.Printf("[INFO] Storage directory initialized: %s", dir)
}

// ConnectToIPFS creates a new IPFS shell connection.
// In production, consider handling connection failures and retries.
func ConnectToIPFS() *shell.Shell {
	return shell.NewShell("localhost:5001")
}

// -----------------------------------------------------------------------------
// Encryption/Decryption (AES-256-GCM)
// -----------------------------------------------------------------------------

// EncryptData encrypts plaintext using AES-256-GCM with the provided key.
func EncryptData(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// DecryptData decrypts ciphertext using AES-256-GCM with the provided key.
func DecryptData(ciphertext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, cipherData := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, cipherData, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt data: %w", err)
	}
	return plaintext, nil
}

// -----------------------------------------------------------------------------
// Sharding and File Upload/Download
// -----------------------------------------------------------------------------

// SplitFileIntoShards splits a file into a fixed number of shards.
// It evenly divides the file and adds any remainder to the last shard.
func SplitFileIntoShards(filename string) ([]Shard, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filename, err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat file %s: %w", filename, err)
	}
	fileSize := fileInfo.Size()
	const numShards = 5

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
			return nil, fmt.Errorf("failed to read shard %d: %w", i, err)
		}
		data = data[:n]
		// Create a unique ID for this shard using SHA-256.
		hash := sha256.Sum256(data)
		shardID := hex.EncodeToString(hash[:])
		log.Printf("[INFO] Shard %d created with %d bytes (ID: %s)", i, len(data), shardID)
		shard := Shard{
			ID:   shardID,
			Data: data,
		}
		shards = append(shards, shard)
	}
	return shards, nil
}

// UploadShardToIPFS encrypts a shard's data, uploads it to IPFS, and saves a permanent local copy.
func UploadShardToIPFS(shard *Shard) error {
	sh := ConnectToIPFS()

	// Encrypt the shard data.
	encryptedData, err := EncryptData(shard.Data, encryptionKey)
	if err != nil {
		return fmt.Errorf("failed to encrypt shard data: %w", err)
	}
	shard.Data = encryptedData
	log.Printf("[INFO] Shard %s encrypted (%d bytes)", shard.ID, len(shard.Data))

	// Write encrypted data to a temporary file.
	tempFile, err := os.CreateTemp("", "shard_")
	if err != nil {
		return fmt.Errorf("failed to create temporary file for shard %s: %w", shard.ID, err)
	}
	defer func() {
		tempFile.Close()
		os.Remove(tempFile.Name())
	}()
	if _, err := tempFile.Write(shard.Data); err != nil {
		return fmt.Errorf("failed to write encrypted data for shard %s: %w", shard.ID, err)
	}
	if _, err := tempFile.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to reset file pointer for shard %s: %w", shard.ID, err)
	}

	// Upload the file to IPFS.
	cid, err := sh.Add(tempFile)
	if err != nil {
		return fmt.Errorf("failed to add shard %s to IPFS: %w", shard.ID, err)
	}
	shard.CID = cid
	log.Printf("[INFO] Shard %s uploaded to IPFS with CID: %s", shard.ID, cid)

	// Store a permanent encrypted copy locally.
	permanentPath := filepath.Join(GetStorageDir(), shard.ID+".bin")
	if err := os.WriteFile(permanentPath, shard.Data, 0644); err != nil {
		log.Printf("[WARNING] Could not store permanent copy for shard %s: %v", shard.ID, err)
	} else {
		log.Printf("[INFO] Permanent copy stored at: %s", permanentPath)
	}
	return nil
}

// UploadFileWithMetadata splits the given file into shards, uploads each shard to IPFS,
// computes a global file CID, and returns file metadata.
func UploadFileWithMetadata(filePath string) (FileMetadata, error) {
	shards, err := SplitFileIntoShards(filePath)
	if err != nil {
		return FileMetadata{}, err
	}

	// Upload each shard to IPFS.
	for i := range shards {
		if err := UploadShardToIPFS(&shards[i]); err != nil {
			return FileMetadata{}, fmt.Errorf("failed to upload shard %d: %w", i, err)
		}
		// Track the shard in a global map (for demonstration).
		mu.Lock()
		ShardMap[shards[i].ID] = true
		mu.Unlock()
	}

	// Concatenate all shard CIDs and compute a global CID.
	var concatenated string
	for _, shard := range shards {
		concatenated += shard.CID
	}
	globalHash := sha256.Sum256([]byte(concatenated))
	// Use the first 16 bytes as a shortened global CID.
	globalCID := hex.EncodeToString(globalHash[:16])

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return FileMetadata{}, fmt.Errorf("failed to stat file %s: %w", filePath, err)
	}

	metadata := FileMetadata{
		FileName: fileInfo.Name(),
		FileSize: fileInfo.Size(),
		CID:      globalCID,
		Shards:   shards,
	}
	log.Printf("[INFO] File %s processed with global CID: %s", metadata.FileName, metadata.CID)
	return metadata, nil
}

// DownloadShardFromIPFS downloads a shard by its IPFS CID and decrypts it.
func DownloadShardFromIPFS(cid string) ([]byte, error) {
	sh := ConnectToIPFS()

	// Retrieve encrypted data from IPFS.
	reader, err := sh.Cat(cid)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve CID %s from IPFS: %w", cid, err)
	}
	defer reader.Close()

	encryptedData, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read data for CID %s: %w", cid, err)
	}
	log.Printf("[INFO] Downloaded encrypted shard for CID %s (%d bytes)", cid, len(encryptedData))
	if len(encryptedData) == 0 {
		return nil, fmt.Errorf("no data received for CID %s", cid)
	}

	// Decrypt the data.
	plainData, err := DecryptData(encryptedData, encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt data for CID %s: %w", cid, err)
	}
	return plainData, nil
}

// DownloadFile reconstructs the original file by downloading and decrypting each shard,
// then writes them sequentially to outputPath.
func DownloadFile(shards []Shard, outputPath string) error {
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file %s: %w", outputPath, err)
	}
	defer outputFile.Close()

	for _, shard := range shards {
		data, err := DownloadShardFromIPFS(shard.CID)
		if err != nil {
			return fmt.Errorf("failed to download shard with CID %s: %w", shard.CID, err)
		}
		if _, err := outputFile.Write(data); err != nil {
			return fmt.Errorf("failed to write shard %s to output: %w", shard.ID, err)
		}
	}
	log.Printf("[INFO] File reconstructed and saved to %s", outputPath)
	return nil
}

// ListFiles retrieves pinned files from IPFS.
func ListFiles() ([]map[string]interface{}, error) {
	sh := ConnectToIPFS()
	ctx := context.Background()

	var result struct {
		Keys map[string]struct {
			Type string `json:"Type"`
		} `json:"Keys"`
	}
	if err := sh.Request("pin/ls").Exec(ctx, &result); err != nil {
		return nil, fmt.Errorf("failed to list pinned files: %w", err)
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
	// In production, maintain a proper mapping of CIDs to local files.
	return filepath.Join(GetStorageDir(), cid+".bin"), nil
}

// GetShardCount returns the number of stored shards.
func GetShardCount() int {
	mu.Lock()
	defer mu.Unlock()
	return len(ShardMap)
}

// StartStorageService initializes the storage service.
func StartStorageService() {
	InitializeStorage()
	log.Println("[INFO] Storage service started")
}
