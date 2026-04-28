package adapter

import "notification-service/internal/models"

// Adapter is the interface all notification channel adapters must implement
type Adapter interface {
	Send(notification models.KafkaMessage) error
	Type() models.NotificationType
}
