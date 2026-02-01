package main

import (
	"Berpg/internal/handler"
	"Berpg/internal/middleware"
	"Berpg/internal/repository"
	"Berpg/internal/service"
	"database/sql"
	"log/slog"
	"os"
	"time"
	_ "time/tzdata"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	echoMiddleware "github.com/labstack/echo/v4/middleware"
	_ "github.com/mattn/go-sqlite3"
	"github.com/redis/go-redis/v9"
)

func init() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		slog.Warn("File .env tidak ditemukan!")
	}

	// Set Timezone Global Aplikasi
	tz := os.Getenv("APP_TIMEZONE")
	if tz == "" {
		tz = "Asia/Jakarta" // default jakarta kalau di .env gak ada
	}

	loc, err := time.LoadLocation(tz)
	if err != nil {
		slog.Error("Gagal load timezone", "tz", tz, "err", err)
	} else {
		// Override waktu Local Go menjadi timezone yang kita mau
		time.Local = loc
		slog.Info("Timezone aplikasi diatur ke", "region", tz)
	}
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	//Database (SQLite WAL)
	db, err := sql.Open("sqlite3", "file:app.db?cache=shared&_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		panic(err)
	}
	db.SetMaxOpenConns(1) // Safe for SQLite Writer

	// Redis
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})

	// Wiring (Dependency Injection)
	userRepo := repository.NewUserRepository(db, rdb)
	statsRepo := repository.NewStatsRepository(db)
	userService := service.NewUserService(userRepo)
	userHandler := handler.NewUserHandler(userService, statsRepo)

	// Server
	e := echo.New()
	e.Use(echoMiddleware.Logger())
	e.Use(echoMiddleware.Recover())

	//Routes global nya di atur ke /api/features/rpg

	g := e.Group("/api/features/rpg")
	g.Use(middleware.TrafficLogger(statsRepo))
	g.Use(middleware.AuthMiddleware())
	{
		// semua routes di sini
		g.GET("/user/:userId", userHandler.GetUser)
		g.POST("/user/:userId", userHandler.UpdateUser)
		g.GET("/leaderboard", userHandler.GetLeaderboard)
		g.GET("/stats", userHandler.GetStats)
		g.POST("/daily/:userId", userHandler.ClaimDaily)
		g.GET("/users/afk", userHandler.GetAFKUsers)
	}

	startDailyScheduler(statsRepo)
	port := os.Getenv("PORT")
	if port == "" {
		port = "3902" // Fallback kalau di .env kosong, tapi default ini untuk server saya sendiri
	}
	e.Logger.Fatal(e.Start(":" + port))
}

// function reset stats agar tidak menumpuk
func startDailyScheduler(statsRepo *repository.StatsRepository) {
	go func() {
		for {
			now := time.Now()
			nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
			duration := nextMidnight.Sub(now)

			slog.Info("Scheduler aktif", "next_reset_in", duration.String())
			time.Sleep(duration)
			slog.Info("reset harian di mulai")
			if err := statsRepo.ResetStats(); err != nil {
				slog.Error("Gagal reset stats", "err", err)
			} else {
				slog.Info("Traffic Stats berhasil dikosongkan.")
			}
		}
	}()
}
