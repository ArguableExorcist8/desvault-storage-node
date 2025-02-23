package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"DesVault/storage-node/encryption"
	"DesVault/storage-node/network"
	"DesVault/storage-node/rewards"
	"DesVault/storage-node/setup"
	"DesVault/storage-node/storage"
	"DesVault/storage-node/utils"

	"github.com/gin-gonic/gin"
	"github.com/quic-go/quic-go"
	"github.com/spf13/cobra"
	"golang.org/x/time/rate"
	"gorm.io/driver/postgres"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ========================
// Configuration Constants
// ========================

const (
	ModerateUploadSpeed   = "5–10 Mbps"  // Informational: moderate upload speed.
	ModerateDownloadSpeed = "10–25 Mbps" // Informational: moderate download speed.
	MaxFileSizeBytes      = 524288000    // 500 MB in bytes.
)

// ========================
// Shared Helpers & Global Variables
// ========================

// getEnv returns the value of an environment variable or a default.
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

var port = getEnv("PORT", "8080")

// generateAuthToken simulates generating an authentication token.
func generateAuthToken() string {
	return fmt.Sprintf("%x", time.Now().UnixNano())
}

// generateCID returns a random 16-digit numeric string.
func generateCID() string {
	digits := make([]byte, 16)
	for i := 0; i < 16; i++ {
		digits[i] = '0' + byte(rand.Intn(10))
	}
	return string(digits)
}

// formatFileSize converts a file size (in bytes) to a human‑readable string.
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

// ========================
// Database Models & Converters
// ========================

// FileMetadataModel is the DB model for file metadata.
type FileMetadataModel struct {
	CID       string         `gorm:"column:cid;primaryKey;not null;size:255" json:"cid"`
	FileName  string         `gorm:"size:255" json:"fileName"`
	Note      string         `gorm:"size:255" json:"note"`
	FileSize  string         `gorm:"size:255" json:"fileSize"`
	Shards    datatypes.JSON `gorm:"type:jsonb" json:"shards"`
	CreatedAt time.Time      `json:"createdAt"`
}

// FileMetadataResponse is used for API responses.
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

// Global database handle.
var db *gorm.DB

func initDB() {
	dbURL := os.Getenv("DATABASE_URL")
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
	// For development, drop the table if it exists.
	if db.Migrator().HasTable(&FileMetadataModel{}) {
		if err := db.Migrator().DropTable(&FileMetadataModel{}); err != nil {
			log.Fatalf("failed to drop table: %v", err)
		}
	}
	if err := db.AutoMigrate(&FileMetadataModel{}); err != nil {
		log.Fatalf("failed to auto-migrate database: %v", err)
	}
}

// ========================
// Middleware Definitions
// ========================

var visitors = make(map[string]*rate.Limiter)
var mtx sync.Mutex

// getVisitor returns the rate limiter for the given IP, creating one if necessary.
func getVisitor(ip string) *rate.Limiter {
	mtx.Lock()
	defer mtx.Unlock()
	limiter, exists := visitors[ip]
	if !exists {
		// 1 request per second with a burst of 5.
		limiter = rate.NewLimiter(1, 5)
		visitors[ip] = limiter
	}
	return limiter
}

func rateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		limiter := getVisitor(ip)
		if !limiter.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code":    http.StatusTooManyRequests,
				"message": "Rate limit exceeded",
			})
			return
		}
		c.Next()
	}
}

func secureHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("X-Content-Type-Options", "nosniff")
		c.Writer.Header().Set("X-Frame-Options", "DENY")
		c.Writer.Header().Set("Content-Security-Policy", "default-src 'self'")
		c.Next()
	}
}

func authMiddleware(c *gin.Context) {
	if c.GetHeader("Authorization") != "Bearer secret123" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"code":    http.StatusUnauthorized,
			"message": "Unauthorized",
		})
		return
	}
	c.Next()
}

// ========================
// CLI Banner Functions
// ========================

func printCLIBanner() {
	fmt.Println(`
██████╗ ███████╗███████╗██╗   ██╗ █████╗ ██╗   ██╗██╗ ████████╗
██╔══██╗██╔════╝██╔════╝██║   ██║██╔══██╗██║   ██║██║ ╚══██╔══╝
██║  ██║█████╗  ███████╗██║   ██║███████║██║   ██║██║    ██║   
██║  ██║██╔══╝  ╚════██║██║   ██║██╔══██║██║   ██║██║    ██║   
██████╔╝███████╗███████║╚██████╔╝██║  ██║╚██████╔╝███████║   
╚═════╝ ╚══════╝╚══════╝ ╚═════╝ ╚═╝  ╚═╝ ╚═════╝ ╚══════╝
`)
	fmt.Println("\nDesVault CLI")
}

