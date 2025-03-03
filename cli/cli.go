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
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ArguableExorcist8/desvault-storage-node/auth"
	"github.com/ArguableExorcist8/desvault-storage-node/encryption"
	"github.com/ArguableExorcist8/desvault-storage-node/network"
	"github.com/ArguableExorcist8/desvault-storage-node/rewards"
	"github.com/ArguableExorcist8/desvault-storage-node/setup"
	"github.com/ArguableExorcist8/desvault-storage-node/storage"
	"github.com/ArguableExorcist8/desvault-storage-node/utils"

	"github.com/gin-gonic/gin"
	"github.com/quic-go/quic-go"
	"github.com/spf13/cobra"
	"golang.org/x/time/rate"
	"gorm.io/driver/postgres"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// -----------------------------------------------------------------------------
// Constants and Global Variables
// -----------------------------------------------------------------------------

const (
	ModerateUploadSpeed   = "5–10 Mbps"
	ModerateDownloadSpeed = "10–25 Mbps"
	MaxFileSizeBytes      = 524288000 // 500 MB
)

var (
	port = getEnv("PORT", "8080")
	db   *gorm.DB
)

// -----------------------------------------------------------------------------
// Database Models and Converters
// -----------------------------------------------------------------------------

type FileMetadataModel struct {
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
		return FileMetadataResponse{}, fmt.Errorf("failed to unmarshal shards: %v", err)
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
		return FileMetadataModel{}, fmt.Errorf("failed to marshal shards: %v", err)
	}
	return FileMetadataModel{
		CID:      metadata.CID,
		FileName: metadata.FileName,
		Shards:   datatypes.JSON(shardsJSON),
	}, nil
}

// -----------------------------------------------------------------------------
// Helper Functions
// -----------------------------------------------------------------------------

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func generateAuthToken() string {
	token, err := utils.GenerateSecureToken(32)
	if err != nil {
		log.Printf("[ERROR] Failed to generate auth token: %v", err)
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return token
}

func generateCID() string {
	cid, err := utils.GenerateCID()
	if err != nil {
		log.Printf("[ERROR] Failed to generate CID: %v", err)
		return fmt.Sprintf("%016d", rand.Int63())
	}
	return cid
}

func formatFileSize(size int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case size < KB:
		return fmt.Sprintf("%d B", size)
	case size < MB:
		return fmt.Sprintf("%.2f KB", float64(size)/KB)
	case size < GB:
		return fmt.Sprintf("%.2f MB", float64(size)/MB)
	default:
		return fmt.Sprintf("%.2f GB", float64(size)/GB)
	}
}

// -----------------------------------------------------------------------------
// Database Initialization
// -----------------------------------------------------------------------------

func initDB() {
	dbURL := getEnv("DATABASE_URL", "host=localhost user=postgres password=postgres dbname=desvault_node port=5432 sslmode=disable")
	var err error
	db, err = gorm.Open(postgres.Open(dbURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatalf("[ERROR] Failed to connect to database: %v", err)
	}
	if err := db.AutoMigrate(&FileMetadataModel{}); err != nil {
		log.Fatalf("[ERROR] Failed to auto-migrate database: %v", err)
	}
	log.Println("[INFO] Database initialized successfully for storage node.")
}

// -----------------------------------------------------------------------------
// Middleware Definitions
// -----------------------------------------------------------------------------

var (
	visitors = make(map[string]*rate.Limiter)
	mtx      sync.Mutex
)

func getVisitor(ip string) *rate.Limiter {
	mtx.Lock()
	defer mtx.Unlock()
	if limiter, exists := visitors[ip]; exists {
		return limiter
	}
	limiter := rate.NewLimiter(rate.Every(time.Second), 5)
	visitors[ip] = limiter
	return limiter
}

func rateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if limiter := getVisitor(c.ClientIP()); !limiter.Allow() {
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
	token := c.GetHeader("Authorization")
	if !strings.HasPrefix(token, "Bearer ") {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"code":    http.StatusUnauthorized,
			"message": "Invalid or missing Authorization header",
		})
		return
	}
	token = strings.TrimPrefix(token, "Bearer ")
	if !auth.ValidateToken(token) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"code":    http.StatusUnauthorized,
			"message": "Invalid or expired token",
		})
		return
	}
	c.Next()
}

