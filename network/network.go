package network

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	gonetwork "github.com/libp2p/go-libp2p/core/network" // Alias for clarity.
	"github.com/libp2p/go-libp2p/core/peer"
	mdns "github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	dht "github.com/libp2p/go-libp2p-kad-dht"
)

const ChatProtocolID = "/desvault/chat/1.0.0"

// Notifee implements the mdns.Notifee interface.
type Notifee struct {
	Host host.Host
}

func (n *Notifee) HandlePeerFound(pi peer.AddrInfo) {
	log.Printf("[mDNS] Found peer: %s\n", pi.ID.String())
	if err := n.Host.Connect(context.Background(), pi); err != nil {
		log.Printf("[mDNS] Error connecting to peer %s: %s\n", pi.ID.String(), err)
	} else {
		log.Printf("[mDNS] Connected to peer: %s\n", pi.ID.String())
	}
}

// AutoDiscoveryService manages peer discovery via mDNS and DHT.
type AutoDiscoveryService struct {
	Host host.Host
	DHT  *dht.IpfsDHT
}

// Start begins mDNS discovery and bootstraps the DHT.
func (s *AutoDiscoveryService) Start(ctx context.Context) {
	// mDNS discovery:
	mdnsService := mdns.NewMdnsService(s.Host, "_desvault._tcp", &Notifee{Host: s.Host})
	if mdnsService != nil {
		if err := mdnsService.Start(); err != nil {
			log.Printf("mDNS service error: %v", err)
		} else {
			log.Println("mDNS service started")
		}
	} else {
		log.Println("mDNS initialization failed")
	}

	// DHT bootstrap:
	if err := s.DHT.Bootstrap(ctx); err == nil {
		log.Println("DHT bootstrap completed")
	} else {
		log.Printf("DHT bootstrap error: %s\n", err)
	}

	log.Println("Peer discovery initialized")
}

// InitializeNode creates a libp2p host with a DHT instance.
// removed the logging of the Peer ID here to avoid duplicate output.
func InitializeNode(ctx context.Context) (*AutoDiscoveryService, error) {
	h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/4001"))
	if err != nil {
		return nil, err
	}

	kademliaDHT, err := dht.New(ctx, h, dht.Mode(dht.ModeAuto))
	if err != nil {
		return nil, err
	}

	// Removed duplicate logging:
	// log.Printf("Node started with Peer ID: %s\n", h.ID().String())

	ads := &AutoDiscoveryService{
		Host: h,
		DHT:  kademliaDHT,
	}
	// Set a stream handler for the chat protocol.
	ads.Host.SetStreamHandler(ChatProtocolID, func(stream gonetwork.Stream) {
		reader := bufio.NewReader(stream)
		for {
			msg, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			fmt.Printf("[Chat] %s: %s\n", stream.Conn().RemotePeer().String(), strings.TrimSpace(msg))
		}
	})

	// Save the global instance for package-level functions.
	SetGlobalAutoDiscoveryService(ads)
	return ads, nil
}

// --- Global Instance and Package-Level Functions ---

var globalADS *AutoDiscoveryService

// SetGlobalAutoDiscoveryService stores the global auto-discovery service.
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
		fmt.Println("[!] AutoDiscoveryService not initialized")
		return
	}
	peerAddr, err := peer.Decode(peerID)
	if err != nil {
		fmt.Println("[!] Invalid peer ID:", err)
		return
	}
	stream, err := globalADS.Host.NewStream(context.Background(), peerAddr, ChatProtocolID)
	if err != nil {
		fmt.Println("[!] Failed to open chat stream:", err)
		return
	}
	defer stream.Close()

	writer := bufio.NewWriter(stream)
	_, err = writer.WriteString(msg + "\n")
	if err != nil {
		fmt.Println("[!] Failed to send message:", err)
		return
	}
	writer.Flush()
}
