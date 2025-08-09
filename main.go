package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
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
		"host=" + dbHost +
		" port=" + dbPort +
		" user=" + dbUser +
		" password=" + dbPassword +
		" dbname=" + dbName +
		" sslmode=" + dbSSLMode)
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
			columns = append(columns, `"` + field + `" TEXT`)
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
			fields = append(fields, `"` + field + `"`)
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

	conn.WriteJSON(gin.H{"message": "Subscribed to channel", "channel": subscription.Channel})

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			msgLock.Lock()
			delete(clients, conn)
			msgLock.Unlock()
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
		conditions = append(conditions, `"` + f.Field + `" ` + op + ` $` + strconv.Itoa(argIdx))
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
	for client, channel := range clients {
		if channel == notif.Channel {
			client.WriteJSON(notif)
		}
	}
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
			conditions = append(conditions, `"` + key + `" = $` + strconv.Itoa(argIdx))
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

	r.Run(":3000")
}
