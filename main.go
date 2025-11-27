package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// Listener interface for all listener types
type Listener interface {
	Start() error
	Stop() error
}

func main() {
	// Parse command-line flags
	configFile := flag.String("config", "config.yaml", "Path to configuration file")
	flag.Parse()

	// Load configuration
	config, err := LoadConfig(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	// Create listeners based on configuration
	var listeners []Listener
	for _, listenerConfig := range config.Listeners {
		var listener Listener
		var err error

		switch listenerConfig.Protocol {
		case ProtocolTCP:
			listener, err = NewTCPListener(listenerConfig)
		case ProtocolUDP:
			listener, err = NewUDPListener(listenerConfig)
		case ProtocolTLS:
			listener, err = NewTLSListener(listenerConfig)
		default:
			fmt.Fprintf(os.Stderr, "Unknown protocol: %s\n", listenerConfig.Protocol)
			continue
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create %s listener on port %d: %v\n",
				listenerConfig.Protocol, listenerConfig.Port, err)
			os.Exit(1)
		}

		listeners = append(listeners, listener)
	}

	// Start all listeners
	for _, listener := range listeners {
		if err := listener.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to start listener: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Printf("Network traffic logger started with %d listener(s)\n", len(listeners))
	fmt.Println("Press Ctrl+C to stop...")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	// Graceful shutdown
	fmt.Println("\nShutting down...")
	for _, listener := range listeners {
		if err := listener.Stop(); err != nil {
			fmt.Fprintf(os.Stderr, "Error stopping listener: %v\n", err)
		}
	}

	fmt.Println("Server stopped")
}
