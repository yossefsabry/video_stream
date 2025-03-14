package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	_"os"
	"time"

	"github.com/gorilla/websocket"
)

// Track connected clients
type StreamServer struct {
	clients map[*websocket.Conn]bool
	addCh   chan *websocket.Conn
	rmCh    chan *websocket.Conn
}

// WebSocket upgrader
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func main() {
	server := &StreamServer{
		clients: make(map[*websocket.Conn]bool),
		addCh:   make(chan *websocket.Conn),
		rmCh:    make(chan *websocket.Conn),
	}

	// Handle WebSocket connections
	http.HandleFunc("/ws", server.handleConnections)

	// Handle file uploads
	http.HandleFunc("/upload", server.handleUpload)

	// Serve the frontend
	http.Handle("/", http.FileServer(http.Dir("./static")))

	fmt.Println("Server started on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// Handle new WebSocket connections
func (s *StreamServer) handleConnections(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Failed to upgrade WebSocket:", err)
		return
	}
	s.addCh <- conn
	defer func() { s.rmCh <- conn }()

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}

// Handle video file upload
func (s *StreamServer) handleUpload(w http.ResponseWriter, r *http.Request) {
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Stream video data to clients
	buffer := make([]byte, 1024*512) // 512 KB chunks
	for {
		n, err := file.Read(buffer)
		if err != nil {
			if err != io.EOF {
				http.Error(w, "Failed to read file", http.StatusInternalServerError)
			}
			break
		}
		s.broadcast(buffer[:n])
		time.Sleep(50 * time.Millisecond) // Simulate network delay
	}
	fmt.Fprintln(w, "Video streaming to clients...")
}

// Broadcast video data to all connected clients
func (s *StreamServer) broadcast(data []byte) {
	for conn := range s.clients {
		if err := conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
			s.rmCh <- conn
		}
	}
}

// Manage client lifecycle
func (s *StreamServer) run() {
	for {
		select {
		case conn := <-s.addCh:
			s.clients[conn] = true
			fmt.Println("New client connected!")
		case conn := <-s.rmCh:
			delete(s.clients, conn)
			conn.Close()
			fmt.Println("Client disconnected!")
		}
	}
}

