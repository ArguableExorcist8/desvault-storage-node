arg@arg-LIFEBOOK-S710:~/DesVault/storage-node$ go build -o bin/desvault main.go
# github.com/ArguableExorcist8/desvault-storage-node/p2p
p2p/udp_peer_discovery.go:10:5: broadcastPort redeclared in this block
	p2p/peer_discovery.go:20:2: other declaration of broadcastPort
p2p/udp_peer_discovery.go:11:5: discoveryMessage redeclared in this block
	p2p/peer_discovery.go:21:2: other declaration of discoveryMessage
p2p/udp_peer_discovery.go:15:6: BroadcastDiscovery redeclared in this block
	p2p/peer_discovery.go:105:6: other declaration of BroadcastDiscovery
p2p/udp_peer_discovery.go:34:6: ListenForPeers redeclared in this block
	p2p/peer_discovery.go:124:6: other declaration of ListenForPeers
p2p/udp_peer_discovery.go:63:6: DiscoverPeers redeclared in this block
	p2p/peer_discovery.go:152:6: other declaration of DiscoverPeers
p2p/noise_protocol.go:107:9: cannot use cs (variable of type []byte) as *"github.com/flynn/noise".CipherState value in return statement
p2p/peer_discovery.go:11:2: "github.com/libp2p/go-libp2p/core/discovery" imported and not used
p2p/peer_discovery.go:68:2: mdnsService declared and not used
p2p/peer_discovery.go:68:22: assignment mismatch: 2 variables but mdns.NewMdnsService returns 1 value
