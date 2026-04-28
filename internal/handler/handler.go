package handler

import (
	"context"
	"net/http"
	"time"

	"notification-service/internal/config"
	"notification-service/internal/models"
	"notification-service/internal/queue"
	"notification-service/internal/store"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	store    *store.MongoStore
	cache    *store.RedisCache
	producer *queue.Producer
	cfg      *config.Config
}

func NewHandler(s *store.MongoStore, c *store.RedisCache, p *queue.Producer, cfg *config.Config) *Handler {
	return &Handler{store: s, cache: c, producer: p, cfg: cfg}
}

// Send handles POST /notification/send
func (h *Handler) Send(c *gin.Context) {
	var req models.SendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	// Defaults
	if req.TriggerType == "" {
		req.TriggerType = models.TriggerInstant
	}
	if req.Priority == "" {
		req.Priority = models.PriorityMedium
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Duplicate check via Redis cache
	if h.cache.CheckDedup(ctx, req.Recipient, req.Type) {
		c.JSON(http.StatusConflict, gin.H{"error": "Duplicate notification suppressed"})
		return
	}

	// Create notification entity
	notif := &models.Notification{
		ID:          uuid.New().String(),
		Recipient:   req.Recipient,
		Type:        req.Type,
		Metadata:    req.Metadata,
		TriggerType: req.TriggerType,
		Status:      models.StatusTriggered,
		Priority:    req.Priority,
		RetryCount:  0,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		ScheduledAt: req.ScheduledAt,
	}

	// Persist
	if err := h.store.Create(ctx, notif); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create notification"})
		return
	}

	// Publish to Kafka
	kafkaMsg := models.KafkaMessage{
		NotificationID: notif.ID,
		Recipient:      notif.Recipient,
		Type:           notif.Type,
		Metadata:       notif.Metadata,
		Priority:       notif.Priority,
		RetryCount:     0,
	}

	if err := h.producer.Publish(ctx, kafkaMsg); err != nil {
		h.store.UpdateStatus(ctx, notif.ID, models.StatusFailed, "Queue publish failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to queue notification"})
		return
	}

	// Set dedup cache
	h.cache.SetDedup(ctx, req.Recipient, req.Type)

	c.JSON(http.StatusAccepted, gin.H{
		"notification_id": notif.ID,
		"status":          notif.Status,
	})
}

// Bulk handles POST /notification/bulk
func (h *Handler) Bulk(c *gin.Context) {
	var req models.BulkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	if req.Priority == "" {
		req.Priority = models.PriorityLow
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	batchSize := h.cfg.BatchSize
	total := len(req.Recipients)
	accepted := 0

	for i := 0; i < total; i += batchSize {
		end := i + batchSize
		if end > total {
			end = total
		}
		batch := req.Recipients[i:end]

		// Create notification entities
		notifications := make([]models.Notification, 0, len(batch))
		kafkaMessages := make([]models.KafkaMessage, 0, len(batch))

		for _, recipient := range batch {
			id := uuid.New().String()
			notifications = append(notifications, models.Notification{
				ID:          id,
				Recipient:   recipient,
				Type:        req.Type,
				Metadata:    req.Metadata,
				TriggerType: models.TriggerBulk,
				Status:      models.StatusTriggered,
				Priority:    req.Priority,
				CampaignID:  req.CampaignID,
				RetryCount:  0,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			})
			kafkaMessages = append(kafkaMessages, models.KafkaMessage{
				NotificationID: id,
				Recipient:      recipient,
				Type:           req.Type,
				Metadata:       req.Metadata,
				Priority:       req.Priority,
				RetryCount:     0,
			})
		}

		// Batch insert to MongoDB
		if err := h.store.CreateMany(ctx, notifications); err != nil {
			continue
		}

		// Batch publish to Kafka
		if err := h.producer.PublishBatch(ctx, kafkaMessages); err != nil {
			continue
		}

		accepted += len(batch)
	}

	c.JSON(http.StatusAccepted, models.BulkResponse{
		CampaignID: req.CampaignID,
		Total:      total,
		Accepted:   accepted,
		Status:     "PROCESSING",
	})
}

// Status handles GET /notification/status/:id
func (h *Handler) Status(c *gin.Context) {
	id := c.Param("id")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check Redis cache first
	if cached, err := h.cache.GetCachedStatus(ctx, id); err == nil {
		c.JSON(http.StatusOK, cached)
		return
	}

	// Cache miss → query MongoDB
	notif, err := h.store.GetByID(ctx, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Notification not found"})
		return
	}

	// Cache the result
	h.cache.CacheStatus(ctx, id, notif)

	c.JSON(http.StatusOK, notif)
}
