package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"ai-relay/internal/config"
	"ai-relay/internal/model"
)

func main() {
	// Load .env file if present; ignore error when file is absent (e.g. in production).
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, reading config from environment")
	}

	cfg := config.Load()

	// Connect to Postgres via GORM.
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	log.Println("Database connection established")

	// Auto-migrate all models to keep the schema in sync.
	if err := db.AutoMigrate(
		&model.User{},
		&model.ApiKey{},
		&model.Channel{},
		&model.ModelConfig{},
		&model.RequestLog{},
		&model.BalanceLog{},
		&model.RedemptionCode{},
		&model.Setting{},
	); err != nil {
		log.Fatalf("AutoMigrate failed: %v", err)
	}
	log.Println("Database migration complete")

	router := gin.Default()

	// Health check endpoint.
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Suppress unused-variable warning; db will be passed to handlers in later tasks.
	_ = db

	addr := ":" + cfg.Port
	log.Printf("Starting server on %s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
