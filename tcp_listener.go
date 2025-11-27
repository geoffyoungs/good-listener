package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"strings"
)

// TCPListener listens for TCP connections and logs traffic
type TCPListener struct {
	config   ListenerConfig
	logger   *RotatingLogger
	listener net.Listener
	stopChan chan struct{}
}

// NewTCPListener creates a new TCP listener
func NewTCPListener(config ListenerConfig) (*TCPListener, error) {
	logger, err := NewRotatingLogger(config.LogFile, config.LogLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	return &TCPListener{
		config:   config,
		logger:   logger,
		stopChan: make(chan struct{}),
	}, nil
}

// Start begins listening for TCP connections
func (tl *TCPListener) Start() error {
	addr := fmt.Sprintf(":%d", tl.config.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to start TCP listener on port %d: %w", tl.config.Port, err)
	}

	tl.listener = listener
	fmt.Printf("TCP listener started on port %d, logging to %s\n", tl.config.Port, tl.config.LogFile)

	go tl.acceptConnections()
	return nil
}

// acceptConnections accepts incoming TCP connections
func (tl *TCPListener) acceptConnections() {
	for {
		conn, err := tl.listener.Accept()
		if err != nil {
			select {
			case <-tl.stopChan:
				return
			default:
				fmt.Printf("TCP listener error on port %d: %v\n", tl.config.Port, err)
				continue
			}
		}

		go tl.handleConnection(conn)
	}
}

// handleConnection handles a single TCP connection
func (tl *TCPListener) handleConnection(conn net.Conn) {
	defer conn.Close()

	// Get remote address
	remoteAddr := conn.RemoteAddr().(*net.TCPAddr)
	sourceIP := remoteAddr.IP.String()
	sourcePort := remoteAddr.Port

	// Read data from connection
	buf := make([]byte, 4096)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			if err != io.EOF {
				fmt.Printf("TCP read error from %s:%d: %v\n", sourceIP, sourcePort, err)
			}
			break
		}

		if n > 0 {
			// Log the received data
			if err := tl.logger.LogData(sourceIP, sourcePort, "TCP", buf[:n]); err != nil {
				fmt.Printf("Failed to log TCP data: %v\n", err)
			}
		}
	}
}

// Stop stops the TCP listener
func (tl *TCPListener) Stop() error {
	close(tl.stopChan)
	if tl.listener != nil {
		tl.listener.Close()
	}
	if tl.logger != nil {
		return tl.logger.Close()
	}
	return nil
}

// TLSListener listens for TLS connections and logs traffic
type TLSListener struct {
	config   ListenerConfig
	logger   *RotatingLogger
	listener net.Listener
	stopChan chan struct{}
}

// NewTLSListener creates a new TLS listener
func NewTLSListener(config ListenerConfig) (*TLSListener, error) {
	logger, err := NewRotatingLogger(config.LogFile, config.LogLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	return &TLSListener{
		config:   config,
		logger:   logger,
		stopChan: make(chan struct{}),
	}, nil
}

// Start begins listening for TLS connections
func (tl *TLSListener) Start() error {
	addr := fmt.Sprintf(":%d", tl.config.Port)

	// Load TLS certificate and key
	cert, err := tls.LoadX509KeyPair(tl.config.TLSCertFile, tl.config.TLSKeyFile)
	if err != nil {
		return fmt.Errorf("failed to load TLS certificate: %w", err)
	}

	// Configure TLS
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	// Create TLS listener
	listener, err := tls.Listen("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to start TLS listener on port %d: %w", tl.config.Port, err)
	}

	tl.listener = listener
	fmt.Printf("TLS listener started on port %d, logging to %s\n", tl.config.Port, tl.config.LogFile)

	go tl.acceptConnections()
	return nil
}

// acceptConnections accepts incoming TLS connections
func (tl *TLSListener) acceptConnections() {
	for {
		conn, err := tl.listener.Accept()
		if err != nil {
			select {
			case <-tl.stopChan:
				return
			default:
				fmt.Printf("TLS listener error on port %d: %v\n", tl.config.Port, err)
				continue
			}
		}

		go tl.handleConnection(conn)
	}
}

// handleConnection handles a single TLS connection
func (tl *TLSListener) handleConnection(conn net.Conn) {
	defer conn.Close()

	// Get remote address
	remoteAddrStr := conn.RemoteAddr().String()
	parts := strings.Split(remoteAddrStr, ":")
	sourceIP := strings.Join(parts[:len(parts)-1], ":")
	sourcePort := 0
	fmt.Sscanf(parts[len(parts)-1], "%d", &sourcePort)

	// Read data from connection
	buf := make([]byte, 4096)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			if err != io.EOF {
				fmt.Printf("TLS read error from %s:%d: %v\n", sourceIP, sourcePort, err)
			}
			break
		}

		if n > 0 {
			// Log the received data
			if err := tl.logger.LogData(sourceIP, sourcePort, "TLS", buf[:n]); err != nil {
				fmt.Printf("Failed to log TLS data: %v\n", err)
			}
		}
	}
}

// Stop stops the TLS listener
func (tl *TLSListener) Stop() error {
	close(tl.stopChan)
	if tl.listener != nil {
		tl.listener.Close()
	}
	if tl.logger != nil {
		return tl.logger.Close()
	}
	return nil
}
