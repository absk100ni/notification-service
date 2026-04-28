package push

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"notification-service/internal/config"
	"notification-service/internal/models"
)

type PushAdapter struct {
	cfg *config.Config
}

func New(cfg *config.Config) *PushAdapter {
	return &PushAdapter{cfg: cfg}
}

func (a *PushAdapter) Type() models.NotificationType {
	return models.TypePush
}

func (a *PushAdapter) Send(msg models.KafkaMessage) error {
	title := msg.Metadata["title"]
	body := msg.Metadata["body"]
	token := msg.Recipient // For push, recipient is device token

	if title == "" {
		title = "Notification"
	}
	if body == "" {
		body = "You have a new notification."
	}

	// If FCM not configured, mock
	if a.cfg.FCMServerKey == "" {
		log.Printf("🔔 [PUSH-MOCK] Token: %s | Title: %s | Body: %s", token[:min(20, len(token))], title, body)
		return nil
	}

	// Real FCM HTTP v1 API
	payload := map[string]interface{}{
		"to": token,
		"notification": map[string]string{
			"title": title,
			"body":  body,
		},
		"data": msg.Metadata,
	}
	data, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", "https://fcm.googleapis.com/fcm/send", bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("push request creation failed: %w", err)
	}
	req.Header.Set("Authorization", "key="+a.cfg.FCMServerKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("push send failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("FCM returned status %d", resp.StatusCode)
	}

	log.Printf("🔔 [PUSH] Sent to %s: %s", token[:min(20, len(token))], title)
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
