package main

import (
	"database/sql"
	"math"
)

type ClusterCell struct {
	Lat       float64 `json:"lat"`
	Lon       float64 `json:"lon"`
	Count     int     `json:"count"`
	Diagnosis string  `json:"diagnosis"`
}

func gridSize(zoom int) float64 {
	switch {
	case zoom <= 4:
		return 2.0
	case zoom <= 6:
		return 1.0
	case zoom <= 8:
		return 0.5
	case zoom <= 11:
		return 0.1
	default:
		return 0.02
	}
}

func queryClusters(db *sql.DB, zoom int, lat1, lon1, lat2, lon2 float64) ([]ClusterCell, error) {
	gs := gridSize(zoom)

	margin := gs
	lat1 -= margin
	lat2 += margin
	lon1 -= margin
	lon2 += margin

	lat1 = math.Max(lat1, 40)
	lat2 = math.Min(lat2, 78)
	lon1 = math.Max(lon1, 18)
	lon2 = math.Min(lon2, 192)

	// Excluded diagnoses: not meaningful for the map
	// - not_in_russia: VPN users, useless noise
	// - no_signal:     E/2G/3G dead zones, not internet freedom data
	// - no_internet:   ambiguous (could be coverage, not filtering)
	query := `
WITH cells AS (
    SELECT
        ROUND(lat / ?) * ?  AS grid_lat,
        ROUND(lon / ?) * ?  AS grid_lon,
        AVG(lat)            AS avg_lat,
        AVG(lon)            AS avg_lon,
        diagnosis,
        COUNT(*)            AS cnt,
        MAX(created_at)     AS last_seen
    FROM reports
    WHERE
        diagnosis NOT IN ('not_in_russia', 'no_signal', 'no_internet')
        AND lat BETWEEN ? AND ?
        AND lon BETWEEN ? AND ?
    GROUP BY grid_lat, grid_lon, diagnosis
),
cell_totals AS (
    SELECT
        grid_lat, grid_lon,
        SUM(cnt)                            AS total_cnt,
        SUM(avg_lat * cnt) / SUM(cnt)       AS cell_lat,
        SUM(avg_lon * cnt) / SUM(cnt)       AS cell_lon
    FROM cells
    GROUP BY grid_lat, grid_lon
),
dominant AS (
    SELECT grid_lat, grid_lon, diagnosis
    FROM (
        SELECT grid_lat, grid_lon, diagnosis,
               ROW_NUMBER() OVER (
                   PARTITION BY grid_lat, grid_lon
                   ORDER BY cnt DESC, last_seen DESC
               ) AS rn
        FROM cells
    )
    WHERE rn = 1
)
SELECT t.cell_lat, t.cell_lon, t.total_cnt, d.diagnosis
FROM dominant d
JOIN cell_totals t ON t.grid_lat = d.grid_lat AND t.grid_lon = d.grid_lon
ORDER BY t.total_cnt DESC
LIMIT 2000
`

	rows, err := db.Query(query,
		gs, gs, gs, gs,
		lat1, lat2, lon1, lon2,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ClusterCell
	for rows.Next() {
		var c ClusterCell
		if err := rows.Scan(&c.Lat, &c.Lon, &c.Count, &c.Diagnosis); err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, rows.Err()
}
