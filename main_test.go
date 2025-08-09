package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// Test setup
func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.Default()
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	})
	r.POST("/notification", authenticate, sendNotification)
	r.GET("/ws", handleWebSocket)
	r.GET("/search", authenticate, searchHandler)
	r.GET("/notifications", authenticate, getNotifications)
	return r
}

// Test authentication
func TestAuthenticate(t *testing.T) {
	r := setupTestRouter()

	// Test valid credentials
	req, _ := http.NewRequest("POST", "/notification", nil)
	req.Header.Set("key", "key")
	req.Header.Set("secret", "secret")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	
	// Should not return 401 (Unauthorized)
	assert.NotEqual(t, http.StatusUnauthorized, w.Code)

	// Test invalid credentials
	req, _ = http.NewRequest("POST", "/notification", nil)
	req.Header.Set("key", "wrong")
	req.Header.Set("secret", "wrong")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// Test notification sending
func TestSendNotification(t *testing.T) {
	r := setupTestRouter()

	// Test valid notification
	notification := Notification{
		Channel: "test_channel",
		Event:   "test_event",
		Data: map[string]interface{}{
			"message": "test message",
			"sender":  "test user",
		},
	}

	jsonData, _ := json.Marshal(notification)
	req, _ := http.NewRequest("POST", "/notification", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("key", "key")
	req.Header.Set("secret", "secret")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, true, response["success"])
}

// Test invalid notification
func TestSendInvalidNotification(t *testing.T) {
	r := setupTestRouter()

	// Test invalid JSON
	req, _ := http.NewRequest("POST", "/notification", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("key", "key")
	req.Header.Set("secret", "secret")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// Test search handler
func TestSearchHandler(t *testing.T) {
	r := setupTestRouter()

	// Test search request
	searchReq := SearchRequest{
		Channel: "test_channel",
		Filters: []struct {
			Field string      `json:"field"`
			Op    string      `json:"op"`
			Value interface{} `json:"value"`
		}{
			{Field: "event", Op: "==", Value: "test_event"},
		},
	}

	jsonData, _ := json.Marshal(searchReq)
	req, _ := http.NewRequest("GET", "/search", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("key", "key")
	req.Header.Set("secret", "secret")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should return either OK or ServiceUnavailable depending on DB availability
	assert.Contains(t, []int{http.StatusOK, http.StatusServiceUnavailable}, w.Code)
}

// Test get notifications
func TestGetNotifications(t *testing.T) {
	r := setupTestRouter()

	req, _ := http.NewRequest("GET", "/notifications?channel=test_channel", nil)
	req.Header.Set("key", "key")
	req.Header.Set("secret", "secret")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should return either OK or ServiceUnavailable depending on DB availability
	assert.Contains(t, []int{http.StatusOK, http.StatusServiceUnavailable}, w.Code)
}

// Test get notifications without channel
func TestGetNotificationsWithoutChannel(t *testing.T) {
	r := setupTestRouter()

	req, _ := http.NewRequest("GET", "/notifications", nil)
	req.Header.Set("key", "key")
	req.Header.Set("secret", "secret")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// Test broadcast notification
func TestBroadcastNotification(t *testing.T) {
	notification := Notification{
		Channel: "test_channel",
		Event:   "test_event",
		Data: map[string]interface{}{
			"message": "test message",
		},
	}

	// Test broadcast with no clients
	broadcastNotification(notification)
	// Should not panic or error
}

// Test allowed operators
func TestAllowedOperators(t *testing.T) {
	expectedOperators := []string{"==", "!=", ">", "<", ">=", "<=", "like", "ilike"}
	
	for _, op := range expectedOperators {
		_, exists := allowedOperators[op]
		assert.True(t, exists, "Operator %s should be allowed", op)
	}
}

// Test notification structure
func TestNotificationStructure(t *testing.T) {
	notification := Notification{
		Channel: "test_channel",
		Event:   "test_event",
		Data: map[string]interface{}{
			"message": "test message",
			"sender":  "test user",
			"amount":  100,
		},
	}

	// Test JSON marshaling
	jsonData, err := json.Marshal(notification)
	assert.NoError(t, err)

	// Test JSON unmarshaling
	var decoded Notification
	err = json.Unmarshal(jsonData, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, notification.Channel, decoded.Channel)
	assert.Equal(t, notification.Event, decoded.Event)
	assert.Equal(t, notification.Data, decoded.Data)
}

// Test search request structure
func TestSearchRequestStructure(t *testing.T) {
	searchReq := SearchRequest{
		Channel: "test_channel",
		Filters: []struct {
			Field string      `json:"field"`
			Op    string      `json:"op"`
			Value interface{} `json:"value"`
		}{
			{Field: "event", Op: "==", Value: "test_event"},
			{Field: "sender", Op: "like", Value: "test%"},
		},
	}

	// Test JSON marshaling
	jsonData, err := json.Marshal(searchReq)
	assert.NoError(t, err)

	// Test JSON unmarshaling
	var decoded SearchRequest
	err = json.Unmarshal(jsonData, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, searchReq.Channel, decoded.Channel)
	assert.Equal(t, len(searchReq.Filters), len(decoded.Filters))
}

// Benchmark tests
func BenchmarkSendNotification(b *testing.B) {
	r := setupTestRouter()
	notification := Notification{
		Channel: "benchmark_channel",
		Event:   "benchmark_event",
		Data: map[string]interface{}{
			"message": "benchmark message",
		},
	}

	jsonData, _ := json.Marshal(notification)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("POST", "/notification", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("key", "key")
		req.Header.Set("secret", "secret")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

// Test concurrent notifications
func TestConcurrentNotifications(t *testing.T) {
	r := setupTestRouter()
	
	// Send multiple notifications concurrently
	done := make(chan bool, 10)
	
	for i := 0; i < 10; i++ {
		go func(id int) {
			notification := Notification{
				Channel: "concurrent_channel",
				Event:   "concurrent_event",
				Data: map[string]interface{}{
					"id":      id,
					"message": "concurrent message",
				},
			}

			jsonData, _ := json.Marshal(notification)
			req, _ := http.NewRequest("POST", "/notification", bytes.NewBuffer(jsonData))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("key", "key")
			req.Header.Set("secret", "secret")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		select {
		case <-done:
			// Success
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for concurrent notifications")
		}
	}
}
