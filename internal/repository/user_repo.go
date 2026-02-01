package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

type UserRepository struct {
	DB    *sql.DB
	Redis *redis.Client
}

func NewUserRepository(db *sql.DB, rdb *redis.Client) *UserRepository {

	query := `
	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		username TEXT,
		money REAL DEFAULT 0,
		level INTEGER DEFAULT 0,
		data TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_money ON users(money);
	CREATE INDEX IF NOT EXISTS idx_level ON users(level);
	`
	_, err := db.Exec(query)
	if err != nil {
		panic(err)
	}
	return &UserRepository{DB: db, Redis: rdb}
}

// GetUser mengambil data (Cek Redis -> Cek SQLite)
func (r *UserRepository) GetUser(ctx context.Context, userID string) (map[string]interface{}, error) {
	//Cek Redis
	val, err := r.Redis.Get(ctx, "user:"+userID).Result()
	if err == nil {
		var data map[string]interface{}
		json.Unmarshal([]byte(val), &data)
		return data, nil
	}

	// Cek SQLite
	var dataJSON string
	err = r.DB.QueryRowContext(ctx, "SELECT data FROM users WHERE id = ?", userID).Scan(&dataJSON)
	if err == sql.ErrNoRows {
		return nil, nil // Not found
	} else if err != nil {
		return nil, err
	}

	var data map[string]interface{}
	json.Unmarshal([]byte(dataJSON), &data)

	// 3. Simpan ke Redis (Cache aside)
	r.Redis.Set(ctx, "user:"+userID, dataJSON, 10*time.Minute)

	return data, nil
}

// SaveUser menyimpan data
func (r *UserRepository) SaveUser(ctx context.Context, userID string, data map[string]interface{}) error {
	dataBytes, _ := json.Marshal(data)
	dataStr := string(dataBytes)

	username, _ := data["username"].(string)
	money, _ := data["money"].(float64)

	// Ambil level dari nested rpg
	level := 0
	if rpg, ok := data["rpg"].(map[string]interface{}); ok {
		if l, ok := rpg["level"].(float64); ok {
			level = int(l)
		}
	}

	// Upsert ke SQLite
	query := `
	INSERT INTO users (id, username, money, level, data)
	VALUES (?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		username=excluded.username,
		money=excluded.money,
		level=excluded.level,
		data=excluded.data;
	`
	_, err := r.DB.ExecContext(ctx, query, userID, username, money, level, dataStr)
	if err != nil {
		return err
	}

	// Update Redis langsung biar sinkron
	r.Redis.Set(ctx, "user:"+userID, dataStr, 10*time.Minute)
	return nil
}

// GetAllUsers untuk Leaderboard
func (r *UserRepository) GetAllUsers(ctx context.Context) ([]map[string]interface{}, error) {
	rows, err := r.DB.QueryContext(ctx, "SELECT data FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []map[string]interface{}
	for rows.Next() {
		var dataStr string
		if err := rows.Scan(&dataStr); err != nil {
			continue
		}
		var u map[string]interface{}
		json.Unmarshal([]byte(dataStr), &u)
		users = append(users, u)
	}
	return users, nil
}

// get user AFK
func (r *UserRepository) GetAFKUsers(ctx context.Context) (map[string]interface{}, error) {
	query := `SELECT id, data FROM users WHERE json_extract(data, '$.afk') > 0`

	rows, err := r.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]interface{})

	for rows.Next() {
		var id string
		var dataJSON string
		if err := rows.Scan(&id, &dataJSON); err != nil {
			continue
		}

		var userData map[string]interface{}
		if err := json.Unmarshal([]byte(dataJSON), &userData); err == nil {
			userData["id"] = id
			result[id] = userData
		}
	}
	return result, nil
}
