package p2p

import (
	"context"
	"log"

	libp2p "github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p-core/host"
)

// SetupDHT creates a new libp2p host and initializes a Kademlia DHT instance.
// It returns the host, the DHT, and an error (if any).
func SetupDHT(ctx context.Context) (host.Host, *dht.IpfsDHT, error) {
	// Create a new libp2p host.
	// Since your version does not support passing a context (via WithContext),
	// we call New() without any options.
	h, err := libp2p.New()
	if err != nil {
		return nil, nil, err
	}

	// Initialize the Kademlia DHT using the provided context.
	kademliaDHT, err := dht.New(ctx, h)
	if err != nil {
		return h, nil, err
	}

	log.Println("[+] Kademlia DHT initialized for shard discovery.")
	return h, kademliaDHT, nil
}
