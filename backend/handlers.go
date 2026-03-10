package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"
)

var validDiagnoses = map[string]bool{
	"whitelist_active":  true,
	"normal_filtering":  true,
	"no_internet":       true,
	"no_signal":         true,
	"everything_works":  true,
	"not_in_russia":     true,
}

// handleTrackingSession accepts a full active-tracking session as a single batch.
// Rate-limited to 10 sessions per IP per hour; max 500 points per session.
func handleTrackingSession(db *sql.DB, rl *RateLimiter, apiSecret string) http.HandlerFunc {
	type pointIn struct {
		Lat        float64 `json:"lat"`
		Lon        float64 `json:"lon"`
		Diagnosis  string  `json:"diagnosis"`
		MeasuredAt string  `json:"measuredAt"` // ISO-8601
	}
	type body struct {
		AppVersion string    `json:"appVersion"`
		Points     []pointIn `json:"points"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if apiSecret != "" && r.Header.Get("X-App-Secret") != apiSecret {
			jsonError(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		ip := realIP(r)
		if !rl.Allow(ip) {
			jsonError(w, "rate limited", http.StatusTooManyRequests)
			return
		}

		var req body
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "bad json", http.StatusBadRequest)
			return
		}
		if len(req.Points) == 0 {
			jsonError(w, "no points", http.StatusBadRequest)
			return
		}
		if len(req.Points) > 500 {
			jsonError(w, "too many points", http.StatusBadRequest)
			return
		}

		appVer := sanitize(req.AppVersion, 16)

		tx, err := db.Begin()
		if err != nil {
			log.Printf("tx begin error: %v", err)
			jsonError(w, "db error", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		stmt, err := tx.Prepare(
			`INSERT INTO reports (lat, lon, diagnosis, carrier, app_version, network_type, created_at)
			 VALUES (?, ?, ?, 'unknown', ?, 'cellular', ?)`,
		)
		if err != nil {
			log.Printf("prepare error: %v", err)
			jsonError(w, "db error", http.StatusInternalServerError)
			return
		}
		defer stmt.Close()

		inserted := 0
		now := time.Now().Unix()
		for _, p := range req.Points {
			if !validDiagnoses[p.Diagnosis] {
				continue
			}
			if p.Lat < 39 || p.Lat > 79 || p.Lon < 17 || p.Lon > 193 {
				continue
			}
			lat := math.Round(p.Lat/0.02) * 0.02
			lon := math.Round(p.Lon/0.02) * 0.02

			ts := now
			if t, err := time.Parse(time.RFC3339, p.MeasuredAt); err == nil {
				ts = t.Unix()
			}

			if _, err := stmt.Exec(lat, lon, p.Diagnosis, appVer, ts); err != nil {
				log.Printf("insert error: %v", err)
				continue
			}
			inserted++
		}

		if err := tx.Commit(); err != nil {
			log.Printf("tx commit error: %v", err)
			jsonError(w, "db error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "inserted": inserted})
	}
}

// handlePurge deletes noisy diagnoses from the database. Admin-only.
func handlePurge(db *sql.DB, apiSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if apiSecret == "" || r.Header.Get("X-App-Secret") != apiSecret {
			jsonError(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		diagnoses := r.URL.Query()["diagnosis"]
		if len(diagnoses) == 0 {
			jsonError(w, "specify ?diagnosis=...", http.StatusBadRequest)
			return
		}
		var total int64
		for _, d := range diagnoses {
			if !validDiagnoses[d] {
				continue
			}
			res, err := db.Exec(`DELETE FROM reports WHERE diagnosis = ?`, d)
			if err != nil {
				log.Printf("purge error: %v", err)
				jsonError(w, "db error", http.StatusInternalServerError)
				return
			}
			n, _ := res.RowsAffected()
			total += n
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "deleted": total})
	}
}

func handleReport(db *sql.DB, rl *RateLimiter, apiSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if apiSecret != "" && r.Header.Get("X-App-Secret") != apiSecret {
			jsonError(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		ip := realIP(r)
		if !rl.Allow(ip) {
			jsonError(w, "rate limited", http.StatusTooManyRequests)
			return
		}

		var body struct {
			Lat         float64 `json:"lat"`
			Lon         float64 `json:"lon"`
			Diagnosis   string  `json:"diagnosis"`
			Carrier     string  `json:"carrier"`
			AppVersion  string  `json:"appVersion"`
			NetworkType string  `json:"networkType"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonError(w, "bad json", http.StatusBadRequest)
			return
		}

		if !validDiagnoses[body.Diagnosis] {
			jsonError(w, "invalid diagnosis", http.StatusBadRequest)
			return
		}

		// Accept only cellular
		if body.NetworkType != "" && body.NetworkType != "cellular" {
			jsonError(w, "cellular only", http.StatusBadRequest)
			return
		}

		// Russia bbox validation (generous margin)
		if body.Lat < 39 || body.Lat > 79 || body.Lon < 17 || body.Lon > 193 {
			jsonError(w, "coords out of range", http.StatusBadRequest)
			return
		}

		// Round to 0.02° grid server-side (defence in depth)
		lat := math.Round(body.Lat/0.02) * 0.02
		lon := math.Round(body.Lon/0.02) * 0.02

		carrier := sanitize(body.Carrier, 64)
		appVer := sanitize(body.AppVersion, 16)

		_, err := db.Exec(
			`INSERT INTO reports (lat, lon, diagnosis, carrier, app_version, network_type, created_at)
			 VALUES (?, ?, ?, ?, ?, 'cellular', ?)`,
			lat, lon, body.Diagnosis, carrier, appVer, time.Now().Unix(),
		)
		if err != nil {
			log.Printf("insert error: %v", err)
			jsonError(w, "db error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}
}

func handleMap(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		zoom, _ := strconv.Atoi(q.Get("zoom"))
		if zoom < 1 {
			zoom = 5
		}
		if zoom > 18 {
			zoom = 18
		}

		bounds := strings.Split(q.Get("bounds"), ",")
		if len(bounds) != 4 {
			// Default to full Russia view
			bounds = []string{"40", "18", "78", "192"}
		}

		lat1, _ := strconv.ParseFloat(bounds[0], 64)
		lon1, _ := strconv.ParseFloat(bounds[1], 64)
		lat2, _ := strconv.ParseFloat(bounds[2], 64)
		lon2, _ := strconv.ParseFloat(bounds[3], 64)

		cells, err := queryClusters(db, zoom, lat1, lon1, lat2, lon2)
		if err != nil {
			log.Printf("cluster query error: %v", err)
			jsonError(w, "db error", http.StatusInternalServerError)
			return
		}
		if cells == nil {
			cells = []ClusterCell{}
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=60")
		json.NewEncoder(w).Encode(map[string]any{
			"cells": cells,
		})
	}
}

func handleStats(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var total, last24h int
		db.QueryRow(`SELECT COUNT(*) FROM reports`).Scan(&total)
		db.QueryRow(`SELECT COUNT(*) FROM reports WHERE created_at > ?`,
			time.Now().Unix()-86400).Scan(&last24h)

		// Breakdown by diagnosis (all time)
		rows, _ := db.Query(`
			SELECT diagnosis, COUNT(*) as cnt
			FROM reports
			GROUP BY diagnosis
			ORDER BY cnt DESC
		`)
		breakdown := map[string]int{}
		if rows != nil {
			defer rows.Close()
			for rows.Next() {
				var d string
				var c int
				rows.Scan(&d, &c)
				breakdown[d] = c
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=30")
		json.NewEncoder(w).Encode(map[string]any{
			"total":     total,
			"last24h":   last24h,
			"breakdown": breakdown,
		})
	}
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func sanitize(s string, maxLen int) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsPrint(r) {
			return r
		}
		return -1
	}, s)
	if len(s) > maxLen {
		return s[:maxLen]
	}
	return s
}
