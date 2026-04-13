# AI Token Relay Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build an AI API Token relay platform where users purchase credits and proxy requests to upstream AI providers (starting with Claude), with admin and user dashboards.

**Architecture:** Go (Gin) backend handles API proxying, auth, billing. React (Vite) frontend serves two SPAs: admin panel and user panel. PostgreSQL persists all data, Redis handles rate limiting and caching. Docker Compose orchestrates local deployment.

**Tech Stack:** Go 1.22+, Gin, GORM, PostgreSQL 16, Redis 7, React 18, Vite, Ant Design, TypeScript, Docker

**Spec:** `docs/superpowers/specs/2026-04-13-ai-token-relay-design.md`

---

## File Map

### Backend (`backend/`)

| File | Responsibility |
|------|---------------|
| `cmd/server/main.go` | Entry point: load config, connect DB/Redis, register routes, start server |
| `internal/config/config.go` | Load config from env/yaml, expose typed Config struct |
| `internal/model/user.go` | User GORM model |
| `internal/model/api_key.go` | ApiKey GORM model |
| `internal/model/channel.go` | Channel GORM model |
| `internal/model/model_config.go` | ModelConfig GORM model |
| `internal/model/request_log.go` | RequestLog GORM model |
| `internal/model/balance_log.go` | BalanceLog GORM model |
| `internal/model/redemption_code.go` | RedemptionCode GORM model |
| `internal/model/setting.go` | System setting GORM model (key-value) |
| `internal/repository/user.go` | User DB queries |
| `internal/repository/api_key.go` | ApiKey DB queries |
| `internal/repository/channel.go` | Channel DB queries |
| `internal/repository/model_config.go` | ModelConfig DB queries |
| `internal/repository/request_log.go` | RequestLog DB queries |
| `internal/repository/balance_log.go` | BalanceLog DB queries |
| `internal/repository/redemption_code.go` | RedemptionCode DB queries |
| `internal/repository/setting.go` | Setting DB queries |
| `internal/service/auth.go` | Register, login, Google OAuth, JWT refresh |
| `internal/service/user.go` | Profile, password change, dashboard stats |
| `internal/service/api_key.go` | API key CRUD, generate sk- keys |
| `internal/service/channel.go` | Channel CRUD, selection with priority/weight, health check |
| `internal/service/billing.go` | Balance deduct, topup, redeem code, balance logs |
| `internal/service/proxy.go` | Route request to adapter, handle streaming, trigger billing |
| `internal/service/admin.go` | Admin dashboard stats, user mgmt, settings, redeem codes |
| `internal/handler/auth.go` | Auth HTTP handlers (register, login, google, refresh) |
| `internal/handler/user.go` | User HTTP handlers (profile, api-keys, logs, redeem, dashboard) |
| `internal/handler/admin.go` | Admin HTTP handlers (all admin CRUD endpoints) |
| `internal/handler/proxy.go` | Proxy HTTP handlers (/v1/messages, /v1/chat/completions, /v1/models) |
| `internal/middleware/auth.go` | JWT auth middleware + API key auth middleware |
| `internal/middleware/ratelimit.go` | Redis token bucket rate limiter |
| `internal/middleware/cors.go` | CORS configuration |
| `internal/middleware/logger.go` | Request logging middleware |
| `internal/adapter/types.go` | Common adapter interfaces and types |
| `internal/adapter/claude.go` | Claude Messages API adapter |
| `internal/adapter/converter.go` | OpenAI ↔ Claude format converter |
| `internal/pkg/jwt.go` | JWT sign/verify with RS256 |
| `internal/pkg/crypto.go` | AES-256-GCM encrypt/decrypt, API key generation, bcrypt helpers |
| `internal/pkg/response.go` | Standardized JSON response helpers |
| `migration/000001_init.sql` | Initial schema migration |
| `migration/seed.sql` | Seed admin user + default model configs |

### Frontend (`frontend/`)

| File | Responsibility |
|------|---------------|
| `src/main.tsx` | React entry, router setup |
| `src/App.tsx` | Root component with route config |
| `src/api/client.ts` | Axios instance with JWT interceptor |
| `src/api/auth.ts` | Auth API calls |
| `src/api/user.ts` | User API calls |
| `src/api/admin.ts` | Admin API calls |
| `src/store/auth.ts` | Zustand auth store (user, token, login/logout) |
| `src/styles/global.css` | Global styles: fonts, colors, terminal theme |
| `src/styles/theme.ts` | Ant Design theme override (no border-radius, monospace) |
| `src/layouts/AdminLayout.tsx` | Admin sidebar + content layout |
| `src/layouts/UserLayout.tsx` | User sidebar + content layout |
| `src/components/StatCard.tsx` | Terminal-style stat card component |
| `src/components/DataTable.tsx` | Styled table wrapper |
| `src/components/CodeBlock.tsx` | Terminal-style code display |
| `src/pages/auth/Login.tsx` | Login page (email + Google) |
| `src/pages/auth/Register.tsx` | Register page |
| `src/pages/user/Overview.tsx` | User dashboard: balance, usage, quick start |
| `src/pages/user/ApiKeys.tsx` | API key management |
| `src/pages/user/UsageLogs.tsx` | Request history |
| `src/pages/user/TopUp.tsx` | Redeem code input |
| `src/pages/user/Balance.tsx` | Balance transaction log |
| `src/pages/user/Docs.tsx` | Client integration guide |
| `src/pages/user/Settings.tsx` | Password change, Google bind |
| `src/pages/admin/Dashboard.tsx` | Admin stats, charts, activity |
| `src/pages/admin/Users.tsx` | User list, search, ban, topup |
| `src/pages/admin/Channels.tsx` | Channel CRUD, test connectivity |
| `src/pages/admin/Models.tsx` | Model config, rate, pricing |
| `src/pages/admin/RedeemCodes.tsx` | Generate/manage redeem codes |
| `src/pages/admin/Logs.tsx` | Global request logs |
| `src/pages/admin/Settings.tsx` | Site settings |

### Infrastructure

| File | Responsibility |
|------|---------------|
| `docker-compose.yml` | Orchestrate all services |
| `backend/Dockerfile` | Multi-stage Go build |
| `frontend/Dockerfile` | Build React + serve with Nginx |
| `frontend/nginx.conf` | Nginx config: serve SPA + proxy API |
| `Makefile` | Dev commands: run, build, migrate, seed |
| `.env.example` | Environment variable template |

---

## Phase 1: Project Scaffolding & Infrastructure

### Task 1: Initialize Go Backend Project

**Files:**
- Create: `backend/cmd/server/main.go`
- Create: `backend/go.mod`
- Create: `backend/internal/config/config.go`
- Create: `backend/internal/pkg/response.go`
- Create: `.env.example`

- [ ] **Step 1: Initialize Go module and install dependencies**

```bash
cd /home/colorful/Documents/claude/token
mkdir -p backend/cmd/server backend/internal/config backend/internal/pkg
cd backend
go mod init ai-relay
go get github.com/gin-gonic/gin@v1.10.0
go get gorm.io/gorm@v1.25.12
go get gorm.io/driver/postgres@v1.5.11
go get github.com/redis/go-redis/v9@v9.7.3
go get github.com/joho/godotenv@v1.5.1
go get golang.org/x/crypto@latest
go get github.com/golang-jwt/jwt/v5@v5.2.2
```

- [ ] **Step 2: Create config loader**

Create `backend/internal/config/config.go`:

```go
package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port            string
	DatabaseURL     string
	RedisURL        string
	JWTSecret       string
	EncryptionKey   string
	GoogleClientID  string
	GoogleSecret    string
	GoogleCallback  string
	AllowedOrigins  string
	DefaultBalance  float64
	RegisterEnabled bool
}

func Load() *Config {
	godotenv.Load()
	defaultBalance, _ := strconv.ParseFloat(getEnv("DEFAULT_BALANCE", "0"), 64)
	return &Config{
		Port:            getEnv("PORT", "8080"),
		DatabaseURL:     getEnv("DATABASE_URL", "postgres://relay:relay@localhost:5432/relay?sslmode=disable"),
		RedisURL:        getEnv("REDIS_URL", "redis://localhost:6379/0"),
		JWTSecret:       getEnv("JWT_SECRET", "change-me-in-production"),
		EncryptionKey:   getEnv("ENCRYPTION_KEY", "change-me-32-byte-key-for-aes!!"),
		GoogleClientID:  getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleSecret:    getEnv("GOOGLE_CLIENT_SECRET", ""),
		GoogleCallback:  getEnv("GOOGLE_CALLBACK_URL", "http://localhost:8080/api/auth/google/callback"),
		AllowedOrigins:  getEnv("ALLOWED_ORIGINS", "http://localhost:5173"),
		DefaultBalance:  defaultBalance,
		RegisterEnabled: getEnv("REGISTER_ENABLED", "true") == "true",
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
```

- [ ] **Step 3: Create response helpers**

Create `backend/internal/pkg/response.go`:

```go
package pkg

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Response{Success: true, Data: data})
}

func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, Response{Success: true, Data: data})
}

func Fail(c *gin.Context, status int, msg string) {
	c.JSON(status, Response{Success: false, Error: msg})
}

func BadRequest(c *gin.Context, msg string) {
	Fail(c, http.StatusBadRequest, msg)
}

func Unauthorized(c *gin.Context, msg string) {
	Fail(c, http.StatusUnauthorized, msg)
}

func Forbidden(c *gin.Context, msg string) {
	Fail(c, http.StatusForbidden, msg)
}

func NotFound(c *gin.Context, msg string) {
	Fail(c, http.StatusNotFound, msg)
}

func InternalError(c *gin.Context, msg string) {
	Fail(c, http.StatusInternalServerError, msg)
}
```

- [ ] **Step 4: Create minimal main.go**

Create `backend/cmd/server/main.go`:

```go
package main

import (
	"log"

	"ai-relay/internal/config"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()

	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	log.Printf("Starting server on :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
```

- [ ] **Step 5: Create .env.example**

Create `.env.example` at project root:

```
PORT=8080
DATABASE_URL=postgres://relay:relay@localhost:5432/relay?sslmode=disable
REDIS_URL=redis://localhost:6379/0
JWT_SECRET=change-me-in-production
ENCRYPTION_KEY=change-me-32-byte-key-for-aes!!
GOOGLE_CLIENT_ID=
GOOGLE_CLIENT_SECRET=
GOOGLE_CALLBACK_URL=http://localhost:8080/api/auth/google/callback
ALLOWED_ORIGINS=http://localhost:5173
DEFAULT_BALANCE=0
REGISTER_ENABLED=true
```

- [ ] **Step 6: Verify the server starts**

```bash
cd /home/colorful/Documents/claude/token/backend
go run cmd/server/main.go &
sleep 2
curl http://localhost:8080/health
# Expected: {"status":"ok"}
kill %1
```

- [ ] **Step 7: Commit**

```bash
git add backend/ .env.example
git commit -m "feat: scaffold Go backend with config and health endpoint"
```

---

### Task 2: Database Models & Migration

**Files:**
- Create: `backend/internal/model/user.go`
- Create: `backend/internal/model/api_key.go`
- Create: `backend/internal/model/channel.go`
- Create: `backend/internal/model/model_config.go`
- Create: `backend/internal/model/request_log.go`
- Create: `backend/internal/model/balance_log.go`
- Create: `backend/internal/model/redemption_code.go`
- Create: `backend/internal/model/setting.go`
- Create: `backend/migration/000001_init.sql`
- Create: `backend/migration/seed.sql`

- [ ] **Step 1: Create all GORM models**

Create `backend/internal/model/user.go`:

```go
package model

import (
	"time"

	"github.com/shopspring/decimal"
)

type User struct {
	ID           int64           `gorm:"primaryKey;autoIncrement" json:"id"`
	Email        string          `gorm:"type:varchar(255);uniqueIndex;not null" json:"email"`
	PasswordHash string          `gorm:"type:varchar(255)" json:"-"`
	GoogleID     string          `gorm:"type:varchar(255);index" json:"-"`
	Role         string          `gorm:"type:varchar(20);default:user;not null" json:"role"`
	Balance      decimal.Decimal `gorm:"type:decimal(12,4);default:0;not null" json:"balance"`
	Status       string          `gorm:"type:varchar(20);default:active;not null" json:"status"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}
```

Create `backend/internal/model/api_key.go`:

```go
package model

import "time"

type ApiKey struct {
	ID         int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID     int64      `gorm:"not null;index" json:"user_id"`
	Key        string     `gorm:"type:varchar(255);uniqueIndex;not null" json:"key"`
	Name       string     `gorm:"type:varchar(100);not null" json:"name"`
	Status     string     `gorm:"type:varchar(20);default:active;not null" json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	User       User       `gorm:"foreignKey:UserID" json:"-"`
}
```

Create `backend/internal/model/channel.go`:

```go
package model

import (
	"time"

	"gorm.io/datatypes"
)

type Channel struct {
	ID        int64          `gorm:"primaryKey;autoIncrement" json:"id"`
	Name      string         `gorm:"type:varchar(100);not null" json:"name"`
	Type      string         `gorm:"type:varchar(50);not null" json:"type"`
	ApiKey    string         `gorm:"type:varchar(500);not null" json:"-"`
	BaseURL   string         `gorm:"type:varchar(500);not null" json:"base_url"`
	Models    datatypes.JSON `gorm:"type:jsonb;not null" json:"models"`
	Status    string         `gorm:"type:varchar(20);default:active;not null" json:"status"`
	Priority  int            `gorm:"default:0;not null" json:"priority"`
	Weight    int            `gorm:"default:1;not null" json:"weight"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}
```

Create `backend/internal/model/model_config.go`:

```go
package model

import (
	"time"

	"github.com/shopspring/decimal"
)

type ModelConfig struct {
	ID          int64           `gorm:"primaryKey;autoIncrement" json:"id"`
	ModelName   string          `gorm:"type:varchar(100);uniqueIndex;not null" json:"model_name"`
	DisplayName string          `gorm:"type:varchar(100);not null" json:"display_name"`
	Rate        decimal.Decimal `gorm:"type:decimal(6,2);default:1.00;not null" json:"rate"`
	InputPrice  decimal.Decimal `gorm:"type:decimal(10,6);not null" json:"input_price"`
	OutputPrice decimal.Decimal `gorm:"type:decimal(10,6);not null" json:"output_price"`
	Enabled     bool            `gorm:"default:true;not null" json:"enabled"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}
```

Create `backend/internal/model/request_log.go`:

```go
package model

import (
	"time"

	"github.com/shopspring/decimal"
)

type RequestLog struct {
	ID               int64           `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID           int64           `gorm:"not null;index" json:"user_id"`
	ApiKeyID         int64           `gorm:"not null" json:"api_key_id"`
	ChannelID        int64           `gorm:"not null;index" json:"channel_id"`
	Model            string          `gorm:"type:varchar(100);not null" json:"model"`
	Type             string          `gorm:"type:varchar(20);not null" json:"type"`
	PromptTokens     int             `gorm:"default:0;not null" json:"prompt_tokens"`
	CompletionTokens int             `gorm:"default:0;not null" json:"completion_tokens"`
	TotalTokens      int             `gorm:"default:0;not null" json:"total_tokens"`
	Cost             decimal.Decimal `gorm:"type:decimal(10,6);default:0;not null" json:"cost"`
	UpstreamCost     decimal.Decimal `gorm:"type:decimal(10,6);default:0;not null" json:"upstream_cost"`
	Status           string          `gorm:"type:varchar(20);not null" json:"status"`
	DurationMs       int             `gorm:"default:0;not null" json:"duration_ms"`
	IPAddress        string          `gorm:"type:varchar(45)" json:"ip_address"`
	CreatedAt        time.Time       `gorm:"index" json:"created_at"`
}
```

Create `backend/internal/model/balance_log.go`:

```go
package model

import (
	"time"

	"github.com/shopspring/decimal"
)

