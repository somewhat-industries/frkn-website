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
		return 0.05
	}
}

func queryClusters(db *sql.DB, zoom int, lat1, lon1, lat2, lon2 float64) ([]ClusterCell, error) {
	gs := gridSize(zoom)

	// Expand bounds slightly to avoid edge artifacts
	margin := gs
	lat1 -= margin
	lat2 += margin
	lon1 -= margin
	lon2 += margin

	// Clamp to Russia bbox
	lat1 = math.Max(lat1, 40)
	lat2 = math.Min(lat2, 78)
	lon1 = math.Max(lon1, 18)
	lon2 = math.Min(lon2, 192)

	query := `
WITH cells AS (
    SELECT
        ROUND(lat / ?) * ?  AS cell_lat,
        ROUND(lon / ?) * ?  AS cell_lon,
        diagnosis,
        COUNT(*)            AS cnt
    FROM reports
    WHERE
        diagnosis != 'not_in_russia'
        AND lat BETWEEN ? AND ?
        AND lon BETWEEN ? AND ?
    GROUP BY cell_lat, cell_lon, diagnosis
),
cell_totals AS (
    SELECT cell_lat, cell_lon, SUM(cnt) AS total_cnt
    FROM cells
    GROUP BY cell_lat, cell_lon
),
dominant AS (
    SELECT cell_lat, cell_lon, diagnosis
    FROM cells c1
    WHERE cnt = (
        SELECT MAX(cnt) FROM cells c2
        WHERE c2.cell_lat = c1.cell_lat AND c2.cell_lon = c1.cell_lon
    )
    GROUP BY cell_lat, cell_lon
)
SELECT d.cell_lat, d.cell_lon, t.total_cnt, d.diagnosis
FROM dominant d
JOIN cell_totals t ON t.cell_lat = d.cell_lat AND t.cell_lon = d.cell_lon
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
