package encryption

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"

	"github.com/quic-go/quic-go"
)

// CreateTLSConfig loads a TLS certificate and key from files and returns a TLS configuration.
// In production, certificates and keys should be managed securely.
func CreateTLSConfig(certFile, keyFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS certificate and key: %v", err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"desvault-quic"},
	}, nil
}

// SecureChannelWithTLS starts a QUIC listener on the given address using the provided TLS and QUIC configurations.
// It accepts incoming connections and handles streams concurrently, echoing received data back.
func SecureChannelWithTLS(addr string, tlsConfig *tls.Config, quicConfig *quic.Config) error {
	listener, err := quic.ListenAddr(addr, tlsConfig, quicConfig)
	if err != nil {
		return fmt.Errorf("failed to create QUIC listener on %s: %v", addr, err)
	}
	defer listener.Close()
	log.Printf("[INFO] QUIC listener established on %s", addr)

	// Continuously accept incoming connections.
	for {
		conn, err := listener.Accept(context.Background())
		if err != nil {
			log.Printf("[ERROR] Failed to accept QUIC connection: %v", err)
			continue
		}
		go handleConnection(conn)
	}
}

// handleConnection handles an individual QUIC connection by continuously accepting streams.
func handleConnection(conn quic.Connection) {
	log.Printf("[INFO] Accepted connection from %v", conn.RemoteAddr())
	defer conn.CloseWithError(0, "connection closed")
	for {
		stream, err := conn.AcceptStream(context.Background())
		if err != nil {
			log.Printf("[ERROR] Failed to accept stream: %v", err)
			return
		}
		go handleStream(stream)
	}
}

// handleStream processes a single QUIC stream by reading data and echoing it back.
func handleStream(stream quic.Stream) {
	log.Printf("[INFO] Accepted new stream (ID: %d)", stream.StreamID())
	defer stream.Close()
	buf := make([]byte, 1024)
	for {
		n, err := stream.Read(buf)
		if err != nil {
			if err == io.EOF {
				log.Println("[INFO] Stream closed by client")
				return
			}
			log.Printf("[ERROR] Error reading from stream: %v", err)
			return
		}
		data := buf[:n]
		log.Printf("[INFO] Received: %s", data)
		if _, err := stream.Write(data); err != nil {
			log.Printf("[ERROR] Error writing to stream: %v", err)
			return
		}
	}
}
