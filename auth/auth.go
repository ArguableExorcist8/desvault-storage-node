package auth

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

var (
	// token holds the currently active authentication token.
	token string
	// tokenLock protects concurrent access to the token.
	tokenLock sync.Mutex

	// walletAddress holds the EVM wallet address.
	walletAddress string
	// walletLock protects concurrent access to the wallet address.
	walletLock sync.Mutex
)

// LoadWalletAddress loads the EVM wallet address from an environment variable or, if not set,
// prompts the user to enter it. The address is then stored globally.
func LoadWalletAddress() string {
	walletLock.Lock()
	defer walletLock.Unlock()

	// Check if the wallet address is provided via an environment variable.
	addr := os.Getenv("EVM_WALLET_ADDRESS")
	if addr != "" {
		walletAddress = addr
		log.Println("[INFO] Loaded EVM wallet address from environment variable.")
		return walletAddress
	}

	// Prompt the user to enter the EVM wallet address.
	fmt.Print("Enter your EVM wallet address: ")
	reader := bufio.NewReader(os.Stdin)
	addr, err := reader.ReadString('\n')
	if err != nil {
		log.Fatalf("Failed to read wallet address: %v", err)
	}
	addr = strings.TrimSpace(addr)
	if addr == "" {
		log.Fatal("No wallet address provided; exiting.")
	}
	walletAddress = addr
	log.Println("[INFO] EVM wallet address stored.")
	return walletAddress
}

// GenerateToken creates a secure authentication token for the node.
// In production, you might load this from configuration or an environment variable.
func GenerateToken() string {
	tokenLock.Lock()
	defer tokenLock.Unlock()

	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		log.Fatalf("Failed to generate token: %v", err)
	}
	token = hex.EncodeToString(bytes)
	log.Println("[INFO] New authentication token generated:", token)
	return token
}

// ValidateToken checks whether a provided token exactly matches the node's token.
func ValidateToken(providedToken string) bool {
	tokenLock.Lock()
	defer tokenLock.Unlock()
	return providedToken == token
}

// ValidateRequest is a Gin middleware that validates incoming requests based on the Authorization header.
// The expected header should be in the format: "Bearer <token>"
func ValidateRequest(c *gin.Context) {
	providedToken := c.GetHeader("Authorization")
	if providedToken == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: No token provided"})
		c.Abort()
		return
	}

	// If the header is in "Bearer <token>" format, extract the token.
	if len(providedToken) > 7 && providedToken[:7] == "Bearer " {
		providedToken = providedToken[7:]
	}

	if !ValidateToken(providedToken) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: Invalid token"})
		c.Abort()
		return
	}

	c.Next() // Proceed if the token is valid.
}
