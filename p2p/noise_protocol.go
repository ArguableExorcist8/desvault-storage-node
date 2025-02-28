package p2p

import (
	"crypto/rand"
	"fmt"
	"io"

	"github.com/flynn/noise"
)

// PerformHandshakeInitiator performs a complete Noise XX handshake as the initiator
// over the provided connection (which implements io.ReadWriter). It exchanges
// three handshake messages and returns the resulting CipherState for secure communication.
func PerformHandshakeInitiator(conn io.ReadWriter, payload []byte) (*noise.CipherState, error) {
	// Configure the handshake state as the initiator.
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

	// --- Message 1: Initiator sends the first handshake message with optional payload.
	msg1, _, err := hs.WriteMessage(nil, payload)
	if err != nil {
		return nil, fmt.Errorf("failed to write handshake message 1: %v", err)
	}
	if _, err := conn.Write(msg1); err != nil {
		return nil, fmt.Errorf("failed to send handshake message 1: %v", err)
	}

	// --- Message 2: Initiator reads responder's handshake message.
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read handshake message 2: %v", err)
	}
	_, _, err = hs.ReadMessage(nil, buf[:n])
	if err != nil {
		return nil, fmt.Errorf("failed to process handshake message 2: %v", err)
	}

	// --- Message 3: Initiator sends the final handshake message.
	msg3, _, err := hs.WriteMessage(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to write handshake message 3: %v", err)
	}
	if _, err := conn.Write(msg3); err != nil {
		return nil, fmt.Errorf("failed to send handshake message 3: %v", err)
	}

	// Now complete the handshake by splitting the handshake state.
	cs, _, err := hs.Split()
	if err != nil {
		return nil, fmt.Errorf("failed to split handshake state: %v", err)
	}

	return cs, nil
}

// PerformHandshakeResponder performs a complete Noise XX handshake as the responder
// over the provided connection. It reads the initiator's handshake message, sends its
// handshake response, reads the final message, and returns the resulting CipherState.
func PerformHandshakeResponder(conn io.ReadWriter, payload []byte) (*noise.CipherState, error) {
	// Configure the handshake state as the responder.
	config := noise.Config{
		Pattern:     noise.HandshakeXX,
		CipherSuite: noise.NewCipherSuite(noise.DH25519, noise.CipherChaChaPoly, noise.HashSHA256),
		Random:      rand.Reader,
		Initiator:   false,
	}
	hs, err := noise.NewHandshakeState(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create handshake state: %v", err)
	}

	// --- Message 1: Responder reads the initiator's handshake message.
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read handshake message 1: %v", err)
	}
	_, _, err = hs.ReadMessage(nil, buf[:n])
	if err != nil {
		return nil, fmt.Errorf("failed to process handshake message 1: %v", err)
	}

	// --- Message 2: Responder sends its handshake message with optional payload.
	msg2, _, err := hs.WriteMessage(nil, payload)
	if err != nil {
		return nil, fmt.Errorf("failed to write handshake message 2: %v", err)
	}
	if _, err := conn.Write(msg2); err != nil {
		return nil, fmt.Errorf("failed to send handshake message 2: %v", err)
	}

	// --- Message 3: Responder reads the initiator's final handshake message.
	n, err = conn.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read handshake message 3: %v", err)
	}
	cs, _, err := hs.ReadMessage(nil, buf[:n])
	if err != nil {
		return nil, fmt.Errorf("failed to process handshake message 3: %v", err)
	}

	// The handshake is complete; return the resulting CipherState.
	return cs, nil
}