type BalanceLog struct {
	ID           int64           `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID       int64           `gorm:"not null;index" json:"user_id"`
	Type         string          `gorm:"type:varchar(20);not null" json:"type"`
	Amount       decimal.Decimal `gorm:"type:decimal(12,4);not null" json:"amount"`
	BalanceAfter decimal.Decimal `gorm:"type:decimal(12,4);not null" json:"balance_after"`
	Description  string          `gorm:"type:varchar(500)" json:"description"`
	RequestLogID *int64          `json:"request_log_id"`
	CreatedAt    time.Time       `json:"created_at"`
}
```

Create `backend/internal/model/redemption_code.go`:

```go
package model

import "time"

type RedemptionCode struct {
	ID        int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	Code      string     `gorm:"type:varchar(50);uniqueIndex;not null" json:"code"`
	Amount    float64    `gorm:"type:decimal(12,4);not null" json:"amount"`
	Status    string     `gorm:"type:varchar(20);default:unused;not null" json:"status"`
	UsedBy    *int64     `json:"used_by"`
	UsedAt    *time.Time `json:"used_at"`
	CreatedBy int64      `gorm:"not null" json:"created_by"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at"`
}
```

Create `backend/internal/model/setting.go`:

```go
package model

type Setting struct {
	Key   string `gorm:"primaryKey;type:varchar(100)" json:"key"`
	Value string `gorm:"type:text;not null" json:"value"`
}
```

- [ ] **Step 2: Install decimal and datatypes dependencies**

```bash
cd /home/colorful/Documents/claude/token/backend
go get github.com/shopspring/decimal@v1.4.0
go get gorm.io/datatypes@v1.2.5
```

- [ ] **Step 3: Create SQL migration**

Create `backend/migration/000001_init.sql`:

```sql
-- Composite index for request_logs
CREATE INDEX IF NOT EXISTS idx_request_logs_user_created
ON request_logs (user_id, created_at DESC);
```

Note: GORM AutoMigrate creates tables and basic indexes from struct tags. This file adds only the composite index that GORM can't express via tags.

- [ ] **Step 4: Create seed data**

Create `backend/migration/seed.sql`:

```sql
-- Default admin user (password: admin123)
-- bcrypt hash generated with cost 12
INSERT INTO users (email, password_hash, role, balance, status, created_at, updated_at)
VALUES ('admin@relay.local', '$2a$12$LJ3m4ys3Lk0TSwMCfNBP4e5VGrHFGpAqNqgPTKCjVJKLNEB7A.hZe', 'admin', 0, 'active', NOW(), NOW())
ON CONFLICT (email) DO NOTHING;

-- Default Claude model configs
INSERT INTO model_configs (model_name, display_name, rate, input_price, output_price, enabled, created_at, updated_at)
VALUES
  ('claude-opus-4-20250514', 'Claude Opus 4', 5.00, 15.000000, 75.000000, true, NOW(), NOW()),
  ('claude-sonnet-4-20250514', 'Claude Sonnet 4', 1.00, 3.000000, 15.000000, true, NOW(), NOW()),
  ('claude-haiku-4-20250414', 'Claude Haiku 4', 0.20, 0.250000, 1.250000, true, NOW(), NOW())
ON CONFLICT (model_name) DO NOTHING;

-- Default settings
INSERT INTO settings (key, value) VALUES
  ('site_name', 'AI Relay'),
  ('register_enabled', 'true'),
  ('default_balance', '0')
ON CONFLICT (key) DO NOTHING;
```

- [ ] **Step 5: Wire up DB connection and AutoMigrate in main.go**

Update `backend/cmd/server/main.go`:

```go
package main

import (
	"log"

	"ai-relay/internal/config"
	"ai-relay/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	cfg := config.Load()

	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect database: %v", err)
	}

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
		log.Fatalf("Failed to migrate database: %v", err)
	}
	log.Println("Database migrated")

	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	log.Printf("Starting server on :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
```

- [ ] **Step 6: Verify models compile**

```bash
cd /home/colorful/Documents/claude/token/backend
go build ./...
```

Expected: no errors.

- [ ] **Step 7: Commit**

```bash
git add backend/
git commit -m "feat: add all GORM models and database migration"
```

---

### Task 3: Docker Compose & Makefile

**Files:**
- Create: `docker-compose.yml`
- Create: `Makefile`
- Create: `backend/Dockerfile`
- Create: `backend/.env` (local dev, gitignored)
- Create: `.gitignore`

- [ ] **Step 1: Create docker-compose.yml for dev dependencies**

Create `docker-compose.yml` at project root:

```yaml
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: relay
      POSTGRES_PASSWORD: relay
      POSTGRES_DB: relay
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redisdata:/data

volumes:
  pgdata:
  redisdata:
```

- [ ] **Step 2: Create Makefile**

Create `Makefile` at project root:

```makefile
.PHONY: dev deps migrate seed build

# Start PostgreSQL and Redis
deps:
	docker compose up -d postgres redis

# Run backend in dev mode
dev: deps
	cd backend && go run cmd/server/main.go

# Run database migration (via GORM AutoMigrate on startup)
migrate: deps
	cd backend && go run cmd/server/main.go &
	sleep 3
	kill $$!

# Seed database with default data
seed: deps
	docker compose exec -T postgres psql -U relay -d relay < backend/migration/seed.sql

# Build backend binary
build:
	cd backend && go build -o ../bin/relay cmd/server/main.go

# Run all backend tests
test:
	cd backend && go test ./... -v

# Stop all services
down:
	docker compose down
```

- [ ] **Step 3: Create .gitignore**

Create `.gitignore` at project root:

```
# Binaries
bin/
*.exe

# Environment
.env
backend/.env

# Dependencies
node_modules/

# Build
frontend/dist/

# IDE
.idea/
.vscode/
*.swp

# OS
.DS_Store

# Brainstorm artifacts
.superpowers/

# Playwright
.playwright-mcp/
nof1-reference.png
```

- [ ] **Step 4: Verify Docker Compose starts and backend connects**

```bash
cd /home/colorful/Documents/claude/token
make deps
sleep 3
cd backend && go run cmd/server/main.go &
sleep 3
curl http://localhost:8080/health
# Expected: {"status":"ok"}
# Check logs for "Database migrated"
kill %1
```

- [ ] **Step 5: Run seed**

```bash
cd /home/colorful/Documents/claude/token
make seed
# Verify seed data
docker compose exec postgres psql -U relay -d relay -c "SELECT email, role FROM users;"
# Expected: admin@relay.local | admin
docker compose exec postgres psql -U relay -d relay -c "SELECT model_name, rate FROM model_configs;"
# Expected: 3 Claude models
```

- [ ] **Step 6: Commit**

```bash
git add docker-compose.yml Makefile .gitignore
git commit -m "feat: add Docker Compose, Makefile, and gitignore"
```

---

## Phase 2: Authentication & Security Utilities

### Task 4: Crypto & JWT Utilities

**Files:**
- Create: `backend/internal/pkg/crypto.go`
- Create: `backend/internal/pkg/jwt.go`
- Create: `backend/internal/pkg/crypto_test.go`
- Create: `backend/internal/pkg/jwt_test.go`

- [ ] **Step 1: Write crypto tests**

Create `backend/internal/pkg/crypto_test.go`:

```go
package pkg

import (
	"strings"
	"testing"
)

func TestGenerateAPIKey(t *testing.T) {
	key, err := GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey failed: %v", err)
	}
	if !strings.HasPrefix(key, "sk-") {
		t.Errorf("API key should start with sk-, got: %s", key)
	}
	if len(key) < 30 {
		t.Errorf("API key too short: %s", key)
	}
}

func TestGenerateAPIKey_Unique(t *testing.T) {
	keys := make(map[string]bool)
	for i := 0; i < 100; i++ {
		key, _ := GenerateAPIKey()
		if keys[key] {
			t.Fatalf("Duplicate key generated: %s", key)
		}
		keys[key] = true
	}
}

func TestHashAndCheckPassword(t *testing.T) {
	password := "testpassword123"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if !CheckPassword(password, hash) {
		t.Error("CheckPassword should return true for correct password")
	}
	if CheckPassword("wrongpassword", hash) {
		t.Error("CheckPassword should return false for wrong password")
	}
}

func TestEncryptDecrypt(t *testing.T) {
	key := "12345678901234567890123456789012" // 32 bytes
	plaintext := "my-secret-api-key-from-upstream"

	encrypted, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	if encrypted == plaintext {
		t.Error("Encrypted text should differ from plaintext")
	}

	decrypted, err := Decrypt(encrypted, key)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	if decrypted != plaintext {
		t.Errorf("Decrypt mismatch: got %s, want %s", decrypted, plaintext)
	}
}

