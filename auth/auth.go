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

	"github.com/ethereum/go-ethereum/accounts/keystore"
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

	// ks is the global keystore instance.
	ks *keystore.KeyStore
)

// InitializeWallet initializes the wallet using the provided configuration.
// In production, walletConfig is the path to the keystore directory.
// It loads the wallet, unlocks the first account using the password from the environment variable,
// and stores the wallet address for later use.
func InitializeWallet(walletConfig string) error {
	if walletConfig == "" {
		return fmt.Errorf("walletConfig is empty")
	}

	// Create a new keystore instance.
	ks = keystore.NewKeyStore(walletConfig, keystore.StandardScryptN, keystore.StandardScryptP)
	if len(ks.Accounts()) == 0 {
		return fmt.Errorf("no accounts found in wallet at %s", walletConfig)
	}

	// Retrieve the wallet password from the environment.
	pass := os.Getenv("WALLET_PASSWORD")
	if pass == "" {
		return fmt.Errorf("WALLET_PASSWORD environment variable not set")
	}

	// Unlock the first account.
	account := ks.Accounts()[0]
	if err := ks.Unlock(account, pass); err != nil {
		return fmt.Errorf("failed to unlock wallet: %v", err)
	}

	// Store the wallet address globally.
	walletAddress = account.Address.Hex()
	log.Println("[INFO] Wallet initialized and unlocked for account:", walletAddress)
	return nil
}

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
// This token can be used to authenticate API requests.
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
// The expected header format is: "Bearer <token>"
func ValidateRequest(c *gin.Context) {
	providedToken := c.GetHeader("Authorization")
	if providedToken == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: No token provided"})
		c.Abort()
		return
	}

	// If the header is in "Bearer <token>" format, extract the token.
	if len(providedToken) > 7 && strings.HasPrefix(providedToken, "Bearer ") {
		providedToken = providedToken[7:]
	}

	if !ValidateToken(providedToken) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: Invalid token"})
		c.Abort()
		return
	}

	c.Next() // Proceed if the token is valid.
}
