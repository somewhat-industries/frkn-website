package main

import (
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data.db"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	db := initDB(dbPath)
	defer db.Close()

	apiSecret := os.Getenv("API_SECRET")

	// 10 reports per IP per hour (single-point tester)
	rl := newRateLimiter(10, time.Hour)
	// 10 sessions per IP per hour (active tracking batch upload)
	sessionRL := newRateLimiter(10, time.Hour)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/report", handleReport(db, rl, apiSecret))
	mux.HandleFunc("POST /api/tracking/session", handleTrackingSession(db, sessionRL, apiSecret))
	mux.HandleFunc("GET /api/map", handleMap(db))
	mux.HandleFunc("GET /api/stats", handleStats(db))

	// Serve frontend static files
	staticDir := os.Getenv("STATIC_DIR")
	if staticDir == "" {
		staticDir = "../frontend/static"
	}
	mux.Handle("/", http.FileServer(http.Dir(staticDir)))

	handler := corsMiddleware(mux)

	log.Printf("FRKN backend listening on :%s", port)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := map[string]bool{
			"https://frknsw.com":     true,
			"https://www.frknsw.com": true,
			"http://localhost:3000":  true,
			"http://localhost:8080":  true,
		}
		if allowed[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Vary", "Origin")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
