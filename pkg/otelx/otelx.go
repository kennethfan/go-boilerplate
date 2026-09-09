// Package otelx 提供 OpenTelemetry TracerProvider 初始化。
// 使用 OTLP gRPC exporter 上报（对接 Jaeger / Tempo / OTel Collector），
// W3C TraceContext 传播，按比例采样。
package otelx

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Options OTel 配置
type Options struct {
	Enable      bool
	ServiceName string
	Endpoint    string // OTLP gRPC 端口，如 127.0.0.1:4317
	Insecure    bool
	SampleRatio float64 // [0, 1]
}

// Init 初始化全局 TracerProvider 与 propagator。
// 返回 shutdown 函数，服务退出时调用以 flush 缓冲的 span。
func Init(ctx context.Context, opts Options) (func(context.Context) error, error) {
	if !opts.Enable {
		// 未启用时返回 no-op，调用方无感知
		return func(context.Context) error { return nil }, nil
	}

	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(opts.Endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("otelx: create exporter failed: %w", err)
	}

	res, err := sdkresource.New(ctx,
		sdkresource.WithAttributes(semconv.ServiceName(opts.ServiceName)),
	)
	if err != nil {
		return nil, fmt.Errorf("otelx: create resource failed: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(opts.SampleRatio)),
	)

	otel.SetTracerProvider(tp)
	// W3C TraceContext：跨服务 trace 透传标准
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}
