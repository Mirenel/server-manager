package main

import (
	"log"
	"net/http"
)

const configPath = "config.json"

func main() {
	cfg, err := loadConfig(configPath)
	if err != nil {
		log.Fatalf("failed to load config.json: %v", err)
	}

	pm := newProcessManager(cfg, configPath)
	pm.run()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/processes", pm.handleGetProcesses)
	mux.HandleFunc("POST /api/processes/start-all", pm.handleStartAll)
	mux.HandleFunc("POST /api/processes/stop-all", pm.handleStopAll)
	mux.HandleFunc("GET /api/processes/{id}/start", pm.handleStart)
	mux.HandleFunc("POST /api/processes/{id}/start", pm.handleStart)
	mux.HandleFunc("POST /api/processes/{id}/stop", pm.handleStop)
	mux.HandleFunc("GET /api/processes/{id}/metrics", pm.handleGetMetrics)
	mux.HandleFunc("PUT /api/processes/{id}/autorestart", pm.handleToggleAutoRestart)
	mux.HandleFunc("GET /api/processes/{id}/logs", pm.handleGetLogs)
	mux.HandleFunc("GET /api/config", pm.handleGetConfig)
	mux.HandleFunc("PUT /api/config", pm.handlePutConfig)
	mux.HandleFunc("GET /api/events", pm.handleGetEvents)
	mux.HandleFunc("/ws", pm.handleWS)

	log.Println("Server manager backend running on http://localhost:8090")
	log.Fatal(http.ListenAndServe(":8090", corsMiddleware(mux)))
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
