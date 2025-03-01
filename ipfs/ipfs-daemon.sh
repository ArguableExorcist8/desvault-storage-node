#!/bin/bash
# ipfs-daemon.sh

IPFS_PATH="$HOME/.ipfs"
export IPFS_PATH

# Set custom API and swarm ports.
API_PORT=5001
SWARM_PORT=4001

ipfs config Addresses.API "/ip4/127.0.0.1/tcp/$API_PORT"
ipfs config Addresses.Swarm "[\"/ip4/0.0.0.0/tcp/$SWARM_PORT\", \"/ip4/0.0.0.0/udp/$SWARM_PORT/quic\"]"

echo "Starting IPFS daemon..."
ipfs daemon > ipfs_daemon.log 2>&1 &
# Give the daemon time to start.
sleep 60
echo "IPFS daemon started."