func TestGenerateRedeemCode(t *testing.T) {
	code := GenerateRedeemCode()
	if len(code) == 0 {
		t.Error("Redeem code should not be empty")
	}
	// Format: XXXX-XXXX-XXXX-XXXX
	parts := strings.Split(code, "-")
	if len(parts) != 4 {
		t.Errorf("Redeem code should have 4 parts, got: %s", code)
	}
	for _, part := range parts {
		if len(part) != 4 {
			t.Errorf("Each part should be 4 chars, got: %s", part)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/colorful/Documents/claude/token/backend
go test ./internal/pkg/ -v -run TestGenerate
# Expected: FAIL — functions not defined
```

- [ ] **Step 3: Implement crypto.go**

Create `backend/internal/pkg/crypto.go`:

```go
package pkg

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

func GenerateAPIKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "sk-" + hex.EncodeToString(bytes), nil
}

func GenerateRedeemCode() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	hex := strings.ToUpper(fmt.Sprintf("%x", bytes))
	// Format as XXXX-XXXX-XXXX-XXXX
	return hex[0:4] + "-" + hex[4:8] + "-" + hex[8:12] + "-" + hex[12:16]
}

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func CheckPassword(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func Encrypt(plaintext, keyStr string) (string, error) {
	key := []byte(keyStr)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func Decrypt(encoded, keyStr string) (string, error) {
	key := []byte(keyStr)
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
```

- [ ] **Step 4: Run crypto tests**

```bash
cd /home/colorful/Documents/claude/token/backend
go test ./internal/pkg/ -v -run "TestGenerate|TestHash|TestEncrypt"
# Expected: all PASS
```

- [ ] **Step 5: Write JWT tests**

Create `backend/internal/pkg/jwt_test.go`:

```go
package pkg

import (
	"testing"
	"time"
)

func TestJWT_SignAndVerify(t *testing.T) {
	secret := "test-secret-key"
	claims := &JWTClaims{
		UserID: 42,
		Email:  "test@example.com",
		Role:   "user",
	}

	token, err := SignJWT(claims, secret, 15*time.Minute)
	if err != nil {
		t.Fatalf("SignJWT failed: %v", err)
	}
	if token == "" {
		t.Fatal("Token should not be empty")
	}

	parsed, err := VerifyJWT(token, secret)
	if err != nil {
		t.Fatalf("VerifyJWT failed: %v", err)
	}
	if parsed.UserID != 42 {
		t.Errorf("UserID mismatch: got %d, want 42", parsed.UserID)
	}
	if parsed.Email != "test@example.com" {
		t.Errorf("Email mismatch: got %s", parsed.Email)
	}
	if parsed.Role != "user" {
		t.Errorf("Role mismatch: got %s", parsed.Role)
	}
}

func TestJWT_Expired(t *testing.T) {
	secret := "test-secret-key"
	claims := &JWTClaims{UserID: 1, Email: "a@b.com", Role: "user"}
	token, _ := SignJWT(claims, secret, -1*time.Minute) // already expired
	_, err := VerifyJWT(token, secret)
	if err == nil {
		t.Error("VerifyJWT should fail for expired token")
	}
}

func TestJWT_WrongSecret(t *testing.T) {
	claims := &JWTClaims{UserID: 1, Email: "a@b.com", Role: "user"}
	token, _ := SignJWT(claims, "secret1", 15*time.Minute)
	_, err := VerifyJWT(token, "secret2")
	if err == nil {
		t.Error("VerifyJWT should fail for wrong secret")
	}
}
```

- [ ] **Step 6: Implement jwt.go**

Create `backend/internal/pkg/jwt.go`:

```go
package pkg

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTClaims struct {
	UserID int64  `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

func SignJWT(claims *JWTClaims, secret string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims.RegisteredClaims = jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func VerifyJWT(tokenStr, secret string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &JWTClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}
```

- [ ] **Step 7: Run all tests**

```bash
cd /home/colorful/Documents/claude/token/backend
go test ./internal/pkg/ -v
# Expected: all PASS
```

- [ ] **Step 8: Commit**

```bash
git add backend/internal/pkg/
git commit -m "feat: add crypto (AES, bcrypt, API key gen) and JWT utilities with tests"
```

---

### Task 5: Auth Middleware

**Files:**
- Create: `backend/internal/middleware/auth.go`
- Create: `backend/internal/middleware/cors.go`
- Create: `backend/internal/middleware/ratelimit.go`

- [ ] **Step 1: Create JWT auth middleware**

Create `backend/internal/middleware/auth.go`:

```go
package middleware

import (
	"strings"

	"ai-relay/internal/pkg"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func JWTAuth(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			pkg.Unauthorized(c, "missing or invalid authorization header")
			c.Abort()
			return
		}
		tokenStr := strings.TrimPrefix(auth, "Bearer ")
		claims, err := pkg.VerifyJWT(tokenStr, jwtSecret)
		if err != nil {
			pkg.Unauthorized(c, "invalid or expired token")
			c.Abort()
			return
		}
		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)
		c.Set("role", claims.Role)
		c.Next()
	}
}

func AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("role")
		if role != "admin" {
			pkg.Forbidden(c, "admin access required")
			c.Abort()
			return
		}
		c.Next()
	}
}

// APIKeyAuth authenticates requests using sk- API keys (for proxy endpoints)
func APIKeyAuth(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := ""
		// Check x-api-key header first
		if k := c.GetHeader("x-api-key"); k != "" {
			key = k
		}
		// Check Authorization: Bearer sk-xxx
		if auth := c.GetHeader("Authorization"); key == "" && strings.HasPrefix(auth, "Bearer sk-") {
			key = strings.TrimPrefix(auth, "Bearer ")
		}
		if key == "" || !strings.HasPrefix(key, "sk-") {
			c.JSON(401, gin.H{"error": gin.H{
				"type":    "authentication_error",
				"message": "invalid x-api-key",
			}})
			c.Abort()
			return
		}

		// Look up API key and get user
		type apiKeyUser struct {
			ApiKeyID int64
			UserID   int64
			Balance  float64
			Status   string
		}
		var result apiKeyUser
		err := db.Raw(`
			SELECT ak.id as api_key_id, ak.user_id, u.balance, u.status
			FROM api_keys ak JOIN users u ON ak.user_id = u.id
			WHERE ak.key = ? AND ak.status = 'active' AND u.status = 'active'
		`, key).Scan(&result).Error
		if err != nil || result.ApiKeyID == 0 {
			c.JSON(401, gin.H{"error": gin.H{
				"type":    "authentication_error",
				"message": "invalid API key",
			}})
			c.Abort()
			return
		}

		c.Set("api_key_id", result.ApiKeyID)
		c.Set("user_id", result.UserID)
		c.Set("balance", result.Balance)

		// Update last_used_at asynchronously
		go func() {
			db.Exec("UPDATE api_keys SET last_used_at = NOW() WHERE id = ?", result.ApiKeyID)
		}()

		c.Next()
	}
}
```

- [ ] **Step 2: Create CORS middleware**

Create `backend/internal/middleware/cors.go`:

```go
package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

func CORS(allowedOrigins string) gin.HandlerFunc {
	origins := strings.Split(allowedOrigins, ",")
	originSet := make(map[string]bool)
	for _, o := range origins {
		originSet[strings.TrimSpace(o)] = true
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if originSet[origin] || originSet["*"] {
			c.Header("Access-Control-Allow-Origin", origin)
		}
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, x-api-key")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
```

- [ ] **Step 3: Create rate limit middleware**

Create `backend/internal/middleware/ratelimit.go`:

```go
package middleware

import (
	"context"
	"fmt"
	"time"

	"ai-relay/internal/pkg"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func RateLimit(rdb *redis.Client, maxRequests int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		if rdb == nil {
			c.Next()
			return
		}
		// Use API key or user_id as rate limit key
		var key string
		if apiKeyID, exists := c.Get("api_key_id"); exists {
			key = fmt.Sprintf("rl:ak:%d", apiKeyID)
		} else if userID, exists := c.Get("user_id"); exists {
			key = fmt.Sprintf("rl:u:%d", userID)
		} else {
			key = fmt.Sprintf("rl:ip:%s", c.ClientIP())
		}

		ctx := context.Background()
		count, err := rdb.Incr(ctx, key).Result()
		if err != nil {
			// If Redis is down, allow the request
			c.Next()
			return
		}
		if count == 1 {
			rdb.Expire(ctx, key, window)
		}
		if count > int64(maxRequests) {
			pkg.Fail(c, 429, "rate limit exceeded")
			c.Abort()
			return
		}
		c.Next()
	}
}
```

- [ ] **Step 4: Verify compilation**

```bash
cd /home/colorful/Documents/claude/token/backend
go build ./...
# Expected: no errors
```

- [ ] **Step 5: Commit**

```bash
git add backend/internal/middleware/
git commit -m "feat: add JWT, API key, CORS, and rate limit middleware"
```

---

## Phase 3: Core Backend Services

### Task 6: Repository Layer

**Files:**
- Create: `backend/internal/repository/user.go`
- Create: `backend/internal/repository/api_key.go`
- Create: `backend/internal/repository/channel.go`
- Create: `backend/internal/repository/model_config.go`
- Create: `backend/internal/repository/request_log.go`
- Create: `backend/internal/repository/balance_log.go`
- Create: `backend/internal/repository/redemption_code.go`
- Create: `backend/internal/repository/setting.go`

- [ ] **Step 1: Create all repository files**

Create `backend/internal/repository/user.go`:

```go
package repository

import (
	"ai-relay/internal/model"

	"gorm.io/gorm"
)

type UserRepo struct{ DB *gorm.DB }

func (r *UserRepo) Create(user *model.User) error {
	return r.DB.Create(user).Error
}

func (r *UserRepo) FindByID(id int64) (*model.User, error) {
	var user model.User
	err := r.DB.First(&user, id).Error
	return &user, err
}

func (r *UserRepo) FindByEmail(email string) (*model.User, error) {
	var user model.User
	err := r.DB.Where("email = ?", email).First(&user).Error
	return &user, err
}

func (r *UserRepo) FindByGoogleID(googleID string) (*model.User, error) {
	var user model.User
	err := r.DB.Where("google_id = ?", googleID).First(&user).Error
	return &user, err
}

func (r *UserRepo) Update(user *model.User) error {
	return r.DB.Save(user).Error
}

func (r *UserRepo) List(page, pageSize int, search string) ([]model.User, int64, error) {
	var users []model.User
	var total int64
	q := r.DB.Model(&model.User{})
	if search != "" {
		q = q.Where("email ILIKE ?", "%"+search+"%")
	}
	q.Count(&total)
	err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&users).Error
	return users, total, err
}

func (r *UserRepo) DeductBalance(userID int64, amount float64) (int64, error) {
	result := r.DB.Exec(
		"UPDATE users SET balance = balance - ?, updated_at = NOW() WHERE id = ? AND balance >= ?",
		amount, userID, amount,
	)
	return result.RowsAffected, result.Error
}

func (r *UserRepo) AddBalance(userID int64, amount float64) error {
	return r.DB.Exec(
		"UPDATE users SET balance = balance + ?, updated_at = NOW() WHERE id = ?",
		amount, userID,
	).Error
}
```

Create `backend/internal/repository/api_key.go`:

```go
package repository

import (
	"ai-relay/internal/model"

	"gorm.io/gorm"
)

type ApiKeyRepo struct{ DB *gorm.DB }

func (r *ApiKeyRepo) Create(key *model.ApiKey) error {
	return r.DB.Create(key).Error
}

func (r *ApiKeyRepo) FindByKey(key string) (*model.ApiKey, error) {
	var ak model.ApiKey
	err := r.DB.Where("key = ?", key).First(&ak).Error
	return &ak, err
}

func (r *ApiKeyRepo) ListByUser(userID int64) ([]model.ApiKey, error) {
	var keys []model.ApiKey
	err := r.DB.Where("user_id = ?", userID).Order("id DESC").Find(&keys).Error
	return keys, err
}

func (r *ApiKeyRepo) Delete(id, userID int64) error {
	return r.DB.Where("id = ? AND user_id = ?", id, userID).Delete(&model.ApiKey{}).Error
}

func (r *ApiKeyRepo) UpdateStatus(id, userID int64, status string) error {
	return r.DB.Model(&model.ApiKey{}).Where("id = ? AND user_id = ?", id, userID).
		Update("status", status).Error
}
```

Create `backend/internal/repository/channel.go`:

```go
package repository

import (
	"ai-relay/internal/model"

	"gorm.io/gorm"
)

type ChannelRepo struct{ DB *gorm.DB }

func (r *ChannelRepo) Create(ch *model.Channel) error {
	return r.DB.Create(ch).Error
}

func (r *ChannelRepo) FindByID(id int64) (*model.Channel, error) {
	var ch model.Channel
	err := r.DB.First(&ch, id).Error
	return &ch, err
}

func (r *ChannelRepo) Update(ch *model.Channel) error {
	return r.DB.Save(ch).Error
}

func (r *ChannelRepo) Delete(id int64) error {
	return r.DB.Delete(&model.Channel{}, id).Error
}

func (r *ChannelRepo) List() ([]model.Channel, error) {
	var channels []model.Channel
	err := r.DB.Order("priority ASC, id ASC").Find(&channels).Error
	return channels, err
}

func (r *ChannelRepo) FindActiveByModel(modelName string) ([]model.Channel, error) {
	var channels []model.Channel
	err := r.DB.Where("status = 'active' AND models @> ?", `["`+modelName+`"]`).
		Order("priority ASC").Find(&channels).Error
	return channels, err
}

func (r *ChannelRepo) UpdateStatus(id int64, status string) error {
	return r.DB.Model(&model.Channel{}).Where("id = ?", id).Update("status", status).Error
}
```

Create `backend/internal/repository/model_config.go`:

```go
package repository

import (
	"ai-relay/internal/model"

	"gorm.io/gorm"
)

type ModelConfigRepo struct{ DB *gorm.DB }

func (r *ModelConfigRepo) Create(mc *model.ModelConfig) error {
	return r.DB.Create(mc).Error
}

func (r *ModelConfigRepo) FindByName(name string) (*model.ModelConfig, error) {
	var mc model.ModelConfig
	err := r.DB.Where("model_name = ?", name).First(&mc).Error
	return &mc, err
}

func (r *ModelConfigRepo) Update(mc *model.ModelConfig) error {
	return r.DB.Save(mc).Error
}

func (r *ModelConfigRepo) List() ([]model.ModelConfig, error) {
	var models []model.ModelConfig
	err := r.DB.Order("id ASC").Find(&models).Error
	return models, err
}

func (r *ModelConfigRepo) ListEnabled() ([]model.ModelConfig, error) {
	var models []model.ModelConfig
	err := r.DB.Where("enabled = true").Order("id ASC").Find(&models).Error
	return models, err
}
```

Create `backend/internal/repository/request_log.go`:

```go
package repository

import (
	"time"

	"ai-relay/internal/model"

	"gorm.io/gorm"
)

type RequestLogRepo struct{ DB *gorm.DB }

func (r *RequestLogRepo) Create(log *model.RequestLog) error {
	return r.DB.Create(log).Error
}

func (r *RequestLogRepo) ListByUser(userID int64, page, pageSize int) ([]model.RequestLog, int64, error) {
	var logs []model.RequestLog
	var total int64
	q := r.DB.Model(&model.RequestLog{}).Where("user_id = ?", userID)
	q.Count(&total)
	err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&logs).Error
	return logs, total, err
}

func (r *RequestLogRepo) ListAll(page, pageSize int, userID int64, modelFilter string) ([]model.RequestLog, int64, error) {
	var logs []model.RequestLog
	var total int64
	q := r.DB.Model(&model.RequestLog{})
	if userID > 0 {
		q = q.Where("user_id = ?", userID)
	}
	if modelFilter != "" {
		q = q.Where("model = ?", modelFilter)
	}
	q.Count(&total)
	err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&logs).Error
	return logs, total, err
}

func (r *RequestLogRepo) StatsToday() (requestCount int64, totalTokens int64, totalCost float64, err error) {
	today := time.Now().Truncate(24 * time.Hour)
	row := r.DB.Model(&model.RequestLog{}).
		Where("created_at >= ? AND status = 'success'", today).
		Select("COUNT(*), COALESCE(SUM(total_tokens),0), COALESCE(SUM(cost),0)").Row()
	err = row.Scan(&requestCount, &totalTokens, &totalCost)
	return
}

func (r *RequestLogRepo) StatsTodayByUser(userID int64) (requestCount int64, totalTokens int64, totalCost float64, err error) {
	today := time.Now().Truncate(24 * time.Hour)
	row := r.DB.Model(&model.RequestLog{}).
		Where("user_id = ? AND created_at >= ? AND status = 'success'", userID, today).
		Select("COUNT(*), COALESCE(SUM(total_tokens),0), COALESCE(SUM(cost),0)").Row()
	err = row.Scan(&requestCount, &totalTokens, &totalCost)
	return
}
```

Create `backend/internal/repository/balance_log.go`:

```go
package repository

import (
	"ai-relay/internal/model"

	"gorm.io/gorm"
)

type BalanceLogRepo struct{ DB *gorm.DB }

func (r *BalanceLogRepo) Create(log *model.BalanceLog) error {
	return r.DB.Create(log).Error
}

func (r *BalanceLogRepo) ListByUser(userID int64, page, pageSize int) ([]model.BalanceLog, int64, error) {
	var logs []model.BalanceLog
	var total int64
	q := r.DB.Model(&model.BalanceLog{}).Where("user_id = ?", userID)
	q.Count(&total)
	err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&logs).Error
	return logs, total, err
}
```

Create `backend/internal/repository/redemption_code.go`:

```go
package repository

import (
	"ai-relay/internal/model"

	"gorm.io/gorm"
)

type RedemptionCodeRepo struct{ DB *gorm.DB }

func (r *RedemptionCodeRepo) Create(code *model.RedemptionCode) error {
	return r.DB.Create(code).Error
}

func (r *RedemptionCodeRepo) FindByCode(code string) (*model.RedemptionCode, error) {
	var rc model.RedemptionCode
	err := r.DB.Where("code = ?", code).First(&rc).Error
	return &rc, err
}

func (r *RedemptionCodeRepo) Update(rc *model.RedemptionCode) error {
	return r.DB.Save(rc).Error
}

func (r *RedemptionCodeRepo) List(page, pageSize int) ([]model.RedemptionCode, int64, error) {
	var codes []model.RedemptionCode
	var total int64
	r.DB.Model(&model.RedemptionCode{}).Count(&total)
	err := r.DB.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&codes).Error
	return codes, total, err
}
```

Create `backend/internal/repository/setting.go`:

```go
package repository

import (
	"ai-relay/internal/model"

	"gorm.io/gorm"
)

type SettingRepo struct{ DB *gorm.DB }

func (r *SettingRepo) Get(key string) (string, error) {
	var s model.Setting
	err := r.DB.Where("key = ?", key).First(&s).Error
	return s.Value, err
}

func (r *SettingRepo) Set(key, value string) error {
	return r.DB.Save(&model.Setting{Key: key, Value: value}).Error
}

func (r *SettingRepo) GetAll() ([]model.Setting, error) {
	var settings []model.Setting
	err := r.DB.Find(&settings).Error
	return settings, err
}
```

- [ ] **Step 2: Verify compilation**

```bash
cd /home/colorful/Documents/claude/token/backend
go build ./...
# Expected: no errors
```

- [ ] **Step 3: Commit**

```bash
git add backend/internal/repository/
git commit -m "feat: add repository layer for all models"
```

---

### Task 7: Auth Service & Handlers

**Files:**
- Create: `backend/internal/service/auth.go`
- Create: `backend/internal/handler/auth.go`

- [ ] **Step 1: Create auth service**

Create `backend/internal/service/auth.go`:

```go
package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"ai-relay/internal/config"
	"ai-relay/internal/model"
	"ai-relay/internal/pkg"
	"ai-relay/internal/repository"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type AuthService struct {
	UserRepo    *repository.UserRepo
	SettingRepo *repository.SettingRepo
	Config      *config.Config
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

func (s *AuthService) Register(email, password string) (*model.User, error) {
	// Check if registration is enabled
	regEnabled, _ := s.SettingRepo.Get("register_enabled")
	if regEnabled == "false" {
		return nil, errors.New("registration is disabled")
	}

	// Check if email already exists
	if _, err := s.UserRepo.FindByEmail(email); err == nil {
		return nil, errors.New("email already registered")
	}

	hash, err := pkg.HashPassword(password)
	if err != nil {
		return nil, err
	}

	// Get default balance
	defaultBal := decimal.NewFromFloat(s.Config.DefaultBalance)

	user := &model.User{
		Email:        email,
		PasswordHash: hash,
		Role:         "user",
		Balance:      defaultBal,
		Status:       "active",
	}
	if err := s.UserRepo.Create(user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *AuthService) Login(email, password string) (*model.User, error) {
	user, err := s.UserRepo.FindByEmail(email)
	if err != nil {
		return nil, errors.New("invalid email or password")
	}
	if user.Status != "active" {
		return nil, errors.New("account is banned")
	}
	if !pkg.CheckPassword(password, user.PasswordHash) {
		return nil, errors.New("invalid email or password")
	}
	return user, nil
}

func (s *AuthService) IssueTokens(user *model.User) (*TokenPair, error) {
	claims := &pkg.JWTClaims{
		UserID: user.ID,
		Email:  user.Email,
		Role:   user.Role,
	}
	access, err := pkg.SignJWT(claims, s.Config.JWTSecret, 15*time.Minute)
	if err != nil {
		return nil, err
	}
	refresh, err := pkg.SignJWT(claims, s.Config.JWTSecret, 7*24*time.Hour)
	if err != nil {
		return nil, err
	}
	return &TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    900,
	}, nil
}

func (s *AuthService) RefreshToken(refreshToken string) (*TokenPair, error) {
	claims, err := pkg.VerifyJWT(refreshToken, s.Config.JWTSecret)
	if err != nil {
		return nil, errors.New("invalid refresh token")
	}
	user, err := s.UserRepo.FindByID(claims.UserID)
	if err != nil {
		return nil, errors.New("user not found")
	}
	if user.Status != "active" {
		return nil, errors.New("account is banned")
	}
	return s.IssueTokens(user)
}

type GoogleTokenResponse struct {
	AccessToken string `json:"access_token"`
	IDToken     string `json:"id_token"`
}

type GoogleUserInfo struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

func (s *AuthService) GoogleAuthURL() string {
	params := url.Values{
		"client_id":     {s.Config.GoogleClientID},
		"redirect_uri":  {s.Config.GoogleCallback},
		"response_type": {"code"},
		"scope":         {"openid email"},
	}
	return "https://accounts.google.com/o/oauth2/v2/auth?" + params.Encode()
}

func (s *AuthService) GoogleCallback(code string) (*model.User, error) {
	// Exchange code for token
	resp, err := http.PostForm("https://oauth2.googleapis.com/token", url.Values{
		"code":          {code},
		"client_id":     {s.Config.GoogleClientID},
		"client_secret": {s.Config.GoogleSecret},
		"redirect_uri":  {s.Config.GoogleCallback},
		"grant_type":    {"authorization_code"},
	})
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var tokenResp GoogleTokenResponse
	json.Unmarshal(body, &tokenResp)
	if tokenResp.AccessToken == "" {
		return nil, errors.New("failed to get Google access token")
	}

	// Get user info
	req, _ := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)
	infoResp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer infoResp.Body.Close()
	var userInfo GoogleUserInfo
	json.NewDecoder(infoResp.Body).Decode(&userInfo)
	if userInfo.Email == "" {
		return nil, errors.New("failed to get Google user info")
	}

	// Find or create user
	user, err := s.UserRepo.FindByGoogleID(userInfo.ID)
	if err == nil {
		return user, nil
	}
	// Check if email exists (link accounts)
	user, err = s.UserRepo.FindByEmail(userInfo.Email)
	if err == nil {
		user.GoogleID = userInfo.ID
		s.UserRepo.Update(user)
		return user, nil
	}
	// Create new user
	if err == gorm.ErrRecordNotFound {
		defaultBal := decimal.NewFromFloat(s.Config.DefaultBalance)
		user = &model.User{
			Email:    userInfo.Email,
			GoogleID: userInfo.ID,
			Role:     "user",
			Balance:  defaultBal,
			Status:   "active",
		}
		if err := s.UserRepo.Create(user); err != nil {
			return nil, err
		}
		return user, nil
	}
	return nil, err
}
```

- [ ] **Step 2: Create auth handler**

Create `backend/internal/handler/auth.go`:

```go
package handler

import (
	"ai-relay/internal/pkg"
	"ai-relay/internal/service"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	AuthService *service.AuthService
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.BadRequest(c, "invalid request: email and password (min 6 chars) required")
		return
	}
	user, err := h.AuthService.Register(req.Email, req.Password)
	if err != nil {
		pkg.BadRequest(c, err.Error())
		return
	}
	tokens, err := h.AuthService.IssueTokens(user)
	if err != nil {
		pkg.InternalError(c, "failed to issue tokens")
		return
	}
	pkg.Created(c, tokens)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.BadRequest(c, "email and password required")
		return
	}
	user, err := h.AuthService.Login(req.Email, req.Password)
	if err != nil {
		pkg.Unauthorized(c, err.Error())
		return
	}
	tokens, err := h.AuthService.IssueTokens(user)
	if err != nil {
		pkg.InternalError(c, "failed to issue tokens")
		return
	}
	pkg.OK(c, tokens)
}

func (h *AuthHandler) GoogleRedirect(c *gin.Context) {
	url := h.AuthService.GoogleAuthURL()
	c.Redirect(302, url)
}

func (h *AuthHandler) GoogleCallback(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		pkg.BadRequest(c, "missing code parameter")
		return
	}
	user, err := h.AuthService.GoogleCallback(code)
	if err != nil {
		pkg.InternalError(c, err.Error())
		return
	}
	tokens, err := h.AuthService.IssueTokens(user)
	if err != nil {
		pkg.InternalError(c, "failed to issue tokens")
		return
	}
	// Redirect to frontend with tokens as query params
	c.Redirect(302, "/auth/callback?access_token="+tokens.AccessToken+"&refresh_token="+tokens.RefreshToken)
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.BadRequest(c, "refresh_token required")
		return
	}
	tokens, err := h.AuthService.RefreshToken(req.RefreshToken)
	if err != nil {
		pkg.Unauthorized(c, err.Error())
		return
	}
	pkg.OK(c, tokens)
}
```

- [ ] **Step 3: Verify compilation**

```bash
cd /home/colorful/Documents/claude/token/backend
go build ./...
```

- [ ] **Step 4: Commit**

```bash
git add backend/internal/service/auth.go backend/internal/handler/auth.go
git commit -m "feat: add auth service (register, login, Google OAuth, JWT refresh) and handlers"
```

---

### Task 8: User Service & Handlers

**Files:**
- Create: `backend/internal/service/api_key.go`
- Create: `backend/internal/service/user.go`
- Create: `backend/internal/service/billing.go`
- Create: `backend/internal/handler/user.go`

- [ ] **Step 1: Create API key service**

Create `backend/internal/service/api_key.go`:

```go
package service

import (
	"ai-relay/internal/model"
	"ai-relay/internal/pkg"
	"ai-relay/internal/repository"
)

type ApiKeyService struct {
	Repo *repository.ApiKeyRepo
}

func (s *ApiKeyService) Create(userID int64, name string) (*model.ApiKey, error) {
	key, err := pkg.GenerateAPIKey()
	if err != nil {
		return nil, err
	}
	ak := &model.ApiKey{
		UserID: userID,
		Key:    key,
		Name:   name,
		Status: "active",
	}
	if err := s.Repo.Create(ak); err != nil {
		return nil, err
	}
	return ak, nil
}

func (s *ApiKeyService) List(userID int64) ([]model.ApiKey, error) {
	return s.Repo.ListByUser(userID)
}

func (s *ApiKeyService) Delete(id, userID int64) error {
	return s.Repo.Delete(id, userID)
}

func (s *ApiKeyService) UpdateStatus(id, userID int64, status string) error {
	if status != "active" && status != "disabled" {
		return nil
	}
	return s.Repo.UpdateStatus(id, userID, status)
}
```

- [ ] **Step 2: Create user service**

Create `backend/internal/service/user.go`:

```go
package service

import (
	"errors"

	"ai-relay/internal/model"
	"ai-relay/internal/pkg"
	"ai-relay/internal/repository"
)

type UserService struct {
	UserRepo       *repository.UserRepo
	RequestLogRepo *repository.RequestLogRepo
}

func (s *UserService) GetProfile(userID int64) (*model.User, error) {
	return s.UserRepo.FindByID(userID)
}

func (s *UserService) ChangePassword(userID int64, oldPass, newPass string) error {
	user, err := s.UserRepo.FindByID(userID)
	if err != nil {
		return err
	}
	if user.PasswordHash != "" && !pkg.CheckPassword(oldPass, user.PasswordHash) {
		return errors.New("incorrect current password")
	}
	hash, err := pkg.HashPassword(newPass)
	if err != nil {
		return err
	}
	user.PasswordHash = hash
	return s.UserRepo.Update(user)
}

func (s *UserService) Dashboard(userID int64) (map[string]any, error) {
	user, err := s.UserRepo.FindByID(userID)
	if err != nil {
		return nil, err
	}
	reqCount, totalTokens, totalCost, _ := s.RequestLogRepo.StatsTodayByUser(userID)
	return map[string]any{
		"balance":        user.Balance,
		"today_requests": reqCount,
		"today_tokens":   totalTokens,
		"today_cost":     totalCost,
	}, nil
}
```

- [ ] **Step 3: Create billing service**

Create `backend/internal/service/billing.go`:

```go
package service

import (
	"errors"
	"fmt"
	"time"

	"ai-relay/internal/model"
	"ai-relay/internal/pkg"
	"ai-relay/internal/repository"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type BillingService struct {
	DB             *gorm.DB
	UserRepo       *repository.UserRepo
	BalanceLogRepo *repository.BalanceLogRepo
	RedeemRepo     *repository.RedemptionCodeRepo
	ModelRepo      *repository.ModelConfigRepo
	RequestLogRepo *repository.RequestLogRepo
}

func (s *BillingService) CalculateCost(modelName string, promptTokens, completionTokens int) (decimal.Decimal, error) {
	mc, err := s.ModelRepo.FindByName(modelName)
	if err != nil {
		return decimal.Zero, fmt.Errorf("model %s not found", modelName)
	}
	// price is per million tokens
	million := decimal.NewFromInt(1000000)
	inputCost := mc.InputPrice.Mul(decimal.NewFromInt(int64(promptTokens))).Div(million)
	outputCost := mc.OutputPrice.Mul(decimal.NewFromInt(int64(completionTokens))).Div(million)
	total := inputCost.Add(outputCost).Mul(mc.Rate)
	return total, nil
}

func (s *BillingService) DeductBalance(userID int64, cost decimal.Decimal, requestLogID int64, description string) error {
	return s.DB.Transaction(func(tx *gorm.DB) error {
		costFloat, _ := cost.Float64()
		result := tx.Exec(
			"UPDATE users SET balance = balance - ?, updated_at = NOW() WHERE id = ? AND balance >= ?",
			costFloat, userID, costFloat,
		)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return errors.New("insufficient balance")
		}
		var user model.User
		tx.First(&user, userID)
		log := &model.BalanceLog{
			UserID:       userID,
			Type:         "consume",
			Amount:       cost.Neg(),
			BalanceAfter: user.Balance,
			Description:  description,
			RequestLogID: &requestLogID,
		}
		return tx.Create(log).Error
	})
}

func (s *BillingService) AdminTopUp(userID int64, amount float64, adminID int64) error {
	return s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(
			"UPDATE users SET balance = balance + ?, updated_at = NOW() WHERE id = ?",
			amount, userID,
		).Error; err != nil {
			return err
		}
		var user model.User
		tx.First(&user, userID)
		log := &model.BalanceLog{
			UserID:       userID,
			Type:         "topup",
			Amount:       decimal.NewFromFloat(amount),
			BalanceAfter: user.Balance,
			Description:  fmt.Sprintf("Admin #%d manual top-up", adminID),
		}
		return tx.Create(log).Error
	})
}

func (s *BillingService) Redeem(userID int64, code string) error {
	return s.DB.Transaction(func(tx *gorm.DB) error {
		var rc model.RedemptionCode
		if err := tx.Where("code = ? AND status = 'unused'", code).First(&rc).Error; err != nil {
			return errors.New("invalid or used redemption code")
		}
		if rc.ExpiresAt != nil && rc.ExpiresAt.Before(time.Now()) {
			return errors.New("redemption code has expired")
		}
		// Mark code as used
		now := time.Now()
		rc.Status = "used"
		rc.UsedBy = &userID
		rc.UsedAt = &now
		if err := tx.Save(&rc).Error; err != nil {
			return err
		}
		// Add balance
		if err := tx.Exec(
			"UPDATE users SET balance = balance + ?, updated_at = NOW() WHERE id = ?",
			rc.Amount, userID,
		).Error; err != nil {
			return err
		}
		var user model.User
		tx.First(&user, userID)
		log := &model.BalanceLog{
			UserID:       userID,
			Type:         "redeem",
			Amount:       decimal.NewFromFloat(rc.Amount),
			BalanceAfter: user.Balance,
			Description:  fmt.Sprintf("Redeem code: %s", code),
		}
		return tx.Create(log).Error
	})
}

func (s *BillingService) GenerateRedeemCodes(adminID int64, amount float64, count int, expiresAt *time.Time) ([]model.RedemptionCode, error) {
	codes := make([]model.RedemptionCode, count)
	for i := 0; i < count; i++ {
		codes[i] = model.RedemptionCode{
			Code:      pkg.GenerateRedeemCode(),
			Amount:    amount,
			Status:    "unused",
			CreatedBy: adminID,
			ExpiresAt: expiresAt,
		}
	}
	err := s.DB.Create(&codes).Error
	return codes, err
}
```

- [ ] **Step 4: Create user handler**

Create `backend/internal/handler/user.go`:

```go
package handler

import (
	"strconv"

	"ai-relay/internal/pkg"
	"ai-relay/internal/service"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	UserService   *service.UserService
	ApiKeyService *service.ApiKeyService
	BillingService *service.BillingService
	RequestLogRepo interface {
		ListByUser(userID int64, page, pageSize int) (any, int64, error)
	}
	BalanceLogRepo interface {
		ListByUser(userID int64, page, pageSize int) (any, int64, error)
	}
}

func getUserID(c *gin.Context) int64 {
	id, _ := c.Get("user_id")
	return id.(int64)
}

func getPage(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 { page = 1 }
	if pageSize < 1 || pageSize > 100 { pageSize = 20 }
	return page, pageSize
}

func (h *UserHandler) GetProfile(c *gin.Context) {
	user, err := h.UserService.GetProfile(getUserID(c))
	if err != nil {
		pkg.NotFound(c, "user not found")
		return
	}
	pkg.OK(c, user)
}

func (h *UserHandler) ChangePassword(c *gin.Context) {
	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.BadRequest(c, "new_password (min 6 chars) required")
		return
	}
	if err := h.UserService.ChangePassword(getUserID(c), req.OldPassword, req.NewPassword); err != nil {
		pkg.BadRequest(c, err.Error())
		return
	}
	pkg.OK(c, gin.H{"message": "password updated"})
}

func (h *UserHandler) ListApiKeys(c *gin.Context) {
	keys, err := h.ApiKeyService.List(getUserID(c))
	if err != nil {
		pkg.InternalError(c, "failed to list API keys")
		return
	}
	// Mask keys for display
	type maskedKey struct {
		ID         int64  `json:"id"`
		Name       string `json:"name"`
		Key        string `json:"key"`
		Status     string `json:"status"`
		CreatedAt  any    `json:"created_at"`
		LastUsedAt any    `json:"last_used_at"`
	}
	masked := make([]maskedKey, len(keys))
	for i, k := range keys {
		keyStr := k.Key
		if len(keyStr) > 8 {
			keyStr = keyStr[:7] + "****" + keyStr[len(keyStr)-4:]
		}
		masked[i] = maskedKey{
			ID: k.ID, Name: k.Name, Key: keyStr,
			Status: k.Status, CreatedAt: k.CreatedAt, LastUsedAt: k.LastUsedAt,
		}
	}
	pkg.OK(c, masked)
}

func (h *UserHandler) CreateApiKey(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.BadRequest(c, "name required")
		return
	}
	key, err := h.ApiKeyService.Create(getUserID(c), req.Name)
	if err != nil {
		pkg.InternalError(c, "failed to create API key")
		return
	}
	// Return full key only on creation
	pkg.Created(c, key)
}

func (h *UserHandler) DeleteApiKey(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.ApiKeyService.Delete(id, getUserID(c)); err != nil {
		pkg.InternalError(c, "failed to delete API key")
		return
	}
	pkg.OK(c, gin.H{"message": "deleted"})
}

func (h *UserHandler) UpdateApiKey(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.BadRequest(c, "status required")
		return
	}
	if err := h.ApiKeyService.UpdateStatus(id, getUserID(c), req.Status); err != nil {
		pkg.InternalError(c, "failed to update API key")
		return
	}
	pkg.OK(c, gin.H{"message": "updated"})
}

func (h *UserHandler) ListLogs(c *gin.Context) {
	page, pageSize := getPage(c)
	logs, total, err := h.RequestLogRepo.ListByUser(getUserID(c), page, pageSize)
	if err != nil {
		pkg.InternalError(c, "failed to list logs")
		return
	}
	pkg.OK(c, gin.H{"data": logs, "total": total, "page": page, "page_size": pageSize})
}

func (h *UserHandler) ListBalanceLogs(c *gin.Context) {
	page, pageSize := getPage(c)
	logs, total, err := h.BalanceLogRepo.ListByUser(getUserID(c), page, pageSize)
	if err != nil {
		pkg.InternalError(c, "failed to list balance logs")
		return
	}
	pkg.OK(c, gin.H{"data": logs, "total": total, "page": page, "page_size": pageSize})
}

func (h *UserHandler) Redeem(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.BadRequest(c, "code required")
		return
	}
	if err := h.BillingService.Redeem(getUserID(c), req.Code); err != nil {
		pkg.BadRequest(c, err.Error())
		return
	}
	pkg.OK(c, gin.H{"message": "redeemed successfully"})
}

func (h *UserHandler) Dashboard(c *gin.Context) {
	data, err := h.UserService.Dashboard(getUserID(c))
	if err != nil {
		pkg.InternalError(c, "failed to load dashboard")
		return
	}
	pkg.OK(c, data)
}
```

- [ ] **Step 5: Verify compilation**

```bash
cd /home/colorful/Documents/claude/token/backend
go build ./...
```

- [ ] **Step 6: Commit**

```bash
git add backend/internal/service/ backend/internal/handler/
git commit -m "feat: add user, API key, and billing services with HTTP handlers"
```

---

### Task 9: Admin Service & Handlers

**Files:**
- Create: `backend/internal/service/admin.go`
- Create: `backend/internal/handler/admin.go`

- [ ] **Step 1: Create admin service**

Create `backend/internal/service/admin.go`:

```go
package service

import (
	"ai-relay/internal/model"
	"ai-relay/internal/repository"
)

type AdminService struct {
	UserRepo       *repository.UserRepo
	ChannelRepo    *repository.ChannelRepo
	ModelRepo      *repository.ModelConfigRepo
	RequestLogRepo *repository.RequestLogRepo
	SettingRepo    *repository.SettingRepo
}

func (s *AdminService) Dashboard() (map[string]any, error) {
	_, userCount, _ := s.UserRepo.List(1, 1, "")
	reqCount, totalTokens, totalRevenue, _ := s.RequestLogRepo.StatsToday()
	return map[string]any{
		"total_users":    userCount,
		"today_requests": reqCount,
		"today_tokens":   totalTokens,
		"today_revenue":  totalRevenue,
	}, nil
}

func (s *AdminService) ListUsers(page, pageSize int, search string) ([]model.User, int64, error) {
	return s.UserRepo.List(page, pageSize, search)
}

func (s *AdminService) UpdateUser(userID int64, role, status string) error {
	user, err := s.UserRepo.FindByID(userID)
	if err != nil {
		return err
	}
	if role != "" {
		user.Role = role
	}
	if status != "" {
		user.Status = status
	}
	return s.UserRepo.Update(user)
}

func (s *AdminService) GetSettings() ([]model.Setting, error) {
	return s.SettingRepo.GetAll()
}

func (s *AdminService) UpdateSetting(key, value string) error {
	return s.SettingRepo.Set(key, value)
}
```

- [ ] **Step 2: Create admin handler**

Create `backend/internal/handler/admin.go`:

```go
package handler

import (
	"encoding/json"
	"strconv"
	"time"

	"ai-relay/internal/config"
	"ai-relay/internal/model"
	"ai-relay/internal/pkg"
	"ai-relay/internal/repository"
	"ai-relay/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

type AdminHandler struct {
	AdminService   *service.AdminService
	BillingService *service.BillingService
	ChannelRepo    *repository.ChannelRepo
	ModelRepo      *repository.ModelConfigRepo
	RedeemRepo     *repository.RedemptionCodeRepo
	RequestLogRepo *repository.RequestLogRepo
	Config         *config.Config
}

func (h *AdminHandler) Dashboard(c *gin.Context) {
	data, err := h.AdminService.Dashboard()
	if err != nil {
		pkg.InternalError(c, "failed to load dashboard")
		return
	}
	pkg.OK(c, data)
}

func (h *AdminHandler) ListUsers(c *gin.Context) {
	page, pageSize := getPage(c)
	search := c.Query("search")
	users, total, err := h.AdminService.ListUsers(page, pageSize, search)
	if err != nil {
		pkg.InternalError(c, "failed to list users")
		return
	}
	pkg.OK(c, gin.H{"data": users, "total": total, "page": page, "page_size": pageSize})
}

func (h *AdminHandler) UpdateUser(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		Role   string `json:"role"`
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.BadRequest(c, "invalid request")
		return
	}
	if err := h.AdminService.UpdateUser(id, req.Role, req.Status); err != nil {
		pkg.InternalError(c, err.Error())
		return
	}
	pkg.OK(c, gin.H{"message": "updated"})
}

func (h *AdminHandler) TopUp(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		Amount float64 `json:"amount" binding:"required,gt=0"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.BadRequest(c, "amount (positive number) required")
		return
	}
	adminID := getUserID(c)
	if err := h.BillingService.AdminTopUp(id, req.Amount, adminID); err != nil {
		pkg.InternalError(c, err.Error())
		return
	}
	pkg.OK(c, gin.H{"message": "topped up"})
}

// Channel CRUD
func (h *AdminHandler) ListChannels(c *gin.Context) {
	channels, err := h.ChannelRepo.List()
	if err != nil {
		pkg.InternalError(c, "failed to list channels")
		return
	}
	pkg.OK(c, channels)
}

func (h *AdminHandler) CreateChannel(c *gin.Context) {
	var req struct {
		Name     string   `json:"name" binding:"required"`
		Type     string   `json:"type" binding:"required"`
		ApiKey   string   `json:"api_key" binding:"required"`
		BaseURL  string   `json:"base_url" binding:"required"`
		Models   []string `json:"models" binding:"required"`
		Priority int      `json:"priority"`
		Weight   int      `json:"weight"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.BadRequest(c, "invalid request")
		return
	}
	// Encrypt the upstream API key
	encrypted, err := pkg.Encrypt(req.ApiKey, h.Config.EncryptionKey)
	if err != nil {
		pkg.InternalError(c, "failed to encrypt API key")
		return
	}
	modelsJSON, _ := json.Marshal(req.Models)
	weight := req.Weight
	if weight < 1 { weight = 1 }
	ch := &model.Channel{
		Name:     req.Name,
		Type:     req.Type,
		ApiKey:   encrypted,
		BaseURL:  req.BaseURL,
		Models:   modelsJSON,
		Status:   "active",
		Priority: req.Priority,
		Weight:   weight,
	}
	if err := h.ChannelRepo.Create(ch); err != nil {
		pkg.InternalError(c, "failed to create channel")
		return
	}
	pkg.Created(c, ch)
}

func (h *AdminHandler) UpdateChannel(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	ch, err := h.ChannelRepo.FindByID(id)
	if err != nil {
		pkg.NotFound(c, "channel not found")
		return
	}
	var req struct {
		Name     string   `json:"name"`
		Type     string   `json:"type"`
		ApiKey   string   `json:"api_key"`
		BaseURL  string   `json:"base_url"`
		Models   []string `json:"models"`
		Status   string   `json:"status"`
		Priority *int     `json:"priority"`
		Weight   *int     `json:"weight"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.BadRequest(c, "invalid request")
		return
	}
	if req.Name != "" { ch.Name = req.Name }
	if req.Type != "" { ch.Type = req.Type }
	if req.BaseURL != "" { ch.BaseURL = req.BaseURL }
	if req.Status != "" { ch.Status = req.Status }
	if req.Priority != nil { ch.Priority = *req.Priority }
	if req.Weight != nil { ch.Weight = *req.Weight }
	if req.ApiKey != "" {
		encrypted, _ := pkg.Encrypt(req.ApiKey, h.Config.EncryptionKey)
		ch.ApiKey = encrypted
	}
	if req.Models != nil {
		modelsJSON, _ := json.Marshal(req.Models)
		ch.Models = modelsJSON
	}
	if err := h.ChannelRepo.Update(ch); err != nil {
		pkg.InternalError(c, "failed to update channel")
		return
	}
	pkg.OK(c, ch)
}

func (h *AdminHandler) DeleteChannel(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.ChannelRepo.Delete(id); err != nil {
		pkg.InternalError(c, "failed to delete channel")
		return
	}
	pkg.OK(c, gin.H{"message": "deleted"})
}

func (h *AdminHandler) TestChannel(c *gin.Context) {
	// Minimal connectivity test: just verify the channel exists and API key decrypts
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	ch, err := h.ChannelRepo.FindByID(id)
	if err != nil {
		pkg.NotFound(c, "channel not found")
		return
	}
	_, err = pkg.Decrypt(ch.ApiKey, h.Config.EncryptionKey)
	if err != nil {
		pkg.InternalError(c, "failed to decrypt API key — encryption key may have changed")
		return
	}
	pkg.OK(c, gin.H{"message": "channel config is valid", "status": ch.Status})
}

// Model config CRUD
func (h *AdminHandler) ListModels(c *gin.Context) {
	models, err := h.ModelRepo.List()
	if err != nil {
		pkg.InternalError(c, "failed to list models")
		return
	}
	pkg.OK(c, models)
}

func (h *AdminHandler) CreateModel(c *gin.Context) {
	var req struct {
		ModelName   string  `json:"model_name" binding:"required"`
		DisplayName string  `json:"display_name" binding:"required"`
		Rate        float64 `json:"rate"`
		InputPrice  float64 `json:"input_price" binding:"required"`
		OutputPrice float64 `json:"output_price" binding:"required"`
		Enabled     *bool   `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.BadRequest(c, "invalid request")
		return
	}
	rate := req.Rate
	if rate <= 0 { rate = 1.0 }
	enabled := true
	if req.Enabled != nil { enabled = *req.Enabled }
	mc := &model.ModelConfig{
		ModelName:   req.ModelName,
		DisplayName: req.DisplayName,
		Rate:        decimal.NewFromFloat(rate),
		InputPrice:  decimal.NewFromFloat(req.InputPrice),
		OutputPrice: decimal.NewFromFloat(req.OutputPrice),
		Enabled:     enabled,
	}
	if err := h.ModelRepo.Create(mc); err != nil {
		pkg.InternalError(c, "failed to create model config")
		return
	}
	pkg.Created(c, mc)
}

func (h *AdminHandler) UpdateModel(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var mc model.ModelConfig
	if err := h.ModelRepo.DB.First(&mc, id).Error; err != nil {
		pkg.NotFound(c, "model not found")
		return
	}
	var req struct {
		DisplayName string   `json:"display_name"`
		Rate        *float64 `json:"rate"`
		InputPrice  *float64 `json:"input_price"`
		OutputPrice *float64 `json:"output_price"`
		Enabled     *bool    `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.BadRequest(c, "invalid request")
		return
	}
	if req.DisplayName != "" { mc.DisplayName = req.DisplayName }
	if req.Rate != nil { mc.Rate = decimal.NewFromFloat(*req.Rate) }
	if req.InputPrice != nil { mc.InputPrice = decimal.NewFromFloat(*req.InputPrice) }
	if req.OutputPrice != nil { mc.OutputPrice = decimal.NewFromFloat(*req.OutputPrice) }
	if req.Enabled != nil { mc.Enabled = *req.Enabled }
	if err := h.ModelRepo.Update(&mc); err != nil {
		pkg.InternalError(c, "failed to update model")
		return
	}
	pkg.OK(c, mc)
}

// Redemption codes
func (h *AdminHandler) ListRedeemCodes(c *gin.Context) {
	page, pageSize := getPage(c)
	codes, total, err := h.RedeemRepo.List(page, pageSize)
	if err != nil {
		pkg.InternalError(c, "failed to list codes")
		return
	}
	pkg.OK(c, gin.H{"data": codes, "total": total, "page": page, "page_size": pageSize})
}

func (h *AdminHandler) CreateRedeemCodes(c *gin.Context) {
	var req struct {
		Amount    float64 `json:"amount" binding:"required,gt=0"`
		Count     int     `json:"count" binding:"required,min=1,max=100"`
		ExpiresAt *string `json:"expires_at"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.BadRequest(c, "amount and count required")
		return
	}
	var expiresAt *time.Time
	if req.ExpiresAt != nil {
		t, err := time.Parse("2006-01-02", *req.ExpiresAt)
		if err == nil {
			expiresAt = &t
		}
	}
	adminID := getUserID(c)
	codes, err := h.BillingService.GenerateRedeemCodes(adminID, req.Amount, req.Count, expiresAt)
	if err != nil {
		pkg.InternalError(c, "failed to generate codes")
		return
	}
	pkg.Created(c, codes)
}

func (h *AdminHandler) UpdateRedeemCode(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.BadRequest(c, "status required")
		return
	}
	var rc model.RedemptionCode
	if err := h.RedeemRepo.DB.First(&rc, id).Error; err != nil {
		pkg.NotFound(c, "code not found")
		return
	}
	rc.Status = req.Status
	if err := h.RedeemRepo.Update(&rc); err != nil {
		pkg.InternalError(c, "failed to update code")
		return
	}
	pkg.OK(c, rc)
}

// Logs
func (h *AdminHandler) ListLogs(c *gin.Context) {
	page, pageSize := getPage(c)
	userID, _ := strconv.ParseInt(c.Query("user_id"), 10, 64)
	modelFilter := c.Query("model")
	logs, total, err := h.RequestLogRepo.ListAll(page, pageSize, userID, modelFilter)
	if err != nil {
		pkg.InternalError(c, "failed to list logs")
		return
	}
	pkg.OK(c, gin.H{"data": logs, "total": total, "page": page, "page_size": pageSize})
}

// Settings
func (h *AdminHandler) GetSettings(c *gin.Context) {
	settings, err := h.AdminService.GetSettings()
	if err != nil {
		pkg.InternalError(c, "failed to get settings")
		return
	}
	pkg.OK(c, settings)
}

func (h *AdminHandler) UpdateSettings(c *gin.Context) {
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.BadRequest(c, "invalid request")
		return
	}
	for key, value := range req {
		h.AdminService.UpdateSetting(key, value)
	}
	pkg.OK(c, gin.H{"message": "settings updated"})
}
```

- [ ] **Step 3: Verify compilation**

```bash
cd /home/colorful/Documents/claude/token/backend
go build ./...
```

- [ ] **Step 4: Commit**

```bash
git add backend/internal/service/admin.go backend/internal/handler/admin.go
git commit -m "feat: add admin service and handlers (users, channels, models, codes, settings)"
```

---

### Task 10: Claude Adapter & Proxy

**Files:**
- Create: `backend/internal/adapter/types.go`
- Create: `backend/internal/adapter/claude.go`
- Create: `backend/internal/adapter/converter.go`
- Create: `backend/internal/service/channel.go`
- Create: `backend/internal/service/proxy.go`
- Create: `backend/internal/handler/proxy.go`

- [ ] **Step 1: Define adapter interfaces**

Create `backend/internal/adapter/types.go`:

```go
package adapter

import (
	"io"
	"net/http"
)

// ProxyResult holds the upstream response info needed for billing
type ProxyResult struct {
	StatusCode       int
	PromptTokens     int
	CompletionTokens int
	Model            string
}

// Adapter forwards requests to an upstream AI provider
type Adapter interface {
	// ProxyRequest forwards the request and streams the response back to the client.
	// It returns token usage after the response is fully sent.
	ProxyRequest(w http.ResponseWriter, body []byte, model, apiKey, baseURL string, stream bool) (*ProxyResult, error)
}

// OpenAI-compatible request/response types for format conversion
type OpenAIChatRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
	Stream      bool            `json:"stream"`
}

type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ClaudeRequest struct {
	Model     string           `json:"model"`
	Messages  []ClaudeMessage  `json:"messages"`
	MaxTokens int              `json:"max_tokens"`
	Stream    bool             `json:"stream"`
	System    string           `json:"system,omitempty"`
}

type ClaudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ClaudeResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Model string `json:"model"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// StreamReader reads SSE events from upstream response
type StreamReader struct {
	Reader io.ReadCloser
}
```

- [ ] **Step 2: Implement Claude adapter**

Create `backend/internal/adapter/claude.go`:

```go
package adapter

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type ClaudeAdapter struct{}

func (a *ClaudeAdapter) ProxyRequest(w http.ResponseWriter, body []byte, model, apiKey, baseURL string, stream bool) (*ProxyResult, error) {
	url := strings.TrimRight(baseURL, "/") + "/v1/messages"

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upstream request failed: %w", err)
	}
	defer resp.Body.Close()

	result := &ProxyResult{StatusCode: resp.StatusCode, Model: model}

	if !stream {
		return a.handleNonStream(w, resp, result)
	}
	return a.handleStream(w, resp, result)
}

func (a *ClaudeAdapter) handleNonStream(w http.ResponseWriter, resp *http.Response, result *ProxyResult) (*ProxyResult, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return result, err
	}

	// Extract usage from response
	if resp.StatusCode == 200 {
		var cr ClaudeResponse
		if json.Unmarshal(body, &cr) == nil {
			result.PromptTokens = cr.Usage.InputTokens
			result.CompletionTokens = cr.Usage.OutputTokens
		}
	}

	// Forward response headers and body
	for k, v := range resp.Header {
		for _, vv := range v {
			w.Header().Set(k, vv)
		}
	}
	w.WriteHeader(resp.StatusCode)
	w.Write(body)
	return result, nil
}

func (a *ClaudeAdapter) handleStream(w http.ResponseWriter, resp *http.Response, result *ProxyResult) (*ProxyResult, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return result, fmt.Errorf("streaming not supported")
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(resp.StatusCode)

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		fmt.Fprintf(w, "%s\n", line)
		flusher.Flush()

		// Parse data lines for usage info
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			var event struct {
				Type  string `json:"type"`
				Usage struct {
					InputTokens  int `json:"input_tokens"`
					OutputTokens int `json:"output_tokens"`
				} `json:"usage"`
			}
			if json.Unmarshal([]byte(data), &event) == nil {
				if event.Type == "message_delta" || event.Type == "message_stop" {
					if event.Usage.InputTokens > 0 {
						result.PromptTokens = event.Usage.InputTokens
					}
					if event.Usage.OutputTokens > 0 {
						result.CompletionTokens = event.Usage.OutputTokens
					}
				}
			}
		}
	}
	return result, scanner.Err()
}
```

- [ ] **Step 3: Implement OpenAI ↔ Claude converter**

Create `backend/internal/adapter/converter.go`:

```go
package adapter

import "encoding/json"

// OpenAIToClaude converts an OpenAI chat completion request to Claude format
func OpenAIToClaude(openaiBody []byte) ([]byte, string, error) {
	var req OpenAIChatRequest
	if err := json.Unmarshal(openaiBody, &req); err != nil {
		return nil, "", err
	}

	claudeReq := ClaudeRequest{
		Model:     req.Model,
		MaxTokens: req.MaxTokens,
		Stream:    req.Stream,
	}
	if claudeReq.MaxTokens == 0 {
		claudeReq.MaxTokens = 4096
	}

	for _, msg := range req.Messages {
		if msg.Role == "system" {
			claudeReq.System = msg.Content
			continue
		}
		role := msg.Role
		if role == "assistant" {
			role = "assistant"
		}
		claudeReq.Messages = append(claudeReq.Messages, ClaudeMessage{
			Role:    role,
			Content: msg.Content,
		})
	}

	body, err := json.Marshal(claudeReq)
	return body, req.Model, err
}

// ClaudeToOpenAIResponse wraps a Claude response in OpenAI format (non-stream)
func ClaudeToOpenAIResponse(claudeBody []byte, model string) ([]byte, error) {
	var cr ClaudeResponse
	if err := json.Unmarshal(claudeBody, &cr); err != nil {
		return nil, err
	}

	text := ""
	for _, c := range cr.Content {
		if c.Type == "text" {
			text += c.Text
		}
	}

	openaiResp := map[string]any{
		"id":      cr.ID,
		"object":  "chat.completion",
		"model":   model,
		"choices": []map[string]any{
			{
				"index": 0,
				"message": map[string]any{
					"role":    "assistant",
					"content": text,
				},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]any{
			"prompt_tokens":     cr.Usage.InputTokens,
			"completion_tokens": cr.Usage.OutputTokens,
			"total_tokens":      cr.Usage.InputTokens + cr.Usage.OutputTokens,
		},
	}
	return json.Marshal(openaiResp)
}
```

- [ ] **Step 4: Create channel selection service**

Create `backend/internal/service/channel.go`:

```go
package service

import (
	"errors"
	"math/rand"

	"ai-relay/internal/config"
	"ai-relay/internal/model"
	"ai-relay/internal/pkg"
	"ai-relay/internal/repository"
)

type ChannelService struct {
	Repo   *repository.ChannelRepo
	Config *config.Config
}

// SelectChannel picks the best available channel for a model using priority + weighted random
func (s *ChannelService) SelectChannel(modelName string) (*model.Channel, string, error) {
	channels, err := s.Repo.FindActiveByModel(modelName)
	if err != nil || len(channels) == 0 {
		return nil, "", errors.New("no available channel for model: " + modelName)
	}

	// Group by priority, pick from highest priority group
	bestPriority := channels[0].Priority
	var candidates []model.Channel
	for _, ch := range channels {
		if ch.Priority == bestPriority {
			candidates = append(candidates, ch)
		}
	}

	// Weighted random selection among same-priority channels
	selected := weightedSelect(candidates)

	// Decrypt the upstream API key
	apiKey, err := pkg.Decrypt(selected.ApiKey, s.Config.EncryptionKey)
	if err != nil {
		return nil, "", errors.New("failed to decrypt channel API key")
	}

	return &selected, apiKey, nil
}

func weightedSelect(channels []model.Channel) model.Channel {
	if len(channels) == 1 {
		return channels[0]
	}
	totalWeight := 0
	for _, ch := range channels {
		totalWeight += ch.Weight
	}
	r := rand.Intn(totalWeight)
	for _, ch := range channels {
		r -= ch.Weight
		if r < 0 {
			return ch
		}
	}
	return channels[0]
}
```

- [ ] **Step 5: Create proxy service**

Create `backend/internal/service/proxy.go`:

```go
package service

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"ai-relay/internal/adapter"
	"ai-relay/internal/model"
	"ai-relay/internal/repository"

	"github.com/shopspring/decimal"
)

type ProxyService struct {
	ChannelService *ChannelService
	BillingService *BillingService
	RequestLogRepo *repository.RequestLogRepo
	Adapters       map[string]adapter.Adapter // "claude" -> ClaudeAdapter
}

type ProxyRequest struct {
	UserID   int64
	ApiKeyID int64
	Model    string
	Body     []byte
	Stream   bool
	Type     string // "native" or "openai_compat"
	IP       string
}

func (s *ProxyService) HandleProxy(w http.ResponseWriter, pr *ProxyRequest) error {
	start := time.Now()

	// Select channel
	channel, apiKey, err := s.ChannelService.SelectChannel(pr.Model)
	if err != nil {
		return err
	}

	// Get adapter for channel type
	adp, ok := s.Adapters[channel.Type]
	if !ok {
		return fmt.Errorf("no adapter for channel type: %s", channel.Type)
	}

	// Forward request
	result, err := adp.ProxyRequest(w, pr.Body, pr.Model, apiKey, channel.BaseURL, pr.Stream)

	duration := time.Since(start).Milliseconds()

	// Log and bill asynchronously
	go func() {
		status := "success"
		if err != nil || (result != nil && result.StatusCode >= 400) {
			status = "error"
		}

		promptTokens := 0
		completionTokens := 0
		if result != nil {
			promptTokens = result.PromptTokens
			completionTokens = result.CompletionTokens
		}

		// Calculate cost
		cost, _ := s.BillingService.CalculateCost(pr.Model, promptTokens, completionTokens)
		upstreamCost := decimal.Zero // Could be calculated based on actual upstream pricing

		// Save request log
		rl := &model.RequestLog{
			UserID:           pr.UserID,
			ApiKeyID:         pr.ApiKeyID,
			ChannelID:        channel.ID,
			Model:            pr.Model,
			Type:             pr.Type,
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
			Cost:             cost,
			UpstreamCost:     upstreamCost,
			Status:           status,
			DurationMs:       int(duration),
			IPAddress:        pr.IP,
		}
		if err := s.RequestLogRepo.Create(rl); err != nil {
			log.Printf("Failed to create request log: %v", err)
			return
		}

		// Deduct balance
		if status == "success" && cost.GreaterThan(decimal.Zero) {
			desc := fmt.Sprintf("%s: %d tok in + %d tok out", pr.Model, promptTokens, completionTokens)
			if err := s.BillingService.DeductBalance(pr.UserID, cost, rl.ID, desc); err != nil {
				log.Printf("Failed to deduct balance for user %d: %v", pr.UserID, err)
			}
		}
	}()

	return err
}

// ExtractModel gets the model name from a request body
func ExtractModel(body []byte) string {
	var req struct {
		Model string `json:"model"`
	}
	json.Unmarshal(body, &req)
	return req.Model
}

// ExtractStream checks if the request wants streaming
func ExtractStream(body []byte) bool {
	var req struct {
		Stream bool `json:"stream"`
	}
	json.Unmarshal(body, &req)
	return req.Stream
}
```

- [ ] **Step 6: Create proxy handler**

Create `backend/internal/handler/proxy.go`:

```go
package handler

import (
	"io"
	"net/http"

	"ai-relay/internal/adapter"
	"ai-relay/internal/pkg"
	"ai-relay/internal/repository"
	"ai-relay/internal/service"

	"github.com/gin-gonic/gin"
)

type ProxyHandler struct {
	ProxyService *service.ProxyService
	ModelRepo    *repository.ModelConfigRepo
}

// NativeMessages handles POST /v1/messages (Claude native format)
func (h *ProxyHandler) NativeMessages(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(400, gin.H{"error": gin.H{"type": "invalid_request_error", "message": "failed to read body"}})
		return
	}

	model := service.ExtractModel(body)
	if model == "" {
		c.JSON(400, gin.H{"error": gin.H{"type": "invalid_request_error", "message": "model is required"}})
		return
	}

	userID, _ := c.Get("user_id")
	apiKeyID, _ := c.Get("api_key_id")

	pr := &service.ProxyRequest{
		UserID:   userID.(int64),
		ApiKeyID: apiKeyID.(int64),
		Model:    model,
		Body:     body,
		Stream:   service.ExtractStream(body),
		Type:     "native",
		IP:       c.ClientIP(),
	}

	if err := h.ProxyService.HandleProxy(c.Writer, pr); err != nil {
		c.JSON(502, gin.H{"error": gin.H{"type": "api_error", "message": err.Error()}})
	}
}

