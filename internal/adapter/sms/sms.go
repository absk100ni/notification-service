package sms

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"notification-service/internal/config"
	"notification-service/internal/models"
)

type SMSAdapter struct {
	cfg *config.Config
}

func New(cfg *config.Config) *SMSAdapter {
	return &SMSAdapter{cfg: cfg}
}

func (a *SMSAdapter) Type() models.NotificationType {
	return models.TypeSMS
}

func (a *SMSAdapter) Send(msg models.KafkaMessage) error {
	body := msg.Metadata["body"]
	to := msg.Recipient

	if body == "" {
		body = "You have a new notification."
	}

	// If Twilio not configured, mock
	if a.cfg.TwilioSID == "" {
		log.Printf("📱 [SMS-MOCK] To: %s | Body: %s", to, body)
		return nil
	}

	// Real Twilio API call
	url := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", a.cfg.TwilioSID)

	payload := map[string]string{
		"To":   to,
		"From": a.cfg.TwilioFrom,
		"Body": body,
	}
	data, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("sms request creation failed: %w", err)
	}
	req.SetBasicAuth(a.cfg.TwilioSID, a.cfg.TwilioToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("sms send failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("twilio returned status %d", resp.StatusCode)
	}

	log.Printf("📱 [SMS] Sent to %s", to)
	return nil
}
