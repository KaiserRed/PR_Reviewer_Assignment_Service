package pr_handler

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

type PRHandler struct {
	storage *storage.Storage
	log     *slog.Logger
}

func NewPRHandler(storage *storage.Storage, log *slog.Logger) *PRHandler {
	return &PRHandler{
		storage: storage,
		log:     log,
	}
}

// CreatePR создает PR с автоматическим назначением ревьюеров
func (h *PRHandler) CreatePR(c *gin.Context) {
	const op = "handlers.pr.CreatePR"

	var req models.CreatePRRequest

	if err := c.BindJSON(&req); err != nil {
		h.log.Error("failed to bind JSON", sl.Err(err))
		c.JSON(http.StatusBadRequest, response.NewErrorResponse("INVALID_INPUT", "Invalid request body"))
		return
	}

	pr, err := h.storage.CreatePR(req)
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "AUTHOR_NOT_FOUND"):
			h.log.Warn("author not found", slog.String("author_id", req.AuthorID))
			c.JSON(http.StatusNotFound, response.NewErrorResponse("NOT_FOUND", "author not found"))
		case strings.Contains(err.Error(), "PR_EXISTS"):
			h.log.Warn("PR already exists", slog.String("pr_id", req.ID))
			c.JSON(http.StatusConflict, response.NewErrorResponse("PR_EXISTS", "PR already exists"))
		default:
			h.log.Error("failed to create PR", sl.Err(err), slog.String("pr_id", req.ID))
			c.JSON(http.StatusInternalServerError, response.NewErrorResponse("INTERNAL_ERROR", err.Error()))
		}
		return
	}

	h.log.Info("PR created successfully",
		slog.String("pr_id", req.ID),
		slog.String("author_id", req.AuthorID),
		slog.Int("reviewers_count", len(pr.AssignedReviewers)))
	c.JSON(http.StatusCreated, response.NewSuccessResponse(gin.H{"pr": pr}))
}

func (h *PRHandler) MergePR(c *gin.Context) {
	const op = "handlers.pr.MergePR"

	var req struct {
		PRID string `json:"pull_request_id" binding:"required"`
	}

	if err := c.BindJSON(&req); err != nil {
		h.log.Error("failed to bind JSON", sl.Err(err))
		c.JSON(http.StatusBadRequest, response.NewErrorResponse("INVALID_INPUT", "Invalid request body"))
		return
	}

	pr, err := h.storage.MergePR(req.PRID)
	if err != nil {
		if strings.Contains(err.Error(), "NOT_FOUND") {
			h.log.Warn("PR not found", slog.String("pr_id", req.PRID))
			c.JSON(http.StatusNotFound, response.NewErrorResponse("NOT_FOUND", "PR not found"))
			return
		}
		h.log.Error("failed to merge PR", sl.Err(err), slog.String("pr_id", req.PRID))
		c.JSON(http.StatusInternalServerError, response.NewErrorResponse("INTERNAL_ERROR", err.Error()))
		return
	}

	h.log.Info("PR merged successfully", slog.String("pr_id", req.PRID))
	c.JSON(http.StatusOK, response.NewSuccessResponse(gin.H{"pr": pr}))
}

func (h *PRHandler) ReassignReviewer(c *gin.Context) {
	const op = "handlers.pr.ReassignReviewer"

	var req models.ReassignRequest

	if err := c.BindJSON(&req); err != nil {
		h.log.Error("failed to bind JSON", sl.Err(err))
		c.JSON(http.StatusBadRequest, response.NewErrorResponse("INVALID_INPUT", "Invalid request body"))
		return
	}

	pr, newReviewer, err := h.storage.ReassignReviewer(req)
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "NOT_FOUND"):
			h.log.Warn("PR or user not found",
				slog.String("pr_id", req.PRID),
				slog.String("old_reviewer", req.OldReviewer))
			c.JSON(http.StatusNotFound, response.NewErrorResponse("NOT_FOUND", "PR or user not found"))
		case strings.Contains(err.Error(), "PR_MERGED"):
			h.log.Warn("cannot reassign on merged PR", slog.String("pr_id", req.PRID))
			c.JSON(http.StatusConflict, response.NewErrorResponse("PR_MERGED", "cannot reassign on merged PR"))
		case strings.Contains(err.Error(), "NOT_ASSIGNED"):
			h.log.Warn("reviewer not assigned to PR",
				slog.String("pr_id", req.PRID),
				slog.String("old_reviewer", req.OldReviewer))
			c.JSON(http.StatusConflict, response.NewErrorResponse("NOT_ASSIGNED", "reviewer is not assigned to this PR"))
		case strings.Contains(err.Error(), "NO_CANDIDATE"):
			h.log.Warn("no replacement candidate found",
				slog.String("pr_id", req.PRID),
				slog.String("old_reviewer", req.OldReviewer))
			c.JSON(http.StatusConflict, response.NewErrorResponse("NO_CANDIDATE", "no active replacement candidate in team"))
		default:
			h.log.Error("failed to reassign reviewer", sl.Err(err),
				slog.String("pr_id", req.PRID),
				slog.String("old_reviewer", req.OldReviewer))
			c.JSON(http.StatusInternalServerError, response.NewErrorResponse("INTERNAL_ERROR", err.Error()))
		}
		return
	}

	h.log.Info("reviewer reassigned successfully",
		slog.String("pr_id", req.PRID),
		slog.String("old_reviewer", req.OldReviewer),
		slog.String("new_reviewer", newReviewer))
	c.JSON(http.StatusOK, response.NewSuccessResponse(gin.H{
		"pr":          pr,
		"replaced_by": newReviewer,
	}))
}

// Health check
func (h *PRHandler) Health(c *gin.Context) {
	h.log.Debug("health check requested")
	c.JSON(http.StatusOK, response.HealthResponse())
}