// ChatCompletions handles POST /v1/chat/completions (OpenAI compatible format)
func (h *ProxyHandler) ChatCompletions(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(400, gin.H{"error": gin.H{"message": "failed to read body"}})
		return
	}

	// Convert OpenAI format to Claude format
	claudeBody, model, err := adapter.OpenAIToClaude(body)
	if err != nil {
		c.JSON(400, gin.H{"error": gin.H{"message": "invalid request format"}})
		return
	}

	userID, _ := c.Get("user_id")
	apiKeyID, _ := c.Get("api_key_id")

	pr := &service.ProxyRequest{
		UserID:   userID.(int64),
		ApiKeyID: apiKeyID.(int64),
		Model:    model,
		Body:     claudeBody,
		Stream:   service.ExtractStream(body),
		Type:     "openai_compat",
		IP:       c.ClientIP(),
	}

	if err := h.ProxyService.HandleProxy(c.Writer, pr); err != nil {
		c.JSON(502, gin.H{"error": gin.H{"message": err.Error()}})
	}
}

// ListModels handles GET /v1/models
func (h *ProxyHandler) ListModels(c *gin.Context) {
	models, err := h.ModelRepo.ListEnabled()
	if err != nil {
		c.JSON(500, gin.H{"error": gin.H{"message": "failed to list models"}})
		return
	}

	data := make([]map[string]any, len(models))
	for i, m := range models {
		data[i] = map[string]any{
			"id":       m.ModelName,
			"object":   "model",
			"owned_by": "ai-relay",
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   data,
	})
}
```

- [ ] **Step 7: Verify compilation**

```bash
cd /home/colorful/Documents/claude/token/backend
go build ./...
```

- [ ] **Step 8: Commit**

```bash
git add backend/internal/adapter/ backend/internal/service/channel.go backend/internal/service/proxy.go backend/internal/handler/proxy.go
git commit -m "feat: add Claude adapter, OpenAI converter, channel selection, and proxy handler"
```

---

### Task 11: Wire Up All Routes in main.go

**Files:**
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: Complete main.go with all routes and dependency wiring**

Replace `backend/cmd/server/main.go` with the full version:

```go
package main

