package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// --- Konfigurasi dan State ---

// Credential untuk login
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// User data (hardcoded, cocok dengan JS Anda)
type User struct {
	Password string `json:"-"` // Jangan kirim password ke client
	Name     string `json:"name"`
	Role     string `json:"role"`
}

var users = map[string]User{
	"root": {Password: "toor", Name: "hacker", Role: "Administrator"},
	"user":  {Password: "user123", Name: "User", Role: "User"},
	"demo":  {Password: "demo", Name: "Demo User", Role: "Demo"},
}

// Sesi untuk menyimpan token
var sessions = make(map[string]string) // map[token]username
var sessionsMutex sync.Mutex

// CurlRequest menampung semua data dari form dashboard
type CurlRequest struct {
	URL       string `json:"url"`
	Method    string `json:"method"`
	UserAgent string `json:"userAgent"`
	Threads   int    `json:"threads"`
	Requests  int    `json:"requests"` // 0 untuk tak terbatas
	Delay     int    `json:"delay"`    // dalam milidetik
	Timeout   int    `json:"timeout"`  // dalam detik
}

// Stats untuk statistik real-time
type Stats struct {
	Total   interface{} `json:"total"` // Bisa string "âˆž" atau int
	Sent    int         `json:"sent"`
	Success int         `json:"success"`
	Failed  int         `json:"failed"`
}

// WSMessage untuk komunikasi WebSocket
type WSMessage struct {
	Type    string      `json:"type"` // "log", "stats", "status"
	Payload interface{} `json:"payload"`
}

// State global untuk proses
var (
	isProcessRunning bool
	processMutex     sync.Mutex
	cancelFunc       context.CancelFunc
	currentStats     Stats
	currentConfig    CurlRequest // Menyimpan konfigurasi yang sedang berjalan
)

// --- WebSocket ---
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}
var clients = make(map[*websocket.Conn]bool)
var broadcast = make(chan []byte)
var clientsMutex sync.Mutex

// --- Fungsi Inti ---

func startStressTest(ctx context.Context, config CurlRequest) {
	processMutex.Lock()
	if isProcessRunning {
		processMutex.Unlock()
		log.Println("Proses sudah berjalan, permintaan baru diabaikan.")
		return
	}
	isProcessRunning = true
	currentConfig = config // Simpan konfigurasi saat ini
	if config.Requests == 0 {
		currentStats = Stats{Total: "âˆž", Sent: 0, Success: 0, Failed: 0}
	} else {
		currentStats = Stats{Total: config.Requests, Sent: 0, Success: 0, Failed: 0}
	}
	processMutex.Unlock()

	defer func() {
		processMutex.Lock()
		isProcessRunning = false
		currentConfig = CurlRequest{} // Reset konfigurasi setelah selesai
		processMutex.Unlock()
		log.Println("Stress test telah berhenti sepenuhnya.")
		broadcastLog("INFO: Eksekusi telah berhenti.")
		broadcastStatusUpdate()
	}()

	broadcastLog(fmt.Sprintf("INFO: Memulai eksekusi ke %s dengan %d threads...", config.URL, config.Threads))
	broadcastStatusUpdate()

	jobs := make(chan bool)
	results := make(chan bool)
	var workerWg sync.WaitGroup

	for w := 1; w <= config.Threads; w++ {
		workerWg.Add(1)
		go worker(ctx, &workerWg, w, config, jobs, results)
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	go func() {
		if config.Requests == 0 {
			for {
				select {
				case <-ctx.Done():
					return
				case jobs <- true:
				}
			}
		} else {
			for j := 0; j < config.Requests; j++ {
				select {
				case jobs <- true:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	go func() {
		<-ctx.Done()
		close(jobs)
		workerWg.Wait()
		close(results)
	}()

	requestsProcessed := 0
	for {
		select {
		case success, ok := <-results:
			if !ok {
				log.Println("Channel results ditutup, keluar dari loop.")
				return
			}
			requestsProcessed++
			processMutex.Lock()
			currentStats.Sent++
			if success {
				currentStats.Success++
			} else {
				currentStats.Failed++
			}
			processMutex.Unlock()

			if config.Requests > 0 && requestsProcessed >= config.Requests {
				log.Println("Jumlah request maksimum tercapai.")
				broadcastLog("INFO: Jumlah request maksimum tercapai. Eksekusi selesai.")
				if cancelFunc != nil {
					cancelFunc()
				}
				return
			}

		case <-ticker.C:
			broadcastStatusUpdate()

		case <-ctx.Done():
			log.Println("Sinyal cancel terdeteksi, menunggu loop utama berhenti...")
		}
	}
}

func worker(ctx context.Context, wg *sync.WaitGroup, id int, config CurlRequest, jobs <-chan bool, results chan<- bool) {
	defer wg.Done()
	for range jobs {
		args := []string{
			"-s", "-L",
			"--max-time", fmt.Sprintf("%d", config.Timeout),
			"-w", "Status: %{http_code} | Waktu: %{time_total}s",
			"-A", config.UserAgent,
			"-X", config.Method,
			config.URL,
		}

		cmd := exec.CommandContext(ctx, "curl", args...)
		output, err := cmd.CombinedOutput()

		success := true
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			success = false
			processMutex.Lock()
			if currentStats.Failed%10 == 0 {
				errMsg := fmt.Sprintf("ERROR (Thread %d): Gagal mengeksekusi curl. Output: %s", id, string(output))
				log.Println(errMsg)
				broadcastLog(errMsg)
			}
			processMutex.Unlock()
		}
		results <- success

		if config.Delay > 0 {
			time.Sleep(time.Duration(config.Delay) * time.Millisecond)
		}
	}
}

// --- Fungsi Helper Broadcast ---

func broadcastMessage(msgType string, payload interface{}) {
	message := WSMessage{Type: msgType, Payload: payload}
	jsonMsg, err := json.Marshal(message)
	if err != nil {
		log.Printf("Error marshaling ws message: %v", err)
		return
	}
	broadcast <- jsonMsg
}

func broadcastLog(logMessage string) {
	broadcastMessage("log", logMessage)
}

func broadcastStatusUpdate() {
	processMutex.Lock()
	statsCopy := currentStats
	isRunningCopy := isProcessRunning
	processMutex.Unlock()

	payload := map[string]interface{}{
		"running": isRunningCopy,
		"stats":   statsCopy,
	}
	broadcastMessage("status", payload)
}

// --- Handlers ---

func generateSecureToken(length int) string {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Request body tidak valid", http.StatusBadRequest)
		return
	}

	user, ok := users[req.Username]
	if !ok || user.Password != req.Password {
		http.Error(w, `{"error": "Invalid username or password"}`, http.StatusUnauthorized)
		return
	}

	token := generateSecureToken(16)
	sessionsMutex.Lock()
	sessions[token] = user.Name
	sessionsMutex.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"user":    user,
		"token":   token,
	})
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	sessionsMutex.Lock()
	delete(sessions, token)
	sessionsMutex.Unlock()
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "Logged out"})
}

