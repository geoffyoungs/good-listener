#!/bin/bash

# Installation script for good-listener on Ubuntu
# This script installs good-listener as a systemd service

set -e

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    echo "ERROR: This script must be run as root (use sudo)"
    exit 1
fi

echo "========================================="
echo "Good Listener Installation Script"
echo "========================================="
echo ""

# Detect the directory where the script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Check if binary exists
if [ ! -f "$SCRIPT_DIR/good-listener-linux-amd64" ]; then
    echo "ERROR: good-listener-linux-amd64 not found in $SCRIPT_DIR"
    echo "Please run ./build-linux.sh first to build the binary"
    exit 1
fi

# Create system user and group if they don't exist
if ! id -u good-listener > /dev/null 2>&1; then
    echo "Creating system user and group 'good-listener'..."
    useradd --system --no-create-home --shell /bin/false good-listener
else
    echo "User 'good-listener' already exists"
fi

# Install binary
echo "Installing binary to /usr/local/bin/good-listener..."
cp "$SCRIPT_DIR/good-listener-linux-amd64" /usr/local/bin/good-listener
chmod 755 /usr/local/bin/good-listener
chown root:root /usr/local/bin/good-listener

# Install configuration file
echo "Installing configuration to /etc/good-listener.yaml..."
if [ -f "/etc/good-listener.yaml" ]; then
    echo "WARNING: /etc/good-listener.yaml already exists"
    echo "Creating backup at /etc/good-listener.yaml.backup"
    cp /etc/good-listener.yaml /etc/good-listener.yaml.backup
fi
cp "$SCRIPT_DIR/good-listener-prod.yaml" /etc/good-listener.yaml
chmod 644 /etc/good-listener.yaml
chown root:root /etc/good-listener.yaml

# Create log directory
echo "Creating log directory at /var/log/good-listener..."
mkdir -p /var/log/good-listener
chown good-listener:good-listener /var/log/good-listener
chmod 755 /var/log/good-listener

# Install systemd service
echo "Installing systemd service..."
cp "$SCRIPT_DIR/good-listener.service" /etc/systemd/system/good-listener.service
chmod 644 /etc/systemd/system/good-listener.service
chown root:root /etc/systemd/system/good-listener.service

# Reload systemd
echo "Reloading systemd daemon..."
systemctl daemon-reload

# Enable service to start on boot
echo "Enabling good-listener service to start on boot..."
systemctl enable good-listener

# Start the service
echo "Starting good-listener service..."
systemctl start good-listener

# Check status
echo ""
echo "========================================="
echo "Installation Complete!"
echo "========================================="
echo ""
echo "Service status:"
systemctl status good-listener --no-pager || true
echo ""
echo "Configuration file: /etc/good-listener.yaml"
echo "Log directory: /var/log/good-listener"
echo "Binary location: /usr/local/bin/good-listener"
echo ""
echo "Useful commands:"
echo "  View logs:       sudo journalctl -u good-listener -f"
echo "  Restart service: sudo systemctl restart good-listener"
echo "  Stop service:    sudo systemctl stop good-listener"
echo "  Service status:  sudo systemctl status good-listener"
echo "  Edit config:     sudo nano /etc/good-listener.yaml"
echo "  After editing config, restart: sudo systemctl restart good-listener"
echo ""
