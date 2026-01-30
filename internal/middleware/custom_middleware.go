package middleware

import (
	"Berpg/internal/repository"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
)

// Auth Middleware
func AuthMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {

			clientKey := c.Request().Header.Get("x-api-key")
			serverKey := os.Getenv("API_KEY")

			if serverKey == "" {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Server misconfiguration: API_KEY not set"})
			}

			if clientKey != serverKey {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "Akses Ditolak: API Key Salah atau Tidak Ada",
				})
			}

			return next(c)
		}
	}
}

// Traffic Logger Middleware
func TrafficLogger(repo *repository.StatsRepository) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.Path() == "/api/features/rpg/stats" || c.Path() == "/favicon.ico" {
				return next(c)
			}
			repo.LogTraffic(c.Request().Method)

			return next(c)
		}
	}
}
