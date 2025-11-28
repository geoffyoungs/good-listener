package main

import (
	"fmt"
	"net"
)

// UDPListener listens for UDP packets and logs traffic
type UDPListener struct {
	config   ListenerConfig
	logger   *RotatingLogger
	conn     *net.UDPConn
	stopChan chan struct{}
}

// NewUDPListener creates a new UDP listener
func NewUDPListener(config ListenerConfig) (*UDPListener, error) {
	logger, err := NewRotatingLogger(config.LogFile, config.LogLevel, config.BinaryEncoding)
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	return &UDPListener{
		config:   config,
		logger:   logger,
		stopChan: make(chan struct{}),
	}, nil
}

// Start begins listening for UDP packets
func (ul *UDPListener) Start() error {
	addr := &net.UDPAddr{
		Port: ul.config.Port,
		IP:   net.ParseIP("0.0.0.0"),
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("failed to start UDP listener on port %d: %w", ul.config.Port, err)
	}

	ul.conn = conn
	fmt.Printf("UDP listener started on port %d, logging to %s\n", ul.config.Port, ul.config.LogFile)

	go ul.receivePackets()
	return nil
}

// receivePackets receives and logs UDP packets
func (ul *UDPListener) receivePackets() {
	buf := make([]byte, 65535) // Maximum UDP packet size

	for {
		select {
		case <-ul.stopChan:
			return
		default:
			n, remoteAddr, err := ul.conn.ReadFromUDP(buf)
			if err != nil {
				select {
				case <-ul.stopChan:
					return
				default:
					fmt.Printf("UDP read error on port %d: %v\n", ul.config.Port, err)
					continue
				}
			}

			if n > 0 {
				sourceIP := remoteAddr.IP.String()
				sourcePort := remoteAddr.Port

				// Log the received data
				if err := ul.logger.LogData(sourceIP, sourcePort, "UDP", buf[:n]); err != nil {
					fmt.Printf("Failed to log UDP data: %v\n", err)
				}
			}
		}
	}
}

// Stop stops the UDP listener
func (ul *UDPListener) Stop() error {
	close(ul.stopChan)
	if ul.conn != nil {
		ul.conn.Close()
	}
	if ul.logger != nil {
		return ul.logger.Close()
	}
	return nil
}
