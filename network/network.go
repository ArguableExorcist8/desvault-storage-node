package network

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p"
	gonetwork "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	mdns "github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"

	"github.com/ArguableExorcist8/desvault-storage-node/setup"
)

const ChatProtocolID = "/desvault/chat/1.0.0"

// -----------------------------------------------------------------------------
// Libp2p-Based Peer Discovery
// -----------------------------------------------------------------------------

// Notifee implements the mdns.Notifee interface for mDNS discovery.
type Notifee struct {
	Host host.Host
}

// HandlePeerFound is invoked when a peer is discovered via mDNS.
func (n *Notifee) HandlePeerFound(pi peer.AddrInfo) {
	log.Printf("[mDNS] Discovered peer: %s", pi.ID.String())
	// Attempt to connect to the discovered peer.
	if err := n.Host.Connect(context.Background(), pi); err != nil {
		log.Printf("[ERROR] Failed to connect to peer %s: %v", pi.ID.String(), err)
	} else {
		log.Printf("[INFO] Connected to peer: %s", pi.ID.String())
	}
}

// AutoDiscoveryService encapsulates the libp2p host, DHT instance, and PubSub.
type AutoDiscoveryService struct {
	Host   host.Host
	DHT    *dht.IpfsDHT
	PubSub *pubsub.PubSub
}

// Start initializes mDNS discovery, bootstraps the DHT, and starts the mDNS service.
func (s *AutoDiscoveryService) Start(ctx context.Context) {
	// Initialize mDNS service.
	mdnsService := mdns.NewMdnsService(s.Host, "_desvault._tcp", &Notifee{Host: s.Host})
	if err := mdnsService.Start(); err != nil {
		log.Printf("[ERROR] mDNS service error: %v", err)
	} else {
		log.Println("[INFO] mDNS service started successfully")
	}

	// Bootstrap the DHT.
	if err := s.DHT.Bootstrap(ctx); err != nil {
		log.Printf("[ERROR] DHT bootstrap error: %v", err)
	} else {
		log.Println("[INFO] DHT bootstrap completed")
	}

	log.Println("[INFO] Peer discovery fully initialized")
}

// AnnounceStorage publishes the node's storage contribution to the network using PubSub.
func (s *AutoDiscoveryService) AnnounceStorage(storageGB int) error {
	topic, err := s.PubSub.Join("storage-announcements")
	if err != nil {
		return fmt.Errorf("failed to join pubsub topic: %v", err)
	}
	// Create and publish the storage announcement message.
	msg := fmt.Sprintf("Node %s offering %dGB storage", s.Host.ID().String(), storageGB)
	if err := topic.Publish(context.Background(), []byte(msg)); err != nil {
		return fmt.Errorf("failed to publish storage announcement: %v", err)
	}
	log.Printf("[INFO] Storage announced: %s", msg)
	return nil
}

// -----------------------------------------------------------------------------
// Node Initialization and Global Helpers
// -----------------------------------------------------------------------------

var globalADS *AutoDiscoveryService

// InitializeNode creates a libp2p host with a DHT and PubSub instance,
// sets up mDNS discovery, and a stream handler for chat messages.
// It returns an AutoDiscoveryService.
func InitializeNode(ctx context.Context) (*AutoDiscoveryService, error) {
	// Create a new libp2p host.
	h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/4001"))
	if err != nil {
		return nil, fmt.Errorf("failed to create libp2p host: %v", err)
	}

	// Initialize the Kademlia DHT.
	kademliaDHT, err := dht.New(ctx, h, dht.Mode(dht.ModeAuto))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize DHT: %v", err)
	}

	// Initialize PubSub using GossipSub.
	ps, err := pubsub.NewGossipSub(ctx, h)
	if err != nil {
		return nil, fmt.Errorf("failed to create pubsub: %v", err)
	}

	ads := &AutoDiscoveryService{
		Host:   h,
		DHT:    kademliaDHT,
		PubSub: ps,
	}

	// Set up a stream handler for the chat protocol.
	h.SetStreamHandler(ChatProtocolID, func(stream gonetwork.Stream) {
		reader := bufio.NewReader(stream)
		for {
			msg, err := reader.ReadString('\n')
			if err != nil {
				log.Printf("[ERROR] Failed to read from chat stream: %v", err)
				return
			}
			fmt.Printf("[Chat] %s: %s\n", stream.Conn().RemotePeer().String(), strings.TrimSpace(msg))
		}
	})

	SetGlobalAutoDiscoveryService(ads)
	return ads, nil
}

// SetGlobalAutoDiscoveryService stores the global AutoDiscoveryService instance.
func SetGlobalAutoDiscoveryService(ads *AutoDiscoveryService) {
	globalADS = ads
}

// GetNodePeerID returns the node's peer ID.
func GetNodePeerID() string {
	if globalADS == nil {
		return ""
	}
	return globalADS.Host.ID().String()
}

// GetConnectedPeers returns a slice of connected peer IDs.
func GetConnectedPeers() []string {
	if globalADS == nil {
		return []string{}
	}
	var peers []string
	for _, p := range globalADS.Host.Peerstore().Peers() {
		peers = append(peers, p.String())
	}
	return peers
}

// SendMessage sends a chat message to the specified peer.
func SendMessage(peerID string, msg string) error {
	if globalADS == nil {
		return fmt.Errorf("AutoDiscoveryService not initialized")
	}

	peerAddr, err := peer.Decode(peerID)
	if err != nil {
		return fmt.Errorf("invalid peer ID: %v", err)
	}

	stream, err := globalADS.Host.NewStream(context.Background(), peerAddr, ChatProtocolID)
	if err != nil {
		return fmt.Errorf("failed to open chat stream: %v", err)
	}
	defer stream.Close()

	writer := bufio.NewWriter(stream)
	_, err = writer.WriteString(msg + "\n")
	if err != nil {
		return fmt.Errorf("failed to send message: %v", err)
	}
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush message: %v", err)
	}

	log.Printf("[INFO] Message sent to peer %s: %s", peerID, msg)
	return nil
}

// -----------------------------------------------------------------------------
// Registration, Announcements & Monitoring
// -----------------------------------------------------------------------------

// RegisterNode registers the node on the network using the provided production configuration.
// This implementation performs an HTTP POST to a registration service endpoint.
// The endpoint is defined here as a default constant, but you can modify this to read from configuration.
func RegisterNode(h host.Host, config *setup.Config) error {
	const defaultRegistrationEndpoint = "http://localhost:8080/register"
	registrationEndpoint := defaultRegistrationEndpoint

	peerID := h.ID().String()
	log.Printf("[INFO] Registering node %s with configuration: %+v", peerID, config)

	// Prepare registration data.
	data := map[string]interface{}{
		"peerID":    peerID,
		"addresses": h.Addrs(),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal registration data: %v", err)
	}

	resp, err := http.Post(registrationEndpoint, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("registration request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("registration failed with status: %s", resp.Status)
	}
	log.Printf("[INFO] Node %s successfully registered with registration service", peerID)
	return nil
}

// MonitorNetwork continuously monitors network connectivity and logs the current number of connected peers.
func MonitorNetwork(h host.Host) {
	log.Println("[INFO] Starting network monitoring...")
	for {
		peers := h.Network().Peers()
		count := len(peers)
		log.Printf("[INFO] Connected to %d peers", count)
		if count == 0 {
			log.Println("[WARN] No peers connected; the network may be isolated")
		}
		time.Sleep(10 * time.Second)
	}
}



// The RegisterNode function uses a default registration endpoint constant (set to http://localhost:8080/register). i need to modify this to read from my configuration.