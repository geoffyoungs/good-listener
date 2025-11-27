#!/bin/bash

# Build script for good-listener (x86_64 Linux target)

set -e

echo "Building good-listener for Linux (x86_64)..."

GOOS=linux GOARCH=amd64 go build -o good-listener-linux-amd64

echo "Build complete: good-listener-linux-amd64"
echo "File size: $(ls -lh good-listener-linux-amd64 | awk '{print $5}')"
