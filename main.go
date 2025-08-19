package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

var (
	validCredentials = map[string]string{
		"key":    "key",
		"secret": "secret",
	}
	dbConn *sql.DB
	useDB  bool // Flag to indicate if database is available

	clients = make(map[*websocket.Conn]string)
	msgLock sync.Mutex

	// Monitoring metrics
	metrics = &Metrics{
		WebSocketStats: &WebSocketStats{
			TotalConnections:    0,
			ActiveConnections:   0,
			TotalMessagesSent:   0,
			TotalMessagesFailed: 0,
			MessagesByChannel:   make(map[string]int),
		},
		ServerStats: &ServerStats{
			StartTime: time.Now(),
		},
	}
	metricsLock sync.RWMutex

	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
)

type Notification struct {
	Channel string                 `json:"channel"`
	Event   string                 `json:"event"`
	Data    map[string]interface{} `json:"data"` // dynamic fields like sender, message
}

type WebSocketStats struct {
	TotalConnections    int            `json:"totalConnections"`
	ActiveConnections   int            `json:"activeConnections"`
	TotalMessagesSent   int            `json:"totalMessagesSent"`
	TotalMessagesFailed int            `json:"totalMessagesFailed"`
	MessagesByChannel   map[string]int `json:"messagesByChannel"`
	LastMessageTime     time.Time      `json:"lastMessageTime"`
}

type ServerStats struct {
	StartTime   time.Time `json:"startTime"`
	Uptime      string    `json:"uptime"`
	MemoryUsage string    `json:"memoryUsage"`
	CPUUsage    string    `json:"cpuUsage"`
	Goroutines  int       `json:"goroutines"`
}

type Metrics struct {
	WebSocketStats *WebSocketStats `json:"websocketStats"`
	ServerStats    *ServerStats    `json:"serverStats"`
}

// ------------------ DB Setup ------------------

func initDB() {
	// Check if database environment variables are set
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")
	dbSSLMode := os.Getenv("DB_SSLMODE")

	// If any required database variable is missing, skip database initialization
	if dbHost == "" || dbPort == "" || dbUser == "" || dbPassword == "" || dbName == "" {
		log.Println("Database environment variables not found. Running without database functionality.")
		useDB = false
		return
	}

	var err error
	dbConn, err = sql.Open("postgres",
		"host="+dbHost+
			" port="+dbPort+
			" user="+dbUser+
			" password="+dbPassword+
			" dbname="+dbName+
			" sslmode="+dbSSLMode)
	if err != nil {
		log.Println("DB connect error:", err)
		useDB = false
		return
	}

	if err := dbConn.Ping(); err != nil {
		log.Println("DB ping failed:", err)
		useDB = false
		return
	}

	useDB = true
	log.Println("Database connected successfully.")
}

