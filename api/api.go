package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
    "DesVault/storage-node/storage"
)

// fileMetadataStore holds metadata for each uploaded file (keyed by its global CID).
// In production, you would persist this data in a database.
var fileMetadataStore = make(map[string]storage.FileMetadata)

// authToken is the static token required for API requests.
const authToken = "secret123"

// authMiddleware checks for the expected Authorization header.
func authMiddleware(c *gin.Context) {
	if c.GetHeader("Authorization") != "Bearer "+authToken {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	c.Next()
}

func main() {
	// Initialize the storage directory.
	storage.InitializeStorage()

	router := gin.Default()

	// Enable basic CORS for development.
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	// Apply the authentication middleware.
	authorized := router.Group("/", authMiddleware)

	// POST /upload: Upload a file.
	authorized.POST("/upload", func(c *gin.Context) {
		file, err := c.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("File not provided: %v", err)})
			return
		}

		tempPath := filepath.Join(os.TempDir(), file.Filename)
		if err := c.SaveUploadedFile(file, tempPath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Could not save file to %s: %v", tempPath, err)})
			return
		}

		// Process the file upload.
		metadata, err := storage.UploadFileWithMetadata(tempPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Upload process failed: %v", err)})
			return
		}

		// Save metadata using the generated global CID as key.
		fileMetadataStore[metadata.CID] = metadata

		c.JSON(http.StatusOK, metadata)
	})

	// GET /files: List uploaded files.
	authorized.GET("/files", func(c *gin.Context) {
		var files []storage.FileMetadata
		for _, metadata := range fileMetadataStore {
			files = append(files, metadata)
		}
		c.JSON(http.StatusOK, files)
	})

	// GET /download/:cid: Download a file using its global CID.
	authorized.GET("/download/:cid", func(c *gin.Context) {
		cid := c.Param("cid")
		metadata, exists := fileMetadataStore[cid]
		if !exists {
			c.JSON(http.StatusNotFound, gin.H{"error": "File metadata not found"})
			return
		}

		// Reconstruct the file from its shards.
		outputPath := filepath.Join(os.TempDir(), metadata.FileName)
		err := storage.DownloadFile(metadata.Shards, outputPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error reconstructing file: %v", err)})
			return
		}

		c.File(outputPath)
	})

	// Run the API server on port 8080.
	router.Run(":8080")
}
