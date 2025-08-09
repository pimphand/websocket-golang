// WebSocket Test Client
// Save this as websocket_test.js and open in browser or run with Node.js

class WebSocketTestClient {
    constructor(url = 'ws://localhost:3000/ws') {
        this.url = url;
        this.ws = null;
        this.isConnected = false;
        this.receivedMessages = [];
        this.maxMessages = 50; // Keep last 50 messages
    }

    connect(channel = 'new_order') {
        console.log(`🔌 Connecting to WebSocket: ${this.url}`);
        
        this.ws = new WebSocket(this.url);
        
        this.ws.onopen = () => {
            console.log('✅ WebSocket connected!');
            this.isConnected = true;
            
            // Subscribe to channel
            const subscription = { channel: channel };
            this.ws.send(JSON.stringify(subscription));
            console.log(`📡 Subscribed to channel: ${channel}`);
        };
        
        this.ws.onmessage = (event) => {
            try {
                const data = JSON.parse(event.data);
                this.handleMessage(data);
            } catch (error) {
                console.error('❌ Error parsing message:', error);
            }
        };
        
        this.ws.onerror = (error) => {
            console.error('❌ WebSocket error:', error);
            this.isConnected = false;
        };
        
        this.ws.onclose = () => {
            console.log('🔌 WebSocket disconnected');
            this.isConnected = false;
        };
    }
    
    handleMessage(data) {
        // Skip subscription confirmation messages
        if (data.message && data.message === 'Subscribed to channel') {
            console.log(`✅ ${data.message}: ${data.channel}`);
            return;
        }
        
        // Handle notification messages
        if (data.event) {
            const timestamp = new Date().toLocaleTimeString();
            const message = {
                timestamp: timestamp,
                channel: data.channel,
                event: data.event,
                data: data.data
            };
            
            this.receivedMessages.unshift(message);
            
            // Keep only last maxMessages
            if (this.receivedMessages.length > this.maxMessages) {
                this.receivedMessages = this.receivedMessages.slice(0, this.maxMessages);
            }
            
            this.displayMessage(message);
            this.updateMessageCount();
        }
    }
    
    displayMessage(message) {
        console.log(`📨 [${message.timestamp}] ${message.event}:`, message.data);
        
        // If running in browser, update DOM
        if (typeof document !== 'undefined') {
            this.updateDOM(message);
        }
    }
    
    updateDOM(message) {
        const container = document.getElementById('notifications') || this.createContainer();
        
        const messageElement = document.createElement('div');
        messageElement.className = 'notification-item';
        messageElement.innerHTML = `
            <div class="notification-header">
                <span class="timestamp">${message.timestamp}</span>
                <span class="channel">${message.channel}</span>
                <span class="event">${message.event}</span>
            </div>
            <div class="notification-data">
                ${this.formatData(message.data)}
            </div>
        `;
        
        container.insertBefore(messageElement, container.firstChild);
        
        // Keep only last 20 messages in DOM
        const items = container.querySelectorAll('.notification-item');
        if (items.length > 20) {
            container.removeChild(items[items.length - 1]);
        }
    }
    
    createContainer() {
        const container = document.createElement('div');
        container.id = 'notifications';
        container.className = 'notifications-container';
        container.style.cssText = `
            max-width: 800px;
            margin: 20px auto;
            padding: 20px;
            background: #f5f5f5;
            border-radius: 8px;
            font-family: Arial, sans-serif;
        `;
        
        const header = document.createElement('h2');
        header.textContent = 'WebSocket Notifications';
        header.style.cssText = 'color: #333; margin-bottom: 20px;';
        
        const countDiv = document.createElement('div');
        countDiv.id = 'message-count';
        countDiv.style.cssText = 'color: #666; margin-bottom: 10px;';
        
        container.appendChild(header);
        container.appendChild(countDiv);
        
        document.body.appendChild(container);
        return container;
    }
    
    formatData(data) {
        if (!data) return '<em>No data</em>';
        
        return Object.entries(data)
            .map(([key, value]) => `<strong>${key}:</strong> ${value}`)
            .join('<br>');
    }
    
    updateMessageCount() {
        const countElement = document.getElementById('message-count');
        if (countElement) {
            countElement.textContent = `Received ${this.receivedMessages.length} messages`;
        }
    }
    
    disconnect() {
        if (this.ws) {
            this.ws.close();
            console.log('🔌 Disconnected from WebSocket');
        }
    }
    
    getMessages() {
        return this.receivedMessages;
    }
    
    clearMessages() {
        this.receivedMessages = [];
        this.updateMessageCount();
        
        const container = document.getElementById('notifications');
        if (container) {
            const items = container.querySelectorAll('.notification-item');
            items.forEach(item => item.remove());
        }
    }
}

// Auto-start if running in browser
if (typeof window !== 'undefined') {
    console.log('🌐 Browser environment detected');
    
    // Create a simple UI
    const style = document.createElement('style');
    style.textContent = `
        .notification-item {
            background: white;
            border: 1px solid #ddd;
            border-radius: 4px;
            padding: 10px;
            margin-bottom: 10px;
            box-shadow: 0 1px 3px rgba(0,0,0,0.1);
        }
        .notification-header {
            display: flex;
            justify-content: space-between;
            margin-bottom: 8px;
            font-size: 12px;
            color: #666;
        }
        .notification-data {
            font-size: 14px;
            line-height: 1.4;
        }
        .timestamp { font-weight: bold; }
        .channel { color: #007bff; }
        .event { color: #28a745; }
    `;
    document.head.appendChild(style);
    
    // Create client and connect
    const client = new WebSocketTestClient();
    client.connect('new_order');
    
    // Make client globally available
    window.wsClient = client;
    
    console.log('💡 Use window.wsClient to interact with the WebSocket client');
    console.log('   Example: window.wsClient.disconnect()');
    console.log('   Example: window.wsClient.getMessages()');
    console.log('   Example: window.wsClient.clearMessages()');
}

// Export for Node.js
if (typeof module !== 'undefined' && module.exports) {
    module.exports = WebSocketTestClient;
}
