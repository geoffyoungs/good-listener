# Good Listener

A lightweight Go-based server that logs network traffic sent to specific ports. Supports TCP, UDP, and TLS protocols with automatic log rotation.

## Features

- **Multiple Protocol Support**: TCP, UDP, and TLS (over TCP)
- **Configurable Logging Levels**:
  - `DATA`: Logs only the raw payload
  - `DEBUG`: Logs JSON with timestamp, source IP/port, payload, and metadata
- **Automatic Log Rotation**:
  - Time-based: Every 24 hours
  - Size-based: When log file exceeds 50MB
  - Appends to existing logs on restart
- **YAML Configuration**: Easy-to-use configuration file
- **Concurrent Listeners**: Run multiple listeners on different ports simultaneously

## Installation

```bash
# Build the binary
go build -o good-listener

# Or run directly
go run .
```

## Configuration

Create a `config.yaml` file with your listener configurations:

```yaml
listeners:
  # TCP listener
  - port: 8080
    protocol: TCP
    log_file: ./logs/tcp_8080.log
    log_level: DEBUG

  # UDP listener
  - port: 5353
    protocol: UDP
    log_file: ./logs/udp_5353.log
    log_level: DATA

  # TLS listener
  - port: 8443
    protocol: TLS
    log_file: ./logs/tls_8443.log
    log_level: DEBUG
    tls_cert_file: ./certs/server.crt
    tls_key_file: ./certs/server.key
```

### Configuration Options

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `port` | int | Yes | Port number to listen on (1-65535) |
| `protocol` | string | Yes | Protocol type: `TCP`, `UDP`, or `TLS` |
| `log_file` | string | Yes | Path to the log file |
| `log_level` | string | Yes | Logging detail: `DATA` or `DEBUG` |
| `tls_cert_file` | string | TLS only | Path to TLS certificate file |
| `tls_key_file` | string | TLS only | Path to TLS private key file |

### Log Levels

**DATA Mode** - Logs only the raw payload:
```
GET / HTTP/1.1
Host: example.com

```

**DEBUG Mode** - Logs structured JSON with metadata:
```json
{"timestamp":"2025-11-27T10:30:00Z","source_ip":"192.168.1.100","source_port":54321,"protocol":"TCP","payload":"GET / HTTP/1.1\nHost: example.com\n\n","payload_len":32,"encoding":"ascii"}
```

The `encoding` field indicates how the payload is encoded:
- **`ascii`**: Pure ASCII text (all bytes < 128, printable characters)
- **`utf8`**: Valid UTF-8 with non-ASCII characters (e.g., emoji, international characters)
- **`base64`**: Binary data or non-printable characters, base64-encoded for safe JSON representation

Example with binary data:
```json
{"timestamp":"2025-11-27T10:30:00Z","source_ip":"192.168.1.100","source_port":54321,"protocol":"TCP","payload":"AAECAwQFBgcICQ==","payload_len":10,"encoding":"base64"}
```

## Usage

Run with default configuration file (`config.yaml`):
```bash
./good-listener
```

Run with custom configuration file:
```bash
./good-listener -config /path/to/config.yaml
```

Stop the server with `Ctrl+C` for graceful shutdown.

## Log Rotation

**On Server Restart**: When the server restarts, it automatically appends to existing log files. The time-based rotation counter continues from the file's last modification time, ensuring logs aren't unnecessarily rotated on restart.

Logs are automatically rotated when:
1. **Time-based**: Every 24 hours from the log file's creation or last rotation
2. **Size-based**: When the log file exceeds 50MB

When a rotation occurs, the existing file is renamed with a timestamp and a new file is created:
```
tcp_8080.log.20251127-103000
tcp_8080.log.20251128-103000
tcp_8080.log              # Current active log
```

## Generating TLS Certificates (for testing)

For testing purposes, you can generate self-signed certificates:

```bash
mkdir -p certs
openssl req -x509 -newkey rsa:4096 -keyout certs/server.key -out certs/server.crt -days 365 -nodes -subj "/CN=localhost"
```

**Warning**: Self-signed certificates should only be used for testing, not production.

## Testing the Listeners

### TCP Listener
```bash
# Send data via telnet
echo "Hello TCP" | nc localhost 8080

# Or use curl
curl http://localhost:8080/test
```

### UDP Listener
```bash
# Send UDP packet
echo "Hello UDP" | nc -u localhost 5353
```

### TLS Listener
```bash
# Send data via OpenSSL
echo "Hello TLS" | openssl s_client -connect localhost:8443 -quiet
```

## Production Deployment (Ubuntu/Systemd)

### Building for Linux

Build the x86_64 Linux binary:
```bash
./build-linux.sh
```

This creates `good-listener-linux-amd64` suitable for deployment on Linux servers.

### Installing as a System Service

On Ubuntu 25.10 (or other systemd-based Linux distributions):

1. Build the Linux binary (see above)
2. Copy the project files to your server
3. Run the installation script as root:

```bash
sudo ./install-ubuntu.sh
```

The installation script will:
- Install the binary to `/usr/local/bin/good-listener`
- Create a system user `good-listener`
- Install the configuration to `/etc/good-listener.yaml` (listening on port 80887)
- Create log directory at `/var/log/good-listener`
- Install and enable the systemd service
- Start the service automatically

### Managing the Service

```bash
# View logs in real-time
sudo journalctl -u good-listener -f

# Restart the service
sudo systemctl restart good-listener

# Stop the service
sudo systemctl stop good-listener

# Check service status
sudo systemctl status good-listener

# Edit configuration
sudo nano /etc/good-listener.yaml
# Then restart to apply changes
sudo systemctl restart good-listener
```

## Project Structure

```
.
├── main.go                    # Main entry point and orchestrator
├── config.go                  # Configuration parsing and validation
├── logger.go                  # Rotating logger implementation
├── tcp_listener.go            # TCP and TLS listener implementations
├── udp_listener.go            # UDP listener implementation
├── config.yaml                # Example configuration file
├── good-listener-prod.yaml    # Production configuration
├── good-listener.service      # Systemd service file
├── build-linux.sh             # Build script for Linux x86_64
├── install-ubuntu.sh          # Ubuntu installation script
└── README.md                  # This file
```

## Requirements

- Go 1.16 or higher
- Network ports available for listening
- Write permissions for log file directories

## License

MIT
