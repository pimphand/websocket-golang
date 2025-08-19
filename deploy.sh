#!/bin/bash

echo "Pulling latest changes..."
git pull origin main

echo "Building Go binary..."
go build -o websocket-server main.go

echo "Restarting service..."
sudo systemctl restart websocket-server
echo "Done!"
