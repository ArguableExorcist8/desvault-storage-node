package noise_protocol

import (
	"fmt"

	"github.com/flynn/noise"
)

// InitializeHandshake creates a new Noise handshake state using the NN pattern.
// The NN pattern requires no static keys and uses ephemeral keys.
func InitializeHandshake(initiator bool) (*noise.HandshakeState, error) {
	config := noise.Config{
		CipherSuite: noise.NewCipherSuite(noise.DH25519, noise.CipherAESGCM, noise.HashSHA256),
		Pattern:     noise.HandshakeNN,
		Initiator:   initiator,
	}
	hs, err := noise.NewHandshakeState(config)
	if err != nil {
		return nil, err
	}
	return hs, nil
}

// ExecuteHandshake performs a complete handshake between an initiator and a responder.
// It returns pointers to the final CipherState for both parties if the handshake is successful.
func ExecuteHandshake() (initiatorCS *noise.CipherState, responderCS *noise.CipherState, err error) {
	// Create handshake states for both parties.
	initiator, err := InitializeHandshake(true)
	if err != nil {
		return nil, nil, fmt.Errorf("initiator handshake init error: %v", err)
	}
	responder, err := InitializeHandshake(false)
	if err != nil {
		return nil, nil, fmt.Errorf("responder handshake init error: %v", err)
	}

	// --- Stage 1: Initiator sends first message ---
	msg1, _, _, err := initiator.WriteMessage(nil, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("initiator write error: %v", err)
	}

	// --- Stage 2: Responder processes first message ---
	_, _, _, err = responder.ReadMessage(nil, msg1)
	if err != nil {
		return nil, nil, fmt.Errorf("responder read error: %v", err)
	}

	// --- Stage 3: Responder sends response ---
	msg2, _, csResponder, err := responder.WriteMessage(nil, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("responder write error: %v", err)
	}

	// --- Stage 4: Initiator processes response ---
	_, _, csInitiator, err := initiator.ReadMessage(nil, msg2)
	if err != nil {
		return nil, nil, fmt.Errorf("initiator read error: %v", err)
	}

	return csInitiator, csResponder, nil
}
