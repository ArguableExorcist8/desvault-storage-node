package auth

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
)

var (
	// token holds the currently active authentication token.
	token string
	// tokenLock protects concurrent access to the token.
	tokenLock sync.Mutex
)

// GenerateToken creates a secure authentication token for the node.
// 
// 
// whaaaat a DRAGGGGGGGGGGGGGG
// i need to load this from configuration or an environment variable.
func GenerateToken() string {
	tokenLock.Lock()
	defer tokenLock.Unlock()

	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		log.Fatalf("Failed to generate token: %v", err)
	}
	token = hex.EncodeToString(bytes)
	log.Println("New authentication token generated:", token)
	return token
}

// ValidateToken checks whether a provided token exactly matches the node’s token.
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