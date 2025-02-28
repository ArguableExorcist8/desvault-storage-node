package p2p

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	libp2p "github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/libp2p/go-libp2p/core/discovery" // Updated import path
	dht "github.com/libp2p/go-libp2p-kad-dht"
)


const (
	discoveryServiceTag = "desvault-discovery"
	broadcastPort       = 9999
	discoveryMessage    = "DesVaultPeerDiscovery"
)

// discoveryNotifee implements the mdns.Notifee interface.
type discoveryNotifee struct {
	h host.Host
}

func (n *discoveryNotifee) HandlePeerFound(pi discovery.DiscoveryPeer) {
	fmt.Printf("[+] Discovered peer: %s\n", pi.ID)
	// Attempt to connect to the discovered peer
	if err := n.h.Connect(context.Background(), pi); err != nil {
		log.Printf("[!] Error connecting to peer %s: %v", pi.ID, err)
	} else {
		fmt.Printf("[+] Connected to peer: %s\n", pi.ID)
	}
}

// StartLibp2pDiscovery initializes a libp2p host, sets up DHT and mDNS discovery,
// and waits up to 30 seconds for peers. If no peers are found, this node becomes the seed.
func StartLibp2pDiscovery() (host.Host, *dht.IpfsDHT, error) {
	ctx := context.Background()

	// Create a new libp2p host.
	h, err := libp2p.New()
	if err != nil {
		return nil, nil, err
	}
	fmt.Printf("[*] Storage Node started with Peer ID: %s\n", h.ID())

	// Set up DHT.
	kademliaDHT, err := dht.New(ctx, h)
	if err != nil {
		return nil, nil, err
	}
	go func() {
		if err := kademliaDHT.Bootstrap(ctx); err != nil {
			log.Printf("[!] DHT bootstrap error: %v", err)
		}
	}()

	// Set up mDNS discovery.
	mdnsService, err := mdns.NewMdnsService(ctx, h, 10*time.Second, discoveryServiceTag)
	if err != nil {
		return nil, nil, err
	}
	notifee := &discoveryNotifee{h: h}
	mdnsService.RegisterNotifee(notifee)

	// Wait for peers for up to 30 seconds.
	fmt.Println("[*] Discovering peers for 30 seconds...")
	timeout := time.After(30 * time.Second)
	peerFound := make(chan bool, 1)
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				// Check if there are more peers than just this node.
				if len(h.Peerstore().Peers()) > 1 {
					peerFound <- true
					return
				}
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

// --- UDP-based Peer Discovery Functions ---

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

// ListenForPeers listens on UDP for incoming discovery messages for the specified timeout duration.
func ListenForPeers(timeout time.Duration) ([]string, error) {
	addr := net.UDPAddr{
		Port: broadcastPort,
		IP:   net.IPv4zero, // Listen on all interfaces.
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
			// Break out on timeout or error.
			break
		}
		msg := string(buffer[:n])
		if msg == discoveryMessage {
			peers = append(peers, remoteAddr.IP.String())
		}
	}
	return peers, nil
}

// DiscoverPeers performs peer discovery over UDP by broadcasting a message and then listening for responses.
func DiscoverPeers(timeout time.Duration) ([]string, error) {
	// Broadcast the discovery message.
	if err := BroadcastDiscovery(); err != nil {
		return nil, err
	}
	// Brief pause to allow responses.
	time.Sleep(1 * time.Second)
	peers, err := ListenForPeers(timeout)
	if err != nil {
		return nil, err
	}
	return peers, nil
}
