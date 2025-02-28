package p2p

import (
	"crypto/rand"
	"fmt"
	"io"

	"github.com/flynn/noise"
)

// PerformHandshakeInitiator performs a complete Noise XX handshake as the initiator
// over the provided connection. The function exchanges three handshake messages,
// then returns the resulting CipherState for secure communication.
// The 'payload' can be used to send initial data if desired.
func PerformHandshakeInitiator(conn io.ReadWriter, payload []byte) (*noise.CipherState, error) {
	// Configure the Noise handshake as initiator using the XX pattern.
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

	// --- Message 1 ---
	// Initiator writes the first handshake message (optionally with a payload).
	msg1, _, err := hs.WriteMessage(nil, payload)
	if err != nil {
		return nil, fmt.Errorf("failed to write handshake message 1: %v", err)
	}
	if _, err := conn.Write(msg1); err != nil {
		return nil, fmt.Errorf("failed to send handshake message 1: %v", err)
	}

	// --- Message 2 ---
	// Initiator reads the responder's handshake message.
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read handshake message 2: %v", err)
	}
	_, _, err = hs.ReadMessage(nil, buf[:n])
	if err != nil {
		return nil, fmt.Errorf("failed to process handshake message 2: %v", err)
	}

	// --- Message 3 ---
	// Initiator writes the final handshake message.
	msg3, cs, err := hs.WriteMessage(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to write handshake message 3: %v", err)
	}
	if _, err := conn.Write(msg3); err != nil {
		return nil, fmt.Errorf("failed to send handshake message 3: %v", err)
	}

	// The handshake is now complete. Return the initiator's CipherState.
	return cs, nil
}

// PerformHandshakeResponder performs a complete Noise XX handshake as the responder
// over the provided connection. It reads the initiator's handshake message, sends its
// response, then reads the final message, returning the resulting CipherState.
func PerformHandshakeResponder(conn io.ReadWriter, payload []byte) (*noise.CipherState, error) {
	// Configure the Noise handshake as responder using the XX pattern.
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

	// --- Message 1 ---
	// Responder reads the initiator's handshake message.
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read handshake message 1: %v", err)
	}
	_, _, err = hs.ReadMessage(nil, buf[:n])
	if err != nil {
		return nil, fmt.Errorf("failed to process handshake message 1: %v", err)
	}

	// --- Message 2 ---
	// Responder writes its handshake message (optionally including a payload).
	msg2, _, err := hs.WriteMessage(nil, payload)
	if err != nil {
		return nil, fmt.Errorf("failed to write handshake message 2: %v", err)
	}
	if _, err := conn.Write(msg2); err != nil {
		return nil, fmt.Errorf("failed to send handshake message 2: %v", err)
	}

	// --- Message 3 ---
	// Responder reads the initiator's final handshake message.
	n, err = conn.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read handshake message 3: %v", err)
	}
	cs, _, err := hs.ReadMessage(nil, buf[:n])
	if err != nil {
		return nil, fmt.Errorf("failed to process handshake message 3: %v", err)
	}

	// The handshake is complete. Return the responder's CipherState.
	return cs, nil
}
