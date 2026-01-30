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

// Fungsi BARU: Mengambil data jam ini + mengisi menit yang kosong dengan 0
func (r *StatsRepository) GetHourlyStats(ctx context.Context) ([]map[string]interface{}, error) {
	// 1. Tentukan Range Waktu (Start of Hour sampai Sekarang)
	now := time.Now()
	// Start: Jam sekarang, menit 0
	startOfHour := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())
	startTs := startOfHour.UnixMilli()

	// End: Menit sekarang (biar realtime sampai detik ini)
	// Kalau mau fix 60 menit full (future), ganti loopnya nanti.
	// Di sini kita loop sampai menit saat ini + 1 buffer
	endOfHour := now.UnixMilli()

	// 2. Query hanya data yang >= Start jam ini
	// Ini otomatis bikin efek "Reset tiap jam", karena data jam lalu tidak akan keambil.
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

	// Simpan hasil DB ke Map dulu biar gampang dicocokkan
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

	// 3. Generate Array Lengkap (Zero Filling)
	// Kita buat loop dari menit 0 sampai menit sekarang
	// Setiap loop nambah 60 detik (60000ms)
	var finalResult []map[string]interface{}

	currentLoop := startTs
	// Loop sampai menit sekarang (agar grafik update terus).
	// Kalau mau fix 60 baris statis, ganti 'endOfHour' dengan 'startTs + (60 * 60000)'
	for currentLoop <= endOfHour {
		if val, exists := dbData[currentLoop]; exists {
			// Kalau ada data di DB, pakai itu
			finalResult = append(finalResult, val)
		} else {
			// Kalau gak ada (kosong), isi dengan 0
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
