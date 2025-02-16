package encryption

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"

	"github.com/quic-go/quic-go"
)

// StartQUICClient connects to a QUIC server at the specified address.
func StartQUICClient(addr string, tlsConfig *tls.Config, quicConfig *quic.Config) error {
	// Fixed: Added context.Background() as the first argument.
	session, err := quic.DialAddr(context.Background(), addr, tlsConfig, quicConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to QUIC server: %v", err)
	}
	log.Printf("Connected to server: %v", session.RemoteAddr())

	// Client can now send/receive data through streams.
	stream, err := session.OpenStreamSync(context.Background())
	if err != nil {
		return fmt.Errorf("failed to open QUIC stream: %v", err)
	}
	defer stream.Close()

	message := "Hello, QUIC Server!"
	_, err = stream.Write([]byte(message))
	if err != nil {
		return fmt.Errorf("failed to send message: %v", err)
	}
	log.Printf("Sent message: %s", message)

	// Read response
	buf := make([]byte, 1024)
	n, err := stream.Read(buf)
	if err != nil {
		return fmt.Errorf("failed to read response: %v", err)
	}
	log.Printf("Received response: %s", buf[:n])

	return nil
}
