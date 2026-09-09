// Package mq 提供 RocketMQ Producer / PushConsumer 封装。
// 生产者：同步发送；消费者：集群模式（Clustering）推送消费。
package mq

import (
	"context"
	"fmt"

	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/apache/rocketmq-client-go/v2/producer"
)

// ---------- Producer ----------

// ProducerOptions 生产者配置
type ProducerOptions struct {
	Endpoints []string // NameServer 地址列表
	Retry     int      // 发送失败重试次数
}

// Producer 同步发送封装
type Producer struct {
	p rocketmq.Producer
}

// NewProducer 创建并启动生产者
func NewProducer(opts ProducerOptions) (*Producer, error) {
	p, err := rocketmq.NewProducer(
		producer.WithNameServer(opts.Endpoints),
		producer.WithRetry(opts.Retry),
	)
	if err != nil {
		return nil, fmt.Errorf("mq: new producer failed: %w", err)
	}
	if err := p.Start(); err != nil {
		return nil, fmt.Errorf("mq: producer start failed: %w", err)
	}
	return &Producer{p: p}, nil
}

// SendSync 同步发送一条消息。
// body 为业务 JSON；tag 用于消费端过滤；返回 MessageQueue 与 MsgId。
func (m *Producer) SendSync(ctx context.Context, topic, tag string, body []byte) (*primitive.SendResult, error) {
	msg := &primitive.Message{
		Topic: topic,
		Body:  body,
	}
	msg.WithTag(tag)

	res, err := m.p.SendSync(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("mq: send sync failed: %w", err)
	}
	return res, nil
}

// Shutdown 关闭生产者（服务退出时调用）
func (m *Producer) Shutdown() error {
	if m == nil || m.p == nil {
		return nil
	}
	return m.p.Shutdown()
}

// ---------- Consumer ----------

// ConsumerOptions 消费者配置
type ConsumerOptions struct {
	Endpoints []string
	Group     string // 消费组（集群模式同一组内负载均衡）
	// ConsumeOrderly 预留：client-go v2.1.2 未提供顺序消费开关，
	// 顺序语义需在 MessageHandler 内自行保证（单队列串行）。
}

// MessageHandler 消费回调：返回 ConsumeSuccess 表示消费成功
type MessageHandler func(ctx context.Context, msgs ...*primitive.MessageExt) (consumer.ConsumeResult, error)

// Consumer 推送消费者封装
type Consumer struct {
	c rocketmq.PushConsumer
}

// NewPushConsumer 创建消费者（未启动）
func NewPushConsumer(opts ConsumerOptions) (*Consumer, error) {
	opts2 := []consumer.Option{
		consumer.WithNameServer(opts.Endpoints),
		consumer.WithGroupName(opts.Group),
		consumer.WithConsumerModel(consumer.Clustering),
	}

	c, err := rocketmq.NewPushConsumer(opts2...)
	if err != nil {
		return nil, fmt.Errorf("mq: new consumer failed: %w", err)
	}
	return &Consumer{c: c}, nil
}

// Subscribe 订阅 topic。selector 传 consumer.MessageSelector{} 订阅全部 tag。
func (m *Consumer) Subscribe(topic string, selector consumer.MessageSelector, handler MessageHandler) error {
	if err := m.c.Subscribe(topic, selector, handler); err != nil {
		return fmt.Errorf("mq: subscribe %s failed: %w", topic, err)
	}
	return nil
}

// Start 启动消费
func (m *Consumer) Start() error {
	if err := m.c.Start(); err != nil {
		return fmt.Errorf("mq: consumer start failed: %w", err)
	}
	return nil
}

// Shutdown 关闭消费者（服务退出时调用）
func (m *Consumer) Shutdown() error {
	if m == nil || m.c == nil {
		return nil
	}
	return m.c.Shutdown()
}
