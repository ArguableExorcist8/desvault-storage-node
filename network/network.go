package network

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p"
	gonetwork "github.com/libp2p/go-libp2p/core/network" // For stream handling.
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	mdns "github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	dht "github.com/libp2p/go-libp2p-kad-dht"

	"github.com/ArguableExorcist8/desvault-storage-node/setup" // Production configuration.
)

const ChatProtocolID = "/desvault/chat/1.0.0"

// -----------------------------------------------------------------------------
// mDNS & DHT-Based Discovery
// -----------------------------------------------------------------------------

// Notifee implements the mdns.Notifee interface for mDNS discovery.
type Notifee struct {
	Host host.Host
}

func (n *Notifee) HandlePeerFound(pi peer.AddrInfo) {
	log.Printf("[mDNS] Found peer: %s", pi.ID.String())
	if err := n.Host.Connect(context.Background(), pi); err != nil {
		log.Printf("[mDNS] Error connecting to peer %s: %v", pi.ID.String(), err)
	} else {
		log.Printf("[mDNS] Connected to peer: %s", pi.ID.String())
	}
}

// AutoDiscoveryService encapsulates the libp2p host and DHT instance.
type AutoDiscoveryService struct {
	Host host.Host
	DHT  *dht.IpfsDHT
}

// Start begins mDNS discovery and bootstraps the DHT.
func (s *AutoDiscoveryService) Start(ctx context.Context) {
	// Start mDNS discovery.
	mdnsService, err := mdns.NewMdnsService(s.Host, "_desvault._tcp", &Notifee{Host: s.Host})
	if err != nil {
		log.Printf("[ERROR] mDNS service error: %v", err)
	} else {
		if err := mdnsService.Start(); err != nil {
			log.Printf("[ERROR] mDNS service failed to start: %v", err)
		} else {
			log.Println("[INFO] mDNS service started")
		}
	}

	// Bootstrap DHT.
	if err := s.DHT.Bootstrap(ctx); err != nil {
		log.Printf("[ERROR] DHT bootstrap error: %v", err)
	} else {
		log.Println("[INFO] DHT bootstrap completed")
	}

	log.Println("[INFO] Peer discovery initialized")
}

// InitializeNode creates a libp2p host with a DHT instance and returns an AutoDiscoveryService.
func InitializeNode(ctx context.Context) (*AutoDiscoveryService, error) {
	h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/4001"))
	if err != nil {
		return nil, err
	}

	kademliaDHT, err := dht.New(ctx, h, dht.Mode(dht.ModeAuto))
	if err != nil {
		return nil, err
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
				return
			}
			fmt.Printf("[Chat] %s: %s\n", stream.Conn().RemotePeer().String(), strings.TrimSpace(msg))
		}
	})

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
func SendMessage(peerID string, msg string) {
	if globalADS == nil {
		fmt.Println("[ERROR] AutoDiscoveryService not initialized")
		return
	}
	peerAddr, err := peer.Decode(peerID)
	if err != nil {
		fmt.Println("[ERROR] Invalid peer ID:", err)
		return
	}
	stream, err := globalADS.Host.NewStream(context.Background(), peerAddr, ChatProtocolID)
	if err != nil {
		fmt.Println("[ERROR] Failed to open chat stream:", err)
		return
	}
	defer stream.Close()

	writer := bufio.NewWriter(stream)
	_, err = writer.WriteString(msg + "\n")
	if err != nil {
		fmt.Println("[ERROR] Failed to send message:", err)
		return
	}
	writer.Flush()
}

// -----------------------------------------------------------------------------
// Registration & Monitoring (Production Implementation)
// -----------------------------------------------------------------------------

// RegisterNode registers the node on the network using the provided configuration.
// In a real production system, this might involve publishing the node's details to a central registry
// or a distributed ledger so that other nodes can discover it.
func RegisterNode(h host.Host, config *setup.Config) error {
	// For production, you would likely call an API or update a distributed registry.
	// Here we simply log the registration event.
	log.Printf("[INFO] Registering node %s with configuration: %+v", h.ID().String(), config)
	// TODO: Implement real registration logic.
	return nil
}

// MonitorNetwork continuously monitors network connectivity and logs the current number of connected peers.
func MonitorNetwork(h host.Host) {
	log.Println("[INFO] Starting network monitoring...")
	for {
		peers := h.Network().Peers()
		log.Printf("[INFO] Connected to %d peers", len(peers))
		time.Sleep(10 * time.Second)
	}
}
