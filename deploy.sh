#!/bin/bash

echo "Pulling latest changes..."
git pull origin main

echo "Building Go binary..."
go build -o websocket-server main.go

echo "Creating systemd service if it doesn't exist..."
sudo tee /etc/systemd/system/websocket-server.service > /dev/null <<EOF
[Unit]
Description=WebSocket Server
After=network.target

[Service]
Type=simple
User=co-026
WorkingDirectory=$(pwd)
ExecStart=$(pwd)/websocket-server
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

echo "Reloading systemd daemon..."
sudo systemctl daemon-reload

echo "Enabling service..."
sudo systemctl enable websocket-server

echo "Restarting service..."
sudo systemctl restart websocket-server
echo "Done!"