func printChatBanner() {
	fmt.Println(`
██████╗ ███████╗███████╗██╗   ██╗ █████╗ ██╗   ██╗██╗ ████████╗
██╔══██╗██╔════╝██╔════╝██║   ██║██╔══██╗██║   ██║██║ ╚══██╔══╝
██║  ██║█████╗  ███████╗██║   ██║███████║██║   ██║██║    ██║   
██║  ██║██╔══╝  ╚════██║██║   ██║██╔══██║██║   ██║██║    ██║   
██████╔╝███████╗███████║╚██████╔╝██║  ██║╚██████╔╝███████║   
╚═════╝ ╚══════╝╚══════╝ ╚═════╝ ╚═╝  ╚═╝ ╚═════╝ ╚══════╝
`)
	fmt.Println("\nDesVault CLI Chatroom")
	fmt.Println("-------------------------")
	fmt.Println("Type your message and press Enter to send. Press Ctrl+C to exit the chatroom.")
}

// ========================
// IPFS Auto-Start Helpers
// ========================

func isIPFSRunning() bool {
	resp, err := http.Get("http://localhost:5001/api/v0/version")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

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
		if strings.Contains(err.Error(), "repo.lock") {
			log.Println("IPFS daemon already running (repo.lock detected).")
			return nil
		}
		return fmt.Errorf("failed to start IPFS daemon: %w", err)
	}
	// Wait up to 20 seconds for IPFS to become available.
	for i := 0; i < 10; i++ {
		time.Sleep(2 * time.Second)
		if isIPFSRunning() {
			log.Println("IPFS daemon started successfully.")
			return nil
		}
	}
	return fmt.Errorf("IPFS daemon did not start in time")
}

// ========================
// Node Startup Functions
// ========================

func startNode(ctx context.Context) {
	authToken := generateAuthToken()
	log.Printf("New authentication token generated: %s", authToken)
	fmt.Println("Node ONLINE")
	fmt.Printf("Region: %s\n", setup.GetRegion())
	fmt.Printf("Uptime: %s\n", setup.GetUptime())
	storageGB, _ := setup.ReadStorageAllocation()
	fmt.Printf("Storage: %d GB\n", storageGB)
	fmt.Printf("Points: %d\n", 0) // Placeholder.
	fmt.Printf("Shards: %d\n", 0) // Placeholder.

	if os.Getenv("SEED_NODE") == "true" {
		log.Println("This node is running as the seed node.")
	} else {
		log.Println("This node is running as a regular node. Attempting to discover seed nodes...")
	}

	ads, err := network.InitializeNode(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize networking: %v", err)
	}
	log.Printf("Node started with Peer ID: %s", ads.Host.ID().String())

	if os.Getenv("SEED_NODE") != "true" {
		timeout := 5 * time.Minute
		startWait := time.Now()
		for {
			peers := network.GetConnectedPeers()
			if len(peers) > 1 {
				log.Println("Seed node discovered.")
				break
			}
			if time.Since(startWait) > timeout {
				log.Println("Timeout waiting for seed node. Proceeding without seed node.")
				break
			}
			log.Println("No seed node found yet. Waiting 5 seconds...")
			time.Sleep(5 * time.Second)
		}
	}

	if err := setup.SetStartTime(); err != nil {
		log.Printf("[!] Failed to set start time: %v", err)
	}

	fmt.Printf("Allocated Storage: %d GB\n", storageGB)
	fmt.Printf("Estimated Rewards: %d pts/hour\n", storageGB*100)

	log.Println("Storage service started")
	log.Printf("Storage Node started successfully. Auth Token: %s", authToken)
	log.Println("mDNS service started")
	log.Println("DHT bootstrap completed")
	log.Println("Peer discovery initialized")
}

// ========================
// API Server Startup Function
// ========================

