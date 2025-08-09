#!/bin/bash
cd /opt/websocket-server || exit

echo "Pulling latest changes..."
git pull origin main

echo "Building Go binary..."
go build -o websocket-server main.go

echo "Restarting service..."
systemctl restart websocket-server
echo "Done!"
