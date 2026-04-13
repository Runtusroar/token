# AI Token Relay — 系统设计文档

## 1. 项目概述

AI API Token 中转站：一个 API 代理/转发平台，管理员配置上游 AI 服务商渠道，用户通过平台分发的 API Key 调用各家 AI 模型。平台负责请求转发、格式适配、Token 计费和用户管理。

**目标用户：**
- 管理员：配置渠道、管理用户、设定定价、生成兑换码
- 终端用户：注册账号、获取 API Key、充值使用

**核心价值：** 用户只需修改 Base URL 和 API Key，即可用官方客户端（Claude Code、ChatGPT-Next-Web、Cursor 等）无缝接入。

## 2. 技术栈

| 层级 | 技术 | 说明 |
|------|------|------|
| 后端 | Go + Gin | 高并发代理网关，goroutine 处理流式连接 |
| ORM | GORM | Go 主流 ORM，支持 PostgreSQL |
| 前端 | React + Vite + Ant Design | 前后端分离 SPA |
| 数据库 | PostgreSQL | 主存储，用户/渠道/日志/账单 |
| 缓存 | Redis | 速率限制、Token 缓存、会话管理 |
| 认证 | JWT + Google OAuth 2.0 | 邮箱密码登录 + Google 登录 |
| 字体 | JetBrains Mono + LXGW WenKai Mono | 等宽字体，中英文覆盖 |

## 3. 系统架构

```
客户端层
├── 管理员面板 (React SPA)
├── 用户面板 (React SPA)
└── API 调用方 (SDK / curl / 客户端)
        │
        ▼
Go 后端服务 (Gin)
├── API Gateway / Router
│   ├── 认证中间件 (JWT 校验 / API Key 校验)
│   ├── 限流中间件 (Redis 令牌桶)
│   └── 日志中间件
├── 用户模块 — 注册、登录、API Key 管理、个人信息
├── 认证模块 — JWT 签发/校验、Google OAuth、RBAC(admin/user)
├── 渠道模块 — 上游渠道 CRUD、负载均衡、健康检查、故障转移
├── 计费模块 — 余额管理、Token 计算、倍率定价、兑换码、流水记录
└── 代理模块 — 请求转发、格式转换、流式 SSE、OpenAI 兼容层
        │
        ▼
数据层
├── PostgreSQL — 持久化存储
└── Redis — 缓存 / 限流 / 会话
        │
        ▼
上游 AI 服务商
├── Claude API (首期)
├── OpenAI API (后续)
├── Google Gemini (后续)
└── 更多...
```

### 3.1 核心数据流

**API 请求处理流程：**

```
用户请求 → 解析 API Key → 查找用户 → 检查余额 → 选择渠道(负载均衡)
    → 格式转换(如需) → 转发上游 → 流式返回响应
    → 计算 Token 消耗 → 按倍率扣费 → 记录日志
```

**格式转换策略：**
- `/v1/messages` — Claude 原生格式，直接透传到 Claude 上游
- `/v1/chat/completions` — OpenAI 兼容格式，内部转换为各模型的原生格式后转发

**流式响应计费：**
- 非流式：上游响应中直接包含 usage 字段（token 数），用于计费
- 流式 SSE：在流结束时上游返回最终 usage 事件，据此计费。若上游未返回 usage，则使用 tiktoken 本地估算
- 计费在请求完成后异步执行（goroutine），不阻塞响应返回

**余额扣费事务安全：**
- 请求前先检查余额是否充足（乐观检查，不加锁）
- 请求完成后扣费：在数据库事务中同时更新 users.balance 和插入 balance_logs
- 使用 `UPDATE users SET balance = balance - ? WHERE id = ? AND balance >= ?` 防止并发超扣

### 3.2 渠道调度策略

同一模型可配置多个渠道（多个上游 API Key）：
1. 按优先级分组，优先使用高优先级渠道
2. 同优先级内按权重加权随机选择
3. 渠道失败时自动跳过，尝试下一个（故障转移）
4. 后台定时健康检查，自动标记不可用渠道

## 4. 数据库设计

