# WebSocket Server with Monitoring

A real-time WebSocket server built with Go and Gin, featuring comprehensive monitoring capabilities.

## Features

- **Real-time WebSocket connections** with channel-based messaging
- **Dynamic database schema** that adapts to your data structure
- **RESTful API** for sending notifications
- **Advanced search functionality** with multiple operators
- **Real-time monitoring dashboard** with beautiful UI
- **Server metrics tracking** (memory, CPU, uptime)
- **WebSocket statistics** (connections, messages, success rates)

## Quick Start

1. **Install dependencies:**
   ```bash
   go mod tidy
   ```

2. **Set up environment variables (optional for database):**
   ```bash
   cp .env.example .env
   # Edit .env with your database credentials
   ```

3. **Run the server:**
   ```bash
   go run main.go
   ```

4. **Access the monitoring dashboard:**
   ```
   http://localhost:3000/monitor
   ```

## API Endpoints

### WebSocket
- `GET /ws` - WebSocket connection endpoint

### Notifications
- `POST /notification` - Send notification (requires authentication)
- `GET /notifications` - Get notifications from database (requires authentication)

### Search
- `GET /search` - Advanced search with filters (requires authentication)

### Monitoring
- `GET /monitor` - Real-time monitoring dashboard
- `GET /api/metrics` - JSON API for metrics data

## Authentication

All protected endpoints require these headers:
```
key: key
secret: secret
```

## Monitoring Dashboard

The monitoring dashboard (`/monitor`) provides:

### WebSocket Statistics
- **Active Connections**: Currently connected WebSocket clients
- **Total Connections**: Total connections since server start
- **Success Rate**: Percentage of successful message deliveries
- **Messages Sent**: Total successful message deliveries
- **Messages Failed**: Total failed message deliveries
- **Channel Statistics**: Message count per channel

### Server Metrics
- **Server Uptime**: How long the server has been running
- **Memory Usage**: Current memory allocation
- **Goroutines**: Number of active Go routines
- **Real-time Updates**: Auto-refreshes every 2 seconds

### Features
- **Responsive Design**: Works on desktop and mobile
- **Beautiful UI**: Modern gradient design with glassmorphism effects
- **Real-time Data**: Live updates without page refresh
- **Interactive Elements**: Hover effects and smooth animations

## Testing

Run the test script to see the monitoring in action:

```bash
./test_monitor.sh
```

This will:
1. Start the server
2. Send test notifications
3. Create WebSocket connections
4. Open the monitoring dashboard

## Database Support

The server can run with or without a database:

- **With Database**: Full functionality including data persistence and search
- **Without Database**: WebSocket and notification features only

Set these environment variables for database support:
```
DB_HOST=localhost
DB_PORT=5432
DB_USER=your_user
DB_PASSWORD=your_password
DB_NAME=your_database
DB_SSLMODE=disable
```

## Example Usage

### Send a notification:
```bash
curl -X POST http://localhost:3000/notification \
  -H "Content-Type: application/json" \
  -H "key: key" \
  -H "secret: secret" \
  -d '{
    "channel": "chat-room-1",
    "event": "new-message",
    "data": {
      "message": "Hello World!",
      "sender": "user123",
      "timestamp": "2024-01-01T12:00:00Z"
    }
  }'
```

### Connect to WebSocket:
```javascript
const ws = new WebSocket('ws://localhost:3000/ws');
ws.onopen = () => {
  ws.send(JSON.stringify({channel: 'chat-room-1'}));
};
ws.onmessage = (event) => {
  console.log('Received:', JSON.parse(event.data));
};
```

## Architecture

- **Gin**: HTTP framework for REST API
- **Gorilla WebSocket**: WebSocket implementation
- **PostgreSQL**: Database (optional)
- **Real-time Monitoring**: Custom metrics tracking
- **Responsive UI**: Modern CSS with JavaScript

## License

MIT License
