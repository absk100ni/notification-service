package models

import "time"

// NotificationType represents the channel
type NotificationType string

const (
	TypeEmail NotificationType = "EMAIL"
	TypeSMS   NotificationType = "SMS"
	TypePush  NotificationType = "PUSH"
)

// TriggerType represents how the notification was triggered
type TriggerType string

const (
	TriggerInstant   TriggerType = "INSTANT"
	TriggerScheduled TriggerType = "SCHEDULED"
	TriggerBulk      TriggerType = "BULK"
)

// Priority levels
type Priority string

const (
	PriorityHigh   Priority = "HIGH"
	PriorityMedium Priority = "MEDIUM"
	PriorityLow    Priority = "LOW"
)

// Status represents notification delivery status
type Status string

const (
	StatusTriggered  Status = "TRIGGERED"
	StatusProcessing Status = "PROCESSING"
	StatusSent       Status = "SENT"
	StatusFailed     Status = "FAILED"
	StatusDLQ        Status = "DEAD_LETTERED"
)

// Notification is the core entity
type Notification struct {
	ID          string            `json:"notification_id" bson:"_id"`
	Recipient   string            `json:"recipient" bson:"recipient"`
	Type        NotificationType  `json:"type" bson:"type"`
	Metadata    map[string]string `json:"metadata" bson:"metadata"`
	TriggerType TriggerType       `json:"trigger_type" bson:"trigger_type"`
	Status      Status            `json:"status" bson:"status"`
	Priority    Priority          `json:"priority" bson:"priority"`
	CampaignID  string            `json:"campaign_id,omitempty" bson:"campaign_id,omitempty"`
	RetryCount  int               `json:"retry_count" bson:"retry_count"`
	Error       string            `json:"error,omitempty" bson:"error,omitempty"`
	CreatedAt   time.Time         `json:"created_at" bson:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at" bson:"updated_at"`
	ScheduledAt *time.Time        `json:"scheduled_at,omitempty" bson:"scheduled_at,omitempty"`
	SentAt      *time.Time        `json:"sent_at,omitempty" bson:"sent_at,omitempty"`
}

// SendRequest is the API request for sending a single notification
type SendRequest struct {
	Type        NotificationType  `json:"type" binding:"required"`
	Recipient   string            `json:"recipient" binding:"required"`
	Metadata    map[string]string `json:"metadata" binding:"required"`
	TriggerType TriggerType       `json:"trigger_type"`
	Priority    Priority          `json:"priority"`
	ScheduledAt *time.Time        `json:"scheduled_at,omitempty"`
}

// BulkRequest is the API request for bulk campaign notifications
type BulkRequest struct {
	CampaignID string            `json:"campaign_id" binding:"required"`
	Type       NotificationType  `json:"type" binding:"required"`
	Recipients []string          `json:"recipients" binding:"required"`
	Metadata   map[string]string `json:"metadata" binding:"required"`
	Priority   Priority          `json:"priority"`
}

// KafkaMessage is what gets published to Kafka
type KafkaMessage struct {
	NotificationID string            `json:"notification_id"`
	Recipient      string            `json:"recipient"`
	Type           NotificationType  `json:"type"`
	Metadata       map[string]string `json:"metadata"`
	Priority       Priority          `json:"priority"`
	RetryCount     int               `json:"retry_count"`
}

// BulkResponse returned after bulk submission
type BulkResponse struct {
	CampaignID string `json:"campaign_id"`
	Total      int    `json:"total"`
	Accepted   int    `json:"accepted"`
	Status     string `json:"status"`
}