func startAPIServer() {
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

		// Check file size (must not exceed 500 MB)
		if file.Size > MaxFileSizeBytes {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    http.StatusBadRequest,
				"message": "File exceeds maximum allowed size (500 MB)",
			})
			return
		}

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
			"uploadSpeed":   ModerateUploadSpeed,
			"downloadSpeed": ModerateDownloadSpeed,
			"maxFileSize":   "500 MB",
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
		// Use FileAttachment to serve the file with its original filename.
		c.FileAttachment(outputPath, model.FileName)
	})

	addr := ":" + port
	log.Printf("API server running on %s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

// ========================
// Additional Helper Functions (Enhanced Service Health Check)
// ========================

func getNodeStatus() string {
	// Check IPFS daemon status.
	ipfsStatus := "DOWN"
	if isIPFSRunning() {
		ipfsStatus = "UP"
	}

	// Check network connectivity.
	peers := network.GetConnectedPeers()
	networkStatus := "LOW"
	if len(peers) > 0 {
		networkStatus = "GOOD"
	}

	// Check database connectivity.
	dbStatus := "UNKNOWN"
	if db != nil {
		if sqlDB, err := db.DB(); err == nil {
			if err := sqlDB.Ping(); err == nil {
				dbStatus = "UP"
			} else {
				dbStatus = "DOWN"
			}
		}
	}

	return fmt.Sprintf("IPFS: %s, Network: %s (%d peers), Database: %s", ipfsStatus, networkStatus, len(peers), dbStatus)
}

// ========================
// CLI Commands
// ========================

var rootCmd = &cobra.Command{
	Use:   "desvault",
	Short: "DesVault Storage Node CLI",
	Long:  "Manage and monitor your DesVault Storage Node",
}

var storageCmd = &cobra.Command{
	Use:   "storage",
	Short: "View/Edit allocated storage",
	Run: func(cmd *cobra.Command, args []string) {
		printCLIBanner()
		storageGB, _ := setup.ReadStorageAllocation()
		fmt.Printf("[+] Current Storage: %d GB\n", storageGB)
		var newStorage int
		fmt.Print("[?] Enter new storage allocation (or 0 to keep current): ")
		fmt.Scan(&newStorage)
		if newStorage > 0 {
			setup.SetStorageAllocation(newStorage)
		}
	},
}
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Start the Storage Node",
	Run: func(cmd *cobra.Command, args []string) {
		printCLIBanner()
		// Run first-time setup routines.
		setup.FirstTimeSetup()

		// Ensure the .desvault directory exists.
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Println("[!] Could not determine home directory:", err)
			return
		}
		desvaultDir := filepath.Join(home, ".desvault")
		if err := os.MkdirAll(desvaultDir, 0755); err != nil {
			fmt.Println("[!] Failed to create directory:", err)
			return
		}
		// Write the current PID to the node.pid file.
		pidFile := filepath.Join(desvaultDir, "node.pid")
		pid := os.Getpid()
		f, err := os.OpenFile(pidFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Println("[!] Could not open PID file:", err)
			return
		}
		defer f.Close()
		if _, err := f.WriteString(fmt.Sprintf("%d\n", pid)); err != nil {
			fmt.Println("[!] Failed to write PID:", err)
		}

		// Start the IPFS daemon automatically.
		if err := startIPFSDaemon(); err != nil {
			log.Fatalf("Failed to start IPFS daemon: %v", err)
		}

		// Initialize the database and storage.
		initDB()
		storage.InitializeStorage()

		// Start the Node (network, peer discovery, etc.)
		ctx := context.Background()
		startNode(ctx)

		// Start the API server in the same process on its own port.
		startAPIServer()
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the Storage Node",
	Run: func(cmd *cobra.Command, args []string) {
		printCLIBanner()
		fmt.Println("[!] Stopping the node...")

		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Println("[!] Could not determine home directory:", err)
			return
		}
		pidFile := filepath.Join(home, ".desvault", "node.pid")
		data, err := os.ReadFile(pidFile)
		if err != nil {
			fmt.Println("[!] No running node found (PID file missing).")
			return
		}
		pids := strings.Split(strings.TrimSpace(string(data)), "\n")
		for _, pidStr := range pids {
			if pidStr == "" {
				continue
			}
			if _, err := strconv.Atoi(pidStr); err != nil {
				fmt.Printf("[!] Invalid PID found: %s\n", pidStr)
				continue
			}
			if err := exec.Command("kill", "-SIGTERM", pidStr).Run(); err != nil {
				fmt.Printf("[!] Failed to stop process %s: %v\n", pidStr, err)
			} else {
				fmt.Printf("[+] Stopped process %s\n", pidStr)
			}
		}
		if err := os.Remove(pidFile); err != nil {
			fmt.Printf("[!] Failed to remove PID file: %v\n", err)
		} else {
			fmt.Println("[+] Node stopped successfully.")
		}
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check Storage Node status",
	Run: func(cmd *cobra.Command, args []string) {
		printCLIBanner()
		fmt.Println("[+] Checking node status...")
		ShowNodeStatus()
	},
}

