# go-boilerplate

[![test](https://github.com/kennethfan/go-boilerplate/actions/workflows/test.yml/badge.svg)](https://github.com/kennethfan/go-boilerplate/actions/workflows/test.yml)

Go 后端脚手架：集成 HTTP / ORM / Redis / MQ 基础组件、JWT + RBAC 权限校验、结构化日志、OpenTelemetry 链路追踪。

## 技术栈

| 能力 | 组件 | 说明 |
|---|---|---|
| HTTP | Gin v1.10 | 统一响应封装、参数校验、CORS、Recovery |
| ORM | GORM v1.31 + MySQL / PostgreSQL / SQLite | 三库可切换、连接池、自动埋点、版本化迁移 |
| 缓存 | go-redis v9 | 连接池、自动埋点 |
| MQ | RocketMQ 5（client v2） | 同步发送 / 集群消费封装 |
| 认证 | golang-jwt/v5 | access + refresh 双令牌 |
| 授权 | Casbin v2（RBAC） | 策略存数据库，keyMatch2 路径匹配 |
| 日志 | zap + lumberjack | 结构化、文件滚动、trace_id 注入 |
| 链路追踪 | OpenTelemetry → OTLP | HTTP / SQL / Redis 全链路 span，对接 Jaeger |
| 监控 | Prometheus + Grafana | HTTP/GORM/Redis/runtime 指标，`/metrics` 暴露 |
| 实时通信 | WebSocket + SSE | gorilla/websocket（Hub 广播）+ 原生流式推送 |
| 注册中心 | Nacos | 启动注册/退出注销 + 健康发现（可扩展 Consul/etcd） |
| 文档 | swaggo | 注解生成 Swagger |
| 配置 | viper + 配置中心 | 本地 YAML / Nacos / Apollo，支持热更新 |
| 迁移 | golang-migrate | up/down 版本化 SQL |
| **定时任务** | **robfig/cron/v3** | **声明式（YAML）+ 代码注册混合；panic 自动 recover；启用/禁用独立** |

## 目录结构

```
go-boilerplate/
├── cmd/
│   ├── server/main.go        # 服务入口（组装 + 优雅退出 + seed）
│   └── migrate/main.go       # 数据库迁移工具
├── configs/                  # config.yaml / casbin_model.conf
├── migrations/               # 版本化迁移 SQL（mysql/postgres/sqlite 分目录）
├── internal/
│   ├── config/               # 配置加载
│   ├── handler/              # HTTP 层（auth / user / health）
│   ├── middleware/           # jwt / casbin / accesslog / recovery / cors
│   ├── model/                # GORM 模型
│   ├── repository/           # 数据访问层
│   ├── router/               # 路由装配
│   └── service/              # 业务逻辑层
├── pkg/                      # 可复用基础库（logger/database/redis/mq/jwt/otelx/casbinx/configcenter/response/errcode）
├── api/docs/                 # Swagger 文档（make swagger 生成）
├── scripts/rocketmq/         # broker 配置（docker-compose 挂载）
├── docker-compose.yml        # MySQL / PostgreSQL + Redis + RocketMQ + Jaeger
├── Makefile
└── .air.toml                 # 热重载
```

## 切换数据库

ORM 层通过 `database.type` 一键切换，业务代码（repository 层使用 `*gorm.DB`）零改动。

| type | 说明 | 需自行准备的依赖 |
|---|---|---|
| `mysql` | 默认，连接池 + 自动埋点 | MySQL 8（`make docker-up`） |
| `postgres` | PostgreSQL 16 | `make docker-up` 已含 |
| `sqlite` | 纯 Go 驱动，无需外部服务，适合本地开发/单测 | 无 |

```yaml
# configs/config.yaml 中修改 type 即可
database:
  type: mysql       # mysql / postgres / sqlite

  # postgres 示例
  # type: postgres
  # host: 127.0.0.1
  # port: 5432
  # username: root
  # password: root123
  # name: go_boilerplate

  # sqlite 示例（零依赖，最快跑通）
  # type: sqlite
  # name: ./data/app.db    # 或 :memory:
```

建表两种方式任选：
- **AutoMigrate（默认）**：`database.auto_migrate: true`，启动时 GORM 按所选方言自动建表，跨库无痛；
- **golang-migrate**：`make migrate-up`，迁移 SQL 已按方言分目录（`migrations/{mysql,postgres,sqlite}/`）。

## 配置中心

业务配置（database / redis / jwt 等）支持三种数据源，`config_center.type` 一键切换；启用后远程配置覆盖本地，并**热更新**（变更实时生效，无需重启）。

| type | 说明 | 远程配置形态 |
|---|---|---|
| `local` | 本地文件（默认，兜底） | 直接读文件 |
| `nacos` | 阿里 Nacos | dataId 内容为完整 YAML |
| `apollo` | 携程 Apollo | namespace 中一个 key（默认 `content`）存放完整 YAML |

```yaml
config_center:
  enable: false      # 开启后业务配置改由远程拉取
  type: local        # local / nacos / apollo

  # nacos 示例
  # type: nacos
  # nacos_addrs: [ "127.0.0.1:8848" ]
  # nacos_data_id: go-boilerplate.yaml
  # nacos_group: DEFAULT_GROUP

  # apollo 示例
  # type: apollo
  # apollo_app_id: go-boilerplate
  # apollo_meta_addr: http://127.0.0.1:8080
  # apollo_content_key: content
```

**热更新边界**：全局配置指针随远程变更原子更新，业务代码经 `config.Get()` 读到最新值（适合动态开关类配置）；连接类组件（数据库/Redis/MQ/HTTP 端口）启动时初始化一次、不热重建。远程拉取失败时自动降级使用本地配置，不阻塞启动。

## 组件开关

所有可插拔组件均提供独立 `enable` 字段，默认 `true` 保持向后兼容。任何开关置 `false` 时相关路由/中间件/客户端**不会初始化、不影响性能、不增加二进制大小**。

| 开关 | 配置路径 | 控制范围 | 关闭后的行为 |
|---|---|---|---|
| `redis.enable` | `configs/config.yaml → redis` | Redis 客户端、`/readyz` 中的 Redis 检查 | 不初始化 `*redis.Client`，`/readyz` 报告 `redis: disabled` |
| `auth.enable` | `configs/config.yaml → auth` | `JWT` + `Casbin` 中间件 | `/api/v1/users` 等受保护路由仍存在但**无鉴权**（公开部署场景，不推荐用于公网） |
| `realtime.enable` | `configs/config.yaml → realtime` | WebSocket、SSE 路由 | 不挂载 `/api/v1/ws`、`/api/v1/sse` |
| `docs.enable` | `configs/config.yaml → docs` | Swagger UI | 不挂载 `/swagger/*any`（适合生产关掉降低攻击面） |
| `metrics.enable` | `configs/config.yaml → metrics` | Prometheus HTTP 中间件、`/metrics` 端点、GORM/Redis 采集 | 不挂中间件、不暴露端点、不注册 GORM/Redis hook |

```yaml
# 最小化部署示例：仅保留 HTTP + 数据库，关闭一切附加能力
redis:
  enable: false
auth:
  enable: false
realtime:
  enable: false
docs:
  enable: false
metrics:
  enable: false
```

完整开关矩阵（其余组件的 `enable` 字段）：

| 组件 | 字段 | 关闭后行为 |
|---|---|---|
| RocketMQ Producer | `rocketmq.producer.enable` | 不启动 Producer 连接 |
| RocketMQ Consumer | `rocketmq.consumer.enable` | 不订阅 demo topic |
| gRPC server | `grpc.enable` | 跳过 gRPC 启动（注意：该选项生效需重新 `go build -tags grpc` 才会真正链接 gRPC stub） |
| 服务注册中心 | `registry.enable` | 不向 Nacos 注册/注销 |
| OpenTelemetry | `otel.enable` | 不初始化 tracer，GORM/Redis 也不挂 span |
| 配置中心 | `config_center.enable` | 不拉远程配置、不监听变更 |

> **注意**：除 `redis/auth/realtime/docs/metrics/cron` 外，其余开关的 close 路径只阻止对应能力的运行时初始化。删除代码仍需重新编译；这六个新开关的 close 路径**也不进入二进制依赖**（条件分支隔离），可放心用于精简部署。

## 定时任务

基于 `robfig/cron/v3 v3.0.1`：声明式（YAML）+ 代码注册混合模式，提供独立 `cron.enable` 总开关。

### 一、声明式任务（YAML，无外部依赖）

适合"无须业务上下文的纯定时逻辑"。每次触发打印一条 zap 日志（包含 `name/spec/now`）。

```yaml
cron:
  enable: true                              # false 则 New/Start/Stop 全 no-op
  location: "Asia/Shanghai"                 # IANA 时区；解析失败回退 Local
  jobs:
    - name: heartbeat_tick
      spec: "@every 30s"
      description: 每 30 秒一次心跳日志
      log: "cron tick from declarative job"
    - name: cleanup_daily
      spec: "0 3 * * *"                     # 标准 5 字段 cron 表达式
      description: 每天凌晨 3 点
```

支持的 spec 形式：
- 标准 cron：`分 时 日 月 周`（5 字段）
- `@every` 描述符：`@every 30s`、`@every 1h30m` 等（time.ParseDuration 合法即可，**注意 <1s 会被 robfig 自动 round up 到 1s**）
- 内置：`@yearly @monthly @weekly @daily @hourly`

### 二、代码注册任务（依赖 service / DB）

适合需要运行时上下文的复杂任务，统一在 `internal/cron/jobs.go` 的 `RegisterAll` 注册：

```go
sched.MustAddFunc(
    "@every 1m",
    "user_heartbeat",
    func(ctx context.Context) error {
        // 直接拿到 deps.UserService、deps.Logger 等
        n, err := deps.UserService.Count(ctx)
        if err != nil { return err }
        deps.Logger.Info("user count", zap.Int64("n", n))
        return nil
    },
)
```

添加新任务的两步流程：
1. 在 `internal/cron/jobs.go` 写 `registerXxx` 函数；
2. 在 `RegisterAll` 末尾追加 `sched.MustAddFunc(...)`。

### 三、能力

- **优雅启停**：context cancel → `Stop(ShutdownTimeout)` 等待在飞任务完成（不再触发新一轮）。
- **panic recover**：由 `cron.Recover` 链式包装，自动 `cron.Recover(cron.Logger)` 记录 `panic+stack`，进程不退出。
- **同任务防并发**：`cron.SkipIfStillRunning` 链式避免上一次未结束就重入。
- **时区感知**：`cron.location` 决定下一次触发时间（如 `Asia/Shanghai`）。
- **结构化日志**：所有生命周期事件走 zap，msg + 键值对。
- **可重入注册**：同名 job 调用 `AddFunc` 自动覆盖（Remove 旧条目）。

### 四、热更新边界

按既有约定：任务启动时一次性注册，**不加运行期热更新**（与 RocketMQ Consumer / WebSocket 同语义）。增减任务 → 改 `cron.jobs` 或 `internal/cron/jobs.go` → 重新部署。

## 快速开始

### 1. 启动依赖（Docker）

```bash
make docker-up   # MySQL 3306 / PostgreSQL 5432 / Redis 6379 / RocketMQ 9876 / Jaeger 16686
```

### 2. 安装依赖并迁移

```bash
make tidy            # go mod tidy（首次必做）
make migrate-up      # 建表
```

### 3. 启动服务

```bash
make run             # 热重载（需 air）；或 go run ./cmd/server
```

启动日志出现 `server started` 即成功。首次启动自动：
- 创建内置管理员 `admin / admin123`（仅空表时）
- 写入 Casbin 初始策略（admin 全权限，user 只读用户详情）

### 4. 验证

```bash
# 健康检查
curl http://127.0.0.1:8080/api/v1/healthz

# 登录拿 token
curl -X POST http://127.0.0.1:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}'

# 带 token 访问（admin 创建用户）
curl -X POST http://127.0.0.1:8080/api/v1/users \
  -H "Authorization: Bearer <access_token>" \
  -H 'Content-Type: application/json' \
  -d '{"username":"test01","password":"test123456","nickname":"测试","role":"user"}'
```

Swagger 文档：`make swagger` 后访问 http://127.0.0.1:8080/swagger/index.html

链路追踪：Jaeger UI http://127.0.0.1:16686 （搜索 service = go-boilerplate，
任意请求的 trace 覆盖 HTTP → GORM SQL → Redis 命令，日志中同步携带 trace_id）。

## 监控

内置 Prometheus 指标，`/metrics` 端点暴露，`make docker-up` 已含 Prometheus + Grafana。

| 指标类别 | 指标 | 说明 |
|---|---|---|
| HTTP | `http_requests_total` / `http_request_duration_seconds` / `http_request_body_bytes` | QPS、延迟分位、请求体大小，按 method/path/status 分组 |
| GORM | `gorm_queries_total` / `gorm_query_duration_seconds` | SQL 查询量、耗时，按 create/query/update/delete/raw 分组 |
| Redis | `redis_commands_total` / `redis_command_duration_seconds` | 命令量、耗时，按 command 分组 |
| Runtime | `go_goroutines` / `go_memstats_*` / `process_cpu_seconds_total` | GC、内存、goroutine、CPU |

```bash
# 本地查看指标
curl http://127.0.0.1:8080/metrics
```

访问入口：
- Prometheus：http://127.0.0.1:9090 （已预置抓取 job `go-boilerplate`）
- Grafana：http://127.0.0.1:3000 （admin / admin，首次需在 Data sources 添加 Prometheus，地址 `http://prometheus:9090`）

配置 `metrics.enable` 控制 GORM/Redis 指标采集（HTTP 指标与 `/metrics` 端点始终可用）。

## 实时通信

内置 WebSocket 与 SSE 两种实时通道，封装于 `pkg/ws` 与 `pkg/sse`。

| 通道 | 端点 | 鉴权 | 说明 |
|---|---|---|---|
| WebSocket | `/api/v1/ws` | `?token=`（JWT） | 双向；收到的消息广播给所有连接（echo 示例） |
| SSE | `/api/v1/sse` | 公开 | 单向推送；每 2 秒推送服务器时间（示例） |

```bash
# WebSocket 连接（浏览器无法带 Authorization 头，用 query 传 token）
# ws://127.0.0.1:8080/api/v1/ws?token=<access_token>

# SSE 监听
curl -N http://127.0.0.1:8080/api/v1/sse
```

核心能力：
- **WebSocket**：`ws.NewHub()` 连接管理（注册/注销/广播/单发），内置心跳 ping/pong 保活与慢连接清理；升级前用 `jwtpkg.ParseToken` 校验 query 参数中的 token。
- **SSE**：`sse.Init()` 设置响应头并返回断连信号，`sse.Event{...}.Write()` 发送事件（支持 event/id/retry），自动 flush。

业务接入：需要服务端主动推送时，注入 Hub 或直接写 SSE 流即可，示例见 `internal/handler/ws.go`、`internal/handler/sse.go`。

## gRPC

集成 gRPC server（与 HTTP 共享 service 层）。为避免未装 `protoc` 时默认构建失败，**所有 gRPC 代码用 `//go:build grpc` build tag 隔离**，默认 `go build` 不引入。

### 启用步骤

```bash
# 1. 装 protoc 工具链（macOS）
brew install protobuf
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# 2. 生成 stub
make proto     # 输出到 proto/gen/go/{user.pb.go, user_grpc.pb.go}

# 3. 启用 gRPC 编译与运行
make grpc-build
make grpc-run  # 等价 go run -tags grpc ./cmd/server
```

### 服务与端点

| 服务 | 方法 | 说明 |
|---|---|---|
| `UserService` | `GetUser` / `ListUsers` / `CreateUser` / `DeleteUser` | 复用 `service.UserService` |
| `Greeter` | `SayHello` | 演示服务 |

默认监听 `0.0.0.0:9091`（`grpc.addr` 可调），启用 `reflection` 可用 grpcurl 直接调试：

```bash
# 列出服务
grpcurl -plaintext 127.0.0.1:9091 list
# 调用
grpcurl -plaintext -d '{"name":"world"}' 127.0.0.1:9091 goboilerplate.v1.Greeter/SayHello
```

业务错误自动映射 gRPC `codes`（`NotFound` / `AlreadyExists` / `InvalidArgument` / `Unauthenticated` / `PermissionDenied` / `Internal`）。

## 服务注册中心

启动时向 Nacos 注册本实例（IP + 端口 + 元数据），退出时主动注销；支持服务发现（`GetHealthy`）与变更订阅（`Subscribe`）。复用 `nacos-sdk-go`（与配置中心同 SDK，但 config/naming client 独立）。

```yaml
registry:
  enable: true
  type: nacos
  nacos_addrs: [ "127.0.0.1:8848" ]
  service_name: go-boilerplate
  group: DEFAULT_GROUP
  ephemeral: true                 # 临时实例（SDK 自动心跳）；持久实例需手动 beat
  metadata: { version: "1.0", env: "dev" }
```

业务接入：

```go
provider, _ := registry.New(registry.Options{...})

// 服务发现
ins, _ := provider.GetHealthy(ctx, "user-service", "DEFAULT_GROUP")

// 订阅变更
provider.Subscribe(ctx, "user-service", "DEFAULT_GROUP", func(ins []registry.Instance) {
    // ins 即为最新健康实例列表
})
```

`pkg/registry` 已定义 `Provider` 接口（`Register` / `Deregister` / `GetHealthy` / `Subscribe` / `Close`），后续可扩展 Consul / etcd 实现。

## 核心 API

| 方法 | 路径 | 认证 | 说明 |
|---|---|---|---|
| POST | /api/v1/auth/login | 公开 | 登录，返回双令牌 |
| POST | /api/v1/auth/refresh | 公开 | 刷新令牌 |
| GET | /api/v1/healthz | 公开 | 存活探针 |
| GET | /api/v1/readyz | 公开 | 就绪探针（检查数据库/Redis） |
| GET | /api/v1/users | admin | 分页用户列表 |
| POST | /api/v1/users | admin | 创建用户 |
| GET | /api/v1/users/:id | admin / user | 用户详情 |
| PUT | /api/v1/users/:id | admin | 更新用户 |
| DELETE | /api/v1/users/:id | admin | 删除用户 |

## 业务扩展示例

新增一个业务模块（以 `order` 为例）：

1. `internal/model/order.go` — 定义 GORM 模型
2. `internal/repository/order.go` — 数据访问
3. `internal/service/order.go` — 业务逻辑
4. `internal/handler/order.go` — HTTP 接口 + swagger 注解
5. `internal/router/router.go` — 注册路由（放进 `protected` 组自动获得 JWT + RBAC）
6. `migrations/{mysql,postgres,sqlite}/000002_add_orders.up.sql` — 各方言建表迁移，`make migrate-up`
7. 需要新权限时在 `pkg/casbinx/casbinx.go` 的 SeedPolicies 中追加策略
8. 发消息：注入 `*mqpkg.Producer`，调用 `SendSync(ctx, topic, tag, body)`
9. `make swagger` 更新文档

## 常用命令

```bash
make run           # 热重载启动
make build         # 编译二进制到 bin/server
make test          # 单元测试
make lint          # go vet
make swagger       # 生成/更新 Swagger
make migrate-up    # 迁移到最新
make migrate-down  # 回滚 1 个版本
make docker-up     # 拉起依赖容器
make docker-down   # 停止依赖容器
```

## 生产部署注意

- `jwt.secret` 必须替换为 32 字节以上随机值
- `app.env` 设为 `prod`（gin release 模式 + JSON 日志）
- `otel.sample_ratio` 调低（如 0.1）
- 数据库/Redis 密码走环境变量或配置中心，勿提交仓库
- 建表二选一：AutoMigrate（`auto_migrate: true`，默认）或 `make migrate-up`
