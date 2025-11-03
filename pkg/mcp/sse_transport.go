package mcp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

// SSEConnection represents a Server-Sent Events connection
type SSEConnection struct {
	ID         string
	writer     http.ResponseWriter
	flusher    http.Flusher
	done       chan struct{}
	mu         sync.Mutex
	lastActive time.Time
	logger     *zap.Logger
}

// SSEBroadcaster manages SSE connections for push notifications
type SSEBroadcaster struct {
	connections map[string]*SSEConnection
	mu          sync.RWMutex
	logger      *zap.Logger
}

// NewSSEBroadcaster creates a new SSE broadcaster
func NewSSEBroadcaster(logger *zap.Logger) *SSEBroadcaster {
	broadcaster := &SSEBroadcaster{
		connections: make(map[string]*SSEConnection),
		logger:      logger,
	}

	// Start cleanup goroutine for stale connections
	go broadcaster.cleanupStaleConnections()

	return broadcaster
}

// AddConnection adds a new SSE connection
func (sb *SSEBroadcaster) AddConnection(id string, w http.ResponseWriter, r *http.Request) (*SSEConnection, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming not supported")
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*") // Adjust for production

	conn := &SSEConnection{
		ID:         id,
		writer:     w,
		flusher:    flusher,
		done:       make(chan struct{}),
		lastActive: time.Now(),
		logger:     sb.logger,
	}

	sb.mu.Lock()
	sb.connections[id] = conn
	sb.mu.Unlock()

	sb.logger.Info("SSE connection established",
		zap.String("connection_id", id),
		zap.Int("total_connections", len(sb.connections)))

	// Send initial connection event
	conn.SendEvent("connected", map[string]interface{}{
		"connection_id": id,
		"timestamp":     time.Now().Format(time.RFC3339),
	})

	return conn, nil
}

// RemoveConnection removes an SSE connection
func (sb *SSEBroadcaster) RemoveConnection(id string) {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	if conn, exists := sb.connections[id]; exists {
		close(conn.done)
		delete(sb.connections, id)
		sb.logger.Info("SSE connection closed",
			zap.String("connection_id", id),
			zap.Int("remaining_connections", len(sb.connections)))
	}
}

// Broadcast sends a notification to all connected clients
func (sb *SSEBroadcaster) Broadcast(event string, data map[string]interface{}) {
	sb.mu.RLock()
	defer sb.mu.RUnlock()

	if len(sb.connections) == 0 {
		sb.logger.Debug("no SSE connections to broadcast to", zap.String("event", event))
		return
	}

	sb.logger.Debug("broadcasting SSE event",
		zap.String("event", event),
		zap.Int("connections", len(sb.connections)))

	var wg sync.WaitGroup
	for _, conn := range sb.connections {
		wg.Add(1)
		go func(c *SSEConnection) {
			defer wg.Done()
			c.SendEvent(event, data)
		}(conn)
	}
	wg.Wait()
}

// SendEvent sends an event to this specific connection
func (c *SSEConnection) SendEvent(event string, data map[string]interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case <-c.done:
		return fmt.Errorf("connection closed")
	default:
	}

	// Marshal data to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		c.logger.Error("failed to marshal SSE data", zap.Error(err))
		return err
	}

	// Send event in SSE format
	_, err = fmt.Fprintf(c.writer, "event: %s\ndata: %s\n\n", event, jsonData)
	if err != nil {
		c.logger.Error("failed to write SSE event", zap.Error(err))
		return err
	}

	c.flusher.Flush()
	c.lastActive = time.Now()

	return nil
}

// cleanupStaleConnections removes connections that haven't been active
func (sb *SSEBroadcaster) cleanupStaleConnections() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		sb.mu.Lock()
		now := time.Now()
		var toRemove []string

		for id, conn := range sb.connections {
			if now.Sub(conn.lastActive) > 5*time.Minute {
				toRemove = append(toRemove, id)
			}
		}

		for _, id := range toRemove {
			if conn, exists := sb.connections[id]; exists {
				close(conn.done)
				delete(sb.connections, id)
				sb.logger.Info("removed stale SSE connection",
					zap.String("connection_id", id))
			}
		}
		sb.mu.Unlock()
	}
}

// ConnectionCount returns the number of active connections
func (sb *SSEBroadcaster) ConnectionCount() int {
	sb.mu.RLock()
	defer sb.mu.RUnlock()
	return len(sb.connections)
}

// HandleSSE creates an HTTP handler for SSE connections
func (sb *SSEBroadcaster) HandleSSE() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Generate connection ID
		connID := fmt.Sprintf("sse-%d", time.Now().UnixNano())

		conn, err := sb.AddConnection(connID, w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Send heartbeat to keep connection alive
		heartbeat := time.NewTicker(15 * time.Second)
		defer heartbeat.Stop()
		defer sb.RemoveConnection(connID)

		// Wait for context cancellation or connection close
		for {
			select {
			case <-r.Context().Done():
				return
			case <-conn.done:
				return
			case <-heartbeat.C:
				if err := conn.SendEvent("heartbeat", map[string]interface{}{
					"timestamp": time.Now().Format(time.RFC3339),
				}); err != nil {
					return
				}
			}
		}
	}
}
