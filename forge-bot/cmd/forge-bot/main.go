package main

import (
	"log/slog"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/shulex/forge/forge-bot/internal/card"
	"github.com/shulex/forge/forge-bot/internal/forgeapi"
	"github.com/shulex/forge/forge-bot/internal/handler"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	port := os.Getenv("BOT_PORT")
	if port == "" {
		port = "8085"
	}

	forgeURL := os.Getenv("FORGE_API_URL")
	if forgeURL == "" {
		forgeURL = "http://localhost:8080"
	}

	dtToken := os.Getenv("DINGTALK_TOKEN")
	dtSecret := os.Getenv("DINGTALK_SECRET")
	dtWebhook := os.Getenv("DINGTALK_WEBHOOK")

	forgeClient := forgeapi.NewClient(forgeURL)
	cardRenderer := card.NewRenderer()

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	h := handler.NewDingTalkHandler(dtToken, dtSecret, dtWebhook, forgeClient, cardRenderer)

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "forge-bot"})
	})

	// DingTalk webhook receiver
	r.POST("/dingtalk/webhook", h.HandleWebhook)

	// DingTalk interactive card callback
	r.POST("/dingtalk/callback", h.HandleCardCallback)

	slog.Info("forge-bot starting", "port", port, "forge_api", forgeURL)
	if err := r.Run(":" + port); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