// -----------------------------------------------------------------------------
// Register with Master API
// -----------------------------------------------------------------------------

func registerWithMasterAPI(ads *network.AutoDiscoveryService) error {
	masterURL := getEnv("MASTER_API_URL", "http://localhost:9000")
	url := fmt.Sprintf("%s/register", masterURL)
	body, err := json.Marshal(map[string]string{
		"peer_id": ads.Host.ID().String(),
		"address": fmt.Sprintf("localhost:%s", port),
	})
	if err != nil {
		return fmt.Errorf("failed to marshal registration data: %w", err)
	}
	resp, err := http.Post(url, "application/json", strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("failed to register with master API: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("master API registration failed with status: %d", resp.StatusCode)
	}
	log.Println("[INFO] Successfully registered with master API")
	return nil
}

// -----------------------------------------------------------------------------
// CLI Banner Functions
// -----------------------------------------------------------------------------

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
	fmt.Println("Type your message and press Enter to send. Press Ctrl+C to exit.")
}

// -----------------------------------------------------------------------------
// IPFS Auto-Start Helpers
// -----------------------------------------------------------------------------

func isIPFSRunning() bool {
	resp, err := http.Get("http://localhost:5001/api/v0/version")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func startIPFSDaemon() error {
	if isIPFSRunning() {
		log.Println("[INFO] IPFS daemon already running.")
		return nil
	}
	log.Println("[INFO] Starting IPFS daemon...")
	cmd := exec.Command("ipfs", "daemon")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		if strings.Contains(err.Error(), "repo.lock") {
			log.Println("[INFO] IPFS daemon already running (repo.lock detected).")
			return nil
		}
		return fmt.Errorf("failed to start IPFS daemon: %w", err)
	}
	// Wait for the daemon to become responsive.
	for i := 0; i < 10; i++ {
		time.Sleep(2 * time.Second)
		if isIPFSRunning() {
			log.Println("[INFO] IPFS daemon started successfully.")
			return nil
		}
	}
	return fmt.Errorf("IPFS daemon failed to start within timeout")
}

// -----------------------------------------------------------------------------
// Node Startup and Service Functions
// -----------------------------------------------------------------------------

