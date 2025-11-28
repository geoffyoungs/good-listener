package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	MaxLogSize       = 50 * 1024 * 1024 // 50MB
	RotationInterval = 24 * time.Hour   // 24 hours
)

// LogEntry represents a debug-level log entry
type LogEntry struct {
	Timestamp  string          `json:"timestamp"`
	SourceIP   string          `json:"source_ip"`
	SourcePort int             `json:"source_port"`
	Protocol   string          `json:"protocol"`
	Payload    string          `json:"payload"`
	PayloadLen int             `json:"payload_len"`
	Encoding   string          `json:"encoding"`          // "ascii", "utf8", or "base64"
	Asterix    *AsterixMessage `json:"asterix,omitempty"` // Decoded ASTERIX data if detected
}

// RotatingLogger handles log writing with automatic rotation
type RotatingLogger struct {
	filename       string
	logLevel       LogLevel
	binaryEncoding BinaryEncoding
	file           *os.File
	currentSize    int64
	lastRotation   time.Time
	mu             sync.Mutex
	rotationTicker *time.Ticker
	stopChan       chan struct{}
}

// NewRotatingLogger creates a new rotating logger
func NewRotatingLogger(filename string, logLevel LogLevel, binaryEncoding BinaryEncoding) (*RotatingLogger, error) {
	logger := &RotatingLogger{
		filename:       filename,
		logLevel:       logLevel,
		binaryEncoding: binaryEncoding,
		lastRotation:   time.Now(),
		stopChan:       make(chan struct{}),
	}

	// Open or create the log file (append mode on restart)
	if err := logger.openExisting(); err != nil {
		return nil, err
	}

	// Start rotation ticker
	logger.rotationTicker = time.NewTicker(1 * time.Minute)
	go logger.checkRotation()

	return logger, nil
}

// openExisting opens an existing log file or creates a new one
func (rl *RotatingLogger) openExisting() error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(rl.filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Check if file exists and get its info
	fileInfo, err := os.Stat(rl.filename)
	if err == nil {
		// File exists - open in append mode and track its current size
		file, err := os.OpenFile(rl.filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}

		rl.file = file
		rl.currentSize = fileInfo.Size()
		rl.lastRotation = fileInfo.ModTime()

		// If file is already over size limit, rotate it now
		if rl.currentSize >= MaxLogSize {
			return rl.rotate()
		}
	} else if os.IsNotExist(err) {
		// File doesn't exist - create new file
		file, err := os.OpenFile(rl.filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("failed to create log file: %w", err)
		}

		rl.file = file
		rl.currentSize = 0
		rl.lastRotation = time.Now()
	} else {
		return fmt.Errorf("failed to stat log file: %w", err)
	}

	return nil
}

// checkRotation periodically checks if rotation is needed
func (rl *RotatingLogger) checkRotation() {
	for {
		select {
		case <-rl.rotationTicker.C:
			rl.mu.Lock()
			if time.Since(rl.lastRotation) >= RotationInterval {
				rl.rotate()
			}
			rl.mu.Unlock()
		case <-rl.stopChan:
			return
		}
	}
}

// rotate closes the current file and opens a new one
func (rl *RotatingLogger) rotate() error {
	// Close existing file
	if rl.file != nil {
		rl.file.Close()
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(rl.filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Rename existing file if it exists
	if _, err := os.Stat(rl.filename); err == nil {
		timestamp := time.Now().Format("20060102-150405")
		rotatedName := fmt.Sprintf("%s.%s", rl.filename, timestamp)
		os.Rename(rl.filename, rotatedName)
	}

	// Open new file
	file, err := os.OpenFile(rl.filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	rl.file = file
	rl.currentSize = 0
	rl.lastRotation = time.Now()

	return nil
}

// encodePayload determines the appropriate encoding for the payload and returns
// the encoded string and encoding type ("ascii", "utf8", "base64", or "hex")
func encodePayload(payload []byte, binaryEncoding BinaryEncoding) (string, string) {
	// Check if it's valid UTF-8
	if !utf8.Valid(payload) {
		// Not valid UTF-8, use configured binary encoding
		return encodeBinary(payload, binaryEncoding)
	}

	// Check if all characters are ASCII (and printable or common whitespace)
	isASCII := true
	for _, b := range payload {
		if b >= 128 {
			isASCII = false
			break
		}
		// Allow printable ASCII (32-126) and common whitespace (9=tab, 10=LF, 13=CR)
		if b < 32 && b != 9 && b != 10 && b != 13 {
			// Contains non-printable control characters, use configured binary encoding
			return encodeBinary(payload, binaryEncoding)
		}
	}

	if isASCII {
		return string(payload), "ascii"
	}

	// Valid UTF-8 with non-ASCII characters
	return string(payload), "utf8"
}

// encodeBinary encodes binary data according to the specified encoding preference
func encodeBinary(payload []byte, binaryEncoding BinaryEncoding) (string, string) {
	if binaryEncoding == BinaryEncodingHex {
		return encodeHex(payload), "hex"
	}
	return base64.StdEncoding.EncodeToString(payload), "base64"
}

// encodeHex encodes binary data as hexadecimal (like hexdump, without offsets)
func encodeHex(payload []byte) string {
	if len(payload) == 0 {
		return ""
	}

	// Format: "00 01 02 03 04 05 06 07 08 09 0a 0b 0c 0d 0e 0f"
	// Similar to hexdump but without offset and ASCII columns
	hexBytes := make([]byte, len(payload)*3-1) // 2 chars per byte + space, minus last space

	for i, b := range payload {
		hexBytes[i*3] = "0123456789abcdef"[b>>4]
		hexBytes[i*3+1] = "0123456789abcdef"[b&0x0f]
		if i < len(payload)-1 {
			hexBytes[i*3+2] = ' '
		}
	}

	return string(hexBytes)
}

// LogData logs data based on the configured log level
func (rl *RotatingLogger) LogData(sourceIP string, sourcePort int, protocol string, payload []byte) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	var logData []byte
	var err error

	if rl.logLevel == LogLevelData {
		// DATA mode: just log the payload
		logData = append(payload, '\n')
	} else {
		// DEBUG mode: log JSON with metadata
		encodedPayload, encoding := encodePayload(payload, rl.binaryEncoding)
		entry := LogEntry{
			Timestamp:  time.Now().Format(time.RFC3339),
			SourceIP:   sourceIP,
			SourcePort: sourcePort,
			Protocol:   protocol,
			Payload:    encodedPayload,
			PayloadLen: len(payload),
			Encoding:   encoding,
		}

		// Check if payload appears to be ASTERIX and decode it
		if isAsterixMessage(payload) {
			asterixData := decodeAsterixMessage(payload)
			entry.Asterix = asterixData
		}

		logData, err = json.Marshal(entry)
		if err != nil {
			return fmt.Errorf("failed to marshal log entry: %w", err)
		}
		logData = append(logData, '\n')
	}

	// Write to file
	n, err := rl.file.Write(logData)
	if err != nil {
		return fmt.Errorf("failed to write to log file: %w", err)
	}

	rl.currentSize += int64(n)

	// Check if rotation is needed due to size
	if rl.currentSize >= MaxLogSize {
		if err := rl.rotate(); err != nil {
			return fmt.Errorf("failed to rotate log file: %w", err)
		}
	}

	return nil
}

// Close closes the logger and stops rotation checks
func (rl *RotatingLogger) Close() error {
	close(rl.stopChan)
	rl.rotationTicker.Stop()

	rl.mu.Lock()
	defer rl.mu.Unlock()

	if rl.file != nil {
		return rl.file.Close()
	}
	return nil
}