func handleStart(w http.ResponseWriter, r *http.Request) {
	var req CurlRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Request body tidak valid: "+err.Error(), http.StatusBadRequest)
		return
	}

	processMutex.Lock()
	if isProcessRunning {
		processMutex.Unlock()
		http.Error(w, `{"error": "Proses lain sedang berjalan"}`, http.StatusConflict)
		return
	}
	processMutex.Unlock()

	var ctx context.Context
	ctx, cancelFunc = context.WithCancel(context.Background())
	go startStressTest(ctx, req)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "Proses dimulai"})
}

func handleStop(w http.ResponseWriter, r *http.Request) {
	processMutex.Lock()
	defer processMutex.Unlock()

	if !isProcessRunning {
		http.Error(w, `{"error": "Tidak ada proses yang berjalan"}`, http.StatusNotFound)
		return
	}

	if cancelFunc != nil {
		cancelFunc()
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "Sinyal berhenti dikirim"})
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	processMutex.Lock()
	defer processMutex.Unlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"running": isProcessRunning})
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	sessionsMutex.Lock()
	_, ok := sessions[token]
	sessionsMutex.Unlock()

	if !ok {
		http.Error(w, "Unauthorized: Invalid token", http.StatusUnauthorized)
		log.Println("Koneksi WebSocket ditolak: token tidak valid")
		return
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	defer ws.Close()

	clientsMutex.Lock()
	clients[ws] = true
	clientsMutex.Unlock()
	log.Println("Client WebSocket baru terhubung")

	processMutex.Lock()
	statusPayload := map[string]interface{}{
		"running": isProcessRunning,
		"stats":   currentStats,
		"config":  currentConfig,
	}
	processMutex.Unlock()

	message := WSMessage{Type: "status", Payload: statusPayload}
	jsonMsg, _ := json.Marshal(message)
	ws.WriteMessage(websocket.TextMessage, jsonMsg)

	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			log.Printf("Client terputus: %v", err)
			clientsMutex.Lock()
			delete(clients, ws)
			clientsMutex.Unlock()
			break
		}
	}
}

func handleMessages() {
	for {
		msg := <-broadcast
		clientsMutex.Lock()
		for client := range clients {
			err := client.WriteMessage(websocket.TextMessage, msg)
			if err != nil {
				log.Printf("Gagal mengirim pesan: %v", err)
				client.Close()
				delete(clients, client)
			}
		}
		clientsMutex.Unlock()
	}
}

func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Unauthorized: Header otorisasi tidak ada", http.StatusUnauthorized)
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		sessionsMutex.Lock()
		_, ok := sessions[token]
		sessionsMutex.Unlock()

		if !ok {
			http.Error(w, "Unauthorized: Token tidak valid", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	mux := http.NewServeMux()

	// Endpoint publik
	mux.HandleFunc("/login", handleLogin)

	// Endpoint yang dilindungi
	mux.HandleFunc("/start", authMiddleware(handleStart))
	mux.HandleFunc("/stop", authMiddleware(handleStop))
	mux.HandleFunc("/status", authMiddleware(handleStatus))
	mux.HandleFunc("/logout", authMiddleware(handleLogout))

	// Endpoint WebSocket (otorisasi ditangani di dalamnya)
	mux.HandleFunc("/ws", handleConnections)

	go handleMessages()

	port := "8080"
	log.Printf("Server berjalan di http://localhost:%s\n", port)

	if err := http.ListenAndServe(":"+port, corsMiddleware(mux)); err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
