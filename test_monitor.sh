#!/bin/bash

echo "🚀 Starting WebSocket Monitor Test"
echo "=================================="

# Start the server in background
echo "Starting server..."
./main &
SERVER_PID=$!

# Wait for server to start
sleep 2

echo "Server started with PID: $SERVER_PID"
echo "Monitor available at: http://localhost:3000/monitor"
echo ""

# Test 1: Send some notifications
echo "📨 Sending test notifications..."
for i in {1..5}; do
    curl -X POST http://localhost:3000/notification \
        -H "Content-Type: application/json" \
        -H "key: key" \
        -H "secret: secret" \
        -d "{
            \"channel\": \"test-channel-$i\",
            \"event\": \"test-event\",
            \"data\": {
                \"message\": \"Test message $i\",
                \"sender\": \"test-user\",
                \"timestamp\": \"$(date)\"
            }
        }" &
done

# Test 2: Create WebSocket connections
echo "🔌 Creating WebSocket connections..."
for i in {1..3}; do
    (
        echo '{"channel": "test-channel-'$i'"}' | websocat ws://localhost:3000/ws &
    ) &
done

echo ""
echo "✅ Test setup complete!"
echo "📊 Check the monitor at: http://localhost:3000/monitor"
echo "Press Ctrl+C to stop the server"

# Keep the script running
wait $SERVER_PID 