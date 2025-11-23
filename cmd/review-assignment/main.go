package main

import (
	"log/slog"
	"os"

	"review-assignment/internal/api/pr_handler"
	"review-assignment/internal/api/team_handler"
	"review-assignment/internal/api/user_handler"
	"review-assignment/internal/config"
	"review-assignment/internal/database"
	"review-assignment/internal/lib/logger/sl"
	"review-assignment/internal/storage"

	"github.com/gin-gonic/gin"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg := config.Load()
	log.Info("starting application")

	db, err := database.New(cfg.GetDBConnString())
	if err != nil {
		log.Error("failed to connect to database", sl.Err(err))
		os.Exit(1)
	}
	defer db.Close()
	log.Info("database connected successfully")

	storage := storage.New(db)

	if err := storage.Init(); err != nil {
		log.Error("failed to init database tables", sl.Err(err))
		os.Exit(1)
	}
	teamHandler := team_handler.NewTeamHandler(storage, log.With(slog.String("handler", "team")))
	userHandler := user_handler.NewUserHandler(storage, log.With(slog.String("handler", "user")))
	prHandler := pr_handler.NewPRHandler(storage, log.With(slog.String("handler", "pr")))

	router := setupRouter(teamHandler, userHandler, prHandler)

	log.Info("server starting", slog.String("port", cfg.ServerPort))
	if err := router.Run(":" + cfg.ServerPort); err != nil {
		log.Error("server failed", sl.Err(err))
		os.Exit(1)
	}
}

func setupRouter(teamHandler *team_handler.TeamHandler, userHandler *user_handler.UserHandler, prHandler *pr_handler.PRHandler) *gin.Engine {
	router := gin.Default()

	router.GET("/health", prHandler.Health)

	router.POST("/team/add", teamHandler.CreateTeam)
	router.GET("/team/get", teamHandler.GetTeam)

	router.POST("/users/setIsActive", userHandler.SetUserActive)
	router.GET("/users/getReview", userHandler.GetUserReviews)

	router.POST("/pullRequest/create", prHandler.CreatePR)
	router.POST("/pullRequest/merge", prHandler.MergePR)
	router.POST("/pullRequest/reassign", prHandler.ReassignReviewer)

	return router
}