func startNode(ctx context.Context) {
	authToken := generateAuthToken()
	log.Printf("[INFO] New authentication token generated: %s", authToken)
	fmt.Println("Node ONLINE")
	fmt.Printf("Region: %s\n", setup.GetRegion())
	fmt.Printf("Uptime: %s\n", setup.GetUptime())
	storageGB, err := setup.ReadStorageAllocation()
	if err != nil {
		storageGB = 0
	}
	fmt.Printf("Storage: %d GB\n", storageGB)
	pointsPerHour := storageGB * 100
	fmt.Printf("Estimated Rewards: %d pts/hour\n", pointsPerHour)
	shardsCount := storage.GetShardCount()
	fmt.Printf("Shards: %d\n", shardsCount)

	if os.Getenv("SEED_NODE") == "true" {
		log.Println("[INFO] Running as seed node.")
	} else {
		log.Println("[INFO] Running as regular node. Discovering seed nodes...")
	}

	ads, err := network.InitializeNode(ctx)
	if err != nil {
		log.Fatalf("[ERROR] Failed to initialize networking: %v", err)
	}
	log.Printf("[INFO] Node started with Peer ID: %s", ads.Host.ID().String())

	// Wait for seed nodes if necessary.
	if os.Getenv("SEED_NODE") != "true" {
		timeout := 5 * time.Minute
		startWait := time.Now()
		for {
			peers := network.GetConnectedPeers()
			if len(peers) > 1 {
				log.Println("[INFO] Seed node discovered.")
				break
			}
			if time.Since(startWait) > timeout {
				log.Println("[WARN] Timeout waiting for seed node. Proceeding without seed.")
				break
			}
			log.Println("[INFO] Waiting for seed node...")
			time.Sleep(5 * time.Second)
		}
	}

	if err := setup.SetStartTime(); err != nil {
		log.Printf("[ERROR] Failed to set start time: %v", err)
	}

	fmt.Printf("Allocated Storage: %d GB\n", storageGB)
	fmt.Printf("Total Uptime: %s\n", setup.GetUptime())

	// Announce storage contribution.
	if err := ads.AnnounceStorage(storageGB); err != nil {
		log.Printf("[ERROR] Failed to announce storage: %v", err)
	} else {
		log.Printf("[INFO] Announced %d GB of storage to the network.", storageGB)
	}

	log.Println("[INFO] Storage service started.")
	log.Printf("[INFO] Node fully operational. Auth Token: %s", authToken)
}

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

	// API endpoints
	authorized.POST("/upload", func(c *gin.Context) {
		file, err := c.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": fmt.Sprintf("File not provided: %v", err)})
			return
		}
		if file.Size > MaxFileSizeBytes {
			c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "File exceeds maximum allowed size (500 MB)"})
			return
		}
		note := c.PostForm("note")
		if note == "" {
			note = "No note available"
		}
		tempPath := filepath.Join(os.TempDir(), file.Filename)
		if err := c.SaveUploadedFile(file, tempPath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": http.StatusInternalServerError, "message": fmt.Sprintf("Could not save file: %v", err)})
			return
		}
		defer os.Remove(tempPath)
		metadata, err := storage.UploadFileWithMetadata(tempPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": http.StatusInternalServerError, "message": fmt.Sprintf("Upload failed: %v", err)})
			return
		}
		// Generate a new CID and update metadata.
		newCID := generateCID()
		metadata.CID = newCID
		log.Printf("[INFO] Generated CID for file %s: %s", file.Filename, newCID)
		fileSizeStr := formatFileSize(file.Size)
		model, err := fileMetadataToModel(metadata)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": http.StatusInternalServerError, "message": fmt.Sprintf("Metadata conversion failed: %v", err)})
			return
		}
		model.Note = note
		model.FileSize = fileSizeStr
		model.CreatedAt = time.Now()
		if err := db.Create(&model).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": http.StatusInternalServerError, "message": fmt.Sprintf("Database error: %v", err)})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"code":          http.StatusOK,
			"message":       "File uploaded successfully",
			"data":          model,
			"uploadSpeed":   ModerateUploadSpeed,
			"downloadSpeed": ModerateDownloadSpeed,
			"maxFileSize":   "500 MB",
		})
	})

	authorized.GET("/files", func(c *gin.Context) {
		var models []FileMetadataModel
		if err := db.Find(&models).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": http.StatusInternalServerError, "message": fmt.Sprintf("Database error: %v", err)})
			return
		}
		responses := make([]FileMetadataResponse, 0, len(models))
		for _, model := range models {
			resp, err := modelToResponse(model)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"code": http.StatusInternalServerError, "message": fmt.Sprintf("Conversion error: %v", err)})
				return
			}
			responses = append(responses, resp)
		}
		c.JSON(http.StatusOK, gin.H{"files": responses})
	})

	authorized.GET("/download/:cid", func(c *gin.Context) {
		cid := c.Param("cid")
		var model FileMetadataModel
		if err := db.First(&model, "cid = ?", cid).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"code": http.StatusNotFound, "message": "File metadata not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"code": http.StatusInternalServerError, "message": fmt.Sprintf("Database error: %v", err)})
			}
			return
		}
		var shards []storage.Shard
		if err := json.Unmarshal(model.Shards, &shards); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": http.StatusInternalServerError, "message": fmt.Sprintf("Error parsing shards: %v", err)})
			return
		}
		metadata := storage.FileMetadata{
			CID:      model.CID,
			FileName: model.FileName,
			Shards:   shards,
		}
		outputPath := filepath.Join(os.TempDir(), model.FileName)
		if err := storage.DownloadFile(metadata.Shards, outputPath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": http.StatusInternalServerError, "message": fmt.Sprintf("Error reconstructing file: %v", err)})
			return
		}
		defer os.Remove(outputPath)
		c.FileAttachment(outputPath, model.FileName)
	})

	addr := ":" + port
	log.Printf("[INFO] Starting API server on %s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatalf("[ERROR] API server failed: %v", err)
	}
}

