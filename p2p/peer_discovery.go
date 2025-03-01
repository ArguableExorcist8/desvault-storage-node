package p2p

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	libp2p "github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	dht "github.com/libp2p/go-libp2p-kad-dht"
)

const (
	discoveryServiceTag = "desvault-discovery"
	broadcastPort       = 9999
	discoveryMessage    = "DesVaultPeerDiscovery"
)

// ---------------------------
// Libp2p-based Discovery
// ---------------------------

// discoveryNotifee implements the mdns.Notifee interface.
type discoveryNotifee struct {
	h host.Host
}

// HandlePeerFound is invoked by mDNS when a peer is discovered.
func (n *discoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	fmt.Printf("[+] Discovered peer: %s\n", pi.ID)
	// Attempt to connect to the discovered peer.
	if err := n.h.Connect(context.Background(), pi); err != nil {
		log.Printf("[!] Error connecting to peer %s: %v", pi.ID, err)
	} else {
		fmt.Printf("[+] Connected to peer: %s\n", pi.ID)
	}
}

// StartLibp2pDiscovery initializes a libp2p host, sets up DHT and mDNS discovery,
// and waits up to 30 seconds for peers. If no peers are found, the node becomes the seed.
func StartLibp2pDiscovery() (host.Host, *dht.IpfsDHT, error) {
    ctx := context.Background()

    // Create a new libp2p host.
    h, err := libp2p.New()
    if err != nil {
        return nil, nil, fmt.Errorf("failed to create libp2p host: %v", err)
    }
    fmt.Printf("[*] Node started with Peer ID: %s\n", h.ID())

    // Set up the DHT.
    kademliaDHT, err := dht.New(ctx, h)
    if err != nil {
        h.Close()
        return nil, nil, fmt.Errorf("failed to create DHT: %v", err)
    }
    go func() {
        if err := kademliaDHT.Bootstrap(ctx); err != nil {
            log.Printf("[!] DHT bootstrap error: %v", err)
        }
    }()

    // Set up mDNS discovery.
    mdnsService := mdns.NewMdnsService(h, discoveryServiceTag, &discoveryNotifee{h: h})
    if err := mdnsService.Start(); err != nil {
        kademliaDHT.Close()
        h.Close()
        return nil, nil, fmt.Errorf("failed to start mDNS service: %v", err)
    }
    // Note: mdnsService should be kept alive; consider storing it or closing it when done.

    // Wait for peers for up to 30 seconds.
    fmt.Println("[*] Discovering peers for 30 seconds...")
    timeout := time.After(30 * time.Second)
    peerFound := make(chan bool, 1)
    go func() {
        ticker := time.NewTicker(2 * time.Second)
        defer ticker.Stop()
        for range ticker.C {
            // h.Peerstore().Peers() includes self; expect > 1 if others are discovered.
            if len(h.Peerstore().Peers()) > 1 {
                peerFound <- true
                return
            }
        }
    }()

    select {
    case <-peerFound:
        fmt.Println("[*] Peers discovered. This node will join the network.")
    case <-timeout:
        fmt.Println("[*] No peers found within 30 seconds. This node is now the network seed.")
    }

    return h, kademliaDHT, nil
}

// ---------------------------
// UDP-based Discovery Functions
// ---------------------------

// BroadcastDiscovery sends a UDP broadcast message to announce this node's presence.
func BroadcastDiscovery() error {
	broadcastAddr := &net.UDPAddr{
		IP:   net.IPv4bcast,
		Port: broadcastPort,
	}
	conn, err := net.DialUDP("udp4", nil, broadcastAddr)
	if err != nil {
		return fmt.Errorf("failed to dial broadcast address: %v", err)
	}
	defer conn.Close()

	_, err = conn.Write([]byte(discoveryMessage))
	if err != nil {
		return fmt.Errorf("failed to send broadcast: %v", err)
	}
	return nil
}

// ListenForPeers listens on UDP for incoming discovery messages for the specified timeout.
func ListenForPeers(timeout time.Duration) ([]string, error) {
	addr := net.UDPAddr{
		Port: broadcastPort,
		IP:   net.IPv4zero,
	}
	conn, err := net.ListenUDP("udp4", &addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on UDP port %d: %v", broadcastPort, err)
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(timeout))
	var peers []string
	buffer := make([]byte, 1024)
	for {
		n, remoteAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			break // Likely a timeout; exit loop.
		}
		msg := string(buffer[:n])
		if msg == discoveryMessage {
			peers = append(peers, remoteAddr.IP.String())
		}
	}
	return peers, nil
}

// DiscoverPeers performs UDP-based peer discovery by broadcasting a message and then listening for responses.
func DiscoverPeers(timeout time.Duration) ([]string, error) {
	if err := BroadcastDiscovery(); err != nil {
		return nil, err
	}
	time.Sleep(1 * time.Second) // Give time for responses.
	return ListenForPeers(timeout)
}