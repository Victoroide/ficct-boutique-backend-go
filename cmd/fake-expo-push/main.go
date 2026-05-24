// Fake Expo Push API.
//
// Implements just enough of https://exp.host/--/api/v2/push/send for Docker
// integration tests. NOT for production traffic. Records every received
// batch in memory so test harnesses can assert on the payload.
//
// Endpoints:
//
//	POST /--/api/v2/push/send       Expo-compatible ticket response
//	GET  /inspect/calls             returns all recorded batches as JSON
//	POST /inspect/reset             clears the recorded batches
//	GET  /inspect/mode              returns current behavior mode
//	POST /inspect/mode?value=...    sets behavior mode
//	GET  /healthz                   liveness
//
// Behavior modes (env FAKE_EXPO_MODE or POST /inspect/mode?value=...):
//
//	ok                      every message gets a fresh "ok" ticket  (default)
//	device_not_registered   every message gets a DeviceNotRegistered ticket
//	per-token               tokens containing "BAD" get DeviceNotRegistered,
//	                        every other token gets ok
//	http500                 server returns HTTP 500 with an error body
package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/google/uuid"
)

type message struct {
	To       string                 `json:"to"`
	Title    string                 `json:"title,omitempty"`
	Body     string                 `json:"body,omitempty"`
	Sound    string                 `json:"sound,omitempty"`
	Data     map[string]interface{} `json:"data,omitempty"`
	Priority string                 `json:"priority,omitempty"`
}

type ticket struct {
	Status  string  `json:"status"`
	ID      string  `json:"id,omitempty"`
	Message string  `json:"message,omitempty"`
	Details *detail `json:"details,omitempty"`
}

type detail struct {
	Error string `json:"error"`
}

type response struct {
	Data []ticket `json:"data"`
}

var (
	mu       sync.Mutex
	captured [][]message
	mode     = getenvDefault("FAKE_EXPO_MODE", "per-token")
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /--/api/v2/push/send", handlePush)
	mux.HandleFunc("GET /inspect/calls", handleInspectCalls)
	mux.HandleFunc("POST /inspect/reset", handleInspectReset)
	mux.HandleFunc("GET /inspect/mode", handleGetMode)
	mux.HandleFunc("POST /inspect/mode", handleSetMode)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	port := getenvDefault("PORT", "8080")
	log.Printf("fake-expo-push: listening on :%s (mode=%s)", port, mode)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("listen: %v", err)
	}
}

func handlePush(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	var batch []message
	if err := json.Unmarshal(body, &batch); err != nil {
		http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
		return
	}

	mu.Lock()
	captured = append(captured, batch)
	currentMode := mode
	mu.Unlock()

	if currentMode == "http500" {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"errors":[{"code":"INTERNAL_SERVER_ERROR"}]}`))
		return
	}

	tickets := make([]ticket, 0, len(batch))
	for _, m := range batch {
		switch currentMode {
		case "device_not_registered":
			tickets = append(tickets, deviceNotRegistered(m.To))
		case "per-token":
			if strings.Contains(m.To, "BAD") {
				tickets = append(tickets, deviceNotRegistered(m.To))
			} else {
				tickets = append(tickets, ticket{Status: "ok", ID: uuid.NewString()})
			}
		default:
			tickets = append(tickets, ticket{Status: "ok", ID: uuid.NewString()})
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response{Data: tickets})
}

func handleInspectCalls(w http.ResponseWriter, _ *http.Request) {
	mu.Lock()
	out := make([][]message, len(captured))
	copy(out, captured)
	mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"count": len(out),
		"calls": out,
	})
}

func handleInspectReset(w http.ResponseWriter, _ *http.Request) {
	mu.Lock()
	captured = nil
	mu.Unlock()
	_, _ = w.Write([]byte("ok"))
}

func handleGetMode(w http.ResponseWriter, _ *http.Request) {
	mu.Lock()
	defer mu.Unlock()
	_, _ = w.Write([]byte(mode))
}

func handleSetMode(w http.ResponseWriter, r *http.Request) {
	v := strings.TrimSpace(r.URL.Query().Get("value"))
	if v == "" {
		http.Error(w, "value query param required", http.StatusBadRequest)
		return
	}
	mu.Lock()
	mode = v
	mu.Unlock()
	_, _ = w.Write([]byte("ok"))
}

func deviceNotRegistered(to string) ticket {
	return ticket{
		Status:  "error",
		Message: `"` + to + `" is not a registered push notification recipient`,
		Details: &detail{Error: "DeviceNotRegistered"},
	}
}

func getenvDefault(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
