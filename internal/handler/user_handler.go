package handler

import (
	"Berpg/internal/repository"
	"Berpg/internal/service"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

type UserHandler struct {
	Service   *service.UserService
	StatsRepo *repository.StatsRepository
}

func NewUserHandler(s *service.UserService, stats *repository.StatsRepository) *UserHandler {
	return &UserHandler{
		Service:   s,
		StatsRepo: stats,
	}
}

// GET /user/:userId
func (h *UserHandler) GetUser(c echo.Context) error {
	userID := c.Param("userId")
	username := c.QueryParam("username")

	user, err := h.Service.GetOrInitUser(c.Request().Context(), userID, username)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"status": false, "message": "Terjadi kesalahan internal.",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":  true,
		"message": "Data untuk user " + userID + " berhasil diambil.",
		"data":    user,
	})
}

// POST /daily/:userId
func (h *UserHandler) ClaimDaily(c echo.Context) error {
	userID := c.Param("userId")

	result, err := h.Service.ClaimDaily(c.Request().Context(), userID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"status":  false,
			"message": err.Error(),
		})
	}
	return c.JSON(http.StatusOK, result)
}

// POST /user/:userId
func (h *UserHandler) UpdateUser(c echo.Context) error {
	userID := c.Param("userId")
	var body map[string]interface{}

	if err := c.Bind(&body); err != nil || len(body) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"status": false, "message": "Body request tidak boleh kosong.",
		})
	}

	err := h.Service.UpdateUser(c.Request().Context(), userID, body)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"status": false, "message": "Terjadi kesalahan internal.",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status": true, "message": "Data untuk user " + userID + " berhasil diperbarui.",
	})
}

// GET /leaderboard
func (h *UserHandler) GetLeaderboard(c echo.Context) error {
	lbType := c.QueryParam("type")
	limitStr := c.QueryParam("limit")
	if limitStr == "" {
		limitStr = "10"
	}

	if lbType == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"status": false, "message": "Parameter 'type' diperlukan (pilihan: money, level, wealth).",
		})
	}

	limit, _ := strconv.Atoi(limitStr)
	data, err := h.Service.GetLeaderboard(c.Request().Context(), lbType, limit)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"status": false, "message": "Terjadi kesalahan internal.",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status": true, "leaderboard_type": lbType, "data": data,
	})
}

// GET /stats
func (h *UserHandler) GetStats(c echo.Context) error {
	data, err := h.StatsRepo.GetHourlyStats(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"status": false, "message": "Gagal mengambil statistik",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status": true,
		"data":   data,
	})
}