// Create table if not exists
func ensureTable(channel string, data map[string]interface{}) error {
	if !useDB {
		return nil
	}

	// Base columns
	columns := []string{
		"id SERIAL PRIMARY KEY",
		"created_at TIMESTAMP DEFAULT NOW()",
		`"event" TEXT`,
	}

	// Add dynamic columns based on data
	for field := range data {
		if field != "id" && field != "created_at" && field != "event" {
			columns = append(columns, `"`+field+`" TEXT`)
		}
	}

	// Create table if not exists
	query := `CREATE TABLE IF NOT EXISTS "` + channel + `" (` + strings.Join(columns, ", ") + `);`
	_, err := dbConn.Exec(query)
	if err != nil {
		return err
	}

	// Check if event column exists
	var eventExists bool
	err = dbConn.QueryRow(`
		SELECT EXISTS (
			SELECT 1 
			FROM information_schema.columns 
			WHERE table_name = $1 AND column_name = 'event'
		)`, channel).Scan(&eventExists)

	if err != nil {
		return err
	}

	// Add event column if it doesn't exist
	if !eventExists {
		alterQuery := `ALTER TABLE "` + channel + `" ADD COLUMN "event" TEXT;`
		_, err := dbConn.Exec(alterQuery)
		if err != nil {
			return err
		}
	}

	// Check and add any missing columns
	for field := range data {
		if field != "id" && field != "created_at" && field != "event" {
			// Check if column exists
			var exists bool
			err := dbConn.QueryRow(`
				SELECT EXISTS (
					SELECT 1 
					FROM information_schema.columns 
					WHERE table_name = $1 AND column_name = $2
				)`, channel, field).Scan(&exists)

			if err != nil {
				return err
			}

			// If column doesn't exist, add it
			if !exists {
				alterQuery := `ALTER TABLE "` + channel + `" ADD COLUMN "` + field + `" TEXT;`
				_, err := dbConn.Exec(alterQuery)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// Save notif to table
func saveToDB(channel string, data map[string]interface{}, event string) error {
	if !useDB {
		return nil
	}

	if err := ensureTable(channel, data); err != nil {
		return err
	}

	// Build dynamic query
	fields := []string{`"event"`}
	placeholders := []string{"$1"}
	values := []interface{}{event}
	valueIndex := 2

	for field, value := range data {
		if field != "id" && field != "created_at" && field != "event" {
			fields = append(fields, `"`+field+`"`)
			placeholders = append(placeholders, "$"+strconv.Itoa(valueIndex))
			values = append(values, value)
			valueIndex++
		}
	}

	// Add created_at
	fields = append(fields, "created_at")
	placeholders = append(placeholders, "$"+strconv.Itoa(valueIndex))
	values = append(values, time.Now())

	stmt := `INSERT INTO "` + channel + `" (` + strings.Join(fields, ", ") + `) VALUES (` + strings.Join(placeholders, ", ") + `)`
	_, err := dbConn.Exec(stmt, values...)
	return err
}

// ------------------ WebSocket ------------------

func handleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	var subscription struct {
		Channel string `json:"channel"`
	}
	if err := conn.ReadJSON(&subscription); err != nil {
		conn.WriteJSON(gin.H{"error": "Invalid subscription request"})
		return
	}

	msgLock.Lock()
	clients[conn] = subscription.Channel
	msgLock.Unlock()

	// Update metrics
	metricsLock.Lock()
	metrics.WebSocketStats.TotalConnections++
	metrics.WebSocketStats.ActiveConnections = len(clients)
	metricsLock.Unlock()

	conn.WriteJSON(gin.H{"message": "Subscribed to channel", "channel": subscription.Channel})

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			msgLock.Lock()
			delete(clients, conn)
			msgLock.Unlock()

			// Update metrics
			metricsLock.Lock()
			metrics.WebSocketStats.ActiveConnections = len(clients)
			metricsLock.Unlock()
			break
		}
	}
}

// ------------------ Notifikasi Handler ------------------

func authenticate(c *gin.Context) {
	key := c.GetHeader("key")
	secret := c.GetHeader("secret")

	if key != validCredentials["key"] || secret != validCredentials["secret"] {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		c.Abort()
		return
	}
	c.Next()
}

func sendNotification(c *gin.Context) {
	var notif Notification
	if err := c.ShouldBindJSON(&notif); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	// Simpan ke DB (jika database tersedia)
	if useDB {
		if err := saveToDB(notif.Channel, notif.Data, notif.Event); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save to DB", "detail": err.Error()})
			return
		}
	}

	// Broadcast ke client
	broadcastNotification(notif)

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Notification sent"})
}

