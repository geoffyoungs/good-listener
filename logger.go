package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	MaxLogSize       = 50 * 1024 * 1024 // 50MB
	RotationInterval = 24 * time.Hour   // 24 hours
)

// LogEntry represents a debug-level log entry
type LogEntry struct {
	Timestamp  string `json:"timestamp"`
	SourceIP   string `json:"source_ip"`
	SourcePort int    `json:"source_port"`
	Protocol   string `json:"protocol"`
	Payload    string `json:"payload"`
	PayloadLen int    `json:"payload_len"`
}

// RotatingLogger handles log writing with automatic rotation
type RotatingLogger struct {
	filename       string
	logLevel       LogLevel
	file           *os.File
	currentSize    int64
	lastRotation   time.Time
	mu             sync.Mutex
	rotationTicker *time.Ticker
	stopChan       chan struct{}
}

// NewRotatingLogger creates a new rotating logger
func NewRotatingLogger(filename string, logLevel LogLevel) (*RotatingLogger, error) {
	logger := &RotatingLogger{
		filename:     filename,
		logLevel:     logLevel,
		lastRotation: time.Now(),
		stopChan:     make(chan struct{}),
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
		entry := LogEntry{
			Timestamp:  time.Now().Format(time.RFC3339),
			SourceIP:   sourceIP,
			SourcePort: sourcePort,
			Protocol:   protocol,
			Payload:    string(payload),
			PayloadLen: len(payload),
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
