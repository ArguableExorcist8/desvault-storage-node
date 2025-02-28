package p2p

import (
    "fmt"
    "net"
    "time"
)

const (
    broadcastPort    = 9999
    discoveryMessage = "DesVaultPeerDiscovery"
)

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

// DiscoverPeers performs peer discovery by broadcasting a message and then listening for responses.
func DiscoverPeers(timeout time.Duration) ([]string, error) {
    if err := BroadcastDiscovery(); err != nil {
        return nil, err
    }
    time.Sleep(1 * time.Second)
    return ListenForPeers(timeout)
}
