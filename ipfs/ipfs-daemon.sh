#!/bin/bash

echo "[+] Starting IPFS daemon..."
ipfs daemon &

echo "[+] Starting DesVault Storage Node..."
cd ~/github.com/ArguableExorcist8/desvault-storage-node/
./desvault run &

echo "[+] IPFS and Storage Node are now running!"