func ShowNodeStatus() {
	storageGB, pointsPerHour := setup.ReadStorageAllocation()
	uptime := setup.GetUptime()
	status := getNodeStatus()
	totalPoints := int(time.Since(setup.GetStartTimeOrNow()).Hours()) * pointsPerHour

	fmt.Println("\n=====================")
	fmt.Println(" DesVault Node Status")
	fmt.Println("=====================")
	fmt.Printf("Status: %s\n", status)
	fmt.Printf("Total Uptime: %s\n", uptime)
	fmt.Printf("Storage Contributed: %d GB\n", storageGB)
	fmt.Printf("Total Points: %d pts\n", totalPoints)
}

var memeCmd = &cobra.Command{
	Use:   "meme",
	Short: "Display a random ASCII meme",
	Run: func(cmd *cobra.Command, args []string) {
		printCLIBanner()
		bigMemes := []string{
			`
(╯°□°）╯︵ ┻━┻
__________________
|                |
|   TABLE FLIP   |
|  "FUCK THIS"   |
|________________|
`,
			`
ʕ•ᴥ•ʔ
/ o o \
(  "  )
\~(*)~/
/   \
( U U )
`,
			`
¯\_(ツ)_/¯
`,
			`
(╯°□°）╯︵ ʞooqǝɔ
`,
			`
(ง'̀-'́)ง
`,
			`
(ಥ_ಥ)
`,
			`
(¬‿¬)
`,
			`
(ʘ‿ʘ)
`,
			`
(☞ﾟヮﾟ)☞
`,
			`
(ノಠ益ಠ)ノ彡┻━┻
`,
			`
┬─┬ノ( º _ ºノ)
`,
		}
		index := time.Now().Unix() % int64(len(bigMemes))
		fmt.Println(bigMemes[index])
	},
}

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Join the P2P chatroom",
	Run: func(cmd *cobra.Command, args []string) {
		printChatBanner()
		startChat()
	},
}

func startChat() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("[*] Entering DesVault Chatroom...")
	fmt.Println("[*] Type 'exit' to leave.")
	peerID := network.GetNodePeerID()
	peers := network.GetConnectedPeers()
	if len(peers) == 0 {
		fmt.Println("[!] No peers found. Try again later.")
		return
	}
	fmt.Printf("[+] Connected Peers: %d\n", len(peers))
	for {
		fmt.Print("[You]: ")
		msg, _ := reader.ReadString('\n')
		msg = strings.TrimSpace(msg)
		if msg == "exit" {
			fmt.Println("[!] Exiting chat.")
			break
		}
		for _, p := range peers {
			network.SendMessage(p, fmt.Sprintf("[%s]: %s", peerID, msg))
		}
		fmt.Printf("[%s]: %s\n", peerID, msg)
	}
}

var rewardsCmd = &cobra.Command{
	Use:   "rewards",
	Short: "Check earned rewards",
	Run: func(cmd *cobra.Command, args []string) {
		printCLIBanner()
		nodeID := utils.GetNodeID()
		CheckRewards(nodeID)
	},
}

func CheckRewards(nodeID string) {
	rewardsData, err := rewards.LoadRewards()
	if err != nil {
		log.Println("[!] Failed to load rewards:", err)
		fmt.Println("[-] Could not retrieve rewards. Ensure the node is active.")
		return
	}
	reward, exists := rewardsData[nodeID]
	if !exists {
		fmt.Println("[-] No rewards found for this node.")
		return
	}
	fmt.Printf("\n🎖️ [Reward Summary for Node %s]\n", reward.NodeID)
	fmt.Printf("🔹 Node Type: %s\n", reward.NodeType)
	fmt.Printf("🔹 Base Points: %d\n", reward.BasePoints)
	fmt.Printf("🔹 Multiplier: %.1fx\n", reward.Multiplier)
	fmt.Printf("🏆 Total Points: %.2f\n", reward.TotalPoints)
}

var tlsCmd = &cobra.Command{
	Use:   "tls",
	Short: "Start a secure QUIC channel using TLS directly",
	Run: func(cmd *cobra.Command, args []string) {
		certFile := "server.crt"
		keyFile := "server.key"
		tlsConfig, err := encryption.CreateTLSConfig(certFile, keyFile)
		if err != nil {
			log.Fatalf("Failed to create TLS config: %v", err)
		}

		quicConfig := &quic.Config{
			EnableDatagrams: true,
			KeepAlivePeriod: 30 * time.Second,
		}

		addr := "0.0.0.0:4242"
		if err := encryption.SecureChannelWithTLS(addr, tlsConfig, quicConfig); err != nil {
			log.Fatalf("Error in secure channel: %v", err)
		}
	},
}

// ========================
// MAIN EXECUTION
// ========================

func Execute() {
	rootCmd.AddCommand(runCmd, stopCmd, statusCmd, storageCmd, memeCmd, chatCmd, rewardsCmd, tlsCmd)
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
