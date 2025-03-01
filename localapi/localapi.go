package localapi

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/ArguableExorcist8/desvault-storage-node/network"
)

// StatusResponse defines the JSON structure for /status.
type StatusResponse struct {
	PeerID string   `json:"peer_id"`
	Peers  []string `json:"peers"`
}

// statusHandler returns the node's status.
func statusHandler(w http.ResponseWriter, r *http.Request) {
	response := StatusResponse{
		PeerID: network.GetNodePeerID(),
		Peers:  network.GetConnectedPeers(),
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("[ERROR] Failed to encode status response: %v", err)
	}
}

// StartServer launches a simple HTTP server.
func StartServer(port string) {
	http.HandleFunc("/status", statusHandler)
	log.Printf("[INFO] Local API server listening on %s", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("[ERROR] Local API server failed: %v", err)
	}
}
