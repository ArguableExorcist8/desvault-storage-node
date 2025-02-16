package encryption

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"

	"github.com/quic-go/quic-go"
)

// CreateTLSConfig loads a TLS certificate and key from files and returns a TLS configuration.
func CreateTLSConfig(certFile, keyFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS certificate and key: %v", err)
	}
	tlsConf := &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"desvault-quic"},
	}
	return tlsConf, nil
}

// SecureChannelWithTLS starts a QUIC listener on the given address, accepts one session and stream,
// and echoes data back to the client.
func SecureChannelWithTLS(addr string, tlsConfig *tls.Config, quicConfig *quic.Config) error {
	listener, err := quic.ListenAddr(addr, tlsConfig, quicConfig)
	if err != nil {
		return fmt.Errorf("failed to create QUIC listener: %v", err)
	}
	defer listener.Close()
	log.Printf("QUIC listener established on %s", addr)

	session, err := listener.Accept(nil)
	if err != nil {
		return fmt.Errorf("failed to accept QUIC session: %v", err)
	}
	log.Printf("Accepted session from %v", session.RemoteAddr())

	stream, err := session.AcceptStream(nil)
	if err != nil {
		return fmt.Errorf("failed to accept stream: %v", err)
	}
	log.Printf("Accepted stream; echoing data back...")

	buf := make([]byte, 1024)
	for {
		n, err := stream.Read(buf)
		if err != nil {
			if err == io.EOF {
				log.Println("Stream closed by client.")
				break
			}
			return fmt.Errorf("error reading from stream: %v", err)
		}
		data := buf[:n]
		log.Printf("Received: %s", data)
		_, err = stream.Write(data)
		if err != nil {
			return fmt.Errorf("error writing to stream: %v", err)
		}
	}
	log.Println("Secure channel terminated.")
	return nil
}
