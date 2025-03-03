#!/bin/bash
# ipfs-daemon.sh

# Set IPFS path
IPFS_PATH="$HOME/.ipfs"
export IPFS_PATH

# Define custom API and swarm ports
API_PORT=5001
SWARM_PORT=4001

# Log file for IPFS daemon output
LOG_FILE="ipfs_daemon.log"

# Ensure IPFS is initialized
if [ ! -d "$IPFS_PATH" ]; then
    echo "IPFS repository not found. Initializing..."
    ipfs init
    if [ $? -ne 0 ]; then
        echo "Failed to initialize IPFS repository. Exiting."
        exit 1
    fi
fi

# Configure IPFS addresses
echo "Configuring IPFS API and Swarm addresses..."
ipfs config Addresses.API "/ip4/127.0.0.1/tcp/$API_PORT"
ipfs config Addresses.Swarm "[\"/ip4/0.0.0.0/tcp/$SWARM_PORT\", \"/ip4/0.0.0.0/udp/$SWARM_PORT/quic\"]"

# Check if IPFS daemon is already running
if curl -s "http://localhost:$API_PORT/api/v0/version" > /dev/null 2>&1; then
    echo "IPFS daemon is already running on port $API_PORT."
    exit 0
fi

# Start IPFS daemon
echo "Starting IPFS daemon..."
rm -f "$LOG_FILE"  # Clear previous log file
ipfs daemon > "$LOG_FILE" 2>&1 &

# Wait for the daemon to become ready
TIMEOUT=180  # Maximum wait time in seconds
INTERVAL=2   # Check interval in seconds
ELAPSED=0

echo "Waiting for IPFS daemon to become ready..."
while true; do
    # Check logs for "Daemon is ready"
    if grep -q "Daemon is ready" "$LOG_FILE"; then
        # Verify API responsiveness
        if curl -s "http://localhost:$API_PORT/api/v0/version" > /dev/null 2>&1; then
            echo "IPFS daemon started successfully and API is responsive."
            exit 0
        else
            echo "Daemon reported ready, but API is not responding yet."
        fi
    fi

    sleep $INTERVAL
    ELAPSED=$((ELAPSED + INTERVAL))

    if [ $ELAPSED -ge $TIMEOUT ]; then
        echo "Timeout ($TIMEOUT seconds) reached waiting for IPFS daemon."
        echo "Last 10 lines of log file ($LOG_FILE):"
        tail -n 10 "$LOG_FILE"
        exit 1
    fi
done