import (
	"context"
	"log"

	"ai-relay/internal/adapter"
	"ai-relay/internal/config"
	"ai-relay/internal/handler"
	"ai-relay/internal/middleware"
	"ai-relay/internal/model"
	"ai-relay/internal/repository"
	"ai-relay/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	cfg := config.Load()

	// Database
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect database: %v", err)
	}
	db.AutoMigrate(
		&model.User{}, &model.ApiKey{}, &model.Channel{},
		&model.ModelConfig{}, &model.RequestLog{}, &model.BalanceLog{},
		&model.RedemptionCode{}, &model.Setting{},
	)
	log.Println("Database migrated")

	// Redis
	opt, err := redis.ParseURL(cfg.RedisURL)
	var rdb *redis.Client
	if err == nil {
		rdb = redis.NewClient(opt)
		if err := rdb.Ping(context.Background()).Err(); err != nil {
			log.Printf("Redis not available, rate limiting disabled: %v", err)
			rdb = nil
		} else {
			log.Println("Redis connected")
		}
	}

	// Repositories
	userRepo := &repository.UserRepo{DB: db}
	apiKeyRepo := &repository.ApiKeyRepo{DB: db}
	channelRepo := &repository.ChannelRepo{DB: db}
	modelRepo := &repository.ModelConfigRepo{DB: db}
	requestLogRepo := &repository.RequestLogRepo{DB: db}
	balanceLogRepo := &repository.BalanceLogRepo{DB: db}
	redeemRepo := &repository.RedemptionCodeRepo{DB: db}
	settingRepo := &repository.SettingRepo{DB: db}

	// Services
	authService := &service.AuthService{UserRepo: userRepo, SettingRepo: settingRepo, Config: cfg}
	userService := &service.UserService{UserRepo: userRepo, RequestLogRepo: requestLogRepo}
	apiKeyService := &service.ApiKeyService{Repo: apiKeyRepo}
	channelService := &service.ChannelService{Repo: channelRepo, Config: cfg}
	billingService := &service.BillingService{
		DB: db, UserRepo: userRepo, BalanceLogRepo: balanceLogRepo,
		RedeemRepo: redeemRepo, ModelRepo: modelRepo, RequestLogRepo: requestLogRepo,
	}
	adminService := &service.AdminService{
		UserRepo: userRepo, ChannelRepo: channelRepo, ModelRepo: modelRepo,
		RequestLogRepo: requestLogRepo, SettingRepo: settingRepo,
	}
	proxyService := &service.ProxyService{
		ChannelService: channelService, BillingService: billingService,
		RequestLogRepo: requestLogRepo,
		Adapters: map[string]adapter.Adapter{
			"claude": &adapter.ClaudeAdapter{},
		},
	}

	// Handlers
	authHandler := &handler.AuthHandler{AuthService: authService}
	userHandler := &handler.UserHandler{
		UserService: userService, ApiKeyService: apiKeyService,
		BillingService: billingService,
		RequestLogRepo: requestLogRepo, BalanceLogRepo: balanceLogRepo,
	}
	adminHandler := &handler.AdminHandler{
		AdminService: adminService, BillingService: billingService,
		ChannelRepo: channelRepo, ModelRepo: modelRepo,
		RedeemRepo: redeemRepo, RequestLogRepo: requestLogRepo, Config: cfg,
	}
	proxyHandler := &handler.ProxyHandler{ProxyService: proxyService, ModelRepo: modelRepo}

	// Router
	r := gin.Default()
	r.Use(middleware.CORS(cfg.AllowedOrigins))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Auth routes (public)
	auth := r.Group("/api/auth")
	{
		auth.POST("/register", authHandler.Register)
		auth.POST("/login", authHandler.Login)
		auth.GET("/google", authHandler.GoogleRedirect)
		auth.GET("/google/callback", authHandler.GoogleCallback)
		auth.POST("/refresh", authHandler.Refresh)
	}

	// User routes (JWT auth)
	user := r.Group("/api/user")
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

	// Admin routes (JWT auth + admin role)
	admin := r.Group("/api/admin")
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

	// Proxy routes (API key auth)
	v1 := r.Group("/v1")
	v1.Use(middleware.APIKeyAuth(db))
	if rdb != nil {
		v1.Use(middleware.RateLimit(rdb, 60, 60_000_000_000)) // 60 req/min
	}
	{
		v1.POST("/messages", proxyHandler.NativeMessages)
		v1.POST("/chat/completions", proxyHandler.ChatCompletions)
		v1.GET("/models", proxyHandler.ListModels)
	}

	log.Printf("Starting server on :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
```

- [ ] **Step 2: Fix any compilation issues**

```bash
cd /home/colorful/Documents/claude/token/backend
go build ./...
```

Note: The `UserHandler` uses interfaces for `RequestLogRepo` and `BalanceLogRepo`. Adjust the handler struct if needed to use concrete types `*repository.RequestLogRepo` and `*repository.BalanceLogRepo` for the `ListByUser` calls.

- [ ] **Step 3: Verify full backend starts**

```bash
cd /home/colorful/Documents/claude/token
make deps
sleep 3
cd backend && go run cmd/server/main.go &
sleep 3
curl http://localhost:8080/health
# Test auth
curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"test@test.com","password":"test123"}'
# Expected: 201 with access_token and refresh_token
kill %1
```

- [ ] **Step 4: Commit**

```bash
git add backend/cmd/server/main.go
git commit -m "feat: wire up all routes, services, and middleware in main.go"
```

---

## Phase 4: React Frontend

### Task 12: Initialize React Project

**Files:**
- Create: `frontend/` (via Vite scaffold)
- Create: `frontend/src/styles/global.css`
- Create: `frontend/src/styles/theme.ts`

- [ ] **Step 1: Scaffold React project with Vite**

```bash
cd /home/colorful/Documents/claude/token
npm create vite@latest frontend -- --template react-ts
cd frontend
npm install
npm install antd @ant-design/icons axios zustand react-router-dom
```

- [ ] **Step 2: Create terminal-style global CSS**

Create `frontend/src/styles/global.css`:

```css
@import url('https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;500;600;700&display=swap');