func searchHandler(c *gin.Context) {
	if !useDB {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Database not available"})
		return
	}

	var req SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	// Validasi nama tabel
	if req.Channel == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Channel required"})
		return
	}

	// Build query
	baseQuery := `SELECT * FROM "` + req.Channel + `"`
	conditions := []string{}
	args := []interface{}{}
	argIdx := 1

	for _, f := range req.Filters {
		op, ok := allowedOperators[f.Op]
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid operator: " + f.Op})
			return
		}
		conditions = append(conditions, `"`+f.Field+`" `+op+` $`+strconv.Itoa(argIdx))
		args = append(args, f.Value)
		argIdx++
	}

	query := baseQuery
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY id DESC LIMIT 100"

	// Eksekusi query
	rows, err := dbConn.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Query error", "detail": err.Error()})
		return
	}
	defer rows.Close()

	// Ambil hasil sebagai map[string]interface{}
	cols, _ := rows.Columns()
	result := []map[string]interface{}{}

	for rows.Next() {
		// prepare holder
		columns := make([]interface{}, len(cols))
		columnPointers := make([]interface{}, len(cols))
		for i := range columns {
			columnPointers[i] = &columns[i]
		}

		if err := rows.Scan(columnPointers...); err != nil {
			continue
		}

		rowMap := make(map[string]interface{})
		for i, colName := range cols {
			val := columnPointers[i].(*interface{})
			rowMap[colName] = *val
		}
		result = append(result, rowMap)
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

type SearchRequest struct {
	Channel string `json:"channel"`
	Filters []struct {
		Field string      `json:"field"`
		Op    string      `json:"op"`
		Value interface{} `json:"value"`
	} `json:"filters"`
}

var allowedOperators = map[string]string{
	"==":    "=",
	"!=":    "!=",
	">":     ">",
	"<":     "<",
	">=":    ">=",
	"<=":    "<=",
	"like":  "LIKE",
	"ilike": "ILIKE",
}

func broadcastNotification(notif Notification) {
	msgLock.Lock()
	defer msgLock.Unlock()

	successCount := 0
	failedCount := 0

	for client, channel := range clients {
		if channel == notif.Channel {
			if err := client.WriteJSON(notif); err != nil {
				failedCount++
			} else {
				successCount++
			}
		}
	}

	// Update metrics
	metricsLock.Lock()
	metrics.WebSocketStats.TotalMessagesSent += successCount
	metrics.WebSocketStats.TotalMessagesFailed += failedCount
	metrics.WebSocketStats.MessagesByChannel[notif.Channel] += successCount
	metrics.WebSocketStats.LastMessageTime = time.Now()
	metricsLock.Unlock()
}

func getNotifications(c *gin.Context) {
	if !useDB {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Database not available"})
		return
	}

	channel := c.Query("channel")
	if channel == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Channel is required"})
		return
	}

	// Build query
	baseQuery := `SELECT * FROM "` + channel + `"`
	conditions := []string{}
	args := []interface{}{}
	argIdx := 1

	// Add filters from query parameters
	for key, value := range c.Request.URL.Query() {
		if key != "channel" && len(value) > 0 {
			conditions = append(conditions, `"`+key+`" = $`+strconv.Itoa(argIdx))
			args = append(args, value[0])
			argIdx++
		}
	}

	query := baseQuery
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY id DESC LIMIT 100"

	// Execute query
	rows, err := dbConn.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Query error", "detail": err.Error()})
		return
	}
	defer rows.Close()

	// Get results as map[string]interface{}
	cols, _ := rows.Columns()
	result := []map[string]interface{}{}

	for rows.Next() {
		// prepare holder
		columns := make([]interface{}, len(cols))
		columnPointers := make([]interface{}, len(cols))
		for i := range columns {
			columnPointers[i] = &columns[i]
		}

		if err := rows.Scan(columnPointers...); err != nil {
			continue
		}

		rowMap := make(map[string]interface{})
		for i, colName := range cols {
			val := columnPointers[i].(*interface{})
			rowMap[colName] = *val
		}
		result = append(result, rowMap)
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// ------------------ Monitoring Functions ------------------

func updateServerStats() {
	metricsLock.Lock()
	defer metricsLock.Unlock()

	// Update uptime
	uptime := time.Since(metrics.ServerStats.StartTime)
	metrics.ServerStats.Uptime = formatDuration(uptime)

	// Update memory usage
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	metrics.ServerStats.MemoryUsage = formatBytes(m.Alloc)

	// Update goroutines count
	metrics.ServerStats.Goroutines = runtime.NumGoroutine()

	// Simple CPU usage estimation (this is a basic implementation)
	// In a real application, you might want to use more sophisticated CPU monitoring
	metrics.ServerStats.CPUUsage = "N/A" // Placeholder for now
}

func formatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm %ds", days, hours, minutes, seconds)
	} else if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func getMetrics() *Metrics {
	updateServerStats()
	metricsLock.RLock()
	defer metricsLock.RUnlock()

	// Create a copy to avoid race conditions
	wsStats := *metrics.WebSocketStats
	serverStats := *metrics.ServerStats

	return &Metrics{
		WebSocketStats: &wsStats,
		ServerStats:    &serverStats,
	}
}