// -----------------------------------------------------------------------------
// Service Health Check
// -----------------------------------------------------------------------------

func getNodeStatus() string {
	ipfsStatus := "DOWN"
	if isIPFSRunning() {
		ipfsStatus = "UP"
	}
	peers := network.GetConnectedPeers()
	networkStatus := "LOW"
	if len(peers) > 0 {
		networkStatus = "GOOD"
	}
	dbStatus := "DOWN"
	if db != nil {
		if sqlDB, err := db.DB(); err == nil && sqlDB.Ping() == nil {
			dbStatus = "UP"
		}
	}
	return fmt.Sprintf("IPFS: %s, Network: %s (%d peers), Database: %s", ipfsStatus, networkStatus, len(peers), dbStatus)
}

func ShowNodeStatus() {
	storageGB, err := setup.ReadStorageAllocation()
	if err != nil {
		storageGB = 0
	}
	uptime := setup.GetUptime()
	status := getNodeStatus()
	pointsPerHour := storageGB * 100
	totalPoints := int(time.Since(setup.GetStartTimeOrNow()).Hours()) * pointsPerHour

	fmt.Println("\n=====================")
	fmt.Println(" DesVault Node Status")
	fmt.Println("=====================")
	fmt.Printf("Status: %s\n", status)
	fmt.Printf("Total Uptime: %s\n", uptime)
	fmt.Printf("Storage Contributed: %d GB\n", storageGB)
	fmt.Printf("Total Points: %d pts\n", totalPoints)
}

// -----------------------------------------------------------------------------
// CLI Commands and Main Execution
// -----------------------------------------------------------------------------

var rootCmd = &cobra.Command{
	Use:   "desvault",
	Short: "DesVault Storage Node CLI",
	Long:  "Manage and monitor your DesVault Storage Node",
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Start the Storage Node",
	Run: func(cmd *cobra.Command, args []string) {
		printCLIBanner()
		setup.FirstTimeSetup()

		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("[ERROR] Could not determine home directory: %v", err)
		}
		desvaultDir := filepath.Join(home, ".desvault")
		if err := os.MkdirAll(desvaultDir, 0755); err != nil {
			log.Fatalf("[ERROR] Failed to create directory: %v", err)
		}
		pidFile := filepath.Join(desvaultDir, "node.pid")
		pid := os.Getpid()
		if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d\n", pid)), 0644); err != nil {
			log.Fatalf("[ERROR] Failed to write PID file: %v", err)
		}
		defer os.Remove(pidFile)

		if err := startIPFSDaemon(); err != nil {
			log.Fatalf("[ERROR] Failed to start IPFS daemon: %v", err)
		}

		initDB()
		storage.InitializeStorage()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ads, err := network.InitializeNode(ctx)
		if err != nil {
			log.Fatalf("[ERROR] Failed to initialize network: %v", err)
		}

		go startNode(ctx)
		go startAPIServer()

		// Register with master API
		if err := registerWithMasterAPI(ads); err != nil {
			log.Printf("[WARN] Failed to register with master API: %v", err)
		} else {
			log.Println("[INFO] Connected to DesVault network via master API")
		}

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		log.Println("[INFO] Shutting down node gracefully...")
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the Storage Node",
	Run: func(cmd *cobra.Command, args []string) {
		printCLIBanner()
		home, err := os.UserHomeDir()
		if err != nil {
			log.Printf("[ERROR] Could not determine home directory: %v", err)
			return
		}
		pidFile := filepath.Join(home, ".desvault", "node.pid")
		data, err := os.ReadFile(pidFile)
		if err != nil {
			fmt.Println("[INFO] No running node found (PID file missing).")
			return
		}
		pidStr := strings.TrimSpace(string(data))
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			fmt.Printf("[ERROR] Invalid PID: %s\n", pidStr)
			return
		}
		if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
			fmt.Printf("[ERROR] Failed to stop process %d: %v\n", pid, err)
		} else {
			fmt.Printf("[INFO] Stopped process %d\n", pid)
			if err := os.Remove(pidFile); err != nil {
				log.Printf("[ERROR] Failed to remove PID file: %v", err)
			}
			fmt.Println("[INFO] Node stopped successfully.")
		}
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check Storage Node status",
	Run: func(cmd *cobra.Command, args []string) {
		printCLIBanner()
		fmt.Println("[INFO] Checking node status...")
		ShowNodeStatus()
	},
}

