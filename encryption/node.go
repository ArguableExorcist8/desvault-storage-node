package encryption

import (
	"fmt"
	"log"

	libp2p "github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
)

// GenerateKeyPair creates a new Ed25519 key pair for P2P security.
func GenerateKeyPair() (crypto.PrivKey, crypto.PubKey, error) {
	privKey, pubKey, err := crypto.GenerateKeyPair(crypto.Ed25519, -1)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate key pair: %v", err)
	}
	return privKey, pubKey, nil
}

// SecureP2PNode creates and returns a secure libp2p node using a newly generated key pair.
// Additional libp2p options can be added here as needed.
func SecureP2PNode() (host.Host, error) {
	privKey, _, err := GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %v", err)
	}

	node, err := libp2p.New(libp2p.Identity(privKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create P2P node: %v", err)
	}

	log.Println("[INFO] Secure P2P Node created with ID:", node.ID())
	return node, nil
}
