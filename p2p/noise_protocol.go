package p2p

import (
	"crypto/rand"
	"fmt"
	"log"

	"github.com/flynn/noise"
)

// SetupNoise initializes the Noise protocol handshake state.
func SetupNoise() (*noise.CipherState, error) {
	// Using a basic Noise configuration (replace with desired handshake pattern)
	config := noise.Config{
		Pattern:     noise.HandshakeXX,
		CipherSuite: noise.NewCipherSuite(noise.DH25519, noise.CipherChaChaPoly, noise.HashSHA256),
		Random:      rand.Reader,
		Initiator:   true,
	}
	hs, err := noise.NewHandshakeState(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create handshake state: %v", err)
	}

	return hs.CipherState(), nil
}

func TestNoise() {
	cs, err := SetupNoise()
	if err != nil {
		log.Fatalf("Noise setup error: %v", err)
	}
	message := []byte("Hello, secure world!")
	encrypted := cs.Encrypt(nil, nil, message)
	decrypted, err := cs.Decrypt(nil, nil, encrypted)
	if err != nil {
		log.Fatalf("Noise decryption error: %v", err)
	}
	log.Printf("[Noise] Original: %s, Decrypted: %s", message, decrypted)
}
