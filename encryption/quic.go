package encryption

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"time"

	"github.com/quic-go/quic-go"
)

// StartQUICClient connects to a QUIC server at the given address using the provided TLS and QUIC configurations.
// It sends a test message over a newly opened stream and logs the response.
func StartQUICClient(addr string, tlsConfig *tls.Config, quicConfig *quic.Config) error {
	// Create a context with timeout to avoid hanging.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session, err := quic.DialAddrContext(ctx, addr, tlsConfig, quicConfig)
	if err != nil {
		return fmt.Errorf("failed to dial QUIC server at %s: %v", addr, err)
	}
	defer session.CloseWithError(0, "client closing session")
	log.Printf("[INFO] Connected to QUIC server at %s", session.RemoteAddr())

	// Open a stream for communication.
	stream, err := session.OpenStreamSync(ctx)
	if err != nil {
		return fmt.Errorf("failed to open QUIC stream: %v", err)
	}
	defer stream.Close()

	message := "Hello, QUIC Server!"
	_, err = stream.Write([]byte(message))
	if err != nil {
		return fmt.Errorf("failed to write message to stream: %v", err)
	}
	log.Printf("[INFO] Sent message: %s", message)

	// Read and log the response.
	buf := make([]byte, 1024)
	n, err := stream.Read(buf)
	if err != nil {
		return fmt.Errorf("failed to read response from stream: %v", err)
	}
	log.Printf("[INFO] Received response: %s", string(buf[:n]))

	return nil
}