:root {
  --bg-primary: #f5f5f0;
  --bg-card: #ffffff;
  --bg-sidebar: #fafaf7;
  --bg-code: #1a1a1a;
  --text-primary: #1a1a1a;
  --text-secondary: #666666;
  --text-muted: #999999;
  --accent-green: #0a8c2d;
  --accent-red: #cc0000;
  --accent-warn: #b8860b;
  --border-color: #1a1a1a;
  --border-light: #dddddd;
  --font-mono: 'JetBrains Mono', 'LXGW WenKai Mono', 'Consolas', monospace;
}

* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

body {
  font-family: var(--font-mono);
  font-size: 13px;
  line-height: 1.6;
  color: var(--text-primary);
  background: var(--bg-primary);
  -webkit-font-smoothing: antialiased;
}

/* Override Ant Design border-radius globally */
.ant-btn,
.ant-input,
.ant-select-selector,
.ant-table,
.ant-card,
.ant-modal-content,
.ant-dropdown-menu,
.ant-tag,
.ant-badge-count,
.ant-pagination-item {
  border-radius: 0 !important;
}

/* Terminal-style scrollbar */
::-webkit-scrollbar {
  width: 6px;
  height: 6px;
}
::-webkit-scrollbar-track {
  background: var(--bg-primary);
}
::-webkit-scrollbar-thumb {
  background: var(--border-light);
}
::-webkit-scrollbar-thumb:hover {
  background: var(--text-muted);
}
```

- [ ] **Step 3: Create Ant Design theme override**

Create `frontend/src/styles/theme.ts`:

```typescript
import type { ThemeConfig } from 'antd';

