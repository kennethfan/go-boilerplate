package cron

import (
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// zapCronLogger 把 robfig cron.Logger 转给 zap。
//
// robfig/cron/v3 v3.0.1 的 Logger 接口：
//
//	type Logger interface {
//	    Info(msg string, keysAndValues ...interface{})
//	    Error(err error, msg string, keysAndValues ...interface{})
//	}
//
// 与 zap 不同的是它接受 k/v 列表而不是 zap.Field 数组；这里统一转 Field。
type zapCronLogger struct {
	log *zap.Logger
}

// ZapCronLogger 构造基于 zap 的 cron.Logger 实现。
//
//	sched := cron.New(cron.Options{Logger: cron.ZapCronLogger(zap.L())})
func ZapCronLogger(log *zap.Logger) cron.Logger {
	return &zapCronLogger{log: log}
}

func (z *zapCronLogger) Info(msg string, keysAndValues ...interface{}) {
	z.log.Info(msg, kvsToFields(keysAndValues)...)
}

func (z *zapCronLogger) Error(err error, msg string, keysAndValues ...interface{}) {
	fields := append([]zap.Field{zap.Error(err)}, kvsToFields(keysAndValues)...)
	z.log.Error(msg, fields...)
}

// kvsToFields 把 [k1, v1, k2, v2, ...] 转为 zap.Field 列表。
// 长度为奇数时丢弃最后一对，避免格式不匹配。
// key 非 string 时退化命名，以 zap.Any 序列化保证不丢信息。
func kvsToFields(kvs []interface{}) []zap.Field {
	n := len(kvs)
	if n == 0 {
		return nil
	}
	if n%2 != 0 {
		kvs = kvs[:n-1]
	}
	fields := make([]zap.Field, 0, len(kvs)/2)
	for i := 0; i < len(kvs); i += 2 {
		key, _ := kvs[i].(string)
		if key == "" {
			key = "arg"
		}
		fields = append(fields, zap.Any(key, kvs[i+1]))
	}
	return fields
}
