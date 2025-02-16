package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"DesVault/storage-node/storage"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// --------------------
// Configuration Helpers
// --------------------

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

var (
	authToken = getEnv("AUTH_TOKEN", "secret123")
	port      = getEnv("PORT", "8080")
	dbURL     = os.Getenv("DATABASE_URL") 
)

// --------------------
// Helper Functions
// --------------------

// generateCID returns a random 16-digit numeric string.
func generateCID() string {
	digits := make([]byte, 16)
	for i := 0; i < 16; i++ {
		digits[i] = '0' + byte(rand.Intn(10))
	}
	return string(digits)
}

// formatFileSize converts a file size in bytes to a human‑readable string.
func formatFileSize(size int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	if size < KB {
		return fmt.Sprintf("%d B", size)
	} else if size < MB {
		return fmt.Sprintf("%.2f KB", float64(size)/KB)
	} else if size < GB {
		return fmt.Sprintf("%.2f MB", float64(size)/MB)
	} else {
		return fmt.Sprintf("%.2f GB", float64(size)/GB)
	}
}

// --------------------
// Database Model
// --------------------

type FileMetadataModel struct {
	// Force the column name to be "cid" so that queries like "cid = ?" work.
	CID       string         `gorm:"column:cid;primaryKey;not null;size:255" json:"cid"`
	FileName  string         `gorm:"size:255" json:"fileName"`
	Note      string         `gorm:"size:255" json:"note"`
	FileSize  string         `gorm:"size:255" json:"fileSize"`
	Shards    datatypes.JSON `gorm:"type:jsonb" json:"shards"`
	CreatedAt time.Time      `json:"createdAt"`
}

type FileMetadataResponse struct {
	CID       string          `json:"cid"`
	FileName  string          `json:"fileName"`
	Note      string          `json:"note"`
	FileSize  string          `json:"fileSize"`
	CreatedAt time.Time       `json:"createdAt"`
	Shards    []storage.Shard `json:"shards"`
}

func modelToResponse(model FileMetadataModel) (FileMetadataResponse, error) {
	var shards []storage.Shard
	if err := json.Unmarshal(model.Shards, &shards); err != nil {
		return FileMetadataResponse{}, err
	}
	return FileMetadataResponse{
		CID:       model.CID,
		FileName:  model.FileName,
		Note:      model.Note,
		FileSize:  model.FileSize,
		CreatedAt: model.CreatedAt,
		Shards:    shards,
	}, nil
}

func fileMetadataToModel(metadata storage.FileMetadata) (FileMetadataModel, error) {
	shardsJSON, err := json.Marshal(metadata.Shards)
	if err != nil {
		return FileMetadataModel{}, err
	}
	return FileMetadataModel{
		CID:      metadata.CID,
		FileName: metadata.FileName,
		Shards:   datatypes.JSON(shardsJSON),
	}, nil
}

// --------------------
// Global Database Handle
// --------------------

var db *gorm.DB

func initDB() {
	if dbURL == "" {
		log.Fatal("DATABASE_URL not set")
	}
	var err error
	db, err = gorm.Open(postgres.Open(dbURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	// For development, drop the table if it exists to refresh the schema.
	if db.Migrator().HasTable(&FileMetadataModel{}) {
		if err := db.Migrator().DropTable(&FileMetadataModel{}); err != nil {
			log.Fatalf("failed to drop table: %v", err)
		}
	}

	if err := db.AutoMigrate(&FileMetadataModel{}); err != nil {
		log.Fatalf("failed to auto-migrate database: %v", err)
	}
}

// --------------------
// Middleware
// --------------------

func authMiddleware(c *gin.Context) {
	if c.GetHeader("Authorization") != "Bearer "+authToken {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"code":    http.StatusUnauthorized,
			"message": "Unauthorized",
		})
		return
	}
	c.Next()
}

func secureHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("X-Content-Type-Options", "nosniff")
		c.Writer.Header().Set("X-Frame-Options", "DENY")
		c.Writer.Header().Set("Content-Security-Policy", "default-src 'self'")
		c.Next()
	}
}

var visitors = make(map[string]*Visitor)
var visitorsMutex sync.Mutex

type Visitor struct {
	LastSeen time.Time
	Requests int
}

func rateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		visitorsMutex.Lock()
		v, exists := visitors[ip]
		if !exists || time.Since(v.LastSeen) > time.Minute {
			visitors[ip] = &Visitor{LastSeen: time.Now(), Requests: 1}
		} else {
			v.Requests++
			if v.Requests > 60 {
				visitorsMutex.Unlock()
				c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
					"code":    http.StatusTooManyRequests,
					"message": "Too many requests",
				})
				return
			}
		}
		visitorsMutex.Unlock()
		c.Next()
	}
}

// --------------------
// IPFS Auto-Start Helpers
// --------------------

