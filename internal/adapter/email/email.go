package email

import (
	"fmt"
	"log"
	"net/smtp"

	"notification-service/internal/config"
	"notification-service/internal/models"
)

type EmailAdapter struct {
	cfg *config.Config
}

func New(cfg *config.Config) *EmailAdapter {
	return &EmailAdapter{cfg: cfg}
}

func (a *EmailAdapter) Type() models.NotificationType {
	return models.TypeEmail
}

func (a *EmailAdapter) Send(msg models.KafkaMessage) error {
	subject := msg.Metadata["subject"]
	body := msg.Metadata["body"]
	to := msg.Recipient

	if subject == "" {
		subject = "Notification"
	}
	if body == "" {
		body = "You have a new notification."
	}

	// If SMTP not configured, log and simulate success
	if a.cfg.SMTPHost == "" {
		log.Printf("📧 [EMAIL-MOCK] To: %s | Subject: %s | Body: %s", to, subject, body[:min(50, len(body))])
		return nil
	}

	// Real SMTP sending
	message := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		a.cfg.SMTPFrom, to, subject, body)

	auth := smtp.PlainAuth("", a.cfg.SMTPUser, a.cfg.SMTPPassword, a.cfg.SMTPHost)
	addr := fmt.Sprintf("%s:%s", a.cfg.SMTPHost, a.cfg.SMTPPort)

	err := smtp.SendMail(addr, auth, a.cfg.SMTPFrom, []string{to}, []byte(message))
	if err != nil {
		return fmt.Errorf("email send failed: %w", err)
	}

	log.Printf("📧 [EMAIL] Sent to %s: %s", to, subject)
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