var storageCmd = &cobra.Command{
	Use:   "storage",
	Short: "View/Edit allocated storage",
	Run: func(cmd *cobra.Command, args []string) {
		printCLIBanner()
		storageGB, err := setup.ReadStorageAllocation()
		if err != nil {
			storageGB = 0
		}
		fmt.Printf("[INFO] Current Storage: %d GB\n", storageGB)
		fmt.Print("[?] Enter new storage allocation (or 0 to keep current): ")
		var newStorage int
		fmt.Scan(&newStorage)
		if newStorage > 0 {
			setup.SetStorageAllocation(newStorage)
			fmt.Printf("[INFO] Storage allocation updated to %d GB\n", newStorage)
		}
	},
}

var memeCmd = &cobra.Command{
	Use:   "meme",
	Short: "Display a random ASCII meme",
	Run: func(cmd *cobra.Command, args []string) {
		printCLIBanner()
		bigMemes := []string{
			"(╯°□°）╯︵ ┻━┻",
			"ʕ•ᴥ•ʔ",
			"¯\\_(ツ)_/¯",
			"(ง'̀-'́)ง",
			"(ಥ_ಥ)",
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
	fmt.Println("[INFO] Entering DesVault Chatroom...")
	fmt.Println("[INFO] Type 'exit' to leave.")
	peerID := network.GetNodePeerID()
	peers := network.GetConnectedPeers()
	if len(peers) == 0 {
		fmt.Println("[WARN] No peers found.")
		return
	}
	fmt.Printf("[INFO] Connected Peers: %d\n", len(peers))
	for {
		fmt.Print("[You]: ")
		msg, _ := reader.ReadString('\n')
		msg = strings.TrimSpace(msg)
		if msg == "exit" {
			fmt.Println("[INFO] Exiting chat.")
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
		log.Printf("[ERROR] Failed to load rewards: %v", err)
		fmt.Println("[ERROR] Could not retrieve rewards.")
		return
	}
	reward, exists := rewardsData[nodeID]
	if !exists {
		fmt.Println("[INFO] No rewards found for this node.")
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
	Short: "Start a secure QUIC channel using TLS",
	Run: func(cmd *cobra.Command, args []string) {
		certFile := "server.crt"
		keyFile := "server.key"
		tlsConfig, err := encryption.CreateTLSConfig(certFile, keyFile)
		if err != nil {
			log.Fatalf("[ERROR] Failed to create TLS config: %v", err)
		}
		quicConfig := &quic.Config{
			EnableDatagrams: true,
			KeepAlivePeriod: 30 * time.Second,
		}
		addr := "0.0.0.0:4242"
		if err := encryption.SecureChannelWithTLS(addr, tlsConfig, quicConfig); err != nil {
			log.Fatalf("[ERROR] Secure channel failed: %v", err)
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	rootCmd.AddCommand(runCmd, stopCmd, statusCmd, storageCmd, memeCmd, chatCmd, rewardsCmd, tlsCmd)
	if err := rootCmd.Execute(); err != nil {
		log.Printf("[ERROR] CLI execution failed: %v", err)
		os.Exit(1)
	}
}
