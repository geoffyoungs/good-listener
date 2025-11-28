package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// LogLevel represents the level of detail in logging
type LogLevel string

const (
	LogLevelData  LogLevel = "DATA"
	LogLevelDebug LogLevel = "DEBUG"
)

// ProtocolType represents the network protocol
type ProtocolType string

const (
	ProtocolTCP ProtocolType = "TCP"
	ProtocolUDP ProtocolType = "UDP"
	ProtocolTLS ProtocolType = "TLS"
)

// BinaryEncoding represents how binary data is encoded in logs
type BinaryEncoding string

const (
	BinaryEncodingBase64 BinaryEncoding = "base64"
	BinaryEncodingHex    BinaryEncoding = "hex"
)

// ListenerConfig represents configuration for a single listener
type ListenerConfig struct {
	Port           int            `yaml:"port"`
	Protocol       ProtocolType   `yaml:"protocol"`
	LogFile        string         `yaml:"log_file"`
	LogLevel       LogLevel       `yaml:"log_level"`
	BinaryEncoding BinaryEncoding `yaml:"binary_encoding,omitempty"` // "base64" or "hex", defaults to "base64"
	// TLS-specific configuration
	TLSCertFile string `yaml:"tls_cert_file,omitempty"`
	TLSKeyFile  string `yaml:"tls_key_file,omitempty"`
}

// Config represents the overall configuration
type Config struct {
	Listeners []ListenerConfig `yaml:"listeners"`
}

// LoadConfig loads and parses the configuration file
func LoadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if len(c.Listeners) == 0 {
		return fmt.Errorf("at least one listener must be configured")
	}

	for i, listener := range c.Listeners {
		if listener.Port < 1 || listener.Port > 65535 {
			return fmt.Errorf("listener %d: invalid port %d", i, listener.Port)
		}

		if listener.Protocol != ProtocolTCP && listener.Protocol != ProtocolUDP && listener.Protocol != ProtocolTLS {
			return fmt.Errorf("listener %d: invalid protocol %s (must be TCP, UDP, or TLS)", i, listener.Protocol)
		}

		if listener.LogFile == "" {
			return fmt.Errorf("listener %d: log_file must be specified", i)
		}

		if listener.LogLevel != LogLevelData && listener.LogLevel != LogLevelDebug {
			return fmt.Errorf("listener %d: invalid log_level %s (must be DATA or DEBUG)", i, listener.LogLevel)
		}

		// Set default binary encoding if not specified
		if listener.BinaryEncoding == "" {
			c.Listeners[i].BinaryEncoding = BinaryEncodingBase64
		} else if listener.BinaryEncoding != BinaryEncodingBase64 && listener.BinaryEncoding != BinaryEncodingHex {
			return fmt.Errorf("listener %d: invalid binary_encoding %s (must be base64 or hex)", i, listener.BinaryEncoding)
		}

		if listener.Protocol == ProtocolTLS {
			if listener.TLSCertFile == "" || listener.TLSKeyFile == "" {
				return fmt.Errorf("listener %d: TLS protocol requires tls_cert_file and tls_key_file", i)
			}
		}
	}

	return nil
}
