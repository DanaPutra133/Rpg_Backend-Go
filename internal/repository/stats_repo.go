package repository

import (
	"context"
	"database/sql"
	"time"
)

type StatsRepository struct {
	DB *sql.DB
}

func NewStatsRepository(db *sql.DB) *StatsRepository {
	// Table creation (sama seperti sebelumnya)
	query := `
	CREATE TABLE IF NOT EXISTS traffic_stats (
		timestamp INTEGER PRIMARY KEY,
		get_count INTEGER DEFAULT 0,
		post_count INTEGER DEFAULT 0,
		put_count INTEGER DEFAULT 0,
		delete_count INTEGER DEFAULT 0
	);
	CREATE INDEX IF NOT EXISTS idx_ts ON traffic_stats(timestamp);
	`
	_, err := db.Exec(query)
	if err != nil {
		panic(err)
	}
	return &StatsRepository{DB: db}
}

// LogTraffic (Sama seperti sebelumnya)
func (r *StatsRepository) LogTraffic(method string) {
	now := time.Now()
	// Rounding ke menit (detik 0, nanosekon 0)
	roundedTime := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), 0, 0, now.Location())
	ts := roundedTime.UnixMilli()

	incGet, incPost, incPut, incDel := 0, 0, 0, 0
	switch method {
	case "GET":
		incGet = 1
	case "POST":
		incPost = 1
	case "PUT":
		incPut = 1
	case "DELETE":
		incDel = 1
	}

	query := `
	INSERT INTO traffic_stats (timestamp, get_count, post_count, put_count, delete_count)
	VALUES (?, ?, ?, ?, ?)
	ON CONFLICT(timestamp) DO UPDATE SET
		get_count = get_count + excluded.get_count,
		post_count = post_count + excluded.post_count,
		put_count = put_count + excluded.put_count,
		delete_count = delete_count + excluded.delete_count;
	`
	go func() {
		r.DB.Exec(query, ts, incGet, incPost, incPut, incDel)
	}()
}

func (r *StatsRepository) GetHourlyStats(ctx context.Context) ([]map[string]interface{}, error) {
	now := time.Now()

	oneHourAgo := now.Add(-1 * time.Hour)

	startTs := time.Date(oneHourAgo.Year(), oneHourAgo.Month(), oneHourAgo.Day(), oneHourAgo.Hour(), oneHourAgo.Minute(), 0, 0, oneHourAgo.Location()).UnixMilli()

	endTs := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), 0, 0, now.Location()).UnixMilli()

	query := `
		SELECT timestamp, get_count, post_count, put_count, delete_count 
		FROM traffic_stats 
		WHERE timestamp >= ? 
		ORDER BY timestamp ASC`

	rows, err := r.DB.QueryContext(ctx, query, startTs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Simpan hasil DB ke Map
	dbData := make(map[int64]map[string]interface{})
	for rows.Next() {
		var ts, get, post, put, del int64
		rows.Scan(&ts, &get, &post, &put, &del)
		dbData[ts] = map[string]interface{}{
			"timestamp": ts,
			"GET":       get,
			"POST":      post,
			"PUT":       put,
			"DELETE":    del,
		}
	}

	// Generate Loop (Zero Filling)
	var finalResult []map[string]interface{}

	// Loop dimulai dari 1 Jam Lalu (startTs) sampai Sekarang (endTs)
	currentLoop := startTs
	for currentLoop <= endTs {
		if val, exists := dbData[currentLoop]; exists {
			finalResult = append(finalResult, val)
		} else {
			// Isi 0 jika tidak ada traffic di menit itu
			finalResult = append(finalResult, map[string]interface{}{
				"timestamp": currentLoop,
				"GET":       0,
				"POST":      0,
				"PUT":       0,
				"DELETE":    0,
			})
		}
		currentLoop += 60000 // Tambah 1 menit
	}

	return finalResult, nil
}
func (r *StatsRepository) ResetStats() error {
	query := `DELETE FROM traffic_stats`
	_, err := r.DB.Exec(query)
	return err
}