export const theme: ThemeConfig = {
  token: {
    fontFamily: "'JetBrains Mono', 'LXGW WenKai Mono', 'Consolas', monospace",
    fontSize: 13,
    colorPrimary: '#1a1a1a',
    colorSuccess: '#0a8c2d',
    colorError: '#cc0000',
    colorWarning: '#b8860b',
    borderRadius: 0,
    colorBgContainer: '#ffffff',
    colorBgLayout: '#f5f5f0',
    colorBorder: '#1a1a1a',
    colorText: '#1a1a1a',
    colorTextSecondary: '#666666',
  },
  components: {
    Button: {
      borderRadius: 0,
    },
    Input: {
      borderRadius: 0,
    },
    Table: {
      borderRadius: 0,
      headerBg: '#fafaf7',
    },
    Card: {
      borderRadius: 0,
    },
    Menu: {
      borderRadius: 0,
      itemBorderRadius: 0,
    },
  },
};
```

- [ ] **Step 4: Update vite.config.ts with API proxy**

Replace `frontend/vite.config.ts`:

```typescript
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://localhost:8080',
      '/v1': 'http://localhost:8080',
    },
  },
})
```

- [ ] **Step 5: Verify dev server starts**

```bash
cd /home/colorful/Documents/claude/token/frontend
npm run dev &
sleep 3
curl -s http://localhost:5173 | head -5
# Expected: HTML output
kill %1
```

- [ ] **Step 6: Commit**

```bash
git add frontend/
git commit -m "feat: scaffold React frontend with terminal theme and Ant Design"
```

---

### Task 13: API Client, Auth Store, Router

**Files:**
- Create: `frontend/src/api/client.ts`
- Create: `frontend/src/api/auth.ts`
- Create: `frontend/src/api/user.ts`
- Create: `frontend/src/api/admin.ts`
- Create: `frontend/src/store/auth.ts`
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/main.tsx`

- [ ] **Step 1: Create axios client with JWT interceptor**

Create `frontend/src/api/client.ts`:

```typescript
import axios from 'axios';

const client = axios.create({
  baseURL: '/api',
});

client.interceptors.request.use((config) => {
  const token = localStorage.getItem('access_token');
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

client.interceptors.response.use(
  (response) => response,
  async (error) => {
    if (error.response?.status === 401) {
      const refreshToken = localStorage.getItem('refresh_token');
      if (refreshToken) {
        try {
          const res = await axios.post('/api/auth/refresh', { refresh_token: refreshToken });
          const { access_token, refresh_token } = res.data.data;
          localStorage.setItem('access_token', access_token);
          localStorage.setItem('refresh_token', refresh_token);
          error.config.headers.Authorization = `Bearer ${access_token}`;
          return axios(error.config);
        } catch {
          localStorage.clear();
          window.location.href = '/login';
        }
      } else {
        localStorage.clear();
        window.location.href = '/login';
      }
    }
    return Promise.reject(error);
  }
);

export default client;
```

- [ ] **Step 2: Create API modules**

Create `frontend/src/api/auth.ts`:

```typescript
import client from './client';

export const authAPI = {
  register: (email: string, password: string) =>
    client.post('/auth/register', { email, password }),
  login: (email: string, password: string) =>
    client.post('/auth/login', { email, password }),
  refresh: (refreshToken: string) =>
    client.post('/auth/refresh', { refresh_token: refreshToken }),
};
```

Create `frontend/src/api/user.ts`:

```typescript
import client from './client';

export const userAPI = {
  getProfile: () => client.get('/user/profile'),
  changePassword: (oldPassword: string, newPassword: string) =>
    client.put('/user/password', { old_password: oldPassword, new_password: newPassword }),
  getDashboard: () => client.get('/user/dashboard'),
  listApiKeys: () => client.get('/user/api-keys'),
  createApiKey: (name: string) => client.post('/user/api-keys', { name }),
  deleteApiKey: (id: number) => client.delete(`/user/api-keys/${id}`),
  updateApiKey: (id: number, status: string) => client.put(`/user/api-keys/${id}`, { status }),
  listLogs: (page: number, pageSize: number) =>
    client.get('/user/logs', { params: { page, page_size: pageSize } }),
  listBalanceLogs: (page: number, pageSize: number) =>
    client.get('/user/balance-logs', { params: { page, page_size: pageSize } }),
  redeem: (code: string) => client.post('/user/redeem', { code }),
};
```

Create `frontend/src/api/admin.ts`:

```typescript
import client from './client';

export const adminAPI = {
  getDashboard: () => client.get('/admin/dashboard'),
  listUsers: (page: number, pageSize: number, search?: string) =>
    client.get('/admin/users', { params: { page, page_size: pageSize, search } }),
  updateUser: (id: number, data: { role?: string; status?: string }) =>
    client.put(`/admin/users/${id}`, data),
  topUp: (id: number, amount: number) =>
    client.post(`/admin/users/${id}/topup`, { amount }),
  listChannels: () => client.get('/admin/channels'),
  createChannel: (data: any) => client.post('/admin/channels', data),
  updateChannel: (id: number, data: any) => client.put(`/admin/channels/${id}`, data),
  deleteChannel: (id: number) => client.delete(`/admin/channels/${id}`),
  testChannel: (id: number) => client.post(`/admin/channels/${id}/test`),
  listModels: () => client.get('/admin/models'),
  createModel: (data: any) => client.post('/admin/models', data),
  updateModel: (id: number, data: any) => client.put(`/admin/models/${id}`, data),
  listRedeemCodes: (page: number, pageSize: number) =>
    client.get('/admin/redeem-codes', { params: { page, page_size: pageSize } }),
  createRedeemCodes: (amount: number, count: number, expiresAt?: string) =>
    client.post('/admin/redeem-codes', { amount, count, expires_at: expiresAt }),
  updateRedeemCode: (id: number, status: string) =>
    client.put(`/admin/redeem-codes/${id}`, { status }),
  listLogs: (page: number, pageSize: number, userId?: number, model?: string) =>
    client.get('/admin/logs', { params: { page, page_size: pageSize, user_id: userId, model } }),
  getSettings: () => client.get('/admin/settings'),
  updateSettings: (data: Record<string, string>) => client.put('/admin/settings', data),
};
```

- [ ] **Step 3: Create auth store**

Create `frontend/src/store/auth.ts`:

```typescript
import { create } from 'zustand';

interface User {
  id: number;
  email: string;
  role: string;
  balance: string;
}

interface AuthState {
  user: User | null;
  isAuthenticated: boolean;
  login: (accessToken: string, refreshToken: string) => void;
  logout: () => void;
  setUser: (user: User) => void;
}

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  isAuthenticated: !!localStorage.getItem('access_token'),
  login: (accessToken, refreshToken) => {
    localStorage.setItem('access_token', accessToken);
    localStorage.setItem('refresh_token', refreshToken);
    set({ isAuthenticated: true });
  },
  logout: () => {
    localStorage.removeItem('access_token');
    localStorage.removeItem('refresh_token');
    set({ user: null, isAuthenticated: false });
  },
  setUser: (user) => set({ user }),
}));
```

- [ ] **Step 4: Set up router in App.tsx**

Replace `frontend/src/App.tsx`:

```tsx
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { ConfigProvider } from 'antd';
import { theme } from './styles/theme';
import { useAuthStore } from './store/auth';

// Auth pages
import Login from './pages/auth/Login';
import Register from './pages/auth/Register';

// Layouts
import UserLayout from './layouts/UserLayout';
import AdminLayout from './layouts/AdminLayout';

// User pages
import Overview from './pages/user/Overview';
import ApiKeys from './pages/user/ApiKeys';
import UsageLogs from './pages/user/UsageLogs';
import TopUp from './pages/user/TopUp';
import Balance from './pages/user/Balance';
import Docs from './pages/user/Docs';
import UserSettings from './pages/user/Settings';

// Admin pages
import Dashboard from './pages/admin/Dashboard';
import Users from './pages/admin/Users';
import Channels from './pages/admin/Channels';
import Models from './pages/admin/Models';
import RedeemCodes from './pages/admin/RedeemCodes';
import Logs from './pages/admin/Logs';
import AdminSettings from './pages/admin/Settings';

function PrivateRoute({ children }: { children: React.ReactNode }) {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);
  return isAuthenticated ? <>{children}</> : <Navigate to="/login" />;
}

function AdminRoute({ children }: { children: React.ReactNode }) {
  const user = useAuthStore((s) => s.user);
  if (user && user.role !== 'admin') return <Navigate to="/user" />;
  return <>{children}</>;
}

export default function App() {
  return (
    <ConfigProvider theme={theme}>
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route path="/register" element={<Register />} />

          <Route path="/user" element={<PrivateRoute><UserLayout /></PrivateRoute>}>
            <Route index element={<Overview />} />
            <Route path="api-keys" element={<ApiKeys />} />
            <Route path="logs" element={<UsageLogs />} />
            <Route path="top-up" element={<TopUp />} />
            <Route path="balance" element={<Balance />} />
            <Route path="docs" element={<Docs />} />
            <Route path="settings" element={<UserSettings />} />
          </Route>

          <Route path="/admin" element={<PrivateRoute><AdminRoute><AdminLayout /></AdminRoute></PrivateRoute>}>
            <Route index element={<Dashboard />} />
            <Route path="users" element={<Users />} />
            <Route path="channels" element={<Channels />} />
            <Route path="models" element={<Models />} />
            <Route path="redeem-codes" element={<RedeemCodes />} />
            <Route path="logs" element={<Logs />} />
            <Route path="settings" element={<AdminSettings />} />
          </Route>

          <Route path="*" element={<Navigate to="/login" />} />
        </Routes>
      </BrowserRouter>
    </ConfigProvider>
  );
}
```

- [ ] **Step 5: Update main.tsx**

Replace `frontend/src/main.tsx`:

```tsx
import React from 'react';
import ReactDOM from 'react-dom/client';
import App from './App';
import './styles/global.css';

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
```

- [ ] **Step 6: Commit**

```bash
git add frontend/src/
git commit -m "feat: add API client, auth store, and router setup"
```

---

### Task 14: Layouts & Shared Components

**Files:**
- Create: `frontend/src/layouts/UserLayout.tsx`
- Create: `frontend/src/layouts/AdminLayout.tsx`
- Create: `frontend/src/components/StatCard.tsx`
- Create: `frontend/src/components/CodeBlock.tsx`

- [ ] **Step 1: Create terminal-style layouts**

These are the sidebar + content layouts following the nof1.ai terminal aesthetic. Both layouts fetch user profile on mount. See spec section 6.1 for the style guide.

Create `frontend/src/layouts/UserLayout.tsx`, `frontend/src/layouts/AdminLayout.tsx`, `frontend/src/components/StatCard.tsx`, `frontend/src/components/CodeBlock.tsx`.

Key implementation details:
- Sidebar with `var(--bg-sidebar)` background, `1px solid var(--border-color)` right border
- Menu items with `border-left: 3px solid transparent`, active state `border-left: 3px solid var(--text-primary)`
- Title prefix `//` in headings
- StatCard: border `1px solid var(--border-color)`, green value color, uppercase label
- CodeBlock: `var(--bg-code)` background, light text, monospace

Each file should be created with the full implementation. The layouts use `Outlet` from react-router-dom for nested routes and `useAuthStore` to load user data.

- [ ] **Step 2: Verify compilation**

```bash
cd /home/colorful/Documents/claude/token/frontend
npx tsc --noEmit
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/layouts/ frontend/src/components/
git commit -m "feat: add terminal-style layouts and shared components"
```

---

### Task 15: Auth Pages (Login & Register)

**Files:**
- Create: `frontend/src/pages/auth/Login.tsx`
- Create: `frontend/src/pages/auth/Register.tsx`

- [ ] **Step 1: Create login and register pages**

Both pages follow the terminal aesthetic: centered card with black border, no border-radius, monospace font, `//` title prefix. Login includes email/password form + Google OAuth button. Register has email/password/confirm password.

On successful auth, call `useAuthStore.login()` with tokens and navigate to `/user`.

- [ ] **Step 2: Verify in browser**

```bash
cd /home/colorful/Documents/claude/token/frontend
npm run dev
# Open http://localhost:5173/login in browser and verify the page renders
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/pages/auth/
git commit -m "feat: add login and register pages with terminal style"
```

---

### Task 16: User Panel Pages

**Files:**
- Create: `frontend/src/pages/user/Overview.tsx`
- Create: `frontend/src/pages/user/ApiKeys.tsx`
- Create: `frontend/src/pages/user/UsageLogs.tsx`
- Create: `frontend/src/pages/user/TopUp.tsx`
- Create: `frontend/src/pages/user/Balance.tsx`
- Create: `frontend/src/pages/user/Docs.tsx`
- Create: `frontend/src/pages/user/Settings.tsx`

- [ ] **Step 1: Create all 7 user pages**

Each page uses Ant Design Table/Form/Card with terminal theme overrides. Key pages:

- **Overview**: StatCard row (balance, today requests, today tokens) + CodeBlock quick start + ApiKey summary table
- **ApiKeys**: Table with create modal, mask keys for display (show full only on creation), copy button, status toggle
- **UsageLogs**: Table with model, tokens, cost, time columns, pagination
- **TopUp**: Single input field for redeem code + submit button
- **Balance**: Table of balance_logs with type, amount, balance_after, description, time
- **Docs**: Static page with CodeBlock examples for Claude Code, OpenAI SDK, curl
- **Settings**: Password change form, Google account bind/unbind button

- [ ] **Step 2: Verify all pages render**

```bash
cd /home/colorful/Documents/claude/token/frontend
npm run dev
# Navigate through all user pages in browser
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/pages/user/
git commit -m "feat: add all user panel pages"
```

---

### Task 17: Admin Panel Pages

**Files:**
- Create: `frontend/src/pages/admin/Dashboard.tsx`
- Create: `frontend/src/pages/admin/Users.tsx`
- Create: `frontend/src/pages/admin/Channels.tsx`
- Create: `frontend/src/pages/admin/Models.tsx`
- Create: `frontend/src/pages/admin/RedeemCodes.tsx`
- Create: `frontend/src/pages/admin/Logs.tsx`
- Create: `frontend/src/pages/admin/Settings.tsx`

- [ ] **Step 1: Create all 7 admin pages**

Key pages:

- **Dashboard**: 4 StatCards (users, requests, tokens, revenue) + activity table
- **Users**: Searchable table, ban/unban button, top-up modal with amount input
- **Channels**: Table with create/edit modal (name, type, API key, base URL, models, priority, weight), test button, delete
- **Models**: Table with inline edit for rate/input_price/output_price, enable/disable toggle
- **RedeemCodes**: Generate modal (amount, count, expiry) + codes table with status
- **Logs**: Table with user_id/model filters, pagination
- **Settings**: Key-value form for site_name, register_enabled, default_balance

- [ ] **Step 2: Verify all pages render**

```bash
cd /home/colorful/Documents/claude/token/frontend
npm run dev
# Navigate through all admin pages in browser
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/pages/admin/
git commit -m "feat: add all admin panel pages"
```

---

## Phase 5: Internationalization (i18n)

### Task 18: Add i18n with react-i18next

**Files:**
- Create: `frontend/src/i18n.ts`
- Create: `frontend/src/locales/en.json`
- Create: `frontend/src/locales/zh.json`
- Modify: `frontend/src/main.tsx`
- Create: `frontend/src/components/LanguageSwitch.tsx`

- [ ] **Step 1: Install i18n dependencies**

```bash
cd /home/colorful/Documents/claude/token/frontend
npm install react-i18next i18next i18next-browser-languagedetector
```

- [ ] **Step 2: Create i18n config**

Create `frontend/src/i18n.ts`:

```typescript
import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import LanguageDetector from 'i18next-browser-languagedetector';
import en from './locales/en.json';
import zh from './locales/zh.json';

i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources: { en: { translation: en }, zh: { translation: zh } },
    fallbackLng: 'en',
    interpolation: { escapeValue: false },
  });

export default i18n;
```

- [ ] **Step 3: Create translation files**

Create `frontend/src/locales/en.json`:

```json
{
  "common": {
    "loading": "Loading...",
    "save": "Save",
    "cancel": "Cancel",
    "delete": "Delete",
    "create": "Create",
    "edit": "Edit",
    "search": "Search",
    "confirm": "Confirm",
    "status": "Status",
    "actions": "Actions",
    "active": "Active",
    "disabled": "Disabled",
    "banned": "Banned",
    "copy": "Copy",
    "copied": "Copied"
  },
  "auth": {
    "login": "Login",
    "register": "Register",
    "email": "Email",
    "password": "Password",
    "confirmPassword": "Confirm Password",
    "loginWithGoogle": "Login with Google",
    "noAccount": "No account?",
    "hasAccount": "Already have an account?",
    "logout": "Logout"
  },
  "nav": {
    "dashboard": "dashboard",
    "overview": "overview",
    "users": "users",
    "channels": "channels",
    "models": "models",
    "apiKeys": "api_keys",
    "usageLogs": "usage_logs",
    "topUp": "top_up",
    "balance": "balance",
    "docs": "docs",
    "settings": "settings",
    "redeemCodes": "redeem_codes",
    "logs": "logs"
  },
  "dashboard": {
    "totalUsers": "Total Users",
    "todayRequests": "Requests Today",
    "todayTokens": "Tokens Today",
    "todayRevenue": "Revenue Today",
    "currentBalance": "Balance",
    "todayCost": "Cost Today",
    "recentActivity": "Recent Activity"
  },
  "apiKeys": {
    "title": "API Keys",
    "newKey": "+ new",
    "keyName": "Key Name",
    "key": "Key",
    "createdAt": "Created",
    "lastUsed": "Last Used",
    "createTitle": "Create API Key",
    "deleteConfirm": "Delete this API key?"
  },
  "channels": {
    "title": "Channels",
    "name": "Name",
    "type": "Type",
    "baseUrl": "Base URL",
    "models": "Models",
    "priority": "Priority",
    "weight": "Weight",
    "test": "Test",
    "testSuccess": "Channel is working",
    "apiKey": "Upstream API Key"
  },
  "billing": {
    "amount": "Amount",
    "redeemCode": "Redeem Code",
    "redeem": "Redeem",
    "redeemSuccess": "Redeemed successfully",
    "generate": "Generate",
    "count": "Count",
    "expiresAt": "Expires",
    "topUp": "Top Up",
    "topUpUser": "Top up user"
  },
  "logs": {
    "model": "Model",
    "promptTokens": "Input Tokens",
    "completionTokens": "Output Tokens",
    "totalTokens": "Total Tokens",
    "cost": "Cost",
    "duration": "Duration",
    "time": "Time",
    "type": "Type",
    "description": "Description",
    "balanceAfter": "Balance After"
  },
  "docs": {
    "title": "Integration Guide",
    "claudeCode": "Claude Code",
    "openaiSdk": "OpenAI SDK (Python)",
    "curl": "curl"
  },
  "settings": {
    "changePassword": "Change Password",
    "oldPassword": "Current Password",
    "newPassword": "New Password",
    "siteName": "Site Name",
    "registerEnabled": "Registration Enabled",
    "defaultBalance": "Default Balance"
  }
}
```

Create `frontend/src/locales/zh.json`:

```json
{
  "common": {
    "loading": "加载中...",
    "save": "保存",
    "cancel": "取消",
    "delete": "删除",
    "create": "创建",
    "edit": "编辑",
    "search": "搜索",
    "confirm": "确认",
    "status": "状态",
    "actions": "操作",
    "active": "活跃",
    "disabled": "已禁用",
    "banned": "已封禁",
    "copy": "复制",
    "copied": "已复制"
  },
  "auth": {
    "login": "登录",
    "register": "注册",
    "email": "邮箱",
    "password": "密码",
    "confirmPassword": "确认密码",
    "loginWithGoogle": "Google 登录",
    "noAccount": "没有账号？",
    "hasAccount": "已有账号？",
    "logout": "退出登录"
  },
  "nav": {
    "dashboard": "仪表盘",
    "overview": "总览",
    "users": "用户管理",
    "channels": "渠道管理",
    "models": "模型配置",
    "apiKeys": "API 密钥",
    "usageLogs": "调用记录",
    "topUp": "充值兑换",
    "balance": "余额流水",
    "docs": "接入文档",
    "settings": "系统设置",
    "redeemCodes": "兑换码",
    "logs": "调用日志"
  },
  "dashboard": {
    "totalUsers": "总用户数",
    "todayRequests": "今日调用",
    "todayTokens": "今日 Token",
    "todayRevenue": "今日收入",
    "currentBalance": "当前余额",
    "todayCost": "今日消耗",
    "recentActivity": "最近活动"
  },
  "apiKeys": {
    "title": "API 密钥",
    "newKey": "+ 新建",
    "keyName": "密钥名称",
    "key": "密钥",
    "createdAt": "创建时间",
    "lastUsed": "最后使用",
    "createTitle": "创建 API 密钥",
    "deleteConfirm": "确定删除此 API 密钥？"
  },
  "channels": {
    "title": "渠道管理",
    "name": "名称",
    "type": "类型",
    "baseUrl": "Base URL",
    "models": "支持模型",
    "priority": "优先级",
    "weight": "权重",
    "test": "测试",
    "testSuccess": "渠道连接正常",
    "apiKey": "上游 API Key"
  },
  "billing": {
    "amount": "金额",
    "redeemCode": "兑换码",
    "redeem": "兑换",
    "redeemSuccess": "兑换成功",
    "generate": "生成",
    "count": "数量",
    "expiresAt": "过期时间",
    "topUp": "充值",
    "topUpUser": "给用户充值"
  },
  "logs": {
    "model": "模型",
    "promptTokens": "输入 Token",
    "completionTokens": "输出 Token",
    "totalTokens": "总 Token",
    "cost": "费用",
    "duration": "耗时",
    "time": "时间",
    "type": "类型",
    "description": "描述",
    "balanceAfter": "变动后余额"
  },
  "docs": {
    "title": "接入文档",
    "claudeCode": "Claude Code",
    "openaiSdk": "OpenAI SDK (Python)",
    "curl": "curl"
  },
  "settings": {
    "changePassword": "修改密码",
    "oldPassword": "当前密码",
    "newPassword": "新密码",
    "siteName": "站点名称",
    "registerEnabled": "开放注册",
    "defaultBalance": "默认余额"
  }
}
```

- [ ] **Step 4: Create language switch component**

Create `frontend/src/components/LanguageSwitch.tsx`:

```tsx
import { useTranslation } from 'react-i18next';

export default function LanguageSwitch() {
  const { i18n } = useTranslation();
  const isZh = i18n.language.startsWith('zh');

  return (
    <button
      onClick={() => i18n.changeLanguage(isZh ? 'en' : 'zh')}
      style={{
        background: 'none',
        border: '1px solid var(--border-color)',
        padding: '2px 8px',
        fontFamily: 'var(--font-mono)',
        fontSize: '11px',
        cursor: 'pointer',
        color: 'var(--text-secondary)',
      }}
    >
      {isZh ? 'EN' : '中文'}
    </button>
  );
}
```

Place this component in both layout headers (UserLayout and AdminLayout).

- [ ] **Step 5: Import i18n in main.tsx**

Add `import './i18n';` to `frontend/src/main.tsx` before the App import.

- [ ] **Step 6: Update all page components to use useTranslation**

Replace hardcoded strings in all pages with `t('key')` calls. Example:

```tsx
import { useTranslation } from 'react-i18next';

export default function Overview() {
  const { t } = useTranslation();
  return <h3>// {t('nav.overview')}</h3>;
}
```

- [ ] **Step 7: Verify both languages work**

```bash
cd /home/colorful/Documents/claude/token/frontend
npm run dev
# Open browser, check Chinese and English switch works
```

- [ ] **Step 8: Commit**

```bash
git add frontend/src/i18n.ts frontend/src/locales/ frontend/src/components/LanguageSwitch.tsx
git add frontend/src/main.tsx frontend/src/pages/ frontend/src/layouts/
git commit -m "feat: add i18n with Chinese and English translations"
```

---

## Phase 6: Docker & Deployment

### Task 19: Docker Build Files

**Files:**
- Create: `backend/Dockerfile`
- Create: `frontend/Dockerfile`
- Create: `frontend/nginx.conf`
- Modify: `docker-compose.yml`

- [ ] **Step 1: Create backend Dockerfile**

Create `backend/Dockerfile`:

```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /relay cmd/server/main.go

FROM alpine:3.20
RUN apk --no-cache add ca-certificates
COPY --from=builder /relay /relay
EXPOSE 8080
CMD ["/relay"]
```

- [ ] **Step 2: Create frontend Dockerfile + nginx config**

Create `frontend/nginx.conf`:

```nginx
server {
    listen 80;
    root /usr/share/nginx/html;
    index index.html;

    location / {
        try_files $uri $uri/ /index.html;
    }

    location /api/ {
        proxy_pass http://backend:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    location /v1/ {
        proxy_pass http://backend:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_buffering off;
        proxy_cache off;
        chunked_transfer_encoding on;
    }
}
```

Create `frontend/Dockerfile`:

```dockerfile
FROM node:20-alpine AS builder
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
```

- [ ] **Step 3: Update docker-compose.yml for full stack**

Replace `docker-compose.yml`:

```yaml
services:
  backend:
    build: ./backend
    environment:
      PORT: "8080"
      DATABASE_URL: postgres://relay:relay@postgres:5432/relay?sslmode=disable
      REDIS_URL: redis://redis:6379/0
      JWT_SECRET: ${JWT_SECRET:-change-me-in-production}
      ENCRYPTION_KEY: ${ENCRYPTION_KEY:-change-me-32-byte-key-for-aes!!}
      GOOGLE_CLIENT_ID: ${GOOGLE_CLIENT_ID:-}
      GOOGLE_CLIENT_SECRET: ${GOOGLE_CLIENT_SECRET:-}
      ALLOWED_ORIGINS: "*"
    depends_on:
      - postgres
      - redis
    restart: unless-stopped

  frontend:
    build: ./frontend
    ports:
      - "80:80"
    depends_on:
      - backend
    restart: unless-stopped

  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: relay
      POSTGRES_PASSWORD: relay
      POSTGRES_DB: relay
    volumes:
      - pgdata:/var/lib/postgresql/data
    restart: unless-stopped

  redis:
    image: redis:7-alpine
    volumes:
      - redisdata:/data
    restart: unless-stopped

volumes:
  pgdata:
  redisdata:
```

- [ ] **Step 4: Test full docker build**

```bash
cd /home/colorful/Documents/claude/token
docker compose build
docker compose up -d
sleep 10
curl http://localhost/health
# Expected: {"status":"ok"}
curl http://localhost
# Expected: HTML page (React app)
```

- [ ] **Step 5: Run seed data**

```bash
docker compose exec -T postgres psql -U relay -d relay < backend/migration/seed.sql
```

- [ ] **Step 6: Commit**

```bash
git add backend/Dockerfile frontend/Dockerfile frontend/nginx.conf docker-compose.yml
git commit -m "feat: add Docker build files and full-stack docker-compose"
```

---

### Task 20: End-to-End Verification

- [ ] **Step 1: Start full stack**

```bash
cd /home/colorful/Documents/claude/token
docker compose up -d
```

- [ ] **Step 2: Test auth flow**

```bash
# Register
curl -X POST http://localhost/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"user1@test.com","password":"test123"}'

# Login
curl -X POST http://localhost/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@relay.local","password":"admin123"}'
# Save the access_token from response
```

- [ ] **Step 3: Test admin channel creation**

```bash
TOKEN="<admin access token from step 2>"
curl -X POST http://localhost/api/admin/channels \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "name": "Claude Primary",
    "type": "claude",
    "api_key": "sk-ant-xxx-your-real-key",
    "base_url": "https://api.anthropic.com",
    "models": ["claude-sonnet-4-20250514"],
    "priority": 0,
    "weight": 1
  }'
```

- [ ] **Step 4: Test API proxy (with a real Claude key)**

```bash
# Create an API key for the test user
USER_TOKEN="<user access token>"
curl -X POST http://localhost/api/user/api-keys \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $USER_TOKEN" \
  -d '{"name":"test"}'
# Get the sk- key from response

# Test native Claude format
curl -X POST http://localhost/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: sk-<key from above>" \
  -d '{
    "model": "claude-sonnet-4-20250514",
    "max_tokens": 100,
    "messages": [{"role":"user","content":"Say hello"}]
  }'
```

- [ ] **Step 5: Verify frontend in browser**

Open `http://localhost` in browser:
1. Register a new account
2. Login
3. Check user dashboard loads
4. Create an API key
5. Login as admin (admin@relay.local / admin123)
6. Check admin dashboard, add a channel, configure models

- [ ] **Step 6: Final commit**

```bash
git add -A
git commit -m "feat: complete AI Token Relay MVP"
```
