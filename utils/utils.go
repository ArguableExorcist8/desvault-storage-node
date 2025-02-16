package utils

import (
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/google/uuid"
)

// GetNodeID returns a persistent node identifier.
// It checks for a "node_id" file in the current directory; if missing, it generates and saves one.
func GetNodeID() string {
	const idFile = "node_id"
	if _, err := os.Stat(idFile); err == nil {
		data, err := ioutil.ReadFile(idFile)
		if err != nil {
			log.Printf("[!] Failed to read node_id file: %v", err)
			return ""
		}
		return strings.TrimSpace(string(data))
	} else {
		newID := uuid.New().String()
		err := ioutil.WriteFile(idFile, []byte(newID), 0644)
		if err != nil {
			log.Printf("[!] Failed to write node_id file: %v", err)
		}
		return newID
	}
}
