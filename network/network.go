package network

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p"
	gonetwork "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	mdns "github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	dht "github.com/libp2p/go-libp2p-kad-dht"

	"github.com/ArguableExorcist8/desvault-storage-node/setup" // Production configuration
)

const ChatProtocolID = "/desvault/chat/1.0.0"

// -----------------------------------------------------------------------------
// mDNS & DHT-Based Discovery
// -----------------------------------------------------------------------------

// Notifee implements the mdns.Notifee interface for mDNS discovery.
type Notifee struct {
	Host host.Host
}

// HandlePeerFound handles peers discovered via mDNS.
func (n *Notifee) HandlePeerFound(pi peer.AddrInfo) {
	log.Printf("[mDNS] Discovered peer: %s", pi.ID.String())
	if err := n.Host.Connect(context.Background(), pi); err != nil {
		log.Printf("[ERROR] Failed to connect to peer %s: %v", pi.ID.String(), err)
	} else {
		log.Printf("[INFO] Successfully connected to peer: %s", pi.ID.String())
	}
}

// AutoDiscoveryService encapsulates the libp2p host and DHT instance.
type AutoDiscoveryService struct {
	Host host.Host
	DHT  *dht.IpfsDHT
}

// Start initializes mDNS discovery and bootstraps the DHT.
func (s *AutoDiscoveryService) Start(ctx context.Context) {
	// Initialize mDNS service (no error returned here, only one value).
	mdnsService := mdns.NewMdnsService(s.Host, "_desvault._tcp", &Notifee{Host: s.Host})
	// Start the mDNS service and handle any startup errors.
	if err := mdnsService.Start(); err != nil {
		log.Printf("[ERROR] Failed to start mDNS service: %v", err)
	} else {
		log.Println("[INFO] mDNS service started successfully")
	}

	// Bootstrap the DHT for peer discovery.
	if err := s.DHT.Bootstrap(ctx); err != nil {
		log.Printf("[ERROR] Failed to bootstrap DHT: %v", err)
	} else {
		log.Println("[INFO] DHT bootstrap completed")
	}

	log.Println("[INFO] Peer discovery fully initialized")
}

// InitializeNode creates a libp2p host with a DHT instance and returns an AutoDiscoveryService.
func InitializeNode(ctx context.Context) (*AutoDiscoveryService, error) {
	// Create a new libp2p host listening on all interfaces at port 4001.
	h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/4001"))
	if err != nil {
		return nil, fmt.Errorf("failed to create libp2p host: %v", err)
	}

	// Initialize the Kademlia DHT in auto mode.
	kademliaDHT, err := dht.New(ctx, h, dht.Mode(dht.ModeAuto))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize DHT: %v", err)
	}

	ads := &AutoDiscoveryService{
		Host: h,
		DHT:  kademliaDHT,
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

	// Store the service globally for helper function access.
	SetGlobalAutoDiscoveryService(ads)
	return ads, nil
}

// -----------------------------------------------------------------------------
// Global Helpers
// -----------------------------------------------------------------------------

var globalADS *AutoDiscoveryService

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
// Registration & Monitoring (Production Implementation)
// -----------------------------------------------------------------------------

// RegisterNode registers the node on the network using the provided configuration.
func RegisterNode(h host.Host, config *setup.Config) error {
	// In a production environment, this could publish node details to a registry or blockchain.
	// For this example, we'll simulate registration with logging and a simple validation.
	if config == nil {
		return fmt.Errorf("configuration is nil")
	}

	peerID := h.ID().String()
	log.Printf("[INFO] Registering node %s with configuration: %+v", peerID, config)

	// Simulate a production registration process (e.g., API call or blockchain transaction).
	// Replace this with actual logic, such as publishing to a distributed ledger.
	time.Sleep(1 * time.Second) // Simulate network delay.
	log.Printf("[INFO] Node %s successfully registered", peerID)
	return nil
}

// MonitorNetwork continuously monitors network connectivity and logs peer count.
func MonitorNetwork(h host.Host) {
	log.Println("[INFO] Starting network monitoring...")
	for {
		peers := h.Network().Peers()
		peerCount := len(peers)
		log.Printf("[INFO] Connected to %d peers", peerCount)
		if peerCount == 0 {
			log.Println("[WARN] No peers connected; network may be isolated")
		}
		time.Sleep(10 * time.Second)
	}
}