// isIPFSRunning checks if the IPFS API is available.
func isIPFSRunning() bool {
	resp, err := http.Get("http://localhost:5001/api/v0/version")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// startIPFSDaemon starts the IPFS daemon if not already running.
func startIPFSDaemon() error {
	if isIPFSRunning() {
		log.Println("IPFS daemon already running.")
		return nil
	}
	log.Println("IPFS daemon not running, starting it...")
	cmd := exec.Command("ipfs", "daemon")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start IPFS daemon: %w", err)
	}
	// Wait up to 20 seconds for IPFS to start.
	for i := 0; i < 10; i++ {
		time.Sleep(2 * time.Second)
		if isIPFSRunning() {
			log.Println("IPFS daemon started successfully.")
			return nil
		}
	}
	return fmt.Errorf("IPFS daemon did not start in time")
}

// --------------------
// Main
// --------------------

func main() {
	rand.Seed(time.Now().UnixNano())

	// Automatically start IPFS daemon if not running.
	if err := startIPFSDaemon(); err != nil {
		log.Fatalf("Failed to start IPFS daemon: %v", err)
	}

	initDB()
	storage.InitializeStorage()

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(secureHeadersMiddleware())
	router.Use(rateLimitMiddleware())
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		c.Writer.Header().Set("Access-Control-Expose-Headers", "Content-Disposition")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	authorized := router.Group("/", authMiddleware)

	// POST /upload: Upload a file.
	authorized.POST("/upload", func(c *gin.Context) {
		file, err := c.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    http.StatusBadRequest,
				"message": fmt.Sprintf("File not provided: %v", err),
			})
			return
		}

		// Get note from form data.
		note := c.PostForm("note")
		if note == "" {
			note = "No note available"
		}

		tempPath := filepath.Join(os.TempDir(), file.Filename)
		if err := c.SaveUploadedFile(file, tempPath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    http.StatusInternalServerError,
				"message": fmt.Sprintf("Could not save file to %s: %v", tempPath, err),
			})
			return
		}

		metadata, err := storage.UploadFileWithMetadata(tempPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    http.StatusInternalServerError,
				"message": fmt.Sprintf("Upload process failed: %v", err),
			})
			return
		}

		newCID := generateCID()
		metadata.CID = newCID
		log.Printf("Generated 16-digit CID for file %s: %s", file.Filename, newCID)

		// Convert file size to human-readable format.
		fileSizeStr := formatFileSize(file.Size)

		model, err := fileMetadataToModel(metadata)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    http.StatusInternalServerError,
				"message": fmt.Sprintf("Metadata conversion failed: %v", err),
			})
			return
		}
		model.CID = newCID
		model.Note = note
		model.FileSize = fileSizeStr
		model.CreatedAt = time.Now()

		if err := db.Create(&model).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    http.StatusInternalServerError,
				"message": fmt.Sprintf("Database error: %v", err),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"code":    http.StatusOK,
			"message": "File uploaded successfully",
			"data":    model,
		})
	})

	// GET /files: List uploaded files.
	authorized.GET("/files", func(c *gin.Context) {
		var models []FileMetadataModel
		if err := db.Find(&models).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    http.StatusInternalServerError,
				"message": fmt.Sprintf("Database error: %v", err),
			})
			return
		}
		var responses []FileMetadataResponse
		for _, model := range models {
			resp, err := modelToResponse(model)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"code":    http.StatusInternalServerError,
					"message": fmt.Sprintf("Conversion error: %v", err),
				})
				return
			}
			responses = append(responses, resp)
		}
		c.JSON(http.StatusOK, gin.H{"files": responses})
	})

	// GET /download/:cid: Download a file using its CID.
	authorized.GET("/download/:cid", func(c *gin.Context) {
		cid := c.Param("cid")
		var model FileMetadataModel
		if err := db.First(&model, "cid = ?", cid).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{
					"code":    http.StatusNotFound,
					"message": "File metadata not found",
				})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{
					"code":    http.StatusInternalServerError,
					"message": fmt.Sprintf("Database error: %v", err),
				})
			}
			return
		}
		var shards []storage.Shard
		if err := json.Unmarshal(model.Shards, &shards); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    http.StatusInternalServerError,
				"message": fmt.Sprintf("Error parsing shards: %v", err),
			})
			return
		}
		metadata := storage.FileMetadata{
			CID:      model.CID,
			FileName: model.FileName,
			Shards:   shards,
		}
		outputPath := filepath.Join(os.TempDir(), model.FileName)
		if err := storage.DownloadFile(metadata.Shards, outputPath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    http.StatusInternalServerError,
				"message": fmt.Sprintf("Error reconstructing file: %v", err),
			})
			return
		}
		c.FileAttachment(outputPath, model.FileName)
	})

	addr := ":" + port
	log.Printf("Server running on %s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
