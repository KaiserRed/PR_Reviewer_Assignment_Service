package user_handler

import (
	"net/http"
	"strings"

	"review-assignment/internal/lib/http/response"
	"review-assignment/internal/lib/logger/sl"
	"review-assignment/internal/models"
	"review-assignment/internal/storage"

	"log/slog"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	storage *storage.Storage
	log     *slog.Logger
}

func NewUserHandler(storage *storage.Storage, log *slog.Logger) *UserHandler {
	return &UserHandler{
		storage: storage,
		log:     log,
	}
}

func (h *UserHandler) SetUserActive(c *gin.Context) {
	const op = "handlers.user.SetUserActive"

	var req struct {
		UserID   string `json:"user_id" binding:"required"`
		IsActive bool   `json:"is_active"`
	}

	if err := c.BindJSON(&req); err != nil {
		h.log.Error("failed to bind JSON",
			sl.Err(err),
			slog.Any("request_body", req))
		c.JSON(http.StatusBadRequest, response.NewErrorResponse("INVALID_INPUT", "Invalid request body"))
		return
	}

	user, err := h.storage.SetUserActive(req.UserID, req.IsActive)
	if err != nil {
		if strings.Contains(err.Error(), "NOT_FOUND") {
			h.log.Warn("user not found", slog.String("user_id", req.UserID))
			c.JSON(http.StatusNotFound, response.NewErrorResponse("NOT_FOUND", "user not found"))
			return
		}
		h.log.Error("failed to set user active", sl.Err(err), slog.String("user_id", req.UserID))
		c.JSON(http.StatusInternalServerError, response.NewErrorResponse("INTERNAL_ERROR", err.Error()))
		return
	}

	h.log.Info("user activity updated",
		slog.String("user_id", req.UserID),
		slog.Bool("is_active", req.IsActive))
	c.JSON(http.StatusOK, response.NewSuccessResponse(gin.H{"user": user}))
}

func (h *UserHandler) GetUserReviews(c *gin.Context) {
	const op = "handlers.user.GetUserReviews"

	userID := c.Query("user_id")
	if userID == "" {
		h.log.Warn("user_id parameter is missing")
		c.JSON(http.StatusBadRequest, response.NewErrorResponse("INVALID_INPUT", "user_id parameter is required"))
		return
	}

	prs, err := h.storage.GetUserReviews(userID)
	if err != nil {
		if strings.Contains(err.Error(), "NOT_FOUND") {
			h.log.Warn("user not found", slog.String("user_id", userID))
			c.JSON(http.StatusNotFound, response.NewErrorResponse("NOT_FOUND", "user not found"))
			return
		}
		h.log.Error("failed to get user reviews", sl.Err(err), slog.String("user_id", userID))
		c.JSON(http.StatusInternalServerError, response.NewErrorResponse("INTERNAL_ERROR", err.Error()))
		return
	}

	prsShort := make([]models.PullRequestShort, len(prs))
	for i, pr := range prs {
		prsShort[i] = models.PullRequestShort{
			ID:       pr.ID,
			Name:     pr.Name,
			AuthorID: pr.AuthorID,
			Status:   pr.Status,
		}
	}

	h.log.Debug("user reviews retrieved",
		slog.String("user_id", userID),
		slog.Int("pr_count", len(prsShort)))
	c.JSON(http.StatusOK, response.NewSuccessResponse(gin.H{
		"user_id":       userID,
		"pull_requests": prsShort,
	}))
}
