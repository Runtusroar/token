package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"ai-relay/internal/adapter"
	"ai-relay/internal/config"
	"ai-relay/internal/handler"
	"ai-relay/internal/middleware"
	"ai-relay/internal/model"
	"ai-relay/internal/repository"
	"ai-relay/internal/service"
)

func main() {
	// Load .env file if present; ignore error when file is absent (e.g. in production).
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, reading config from environment")
	}

	// 1. Load config.
	cfg := config.Load()

	// 2. Connect to PostgreSQL via GORM.
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	log.Println("Database connection established")

	// 3. AutoMigrate all 8 models.
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

	// 4. Connect to Redis (graceful: if fails, log warning and set rdb=nil).
	var rdb *redis.Client
	redisOpts, redisErr := redis.ParseURL(cfg.RedisURL)
	if redisErr != nil {
		log.Printf("Warning: failed to parse Redis URL: %v — rate limiting disabled", redisErr)
	} else {
		rdb = redis.NewClient(redisOpts)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if pingErr := rdb.Ping(ctx).Err(); pingErr != nil {
			log.Printf("Warning: Redis unavailable: %v — rate limiting disabled", pingErr)
			rdb = nil
		} else {
			log.Println("Redis connection established")
		}
	}

	// 5. Create all repository instances.
	userRepo := &repository.UserRepo{DB: db}
	apiKeyRepo := &repository.ApiKeyRepo{DB: db}
	channelRepo := &repository.ChannelRepo{DB: db}
	modelRepo := &repository.ModelConfigRepo{DB: db}
	requestLogRepo := &repository.RequestLogRepo{DB: db}
	balanceLogRepo := &repository.BalanceLogRepo{DB: db}
	redeemRepo := &repository.RedemptionCodeRepo{DB: db}
	settingRepo := &repository.SettingRepo{DB: db}

	// 6. Create all service instances.
	authService := &service.AuthService{
		UserRepo:    userRepo,
		SettingRepo: settingRepo,
		Config:      cfg,
	}
	userService := &service.UserService{
		UserRepo:       userRepo,
		RequestLogRepo: requestLogRepo,
	}
	apiKeyService := &service.ApiKeyService{
		Repo: apiKeyRepo,
	}
	channelService := &service.ChannelService{
		Repo:   channelRepo,
		Config: cfg,
	}
	billingService := &service.BillingService{
		DB:             db,
		UserRepo:       userRepo,
		BalanceLogRepo: balanceLogRepo,
		RedeemRepo:     redeemRepo,
		ModelRepo:      modelRepo,
		RequestLogRepo: requestLogRepo,
	}
	adminService := &service.AdminService{
		UserRepo:       userRepo,
		ChannelRepo:    channelRepo,
		ModelRepo:      modelRepo,
		RequestLogRepo: requestLogRepo,
		SettingRepo:    settingRepo,
	}
	proxyService := &service.ProxyService{
		ChannelService: channelService,
		BillingService: billingService,
		RequestLogRepo: requestLogRepo,
		Adapters:       map[string]adapter.Adapter{"claude": &adapter.ClaudeAdapter{}},
	}

	// 7. Create all handler instances.
	authHandler := &handler.AuthHandler{
		AuthService: authService,
	}
	userHandler := &handler.UserHandler{
		UserService:    userService,
		ApiKeyService:  apiKeyService,
		BillingService: billingService,
		RequestLogRepo: requestLogRepo,
		BalanceLogRepo: balanceLogRepo,
	}
	adminHandler := &handler.AdminHandler{
		AdminService:   adminService,
		BillingService: billingService,
		ChannelRepo:    channelRepo,
		ModelRepo:      modelRepo,
		RedeemRepo:     redeemRepo,
		RequestLogRepo: requestLogRepo,
		Config:         cfg,
	}
	proxyHandler := &handler.ProxyHandler{
		ProxyService: proxyService,
		ModelRepo:    modelRepo,
	}

	// 8. Set up Gin router with CORS middleware.
	router := gin.Default()
	router.Use(middleware.CORS(cfg.AllowedOrigins))

	// 9. Register routes.

	// Health check (public).
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Auth routes (public).
	auth := router.Group("/api/auth")
	{
		auth.POST("/register", authHandler.Register)
		auth.POST("/login", authHandler.Login)
		auth.GET("/google", authHandler.GoogleRedirect)
		auth.GET("/google/callback", authHandler.GoogleCallback)
		auth.POST("/refresh", authHandler.Refresh)
	}

	// User routes (JWT auth).
	user := router.Group("/api/user")
	user.Use(middleware.JWTAuth(cfg.JWTSecret))
	{
		user.GET("/profile", userHandler.GetProfile)
		user.PUT("/password", userHandler.ChangePassword)
		user.GET("/api-keys", userHandler.ListApiKeys)
		user.POST("/api-keys", userHandler.CreateApiKey)
		user.DELETE("/api-keys/:id", userHandler.DeleteApiKey)
		user.PUT("/api-keys/:id", userHandler.UpdateApiKey)
		user.GET("/logs", userHandler.ListLogs)
		user.GET("/balance-logs", userHandler.ListBalanceLogs)
		user.POST("/redeem", userHandler.Redeem)
		user.GET("/dashboard", userHandler.Dashboard)
	}

	// Admin routes (JWT auth + AdminOnly).
	admin := router.Group("/api/admin")
	admin.Use(middleware.JWTAuth(cfg.JWTSecret), middleware.AdminOnly())
	{
		admin.GET("/dashboard", adminHandler.Dashboard)
		admin.GET("/users", adminHandler.ListUsers)
		admin.PUT("/users/:id", adminHandler.UpdateUser)
		admin.POST("/users/:id/topup", adminHandler.TopUp)
		admin.GET("/channels", adminHandler.ListChannels)
		admin.POST("/channels", adminHandler.CreateChannel)
		admin.PUT("/channels/:id", adminHandler.UpdateChannel)
		admin.DELETE("/channels/:id", adminHandler.DeleteChannel)
		admin.POST("/channels/:id/test", adminHandler.TestChannel)
		admin.GET("/models", adminHandler.ListModels)
		admin.POST("/models", adminHandler.CreateModel)
		admin.PUT("/models/:id", adminHandler.UpdateModel)
		admin.GET("/redeem-codes", adminHandler.ListRedeemCodes)
		admin.POST("/redeem-codes", adminHandler.CreateRedeemCodes)
		admin.PUT("/redeem-codes/:id", adminHandler.UpdateRedeemCode)
		admin.GET("/logs", adminHandler.ListLogs)
		admin.GET("/settings", adminHandler.GetSettings)
		admin.PUT("/settings", adminHandler.UpdateSettings)
	}

	// Proxy routes (API key auth + optional rate limit).
	v1 := router.Group("/v1")
	v1.Use(middleware.APIKeyAuth(db))
	if rdb != nil {
		v1.Use(middleware.RateLimit(rdb, 60, 60*time.Second))
	}
	{
		v1.POST("/messages", proxyHandler.NativeMessages)
		v1.POST("/chat/completions", proxyHandler.ChatCompletions)
		v1.GET("/models", proxyHandler.ListModels)
	}

	addr := ":" + cfg.Port
	log.Printf("Starting server on %s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