### 4.1 users — 用户表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | bigint PK | 自增主键 |
| email | varchar(255) UNIQUE | 邮箱 |
| password_hash | varchar(255) | bcrypt 哈希，Google OAuth 用户可为空 |
| google_id | varchar(255) | Google OAuth ID |
| role | varchar(20) | admin / user |
| balance | decimal(12,4) | 当前余额（单位：元） |
| status | varchar(20) | active / banned |
| created_at | timestamp | 创建时间 |
| updated_at | timestamp | 更新时间 |

### 4.2 api_keys — API 密钥表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | bigint PK | 自增主键 |
| user_id | bigint FK | 所属用户 |
| key | varchar(255) UNIQUE | API Key（sk-前缀） |
| name | varchar(100) | 密钥名称 |
| status | varchar(20) | active / disabled |
| created_at | timestamp | 创建时间 |
| last_used_at | timestamp | 最后使用时间 |

### 4.3 channels — 上游渠道表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | bigint PK | 自增主键 |
| name | varchar(100) | 渠道名称 |
| type | varchar(50) | 类型：claude / openai / gemini |
| api_key | varchar(500) | 上游 API Key（加密存储） |
| base_url | varchar(500) | 上游 API Base URL |
| models | jsonb | 支持的模型列表 |
| status | varchar(20) | active / disabled / error |
| priority | int | 优先级（数字越小越优先） |
| weight | int | 权重（同优先级内加权随机） |
| created_at | timestamp | 创建时间 |
| updated_at | timestamp | 更新时间 |

### 4.4 model_configs — 模型配置表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | bigint PK | 自增主键 |
| model_name | varchar(100) UNIQUE | 模型标识，如 claude-sonnet-4-20250514 |
| display_name | varchar(100) | 显示名称，如 Claude Sonnet 4 |
| rate | decimal(6,2) | 计费倍率，如 1.0 |
| input_price | decimal(10,6) | 输入价格（每百万 Token，单位：元） |
| output_price | decimal(10,6) | 输出价格（每百万 Token，单位：元） |
| enabled | boolean | 是否启用 |
| created_at | timestamp | 创建时间 |
| updated_at | timestamp | 更新时间 |

### 4.5 request_logs — 请求日志表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | bigint PK | 自增主键 |
| user_id | bigint FK | 调用用户 |
| api_key_id | bigint FK | 使用的 API Key |
| channel_id | bigint FK | 使用的渠道 |
| model | varchar(100) | 调用模型 |
| type | varchar(20) | 接口类型：native / openai_compat |
| prompt_tokens | int | 输入 Token 数 |
| completion_tokens | int | 输出 Token 数 |
| total_tokens | int | 总 Token 数 |
| cost | decimal(10,6) | 用户扣费金额 |
| upstream_cost | decimal(10,6) | 上游成本 |
| status | varchar(20) | success / error |
| duration_ms | int | 请求耗时（毫秒） |
| ip_address | varchar(45) | 调用方 IP |
| created_at | timestamp | 创建时间 |

**索引：** user_id + created_at 复合索引，channel_id 索引，created_at 索引（用于按时间范围查询和清理）

### 4.6 balance_logs — 余额流水表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | bigint PK | 自增主键 |
| user_id | bigint FK | 用户 |
| type | varchar(20) | topup / consume / redeem / admin_adjust |
| amount | decimal(12,4) | 变动金额（正为充值，负为消费） |
| balance_after | decimal(12,4) | 变动后余额 |
| description | varchar(500) | 描述 |
| request_log_id | bigint FK | 关联的请求日志（消费时） |
| created_at | timestamp | 创建时间 |

### 4.7 redemption_codes — 兑换码表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | bigint PK | 自增主键 |
| code | varchar(50) UNIQUE | 兑换码 |
| amount | decimal(12,4) | 额度金额 |
| status | varchar(20) | unused / used / disabled |
| used_by | bigint FK | 使用者用户 ID |
| used_at | timestamp | 使用时间 |
| created_by | bigint FK | 创建者（管理员） |
| created_at | timestamp | 创建时间 |
| expires_at | timestamp | 过期时间 |

## 5. API 设计

### 5.1 代理接口（核心）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/v1/messages` | Claude 原生格式（Messages API） |
| POST | `/v1/chat/completions` | OpenAI 兼容格式 |
| GET | `/v1/models` | 获取可用模型列表 |

认证方式：`Authorization: Bearer sk-xxx` 或 `x-api-key: sk-xxx`

