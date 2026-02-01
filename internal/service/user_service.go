package service

import (
	"Berpg/internal/entity"
	"Berpg/internal/repository"
	"context"
	"errors"
	"fmt"
	"sort"
	"time"
)

func getFloat(m map[string]interface{}, key string) float64 {
	if val, ok := m[key]; ok {
		if f, ok := val.(float64); ok {
			return f
		}
	}
	return 0
}

type UserService struct {
	Repo *repository.UserRepository
}

func NewUserService(repo *repository.UserRepository) *UserService {
	return &UserService{Repo: repo}
}

func (s *UserService) UpdateUser(ctx context.Context, userID string, body map[string]interface{}) error {
	// cek user apakah ada
	_, err := s.Repo.GetUser(ctx, userID)
	if err != nil {
		return err
	}

	// Simpan data baru (menimpa data lama)
	return s.Repo.SaveUser(ctx, userID, body)
}

// GetOrInitUser: Logic inti sinkronisasi data
func (s *UserService) GetOrInitUser(ctx context.Context, userID string, usernameQuery string) (map[string]interface{}, error) {
	userMap, err := s.Repo.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	needsSave := false
	defaults := entity.GetDefaultUserMap()

	// Jika user baru (belum ada di DB)
	if userMap == nil {
		userMap = defaults
		userMap["username"] = "New User"
		if usernameQuery != "" {
			userMap["username"] = usernameQuery
		}
		// Set nested objects default
		userMap["rpg"] = entity.GetDefaultRPG()
		userMap["jail"] = entity.JailStats{Status: false}

		needsSave = true
	} else {
		// Logic Sync: Cek properti yang hilang
		// Loop default keys
		for k, v := range defaults {
			if _, exists := userMap[k]; !exists {
				userMap[k] = v
				needsSave = true
			}
		}

		// Sync RPG Object
		rpgDefault := entity.GetDefaultRPG()
		// Convert struct to map for comparison logic (simplified)
		if _, ok := userMap["rpg"]; !ok {
			userMap["rpg"] = rpgDefault
			needsSave = true
		} else {
			// Deep check RPG properties
			rpgMap := userMap["rpg"].(map[string]interface{})
			// Cek field penting RPG, misal level, exp
			if _, ok := rpgMap["level"]; !ok {
				rpgMap["level"] = 1
				needsSave = true
			}
			if _, ok := rpgMap["health"]; !ok {
				rpgMap["health"] = 100
				needsSave = true
			}
		}

		// Update username jika ada di query
		currName, _ := userMap["username"].(string)
		if usernameQuery != "" && currName != usernameQuery {
			userMap["username"] = usernameQuery
			needsSave = true
		}
	}

	//Save jika ada perubahan
	if needsSave {
		// Pastikan ID tersimpan di dalam map juga (opsional, tp bagus buat konsistensi)
		userMap["id"] = userID
		err = s.Repo.SaveUser(ctx, userID, userMap)
		if err != nil {
			return nil, err
		}
	}

	return userMap, nil
}

func (s *UserService) ClaimDaily(ctx context.Context, userID string) (map[string]interface{}, error) {
	//Ambil User
	user, err := s.Repo.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("user not found")
	}

	//  Cek Cooldown (86400000 ms = 24 jam)
	lastDaily := getFloat(user, "lastDaily")
	now := time.Now().UnixMilli()

	cooldown := int64(86400000)
	timePassed := now - int64(lastDaily)

	if timePassed < cooldown {
		// Hitung sisa waktu
		sisa := cooldown - timePassed
		hours := sisa / 3600000
		minutes := (sisa % 3600000) / 60000
		return nil, fmt.Errorf("cooldown! tunggu %d jam %d menit lagi", hours, minutes)
	}

	// Berikan Hadiah
	rewardMoney := 10000.0
	rewardExp := 200.0
	rewardDiamond := 1.0

	// Update Money
	currentMoney := getFloat(user, "money")
	user["money"] = currentMoney + rewardMoney
	currentDiamond := getFloat(user, "diamond")
	user["diamond"] = currentDiamond + rewardDiamond

	// Update Exp (Nested Logic)
	if rpg, ok := user["rpg"].(map[string]interface{}); ok {
		currentExp := getFloat(rpg, "exp")
		rpg["exp"] = currentExp + rewardExp
		user["rpg"] = rpg
	}

	// Update Waktu
	user["lastDaily"] = float64(now) // Simpan sebagai float biar konsisten JSON

	// Simpan
	err = s.Repo.SaveUser(ctx, userID, user)
	if err != nil {
		return nil, err
	}

	// Return data user terbaru atau pesan sukses
	return map[string]interface{}{
		"status":    true,
		"message":   fmt.Sprintf("Berhasil klaim harian! Dapat Rp %.0f dan %.0f Diamond.", rewardMoney, rewardDiamond),
		"money":     user["money"],
		"diamond":   user["diamond"],
		"lastDaily": user["lastDaily"],
	}, nil
}

// Logic Leaderboard
func (s *UserService) GetLeaderboard(ctx context.Context, lbType string, limit int) ([]map[string]interface{}, error) {
	users, err := s.Repo.GetAllUsers(ctx)
	if err != nil {
		return nil, err
	}

	// Sorting Logic (Go sort)
	sort.Slice(users, func(i, j int) bool {
		a := users[i]
		b := users[j]

		// Helper to get float val safely
		getVal := func(m map[string]interface{}, key string) float64 {
			if v, ok := m[key].(float64); ok {
				return v
			}
			return 0
		}

		getNested := func(m map[string]interface{}, p1, p2 string) float64 {
			if parent, ok := m[p1].(map[string]interface{}); ok {
				if v, ok := parent[p2].(float64); ok {
					return v
				}
			}
			return 0
		}

		switch lbType {
		case "money":
			return getVal(a, "money") > getVal(b, "money")
		case "level":
			return getNested(a, "rpg", "level") > getNested(b, "rpg", "level")
		case "wealth":
			wa := getVal(a, "money") + getVal(a, "diamond")
			wb := getVal(b, "money") + getVal(b, "diamond")
			return wa > wb
		}
		return false
	})

	// Limit
	if limit > len(users) {
		limit = len(users)
	}
	topUsers := users[:limit]

	// Mapping result (Select fields only)
	var result []map[string]interface{}
	for _, u := range topUsers {
		res := map[string]interface{}{
			"userId":   u["id"],
			"username": u["username"],
			"money":    u["money"],
			"diamond":  u["diamond"],
		}

		// Extract Level aman
		if rpg, ok := u["rpg"].(map[string]interface{}); ok {
			res["level"] = rpg["level"]
		} else {
			res["level"] = 0
		}

		if lbType == "wealth" {
			money, _ := u["money"].(float64)
			diamond, _ := u["diamond"].(float64)
			res["wealth"] = money + diamond
		}
		result = append(result, res)
	}

	return result, nil
}

func (s *UserService) GetAFKUsers(ctx context.Context) (map[string]interface{}, error) {
	return s.Repo.GetAFKUsers(ctx)
}
