package queue

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"notification-service/internal/config"
	"notification-service/internal/models"

	"github.com/segmentio/kafka-go"
)

type Producer struct {
	writers map[string]*kafka.Writer
	cfg     *config.Config
}

func NewProducer(cfg *config.Config) *Producer {
	brokers := strings.Split(cfg.KafkaBrokers, ",")

	writers := map[string]*kafka.Writer{
		string(models.PriorityHigh): &kafka.Writer{
			Addr:     kafka.TCP(brokers...),
			Topic:    cfg.TopicHigh,
			Balancer: &kafka.LeastBytes{},
			Async:    true,
		},
		string(models.PriorityMedium): &kafka.Writer{
			Addr:     kafka.TCP(brokers...),
			Topic:    cfg.TopicMedium,
			Balancer: &kafka.LeastBytes{},
			Async:    true,
		},
		string(models.PriorityLow): &kafka.Writer{
			Addr:     kafka.TCP(brokers...),
			Topic:    cfg.TopicLow,
			Balancer: &kafka.LeastBytes{},
			Async:    true,
		},
		"DLQ": &kafka.Writer{
			Addr:     kafka.TCP(brokers...),
			Topic:    cfg.TopicDLQ,
			Balancer: &kafka.LeastBytes{},
		},
	}

	return &Producer{writers: writers, cfg: cfg}
}

func (p *Producer) Publish(ctx context.Context, msg models.KafkaMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	priority := string(msg.Priority)
	if priority == "" {
		priority = string(models.PriorityMedium)
	}

	writer, ok := p.writers[priority]
	if !ok {
		writer = p.writers[string(models.PriorityMedium)]
	}

	err = writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(msg.NotificationID),
		Value: data,
	})
	if err != nil {
		log.Printf("❌ Kafka publish failed for %s: %v", msg.NotificationID, err)
	}
	return err
}

func (p *Producer) PublishToDLQ(ctx context.Context, msg models.KafkaMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	writer := p.writers["DLQ"]
	return writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(msg.NotificationID),
		Value: data,
	})
}

func (p *Producer) PublishBatch(ctx context.Context, messages []models.KafkaMessage) error {
	grouped := make(map[string][]kafka.Message)

	for _, msg := range messages {
		data, err := json.Marshal(msg)
		if err != nil {
			continue
		}
		priority := string(msg.Priority)
		if priority == "" {
			priority = string(models.PriorityMedium)
		}
		grouped[priority] = append(grouped[priority], kafka.Message{
			Key:   []byte(msg.NotificationID),
			Value: data,
		})
	}

	for priority, msgs := range grouped {
		writer, ok := p.writers[priority]
		if !ok {
			writer = p.writers[string(models.PriorityMedium)]
		}
		if err := writer.WriteMessages(ctx, msgs...); err != nil {
			log.Printf("❌ Batch publish failed for priority %s: %v", priority, err)
			return err
		}
	}
	return nil
}

func (p *Producer) Close() {
	for _, w := range p.writers {
		w.Close()
	}
}
