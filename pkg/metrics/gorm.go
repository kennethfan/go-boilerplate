package metrics

import (
	"time"

	"gorm.io/gorm"
)

// GormPlugin 采集 GORM SQL 指标（查询量 / 耗时 / 错误）。
// 通过 gorm 回调在操作前后埋点，按操作类型（create/query/update/delete/raw）分组。
type GormPlugin struct{}

// Name 插件名
func (GormPlugin) Name() string { return "gorm_prometheus" }

// Initialize 注册各操作的 before/after 回调
func (GormPlugin) Initialize(db *gorm.DB) error {
	cb := db.Callback()

	// 每个执行链：before 记录开始时间，after 统计耗时/结果
	if err := cb.Query().Before("gorm:query").Register("metrics:before_query", beforeQuery); err != nil {
		return err
	}
	if err := cb.Query().After("gorm:query").Register("metrics:after_query", afterQuery("query")); err != nil {
		return err
	}
	if err := cb.Create().Before("gorm:create").Register("metrics:before_create", beforeQuery); err != nil {
		return err
	}
	if err := cb.Create().After("gorm:create").Register("metrics:after_create", afterQuery("create")); err != nil {
		return err
	}
	if err := cb.Update().Before("gorm:update").Register("metrics:before_update", beforeQuery); err != nil {
		return err
	}
	if err := cb.Update().After("gorm:update").Register("metrics:after_update", afterQuery("update")); err != nil {
		return err
	}
	if err := cb.Delete().Before("gorm:delete").Register("metrics:before_delete", beforeQuery); err != nil {
		return err
	}
	if err := cb.Delete().After("gorm:delete").Register("metrics:after_delete", afterQuery("delete")); err != nil {
		return err
	}
	if err := cb.Raw().Before("gorm:raw").Register("metrics:before_raw", beforeQuery); err != nil {
		return err
	}
	return cb.Raw().After("gorm:raw").Register("metrics:after_raw", afterQuery("raw"))
}

const metricsStartKey = "metrics_start_time"

func beforeQuery(db *gorm.DB) {
	db.InstanceSet(metricsStartKey, time.Now())
}

func afterQuery(operation string) func(*gorm.DB) {
	return func(db *gorm.DB) {
		start, ok := db.InstanceGet(metricsStartKey)
		if !ok {
			return
		}
		dur := time.Since(start.(time.Time)).Seconds()

		status := "ok"
		if db.Error != nil {
			status = "error"
		}
		GormQueriesTotal.WithLabelValues(operation, status).Inc()
		GormQueryDuration.WithLabelValues(operation).Observe(dur)
	}
}
