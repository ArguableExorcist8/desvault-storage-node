package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Configuration Helpers
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

var (
	port = getEnv("MASTER_PORT", "9000")
	db   *gorm.DB
)

// Database Models
type Node struct {
	PeerID    string         `gorm:"primaryKey;not null;size:255" json:"peer_id"`
	Address   string         `gorm:"size:255" json:"address"`
	RegisteredAt time.Time `json:"registered_at"`
}

// Database Initialization
func initDB() {
	dbURL := getEnv("MASTER_DATABASE_URL", "host=localhost user=postgres password=postgres dbname=desvault_master port=5432 sslmode=disable")
	var err error
	db, err = gorm.Open(postgres.Open(dbURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatalf("[ERROR] Failed to connect to master database: %v", err)
	}
	if err := db.AutoMigrate(&Node{}); err != nil {
		log.Fatalf("[ERROR] Failed to auto-migrate master database: %v", err)
	}
	log.Println("[INFO] Master database initialized successfully.")
}

// Main
func main() {
	initDB()

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// Registration Endpoint
	router.POST("/register", func(c *gin.Context) {
		var node struct {
			PeerID  string `json:"peer_id"`
			Address string `json:"address"`
		}
		if err := c.BindJSON(&node); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}
		newNode := Node{
			PeerID:    node.PeerID,
			Address:   node.Address,
			RegisteredAt: time.Now(),
		}
		if err := db.Create(&newNode).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register node"})
			log.Printf("[ERROR] Failed to save node %s: %v", node.PeerID, err)
			return
		}
		log.Printf("[INFO] Registered node: %s at %s", node.PeerID, node.Address)
		c.JSON(http.StatusOK, gin.H{"status": "registered", "peer_id": node.PeerID})
	})

	// List Nodes Endpoint (for debugging)
	router.GET("/nodes", func(c *gin.Context) {
		var nodes []Node
		if err := db.Find(&nodes).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch nodes"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"nodes": nodes})
	})

	addr := ":" + port
	log.Printf("[INFO] Master API running on %s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatalf("[ERROR] Master API failed: %v", err)
	}
}