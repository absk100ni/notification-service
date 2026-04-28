package worker

import (
	"context"
	"encoding/json"
	"log"
	"math"
	"strings"
	"time"

	"notification-service/internal/adapter"
	"notification-service/internal/adapter/email"
	"notification-service/internal/adapter/push"
	"notification-service/internal/adapter/sms"
	"notification-service/internal/circuit"
	"notification-service/internal/config"
	"notification-service/internal/models"
	"notification-service/internal/queue"
	"notification-service/internal/store"

	"github.com/segmentio/kafka-go"
)

type Worker struct {
	cfg      *config.Config
	store    *store.MongoStore
	cache    *store.RedisCache
	producer *queue.Producer
	adapters map[models.NotificationType]adapter.Adapter
	breakers map[models.NotificationType]*circuit.Breaker
	readers  []*kafka.Reader
}

func NewWorker(cfg *config.Config, s *store.MongoStore, c *store.RedisCache, p *queue.Producer) *Worker {
	// Initialize adapters
	adapters := map[models.NotificationType]adapter.Adapter{
		models.TypeEmail: email.New(cfg),
		models.TypeSMS:   sms.New(cfg),
		models.TypePush:  push.New(cfg),
	}

	// Initialize circuit breakers per channel
	breakers := map[models.NotificationType]*circuit.Breaker{
		models.TypeEmail: circuit.NewBreaker(5, 30*time.Second),
		models.TypeSMS:   circuit.NewBreaker(5, 30*time.Second),
		models.TypePush:  circuit.NewBreaker(5, 30*time.Second),
	}

	brokers := strings.Split(cfg.KafkaBrokers, ",")

	// Create readers for each priority topic (high gets more consumers)
	readers := []*kafka.Reader{
		kafka.NewReader(kafka.ReaderConfig{
			Brokers:  brokers,
			Topic:    cfg.TopicHigh,
			GroupID:  "notif-worker-high",
			MinBytes: 1,
			MaxBytes: 10e6,
			MaxWait:  500 * time.Millisecond,
		}),
		kafka.NewReader(kafka.ReaderConfig{
			Brokers:  brokers,
			Topic:    cfg.TopicMedium,
			GroupID:  "notif-worker-medium",
			MinBytes: 1,
			MaxBytes: 10e6,
			MaxWait:  1 * time.Second,
		}),
		kafka.NewReader(kafka.ReaderConfig{
			Brokers:  brokers,
			Topic:    cfg.TopicLow,
			GroupID:  "notif-worker-low",
			MinBytes: 1,
			MaxBytes: 10e6,
			MaxWait:  2 * time.Second,
		}),
	}

	return &Worker{
		cfg:      cfg,
		store:    s,
		cache:    c,
		producer: p,
		adapters: adapters,
		breakers: breakers,
		readers:  readers,
	}
}

func (w *Worker) Start(ctx context.Context) {
	log.Println("🔧 Notification worker started")

	// Start a consumer goroutine for each topic
	topics := []string{"HIGH", "MEDIUM", "LOW"}
	for i, reader := range w.readers {
		go w.consumeTopic(ctx, reader, topics[i])
	}

	<-ctx.Done()
	log.Println("Worker shutting down...")
	for _, r := range w.readers {
		r.Close()
	}
}

func (w *Worker) consumeTopic(ctx context.Context, reader *kafka.Reader, priority string) {
	log.Printf("📨 Consuming from %s priority topic", priority)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		msg, err := reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("Read error [%s]: %v", priority, err)
			time.Sleep(time.Second)
			continue
		}

		var kafkaMsg models.KafkaMessage
		if err := json.Unmarshal(msg.Value, &kafkaMsg); err != nil {
			log.Printf("Unmarshal error: %v", err)
			continue
		}

		w.processMessage(ctx, kafkaMsg)
	}
}

func (w *Worker) processMessage(ctx context.Context, msg models.KafkaMessage) {
	id := msg.NotificationID
	log.Printf("📦 Processing %s [%s] → %s", msg.Type, id[:8], msg.Recipient)

	// Update status to PROCESSING
	w.store.UpdateStatus(ctx, id, models.StatusProcessing, "")
	w.cache.InvalidateStatus(ctx, id)

	// Get adapter for this notification type
	adpt, ok := w.adapters[msg.Type]
	if !ok {
		log.Printf("❌ Unknown notification type: %s", msg.Type)
		w.store.UpdateStatus(ctx, id, models.StatusFailed, "Unknown notification type")
		return
	}

	// Get circuit breaker
	breaker, ok := w.breakers[msg.Type]
	if !ok {
		breaker = circuit.NewBreaker(5, 30*time.Second)
	}

	// Execute with circuit breaker
	err := breaker.Execute(func() error {
		return adpt.Send(msg)
	})

	if err != nil {
		w.handleFailure(ctx, msg, err)
		return
	}

	// Success!
	w.store.UpdateStatus(ctx, id, models.StatusSent, "")
	w.cache.InvalidateStatus(ctx, id)
	log.Printf("✅ Sent %s [%s] → %s", msg.Type, id[:8], msg.Recipient)
}

func (w *Worker) handleFailure(ctx context.Context, msg models.KafkaMessage, err error) {
	id := msg.NotificationID
	msg.RetryCount++

	log.Printf("❌ Failed %s [%s] attempt %d: %v", msg.Type, id[:8], msg.RetryCount, err)

	// Increment retry in DB
	w.store.IncrementRetry(ctx, id)

	if msg.RetryCount >= w.cfg.MaxRetries {
		// Max retries exhausted → try fallback or send to DLQ
		if w.tryFallback(ctx, msg) {
			return
		}

		// Dead letter queue
		log.Printf("💀 DLQ: %s [%s] after %d retries", msg.Type, id[:8], msg.RetryCount)
		w.store.UpdateStatus(ctx, id, models.StatusDLQ, err.Error())
		w.producer.PublishToDLQ(ctx, msg)
		w.cache.InvalidateStatus(ctx, id)
		return
	}

	// Retry with exponential backoff
	backoff := time.Duration(math.Pow(2, float64(msg.RetryCount))) * time.Second
	log.Printf("🔄 Retrying %s [%s] in %v", msg.Type, id[:8], backoff)

	time.Sleep(backoff)

	// Re-publish to same priority topic
	if pubErr := w.producer.Publish(ctx, msg); pubErr != nil {
		w.store.UpdateStatus(ctx, id, models.StatusFailed, "Retry publish failed: "+pubErr.Error())
	}
}

// tryFallback attempts to send via fallback channel (Push → SMS)
func (w *Worker) tryFallback(ctx context.Context, msg models.KafkaMessage) bool {
	var fallbackType models.NotificationType

	switch msg.Type {
	case models.TypePush:
		fallbackType = models.TypeSMS
	default:
		return false
	}

	fallbackAdapter, ok := w.adapters[fallbackType]
	if !ok {
		return false
	}

	log.Printf("🔀 Fallback: %s → %s for [%s]", msg.Type, fallbackType, msg.NotificationID[:8])

	fallbackMsg := msg
	fallbackMsg.Type = fallbackType

	if err := fallbackAdapter.Send(fallbackMsg); err != nil {
		log.Printf("❌ Fallback %s also failed: %v", fallbackType, err)
		return false
	}

	w.store.UpdateStatus(ctx, msg.NotificationID, models.StatusSent, "Delivered via fallback: "+string(fallbackType))
	w.cache.InvalidateStatus(ctx, msg.NotificationID)
	log.Printf("✅ Fallback success: %s [%s]", fallbackType, msg.NotificationID[:8])
	return true
}
