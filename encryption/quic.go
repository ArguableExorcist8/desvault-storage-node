package encryption

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"time"

	"github.com/quic-go/quic-go"
)

// DialAddrContext dials a QUIC server with a given context, TLS configuration, and QUIC configuration.
func DialAddrContext(ctx context.Context, addr string, tlsConfig *tls.Config, quicConfig *quic.Config) (quic.Connection, error) {
	// Use a channel to capture the dial result.
	type dialResult struct {
		conn quic.Connection
		err  error
	}
	resultCh := make(chan dialResult, 1)

	go func() {
		conn, err := quic.DialAddr(ctx, addr, tlsConfig, quicConfig)
		resultCh <- dialResult{conn: conn, err: err}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-resultCh:
		return res.conn, res.err
	}
}

// DialAddrContextWithRetry attempts to dial a QUIC server with retries and exponential backoff.
// retries: number of additional attempts (total attempts = retries + 1)
// initialDelay: delay for the first retry attempt.
func DialAddrContextWithRetry(ctx context.Context, addr string, tlsConfig *tls.Config, quicConfig *quic.Config, retries int, initialDelay time.Duration) (quic.Connection, error) {
	var conn quic.Connection
	var err error
	delay := initialDelay

	for attempt := 0; attempt <= retries; attempt++ {
		conn, err = DialAddrContext(ctx, addr, tlsConfig, quicConfig)
		if err == nil {
			return conn, nil
		}
		log.Printf("[WARN] Dial attempt %d failed: %v", attempt+1, err)

		// Check if the context is done before retrying.
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
			// Increase delay (exponential backoff).
			delay *= 2
		}
	}
	return nil, fmt.Errorf("all dial attempts failed: last error: %v", err)
}

// StartQUICClient connects to a QUIC server using our custom dialer with retry logic,
// sends a test message over a new stream, and logs the echoed response.
func StartQUICClient(addr string, tlsConfig *tls.Config, quicConfig *quic.Config) error {
	// Create a context with a timeout to avoid hanging indefinitely.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Attempt to dial the QUIC server with retries.
	conn, err := DialAddrContextWithRetry(ctx, addr, tlsConfig, quicConfig, 3, 1*time.Second)
	if err != nil {
		return fmt.Errorf("failed to dial QUIC server at %s: %v", addr, err)
	}
	defer conn.CloseWithError(0, "client closing session")
	log.Printf("[INFO] Connected to QUIC server at %s", conn.RemoteAddr())

	// Open a stream on the connection.
	stream, err := conn.OpenStreamSync(ctx)
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

	// Read the response.
	buf := make([]byte, 1024)
	n, err := stream.Read(buf)
	if err != nil {
		return fmt.Errorf("failed to read response from stream: %v", err)
	}
	log.Printf("[INFO] Received response: %s", string(buf[:n]))

	return nil
}
