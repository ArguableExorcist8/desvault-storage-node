package p2p

import (
	"fmt"
	"io"

	"github.com/flynn/noise"
	"crypto/rand"
)

// SecureChannel holds the cipher states for sending and receiving encrypted messages.
type SecureChannel struct {
	SendCipher *noise.CipherState
	RecvCipher *noise.CipherState
}

// PerformHandshakeInitiator performs a complete Noise XX handshake as the initiator
// over the provided connection. It exchanges three handshake messages and returns
// a SecureChannel containing the cipher states for secure communication.
func PerformHandshakeInitiator(conn io.ReadWriter, payload []byte) (*SecureChannel, error) {
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

	// --- Message 1: Initiator -> Responder ---
	msg1, _, _, err := hs.WriteMessage(nil, payload)
	if err != nil {
		return nil, fmt.Errorf("failed to write handshake message 1: %v", err)
	}
	if _, err := conn.Write(msg1); err != nil {
		return nil, fmt.Errorf("failed to send handshake message 1: %v", err)
	}

	// --- Message 2: Responder -> Initiator ---
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read handshake message 2: %v", err)
	}
	_, _, _, err = hs.ReadMessage(nil, buf[:n])
	if err != nil {
		return nil, fmt.Errorf("failed to process handshake message 2: %v", err)
	}

	// --- Message 3: Initiator -> Responder ---
	msg3, csSend, csRecv, err := hs.WriteMessage(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to write handshake message 3: %v", err)
	}
	if _, err := conn.Write(msg3); err != nil {
		return nil, fmt.Errorf("failed to send handshake message 3: %v", err)
	}

	// Return both cipher states for encryption (send) and decryption (receive)
	return &SecureChannel{
		SendCipher: csSend,
		RecvCipher: csRecv,
	}, nil
}

// PerformHandshakeResponder performs a complete Noise XX handshake as the responder
// over the provided connection. It processes the initiator's messages, responds
// accordingly, and returns a SecureChannel with the cipher states.
func PerformHandshakeResponder(conn io.ReadWriter, payload []byte) (*SecureChannel, error) {
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

	// --- Message 1: Initiator -> Responder ---
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read handshake message 1: %v", err)
	}
	_, _, _, err = hs.ReadMessage(nil, buf[:n])
	if err != nil {
		return nil, fmt.Errorf("failed to process handshake message 1: %v", err)
	}

	// --- Message 2: Responder -> Initiator ---
	msg2, _, _, err := hs.WriteMessage(nil, payload)
	if err != nil {
		return nil, fmt.Errorf("failed to write handshake message 2: %v", err)
	}
	if _, err := conn.Write(msg2); err != nil {
		return nil, fmt.Errorf("failed to send handshake message 2: %v", err)
	}

	// --- Message 3: Initiator -> Responder ---
	n, err = conn.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read handshake message 3: %v", err)
	}
	_, csRecv, csSend, err := hs.ReadMessage(nil, buf[:n])
	if err != nil {
		return nil, fmt.Errorf("failed to process handshake message 3: %v", err)
	}

	// Return both cipher states for encryption (send) and decryption (receive)
	return &SecureChannel{
		SendCipher: csSend,
		RecvCipher: csRecv,
	}, nil
}