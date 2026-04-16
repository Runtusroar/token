package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
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
	// Ensure decimal values are marshaled as JSON numbers (not strings).
	decimal.MarshalJSONWithoutQuotes = true

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

	// Configure connection pool for concurrent usage.
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("Failed to get underlying sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(50)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)
	log.Println("Database connection established (pool: 50 open, 10 idle)")

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
		&model.AuditLog{},
	); err != nil {
		log.Fatalf("AutoMigrate failed: %v", err)
	}
	log.Println("Database migration complete")

	// 3b. Seed default data if tables are empty.
	seedDatabase(db)

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
		DB:             db,
		UserRepo:       userRepo,
		ChannelRepo:    channelRepo,
		ModelRepo:      modelRepo,
		RequestLogRepo: requestLogRepo,
		SettingRepo:    settingRepo,
	}
	// Shared HTTP client for upstream API calls (connection pooling + timeouts).
	upstreamClient := &http.Client{
		Timeout: 5 * time.Minute, // long timeout for streaming responses
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 20,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	proxyService := &service.ProxyService{
		ChannelService: channelService,
		BillingService: billingService,
		RequestLogRepo: requestLogRepo,
		Adapters:       map[string]adapter.Adapter{"claude": &adapter.ClaudeAdapter{HTTPClient: upstreamClient}},
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
		DB:             db,
		AdminService:   adminService,
		BillingService: billingService,
		ChannelService: channelService,
		ChannelRepo:    channelRepo,
		ModelRepo:      modelRepo,
		RedeemRepo:     redeemRepo,
		RequestLogRepo: requestLogRepo,
		BalanceLogRepo: balanceLogRepo,
		Config:         cfg,
	}
	proxyHandler := &handler.ProxyHandler{
		ProxyService: proxyService,
		ModelRepo:    modelRepo,
	}

	// 8. Set up Gin router with global middleware.
	router := gin.Default()
	router.Use(middleware.CORS(cfg.AllowedOrigins))

	// Request ID middleware — adds X-Request-ID to every response.
	router.Use(func(c *gin.Context) {
		rid := uuid.New().String()
		c.Set("request_id", rid)
		c.Header("X-Request-ID", rid)
		c.Next()
	})

	// Security headers.
	router.Use(func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Next()
	})

	// Global request body limit (10 MB).
	router.Use(func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 10<<20)
		}
		c.Next()
	})

	// 9. Register routes.

	// Health check with DB + Redis probes.
	router.GET("/health", healthHandler(sqlDB, rdb))

	// Auth routes (public, IP rate-limited).
	auth := router.Group("/api/auth")
	if rdb != nil {
		auth.Use(middleware.RateLimit(rdb, 20, 60*time.Second))
	}
	{
		auth.POST("/register", authHandler.Register)
		auth.POST("/login", authHandler.Login)
		auth.GET("/google", authHandler.GoogleRedirect)
		auth.GET("/google/callback", authHandler.GoogleCallback)
		auth.POST("/refresh", authHandler.Refresh)
	}

	// User routes (JWT auth + active check).
	user := router.Group("/api/user")
	user.Use(middleware.JWTAuth(cfg.JWTSecret), middleware.UserActiveCheck(db))
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
		user.GET("/daily-stats", userHandler.DailyStats)
	}

	// Admin routes (JWT auth + active check + AdminOnly).
	admin := router.Group("/api/admin")
	admin.Use(middleware.JWTAuth(cfg.JWTSecret), middleware.UserActiveCheck(db), middleware.AdminOnly())
	{
		admin.GET("/dashboard", adminHandler.Dashboard)
		admin.GET("/daily-stats", adminHandler.DailyStats)
		admin.GET("/users", adminHandler.ListUsers)
		admin.PUT("/users/:id", adminHandler.UpdateUser)
		admin.POST("/users/:id/topup", adminHandler.TopUp)
		admin.GET("/users/:id/balance-logs", adminHandler.UserBalanceLogs)
		admin.GET("/users/:id/request-logs", adminHandler.UserRequestLogs)
		admin.GET("/channels", adminHandler.ListChannels)
		admin.POST("/channels", adminHandler.CreateChannel)
		admin.PUT("/channels/:id", adminHandler.UpdateChannel)
		admin.DELETE("/channels/:id", adminHandler.DeleteChannel)
		admin.POST("/channels/:id/test", adminHandler.TestChannel)
		admin.GET("/models", adminHandler.ListModels)
		admin.POST("/models", adminHandler.CreateModel)
		admin.PUT("/models/:id", adminHandler.UpdateModel)
		admin.DELETE("/models/:id", adminHandler.DeleteModel)
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

	// 10. Graceful shutdown.
	addr := ":" + cfg.Port
	srv := &http.Server{Addr: addr, Handler: router}

	go func() {
		log.Printf("Starting server on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server (30s grace)...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced shutdown: %v", err)
	}
	log.Println("Server exited cleanly")
}

// seedDatabase inserts default admin, models, and settings if the tables are empty.
func seedDatabase(db *gorm.DB) {
	// Seed admin user.
	var userCount int64
	db.Model(&model.User{}).Count(&userCount)
	if userCount == 0 {
		db.Exec(`INSERT INTO users (email, password_hash, role, balance, status, created_at, updated_at)
			VALUES ('admin@relay.local', '$2a$12$erGXRlx1uoz8krEiMV9ZAO1Nxk1ZjgfTGyWwa26CTZkPtlX7cA9iu',
			'admin', 0, 'active', NOW(), NOW())`)
		log.Println("Seed: created default admin user (admin@relay.local / admin123)")
	}

	// Seed model configs.
	var modelCount int64
	db.Model(&model.ModelConfig{}).Count(&modelCount)
	if modelCount == 0 {
		db.Exec(`INSERT INTO model_configs (model_name, provider, display_name, rate, input_price, output_price, enabled, created_at, updated_at) VALUES
			('claude-opus-4', 'claude', 'Claude Opus 4', 5.0, 15.000000, 75.000000, true, NOW(), NOW()),
			('claude-sonnet-4', 'claude', 'Claude Sonnet 4', 1.0, 3.000000, 15.000000, true, NOW(), NOW()),
			('claude-haiku-4', 'claude', 'Claude Haiku 4', 0.2, 0.250000, 1.250000, true, NOW(), NOW())`)
		log.Println("Seed: created default model configs")
	}

	// Seed settings.
	var settingCount int64
	db.Model(&model.Setting{}).Count(&settingCount)
	if settingCount == 0 {
		db.Exec(`INSERT INTO settings (key, value) VALUES
			('site_name', 'AI Relay'),
			('register_enabled', 'true'),
			('default_balance', '0')`)
		log.Println("Seed: created default settings")
	}
}

// healthHandler returns a gin handler that checks DB and Redis connectivity.
func healthHandler(sqlDB *sql.DB, rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		status := gin.H{"status": "ok"}

		if err := sqlDB.Ping(); err != nil {
			status["status"] = "degraded"
			status["db"] = "down"
		} else {
			status["db"] = "ok"
		}

		if rdb != nil {
			if err := rdb.Ping(c.Request.Context()).Err(); err != nil {
				status["redis"] = "down"
			} else {
				status["redis"] = "ok"
			}
		} else {
			status["redis"] = "disabled"
		}

		code := http.StatusOK
		if status["status"] == "degraded" {
			code = http.StatusServiceUnavailable
		}
		c.JSON(code, status)
	}
}
