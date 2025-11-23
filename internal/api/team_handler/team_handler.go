package team_handler

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

type TeamHandler struct {
	storage *storage.Storage
	log     *slog.Logger
}

func NewTeamHandler(storage *storage.Storage, log *slog.Logger) *TeamHandler {
	return &TeamHandler{
		storage: storage,
		log:     log,
	}
}

func (h *TeamHandler) CreateTeam(c *gin.Context) {
	const op = "handlers.team.CreateTeam"

	var req models.CreateTeamRequest

	if err := c.BindJSON(&req); err != nil {
		h.log.Error("failed to bind JSON", sl.Err(err))
		c.JSON(http.StatusBadRequest, response.NewErrorResponse("INVALID_INPUT", "Invalid request body"))
		return
	}

	team := models.Team{
		Name:    req.Name,
		Members: req.Members,
	}

	if err := h.storage.CreateTeam(team); err != nil {
		if strings.Contains(err.Error(), "TEAM_EXISTS") {
			h.log.Warn("team already exists", slog.String("team_name", req.Name))
			c.JSON(http.StatusBadRequest, response.NewErrorResponse("TEAM_EXISTS", "team already exists"))
			return
		}
		h.log.Error("failed to create team", sl.Err(err))
		c.JSON(http.StatusInternalServerError, response.NewErrorResponse("INTERNAL_ERROR", err.Error()))
		return
	}

	h.log.Info("team created successfully", slog.String("team_name", req.Name))
	c.JSON(http.StatusCreated, response.NewSuccessResponse(gin.H{"team": team}))
}

func (h *TeamHandler) GetTeam(c *gin.Context) {
	const op = "handlers.team.GetTeam"

	teamName := c.Query("team_name")
	if teamName == "" {
		h.log.Warn("team_name parameter is missing")
		c.JSON(http.StatusBadRequest, response.NewErrorResponse("INVALID_INPUT", "team_name parameter is required"))
		return
	}

	team, err := h.storage.GetTeam(teamName)
	if err != nil {
		if strings.Contains(err.Error(), "NOT_FOUND") {
			h.log.Warn("team not found", slog.String("team_name", teamName))
			c.JSON(http.StatusNotFound, response.NewErrorResponse("NOT_FOUND", "team not found"))
			return
		}
		h.log.Error("failed to get team", sl.Err(err), slog.String("team_name", teamName))
		c.JSON(http.StatusInternalServerError, response.NewErrorResponse("INTERNAL_ERROR", err.Error()))
		return
	}

	h.log.Debug("team retrieved successfully", slog.String("team_name", teamName))
	c.JSON(http.StatusOK, response.NewSuccessResponse(team))
}