func monitorHandler(c *gin.Context) {
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>WebSocket Monitor</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); min-height: 100vh; padding: 20px; }
        .container { max-width: 1200px; margin: 0 auto; }
        .header { text-align: center; color: white; margin-bottom: 30px; }
        .header h1 { font-size: 2.5rem; margin-bottom: 10px; text-shadow: 2px 2px 4px rgba(0,0,0,0.3); }
        .header p { font-size: 1.1rem; opacity: 0.9; }
        .stats-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 20px; margin-bottom: 30px; }
        .stat-card { background: rgba(255, 255, 255, 0.95); border-radius: 15px; padding: 25px; box-shadow: 0 8px 32px rgba(0,0,0,0.1); backdrop-filter: blur(10px); border: 1px solid rgba(255, 255, 255, 0.2); transition: transform 0.3s ease, box-shadow 0.3s ease; }
        .stat-card:hover { transform: translateY(-5px); box-shadow: 0 12px 40px rgba(0,0,0,0.15); }
        .stat-card h3 { color: #333; font-size: 1.2rem; margin-bottom: 15px; display: flex; align-items: center; gap: 10px; }
        .stat-value { font-size: 2.5rem; font-weight: bold; color: #667eea; margin-bottom: 10px; }
        .stat-label { color: #666; font-size: 0.9rem; text-transform: uppercase; letter-spacing: 1px; }
        .progress-bar { width: 100%; height: 8px; background: #e0e0e0; border-radius: 4px; overflow: hidden; margin-top: 10px; }
        .progress-fill { height: 100%; background: linear-gradient(90deg, #4CAF50, #45a049); border-radius: 4px; transition: width 0.3s ease; }
        .channel-stats { background: rgba(255, 255, 255, 0.95); border-radius: 15px; padding: 25px; box-shadow: 0 8px 32px rgba(0,0,0,0.1); backdrop-filter: blur(10px); border: 1px solid rgba(255, 255, 255, 0.2); }
        .channel-stats h3 { color: #333; font-size: 1.2rem; margin-bottom: 20px; }
        .channel-item { display: flex; justify-content: space-between; align-items: center; padding: 10px 0; border-bottom: 1px solid #eee; }
        .channel-item:last-child { border-bottom: none; }
        .channel-name { font-weight: 500; color: #333; }
        .channel-count { background: #667eea; color: white; padding: 4px 12px; border-radius: 20px; font-size: 0.9rem; font-weight: 500; }
        .status-indicator { display: inline-block; width: 12px; height: 12px; border-radius: 50%; margin-right: 8px; }
        .status-online { background: #4CAF50; box-shadow: 0 0 10px rgba(76, 175, 80, 0.5); }
        .status-offline { background: #f44336; }
        .refresh-btn { position: fixed; bottom: 30px; right: 30px; background: #667eea; color: white; border: none; padding: 15px 20px; border-radius: 50px; cursor: pointer; font-size: 1rem; box-shadow: 0 4px 15px rgba(0,0,0,0.2); transition: all 0.3s ease; }
        .refresh-btn:hover { background: #5a6fd8; transform: translateY(-2px); box-shadow: 0 6px 20px rgba(0,0,0,0.3); }
        .last-update { text-align: center; color: white; margin-top: 20px; opacity: 0.8; }
        @media (max-width: 768px) { .stats-grid { grid-template-columns: 1fr; } .header h1 { font-size: 2rem; } .stat-value { font-size: 2rem; } }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🚀 WebSocket Monitor</h1>
            <p>Real-time monitoring dashboard for WebSocket connections and server metrics</p>
        </div>
        
        <div class="stats-grid">
            <div class="stat-card">
                <h3><span class="status-indicator status-online"></span>Active Connections</h3>
                <div class="stat-value" id="activeConnections">0</div>
                <div class="stat-label">Currently Connected</div>
            </div>
            
            <div class="stat-card">
                <h3>📊 Total Connections</h3>
                <div class="stat-value" id="totalConnections">0</div>
                <div class="stat-label">Since Server Start</div>
            </div>
            
            <div class="stat-card">
                <h3>✅ Success Rate</h3>
                <div class="stat-value" id="successRate">0%</div>
                <div class="stat-label">Message Delivery</div>
                <div class="progress-bar">
                    <div class="progress-fill" id="successProgress" style="width: 0%"></div>
                </div>
            </div>
            
            <div class="stat-card">
                <h3>📨 Messages Sent</h3>
                <div class="stat-value" id="messagesSent">0</div>
                <div class="stat-label">Total Successful</div>
            </div>
            
            <div class="stat-card">
                <h3>❌ Messages Failed</h3>
                <div class="stat-value" id="messagesFailed">0</div>
                <div class="stat-label">Delivery Failures</div>
            </div>
            
            <div class="stat-card">
                <h3>⏱️ Server Uptime</h3>
                <div class="stat-value" id="uptime">0s</div>
                <div class="stat-label">Running Time</div>
            </div>
            
            <div class="stat-card">
                <h3>💾 Memory Usage</h3>
                <div class="stat-value" id="memoryUsage">0 B</div>
                <div class="stat-label">Current Allocation</div>
            </div>
            
            <div class="stat-card">
                <h3>🔄 Goroutines</h3>
                <div class="stat-value" id="goroutines">0</div>
                <div class="stat-label">Active Threads</div>
            </div>
        </div>
        
        <div class="channel-stats">
            <h3>📺 Channel Statistics</h3>
            <div id="channelStats">
                <div class="channel-item">
                    <span class="channel-name">No channels active</span>
                    <span class="channel-count">0</span>
                </div>
            </div>
        </div>
        
        <div class="last-update" id="lastUpdate">
            Last updated: Never
        </div>
    </div>
    
    <button class="refresh-btn" onclick="refreshData()">
        🔄 Refresh
    </button>
    
    <script>
        function formatNumber(num) {
            return num.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ",");
        }
        
        function updateMetrics() {
            fetch('/api/metrics')
                .then(response => response.json())
                .then(data => {
                    document.getElementById('activeConnections').textContent = formatNumber(data.websocketStats.activeConnections);
                    document.getElementById('totalConnections').textContent = formatNumber(data.websocketStats.totalConnections);
                    document.getElementById('messagesSent').textContent = formatNumber(data.websocketStats.totalMessagesSent);
                    document.getElementById('messagesFailed').textContent = formatNumber(data.websocketStats.totalMessagesFailed);
                    
                    const total = data.websocketStats.totalMessagesSent + data.websocketStats.totalMessagesFailed;
                    const successRate = total > 0 ? Math.round((data.websocketStats.totalMessagesSent / total) * 100) : 0;
                    document.getElementById('successRate').textContent = successRate + '%';
                    document.getElementById('successProgress').style.width = successRate + '%';
                    
                    document.getElementById('uptime').textContent = data.serverStats.uptime;
                    document.getElementById('memoryUsage').textContent = data.serverStats.memoryUsage;
                    document.getElementById('goroutines').textContent = formatNumber(data.serverStats.goroutines);
                    
                    const channelStats = document.getElementById('channelStats');
                    const channels = data.websocketStats.messagesByChannel;
                    
                    if (Object.keys(channels).length === 0) {
                        channelStats.innerHTML = '<div class="channel-item"><span class="channel-name">No channels active</span><span class="channel-count">0</span></div>';
                    } else {
                        channelStats.innerHTML = '';
                        Object.entries(channels).forEach(([channel, count]) => {
                            const channelItem = document.createElement('div');
                            channelItem.className = 'channel-item';
                            channelItem.innerHTML = '<span class="channel-name">' + channel + '</span><span class="channel-count">' + formatNumber(count) + '</span>';
                            channelStats.appendChild(channelItem);
                        });
                    }
                    
                    const now = new Date();
                    document.getElementById('lastUpdate').textContent = 'Last updated: ' + now.toLocaleTimeString();
                })
                .catch(error => {
                    console.error('Error fetching metrics:', error);
                });
        }
        
        function refreshData() {
            updateMetrics();
        }
        
        setInterval(updateMetrics, 2000);
        updateMetrics();
    </script>
</body>
</html>`

	c.Header("Content-Type", "text/html")
	c.String(http.StatusOK, html)
}

func metricsAPIHandler(c *gin.Context) {
	metrics := getMetrics()
	c.JSON(http.StatusOK, metrics)
}

// ------------------ MAIN ------------------

func main() {
	godotenv.Load()
	initDB()

	r := gin.Default()
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	})
	r.POST("/notification", authenticate, sendNotification)
	r.GET("/ws", handleWebSocket)
	r.GET("/search", authenticate, searchHandler)
	r.GET("/notifications", authenticate, getNotifications)
	r.GET("/monitor", monitorHandler)
	r.GET("/api/metrics", metricsAPIHandler)

	r.Run(":3000")
}