### 5.2 认证接口

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/auth/register` | 邮箱密码注册 |
| POST | `/api/auth/login` | 邮箱密码登录 |
| GET | `/api/auth/google` | Google OAuth 跳转 |
| GET | `/api/auth/google/callback` | Google OAuth 回调 |
| POST | `/api/auth/refresh` | 刷新 JWT Token |

### 5.3 用户接口

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/user/profile` | 获取个人信息 |
| PUT | `/api/user/profile` | 更新个人信息 |
| PUT | `/api/user/password` | 修改密码 |
| GET | `/api/user/api-keys` | 获取 API Key 列表 |
| POST | `/api/user/api-keys` | 创建 API Key |
| DELETE | `/api/user/api-keys/:id` | 删除 API Key |
| PUT | `/api/user/api-keys/:id` | 更新 API Key 状态 |
| GET | `/api/user/logs` | 获取调用记录 |
| GET | `/api/user/balance-logs` | 获取余额流水 |
| POST | `/api/user/redeem` | 兑换码充值 |
| GET | `/api/user/dashboard` | 用户仪表盘数据 |

### 5.4 管理员接口

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/admin/dashboard` | 管理员仪表盘数据 |
| GET | `/api/admin/users` | 用户列表（分页/搜索） |
| PUT | `/api/admin/users/:id` | 更新用户（角色/状态） |
| POST | `/api/admin/users/:id/topup` | 手动充值 |
| GET | `/api/admin/channels` | 渠道列表 |
| POST | `/api/admin/channels` | 创建渠道 |
| PUT | `/api/admin/channels/:id` | 更新渠道 |
| DELETE | `/api/admin/channels/:id` | 删除渠道 |
| POST | `/api/admin/channels/:id/test` | 测试渠道连通性 |
| GET | `/api/admin/models` | 模型配置列表 |
| POST | `/api/admin/models` | 创建模型配置 |
| PUT | `/api/admin/models/:id` | 更新模型配置 |
| GET | `/api/admin/redeem-codes` | 兑换码列表 |
| POST | `/api/admin/redeem-codes` | 批量生成兑换码 |
| PUT | `/api/admin/redeem-codes/:id` | 更新兑换码状态 |
| GET | `/api/admin/logs` | 全局请求日志 |
| GET | `/api/admin/settings` | 获取系统设置 |
| PUT | `/api/admin/settings` | 更新系统设置 |

## 6. 前端设计

### 6.1 UI 风格

参考 nof1.ai 风格，核心特征：
- **配色：** 白色/浅灰背景 (#f5f5f0)，黑色文字 (#1a1a1a)，绿色数值 (#0a8c2d)
- **边框：** 1px 黑色实线边框，无圆角，无阴影
- **字体：** JetBrains Mono（英文/代码）+ LXGW WenKai Mono（中文），全站等宽
- **布局：** 左侧边栏导航 + 右侧内容区，经典后台布局
- **风格：** 极简扁平，终端/命令行美学，`//` 标题前缀，下划线命名

### 6.2 管理员面板页面

| 页面 | 功能 |
|------|------|
| dashboard | 统计卡片（用户数/调用量/Token/收入）、7日趋势图、最近活动 |
| users | 用户列表（搜索/筛选）、查看详情、封禁/解封、手动充值 |
| channels | 渠道列表、添加/编辑渠道、测试连通性、状态标识 |
| models | 模型列表、启用/禁用、倍率和单价配置 |
| redeem_codes | 批量生成、列表（状态/使用者/时间）、禁用 |
| logs | 全局请求日志、按用户/模型/时间筛选、详情展开 |
| settings | 站点名称、注册开关、新用户默认余额 |

### 6.3 用户面板页面

| 页面 | 功能 |
|------|------|
| overview | 余额/今日用量统计、快速接入代码块（Claude Code / OpenAI 格式） |
| api_keys | 创建/删除/禁用 Key、显示脱敏 Key、复制功能 |
| usage_logs | 调用记录列表、Token 消耗明细、按模型/时间筛选 |
| top_up | 输入兑换码充值 |
| balance | 余额变动流水（充值/消费/兑换），每条记录含变动后余额 |
| docs | 各客户端接入说明（Claude Code、OpenAI SDK、curl 示例） |
| settings | 修改密码、绑定/解绑 Google 账号 |

