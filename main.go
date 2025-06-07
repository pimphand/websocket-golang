package main

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	_ "github.com/lib/pq"
)

var (
	validCredentials = map[string]string{
		"key":    "key",
		"secret": "secret",
	}
	dbConn *sql.DB

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
	var err error
	dbConn, err = sql.Open("postgres", "host=localhost port=5432 user=sammy password='password' dbname=notifikasi sslmode=disable")
	if err != nil {
		log.Fatal("DB connect error:", err)
	}

	if err := dbConn.Ping(); err != nil {
		log.Fatal("DB ping failed:", err)
	}
}

// Create table if not exists
func ensureTable(channel string) error {
	query := `
	CREATE TABLE IF NOT EXISTS ` + channel + ` (
		id SERIAL PRIMARY KEY,
		sender TEXT,
		message TEXT,
		created_at TIMESTAMP DEFAULT NOW()
	);`
	_, err := dbConn.Exec(query)
	return err
}

// Save notif to table
func saveToDB(channel string, data map[string]interface{}) error {
	if err := ensureTable(channel); err != nil {
		return err
	}

	stmt := `INSERT INTO ` + channel + ` (sender, message, created_at) VALUES ($1, $2, $3)`
	_, err := dbConn.Exec(stmt, data["sender"], data["message"], time.Now())
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

	// Simpan ke DB
	if err := saveToDB(notif.Channel, notif.Data); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save to DB", "detail": err.Error()})
		return
	}

	// Broadcast ke client
	broadcastNotification(notif)

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Notification sent"})
}

func searchHandler(c *gin.Context) {
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
	baseQuery := "SELECT * FROM " + req.Channel
	conditions := []string{}
	args := []interface{}{}
	argIdx := 1

	for _, f := range req.Filters {
		op, ok := allowedOperators[f.Op]
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid operator: " + f.Op})
			return
		}
		conditions = append(conditions, f.Field+" "+op+" $"+strconv.Itoa(argIdx))
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

// ------------------ MAIN ------------------

func main() {
	initDB()

	r := gin.Default()
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	})
	r.POST("/notification", authenticate, sendNotification)
	r.GET("/ws", handleWebSocket)
	r.POST("/search", authenticate, searchHandler)

	log.Println("Server started on :3003")
	r.Run(":3003")
}
