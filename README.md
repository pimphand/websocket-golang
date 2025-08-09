# WebSocket Notification System

A real-time notification system built with Go, WebSocket, and PostgreSQL (optional).

## Features

- 🔔 Real-time WebSocket notifications
- 🔐 Authentication with key/secret
- 📊 Dynamic database schema (PostgreSQL)
- 🔍 Search and filter notifications
- 📱 Modern web interface
- 🧪 Comprehensive testing tools

## Quick Start

### 1. Start the Server

```bash
# Run without database (recommended for testing)
go run main.go

# Or with database (requires PostgreSQL)
# Set environment variables first
export DB_HOST=localhost
export DB_PORT=5432
export DB_USER=your_user
export DB_PASSWORD=your_password
export DB_NAME=your_database
export DB_SSLMODE=disable

go run main.go
```

The server will start on `http://localhost:3000`

### 2. Open the Web Interface

Open `index.html` in your browser to see the real-time notification interface.

### 3. Test with Curl

Use the provided test script to send notifications:

```bash
./test_notifications.sh
```

## API Endpoints

### Send Notification
```bash
curl -X POST http://localhost:3000/notification \
  -H "Content-Type: application/json" \
  -H "key: key" \
  -H "secret: secret" \
  -d '{
    "channel": "new_order",
    "event": "order_created",
    "data": {
      "message": "New order received",
      "sender": "customer123",
      "amount": 150000
    }
  }'
```

### WebSocket Connection
```javascript
const ws = new WebSocket("ws://localhost:3000/ws");
ws.onopen = () => {
  ws.send(JSON.stringify({ channel: "new_order" }));
};
ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log(data);
};
```

### Search Notifications
```bash
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
```

### Get Notifications
```bash
curl -X GET "http://localhost:3000/notifications?channel=new_order" \
  -H "key: key" \
  -H "secret: secret"
```

## Authentication

All API endpoints require authentication using headers:
- `key: key`
- `secret: secret`

## Database (Optional)

The system can run with or without a PostgreSQL database:

- **Without database**: Notifications are only broadcast via WebSocket
- **With database**: Notifications are stored and can be searched/retrieved

### Database Setup

1. Install PostgreSQL
2. Create a database
3. Set environment variables:
   ```bash
   export DB_HOST=localhost
   export DB_PORT=5432
   export DB_USER=your_user
   export DB_PASSWORD=your_password
   export DB_NAME=your_database
   export DB_SSLMODE=disable
   ```

The system will automatically create tables based on the notification data structure.

## Testing

### Automated Tests

```bash
# Run Go tests
go test

# Run with verbose output
go test -v

# Run benchmarks
go test -bench=.
```

### Manual Testing

1. **Start the server**: `go run main.go`
2. **Open index.html** in your browser
3. **Connect to WebSocket** using the interface
4. **Send notifications** using the test script: `./test_notifications.sh`
5. **Watch real-time updates** in the browser

### Test Scenarios

The test script includes various scenarios:
- 📦 New order notifications
- 💰 Payment success notifications
- 💬 Chat messages
- ⚠️ System alerts
- 👤 User activity
- 📦 Inventory updates
- 📢 Marketing campaigns

## File Structure

```
websocket-golang/
├── main.go              # Main server application
├── main_test.go         # Go tests
├── index.html           # Web interface
├── test_notifications.sh # Test script
├── websocket_test.js    # WebSocket test client
├── go.mod              # Go dependencies
├── go.sum              # Go dependencies checksum
└── README.md           # This file
```

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DB_HOST` | PostgreSQL host | - |
| `DB_PORT` | PostgreSQL port | - |
| `DB_USER` | PostgreSQL user | - |
| `DB_PASSWORD` | PostgreSQL password | - |
| `DB_NAME` | PostgreSQL database name | - |
| `DB_SSLMODE` | PostgreSQL SSL mode | - |

### Authentication

Default credentials:
- Key: `key`
- Secret: `secret`

## WebSocket Protocol

### Subscription Message
```json
{
  "channel": "channel_name"
}
```

### Notification Message
```json
{
  "channel": "channel_name",
  "event": "event_type",
  "data": {
    "field1": "value1",
    "field2": "value2"
  }
}
```

## Search Operators

| Operator | SQL Equivalent | Description |
|----------|----------------|-------------|
| `==` | `=` | Equal |
| `!=` | `!=` | Not equal |
| `>` | `>` | Greater than |
| `<` | `<` | Less than |
| `>=` | `>=` | Greater than or equal |
| `<=` | `<=` | Less than or equal |
| `like` | `LIKE` | Pattern matching |
| `ilike` | `ILIKE` | Case-insensitive pattern matching |

## Error Handling

The system handles various error scenarios:
- Invalid authentication (401 Unauthorized)
- Invalid JSON (400 Bad Request)
- Missing required fields (400 Bad Request)
- Database connection issues (graceful fallback)
- WebSocket connection errors

## Performance

- Supports multiple concurrent WebSocket connections
- Efficient message broadcasting
- Database operations are optional
- Memory-efficient message storage

## Browser Compatibility

The web interface works with modern browsers that support:
- WebSocket API
- ES6+ JavaScript
- CSS Grid and Flexbox

## Troubleshooting

### Common Issues

1. **Server won't start**
   - Check if port 3000 is available
   - Verify Go is installed: `go version`

2. **WebSocket connection fails**
   - Ensure server is running
   - Check browser console for errors
   - Verify WebSocket URL format

3. **Database connection fails**
   - Check environment variables
   - Verify PostgreSQL is running
   - Test connection manually

4. **Notifications not appearing**
   - Check WebSocket connection status
   - Verify channel subscription
   - Check browser console for errors

### Debug Mode

Enable debug logging by setting the environment variable:
```bash
export GIN_MODE=debug
go run main.go
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## License

This project is open source and available under the MIT License.