## 7. 安全设计

- **API Key 生成：** crypto/rand 生成 32 字节随机数，Base62 编码，sk- 前缀
- **密码存储：** bcrypt 哈希，cost factor 12
- **上游 Key 存储：** AES-256-GCM 加密存储在数据库中
- **JWT：** RS256 签名，access token 15分钟过期，refresh token 7天
- **速率限制：** Redis 令牌桶，按 API Key 维度限流
- **SQL 注入防护：** GORM 参数化查询，禁止原始 SQL 拼接
- **XSS 防护：** React 默认转义 + Content-Security-Policy 头
- **CORS：** 严格配置允许的域名

## 8. 项目结构

```
ai-relay/
├── backend/                    # Go 后端
│   ├── cmd/
│   │   └── server/
│   │       └── main.go         # 入口
│   ├── internal/
│   │   ├── config/             # 配置加载
│   │   ├── middleware/         # Gin 中间件（auth, ratelimit, cors, logger）
│   │   ├── model/              # GORM 模型定义
│   │   ├── handler/            # HTTP 处理器
│   │   │   ├── auth.go
│   │   │   ├── user.go
│   │   │   ├── admin.go
│   │   │   └── proxy.go
│   │   ├── service/            # 业务逻辑
│   │   │   ├── auth.go
│   │   │   ├── user.go
│   │   │   ├── channel.go
│   │   │   ├── billing.go
│   │   │   └── proxy.go
│   │   ├── repository/         # 数据访问层
│   │   ├── adapter/            # 上游 API 适配器
│   │   │   ├── claude.go
│   │   │   ├── openai.go
│   │   │   └── converter.go    # OpenAI ↔ 各模型格式转换
│   │   └── pkg/                # 内部工具包
│   │       ├── jwt.go
│   │       ├── crypto.go
│   │       └── response.go
│   ├── migration/              # 数据库迁移文件
│   ├── go.mod
│   └── go.sum
├── frontend/                   # React 前端
│   ├── src/
│   │   ├── api/                # API 请求封装
│   │   ├── components/         # 通用组件
│   │   ├── layouts/            # 布局（AdminLayout / UserLayout）
│   │   ├── pages/
│   │   │   ├── auth/           # 登录/注册
│   │   │   ├── admin/          # 管理员面板页面
│   │   │   └── user/           # 用户面板页面
│   │   ├── hooks/              # 自定义 hooks
│   │   ├── store/              # 状态管理
│   │   ├── styles/             # 全局样式 + 主题
│   │   ├── utils/              # 工具函数
│   │   ├── App.tsx
│   │   └── main.tsx
│   ├── index.html
│   ├── vite.config.ts
│   ├── tsconfig.json
│   └── package.json
├── docker-compose.yml          # 本地部署编排
├── Makefile                    # 构建命令
└── README.md
```

## 9. 部署方案

本地部署，使用 Docker Compose 编排：

```yaml
services:
  backend:     # Go 后端服务
  frontend:    # Nginx 托管前端静态文件 + 反向代理
  postgres:    # PostgreSQL 数据库
  redis:       # Redis 缓存
```

启动命令：`docker-compose up -d`

未来扩展路径：
- 水平扩展：后端无状态，可直接多实例 + Nginx 负载均衡
- 数据库：PostgreSQL 读写分离
- 缓存：Redis Sentinel / Cluster
- 日志表：按月分区或归档到 ClickHouse

## 10. 首期范围

首期实现（MVP）：
- 支持 Claude 模型（Messages API）
- OpenAI 兼容格式转换
- 完整的用户系统（注册/登录/Google OAuth）
- API Key 管理
- 渠道管理（单渠道即可，预留多渠道接口）
- Token 计费 + 模型倍率
- 管理员手动充值 + 兑换码
- 管理员面板 + 用户面板
- Docker Compose 本地部署

前端国际化（i18n）：
- 使用 react-i18next 实现中英双语
- 默认语言跟随浏览器，用户可手动切换
- 翻译文件放在 frontend/src/locales/ 目录

不在首期范围：
- OpenAI / Gemini 等其他上游（架构已预留，后续按需添加 adapter）
- 在线支付（支付宝/微信）
- 邮件通知
