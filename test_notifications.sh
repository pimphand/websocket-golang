#!/bin/bash

# Test script for WebSocket notifications
# Make sure the Go server is running on localhost:3000

echo "🚀 WebSocket Notification Test Script"
echo "======================================"
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to send notification
send_notification() {
    local channel=$1
    local event=$2
    local message=$3
    local sender=$4
    local amount=$5
    
    echo -e "${BLUE}📤 Sending notification...${NC}"
    echo "Channel: $channel"
    echo "Event: $event"
    echo "Message: $message"
    echo "Sender: $sender"
    echo "Amount: $amount"
    echo ""
    
    curl -X POST http://localhost:3000/notification \
        -H "Content-Type: application/json" \
        -H "key: key" \
        -H "secret: secret" \
        -d "{
            \"channel\": \"$channel\",
            \"event\": \"$event\",
            \"data\": {
                \"message\": \"$message\",
                \"sender\": \"$sender\",
                \"amount\": $amount,
                \"timestamp\": \"$(date -u +"%Y-%m-%dT%H:%M:%SZ")\"
            }
        }"
    
    echo ""
    echo -e "${GREEN}✅ Notification sent!${NC}"
    echo "----------------------------------------"
    echo ""
}

# Function to test different scenarios
test_scenarios() {
    echo -e "${YELLOW}🧪 Testing Different Notification Scenarios${NC}"
    echo ""
    
    # Test 1: New Order
    echo -e "${YELLOW}📦 Test 1: New Order Notification${NC}"
    send_notification "new_order" "order_created" "New order received" "customer123" 150000
    
    sleep 2
    
    # Test 2: Payment Success
    echo -e "${YELLOW}💰 Test 2: Payment Success Notification${NC}"
    send_notification "payment" "payment_success" "Payment processed successfully" "payment_gateway" 150000
    
    sleep 2
    
    # Test 3: Order Status Update
    echo -e "${YELLOW}📋 Test 3: Order Status Update${NC}"
    send_notification "new_order" "status_updated" "Order status changed to 'Processing'" "system" 0
    
    sleep 2
    
    # Test 4: Chat Message
    echo -e "${YELLow}💬 Test 4: Chat Message${NC}"
    send_notification "chat" "new_message" "Hello, how can I help you?" "support_agent" 0
    
    sleep 2
    
    # Test 5: System Alert
    echo -e "${YELLOW}⚠️  Test 5: System Alert${NC}"
    send_notification "system" "high_cpu" "CPU usage is above 80%" "monitoring_system" 85
    
    sleep 2
    
    # Test 6: User Activity
    echo -e "${YELLOW}👤 Test 6: User Activity${NC}"
    send_notification "user_activity" "login" "User logged in successfully" "user456" 0
    
    sleep 2
    
    # Test 7: Inventory Update
    echo -e "${YELLOW}📦 Test 7: Inventory Update${NC}"
    send_notification "inventory" "stock_low" "Product XYZ is running low on stock" "inventory_system" 5
    
    sleep 2
    
    # Test 8: Marketing Campaign
    echo -e "${YELLOW}📢 Test 8: Marketing Campaign${NC}"
    send_notification "marketing" "campaign_launched" "New summer sale campaign launched" "marketing_team" 0
}

# Function to test error scenarios
test_errors() {
    echo -e "${YELLOW}❌ Testing Error Scenarios${NC}"
    echo ""
    
    # Test 1: Invalid credentials
    echo -e "${RED}🔐 Test 1: Invalid Credentials${NC}"
    curl -X POST http://localhost:3000/notification \
        -H "Content-Type: application/json" \
        -H "key: wrong_key" \
        -H "secret: wrong_secret" \
        -d '{
            "channel": "test",
            "event": "test_event",
            "data": {
                "message": "This should fail"
            }
        }'
    echo ""
    echo "Expected: 401 Unauthorized"
    echo ""
    
    sleep 1
    
    # Test 2: Invalid JSON
    echo -e "${RED}📝 Test 2: Invalid JSON${NC}"
    curl -X POST http://localhost:3000/notification \
        -H "Content-Type: application/json" \
        -H "key: key" \
        -H "secret: secret" \
        -d 'invalid json here'
    echo ""
    echo "Expected: 400 Bad Request"
    echo ""
    
    sleep 1
    
    # Test 3: Missing required fields
    echo -e "${RED}📋 Test 3: Missing Required Fields${NC}"
    curl -X POST http://localhost:3000/notification \
        -H "Content-Type: application/json" \
        -H "key: key" \
        -H "secret: secret" \
        -d '{
            "channel": "test"
        }'
    echo ""
    echo "Expected: 400 Bad Request"
    echo ""
}

# Function to test search functionality
test_search() {
    echo -e "${YELLOW}🔍 Testing Search Functionality${NC}"
    echo ""
    
    # Test search with filters
    echo -e "${BLUE}🔍 Test 1: Search with Filters${NC}"
    curl -X GET "http://localhost:3000/search" \
        -H "Content-Type: application/json" \
        -H "key: key" \
        -H "secret: secret" \
        -d '{
            "channel": "new_order",
            "filters": [
                {
                    "field": "event",
                    "op": "==",
                    "value": "order_created"
                }
            ]
        }'
    echo ""
    echo ""
    
    sleep 1
    
    # Test get notifications
    echo -e "${BLUE}📋 Test 2: Get Notifications${NC}"
    curl -X GET "http://localhost:3000/notifications?channel=new_order" \
        -H "key: key" \
        -H "secret: secret"
    echo ""
    echo ""
}

# Function to show WebSocket connection info
show_websocket_info() {
    echo -e "${YELLOW}🔌 WebSocket Connection Information${NC}"
    echo ""
    echo "To test WebSocket connections, open index.html in your browser"
    echo "Or use a WebSocket client to connect to: ws://localhost:3000/ws"
    echo ""
    echo "Example WebSocket subscription message:"
    echo '{"channel": "new_order"}'
    echo ""
    echo "You should receive notifications in real-time when they are sent via curl"
    echo ""
}

# Main menu
main_menu() {
    echo "Choose an option:"
    echo "1) Run all notification tests"
    echo "2) Test error scenarios"
    echo "3) Test search functionality"
    echo "4) Show WebSocket connection info"
    echo "5) Exit"
    echo ""
    read -p "Enter your choice (1-5): " choice
    
    case $choice in
        1)
            test_scenarios
            ;;
        2)
            test_errors
            ;;
        3)
            test_search
            ;;
        4)
            show_websocket_info
            ;;
        5)
            echo "Goodbye! 👋"
            exit 0
            ;;
        *)
            echo "Invalid choice. Please try again."
            main_menu
            ;;
    esac
}

# Check if server is running
check_server() {
    echo -e "${BLUE}🔍 Checking if server is running...${NC}"
    if curl -s http://localhost:3000 > /dev/null 2>&1; then
        echo -e "${GREEN}✅ Server is running on localhost:3000${NC}"
        echo ""
    else
        echo -e "${RED}❌ Server is not running on localhost:3000${NC}"
        echo "Please start the Go server first:"
        echo "go run main.go"
        echo ""
        exit 1
    fi
}

# Run the script
check_server
main_menu
