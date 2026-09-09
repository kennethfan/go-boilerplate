package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/kenneth/go-boilerplate/pkg/response"
)

// HealthHandler 健康检查接口
type HealthHandler struct {
	db  *gorm.DB
	rdb *redis.Client // 可为 nil：redis.enable=false 时跳过 Redis 检查
}

// NewHealthHandler 构造
// rdb 允许为 nil（redis.enable=false），此时 Readyz 不检查 Redis 连通性
func NewHealthHandler(db *gorm.DB, rdb *redis.Client) *HealthHandler {
	return &HealthHandler{db: db, rdb: rdb}
}

// Healthz godoc
// @Summary      存活探针
// @Description  K8s liveness：进程存活即返回 200
// @Tags         health
// @Produce      json
// @Success      200  {object}  response.Body
// @Router       /healthz [get]
func (h *HealthHandler) Healthz(c *gin.Context) {
	response.OK(c, gin.H{"status": "ok"})
}

// Readyz godoc
// @Summary      就绪探针
// @Description  K8s readiness：MySQL / Redis 连通才返回 200
// @Tags         health
// @Produce      json
// @Success      200  {object}  response.Body
// @Failure      503  {object}  response.Body
// @Router       /readyz [get]
func (h *HealthHandler) Ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	checks := gin.H{}

	// MySQL
	if sqlDB, err := h.db.DB(); err == nil {
		if err := sqlDB.PingContext(ctx); err != nil {
			checks["mysql"] = "down"
		} else {
			checks["mysql"] = "up"
		}
	} else {
		checks["mysql"] = "down"
	}

	// Redis
	if err := h.rdb.Ping(ctx).Err(); err != nil {
		checks["redis"] = "down"
	} else {
		checks["redis"] = "up"
	}

	for _, v := range checks {
		if v == "down" {
			c.JSON(http.StatusServiceUnavailable, response.Body{Code: 10005, Message: "依赖不可用", Data: checks})
			return
		}
	}
	response.OK(c, checks)
}
