#!/bin/bash

# Deteksi user dan folder kerja
CURRENT_USER=$(whoami)
WORK_DIR=$(pwd)

echo "Mengambil perubahan terbaru..."
git pull origin main

echo "Membangun binary Go..."
go build -o websocket-server main.go

echo "Membuat systemd service jika belum ada..."
sudo tee /etc/systemd/system/websocket-server.service > /dev/null <<EOF
[Unit]
Description=WebSocket Server
After=network.target

[Service]
Type=simple
User=${CURRENT_USER}
WorkingDirectory=${WORK_DIR}
ExecStart=${WORK_DIR}/websocket-server
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

echo "Reload systemd daemon..."
sudo systemctl daemon-reload

echo "Enable service agar jalan saat boot..."
sudo systemctl enable websocket-server

echo "Restart service..."
sudo systemctl restart websocket-server

echo "Selesai! Service berjalan dengan user: ${CURRENT_USER}, folder: ${WORK_DIR}"
