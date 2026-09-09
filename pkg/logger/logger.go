// Package logger 提供 zap 结构化日志封装：
//   - 开发环境：console 彩色输出
//   - 生产环境：JSON 输出 + lumberjack 文件滚动
//   - FromContext 自动注入 OTel trace_id，实现日志与链路追踪联动
package logger

import (
	"context"
	"io"
	"os"
	"sync"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Options 日志配置
type Options struct {
	Level       string // debug / info / warn / error
	Dir         string // 日志目录，为空则不写文件
	FileName    string
	MaxSizeMB   int  // 单文件最大体积（MB）
	MaxBackups  int  // 保留旧文件数
	MaxAgeDays  int  // 旧文件保留天数
	Compress    bool // 压缩旧文件
	Development bool // true: console 格式；false: JSON 格式
}

// 全局 logger，业务代码通过 logger.FromContext(ctx) 获取
var (
	global     *zap.Logger
	globalOnce sync.Once
)

// New 初始化全局 logger。重复调用仅首次生效。
func New(opts Options) (*zap.Logger, error) {
	level := parseLevel(opts.Level)

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	var encoder zapcore.Encoder
	if opts.Development {
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	// 输出目标：生产环境同时写 stderr 与滚动文件；开发环境写 stdout
	var writers []zapcore.WriteSyncer
	if opts.Development {
		writers = append(writers, zapcore.AddSync(os.Stdout))
	} else {
		writers = append(writers, zapcore.AddSync(os.Stderr))
		if opts.Dir != "" && opts.FileName != "" {
			writers = append(writers, zapcore.AddSync(newFileWriter(opts)))
		}
	}
	core := zapcore.NewCore(encoder, zapcore.NewMultiWriteSyncer(writers...), level)

	l := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(0), zap.AddStacktrace(zapcore.ErrorLevel))

	globalOnce.Do(func() { global = l })
	return l, nil
}

// newFileWriter lumberjack 文件滚动
func newFileWriter(opts Options) io.Writer {
	return &lumberjack.Logger{
		Filename:   opts.Dir + "/" + opts.FileName,
		MaxSize:    opts.MaxSizeMB,
		MaxBackups: opts.MaxBackups,
		MaxAge:     opts.MaxAgeDays,
		Compress:   opts.Compress,
	}
}

func parseLevel(s string) zapcore.Level {
	switch s {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

// L 返回全局 logger（未初始化时返回 Nop，避免空指针）
func L() *zap.Logger {
	if global == nil {
		return zap.NewNop()
	}
	return global
}

// FromContext 从 ctx 提取 trace_id 返回带字段的 logger。
// 与 otelgin / OTel SDK 配合：HTTP 入口生成 trace 后，所有业务日志自动携带同一 trace_id。
func FromContext(ctx context.Context) *zap.Logger {
	l := L()
	if ctx == nil {
		return l
	}
	if sc := trace.SpanContextFromContext(ctx); sc.HasTraceID() {
		return l.With(zap.String("trace_id", sc.TraceID().String()))
	}
	return l
}

// Sync 退出前刷新缓冲
func Sync() {
	_ = L().Sync()
}
