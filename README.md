# 🔔 Notification Service

A Go microservice for handling email, SMS, and push notifications with Kafka-based async processing, circuit breaker pattern, and priority queues. Supports Twilio (SMS), SMTP (Email), and FCM (Push).

> **Note:** For the e-commerce platform, order SMS notifications are now sent directly via MSG91 from the core service. This notification service is for advanced use cases (email, push, bulk notifications).

## 🚀 Quick Start

```bash
cd notification-service
go mod tidy
MONGO_URI="mongodb://localhost:27017/notifications" go run ./cmd/api/
```

Service starts on **http://localhost:9090**

## ✅ What's Done

| Feature | Status | Details |
|---------|--------|---------|
| REST API | ✅ Complete | Send notifications via HTTP endpoint |
| Kafka Queues | ✅ Complete | Priority queues (HIGH, MEDIUM, LOW) + DLQ |
| Email Adapter | ✅ Complete | SMTP integration (Gmail, SendGrid, SES) |
| SMS Adapter | ✅ Complete | Twilio integration |
| Push Adapter | ✅ Complete | Firebase Cloud Messaging (FCM) |
| Circuit Breaker | ✅ Complete | Auto-opens on failures, half-open recovery |
| Worker Pool | ✅ Complete | Concurrent workers with batch processing |
| MongoDB Storage | ✅ Complete | Notification history and status tracking |
| Redis Dedup | ✅ Complete | Deduplication to prevent duplicate sends |
| Retry Logic | ✅ Complete | Configurable retries with backoff |
| Docker | ✅ Complete | Dockerfile + docker-compose with Kafka, MongoDB, Redis |

## ❌ What's Left To Do

### 🔴 Must Have (Before Production)
- [ ] **Kafka Setup** — Need Kafka running (use docker-compose or Confluent Cloud free tier)
- [ ] **SMTP Credentials** — Set `SMTP_HOST`, `SMTP_USER`, `SMTP_PASSWORD` for email
- [ ] **Twilio Account** — Set `TWILIO_SID`, `TWILIO_TOKEN`, `TWILIO_FROM` for SMS (or skip — core service uses MSG91 directly)
- [ ] **FCM Server Key** — Set `FCM_SERVER_KEY` for push notifications (or skip if not needed)

### 🟡 Should Have
- [ ] **Email Templates** — HTML templates for order confirmation, shipping updates, welcome email
- [ ] **Notification Preferences** — User opt-in/opt-out per channel
- [ ] **Scheduled Notifications** — Send at specific time (promotional campaigns)
- [ ] **Bulk Send** — Batch notifications to multiple recipients
- [ ] **Delivery Tracking** — Track email opens, click-through rates
- [ ] **Admin API** — List all notifications, retry failed, view analytics
- [ ] **Rate Limiting** — Per-recipient rate limiting to prevent spam

### 🔵 Nice To Have
- [ ] **WhatsApp Integration** — WhatsApp Business API for notifications
- [ ] **Webhook Callbacks** — Notify the calling service when notification is delivered/failed
- [ ] **Template Engine** — Dynamic templates with variable substitution
- [ ] **Multi-tenant** — Support multiple apps/services
- [ ] **Dashboard UI** — Web UI for notification history and analytics
- [ ] **SNS/SQS Support** — AWS-native alternative to Kafka

## 🏗 Architecture

```
notification-service/
├── cmd/api/main.go                    # Entry point
├── internal/
│   ├── handler/handler.go             # REST API handler
│   ├── worker/worker.go               # Kafka consumer + processor
│   ├── queue/producer.go              # Kafka producer
│   ├── adapter/
│   │   ├── adapter.go                 # Adapter interface
│   │   ├── email/email.go             # SMTP email sender
│   │   ├── sms/sms.go                 # Twilio SMS sender
│   │   └── push/push.go              # FCM push sender
│   ├── circuit/breaker.go             # Circuit breaker pattern
│   ├── store/
│   │   ├── mongo.go                   # MongoDB persistence
│   │   └── redis.go                   # Redis dedup cache
│   ├── models/notification.go         # Data models
│   └── config/config.go               # Configuration
├── docker-compose.yml                 # Kafka + MongoDB + Redis + Service
├── Dockerfile
└── go.mod
```

## ⚙️ Configuration

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| PORT | 9090 | No | Service port |
| MONGO_URI | mongodb://localhost:27017/notifications | **Yes** | MongoDB |
| REDIS_ADDR | localhost:6379 | Yes | Redis for dedup |
| KAFKA_BROKERS | localhost:9092 | Yes | Kafka brokers |
| SMTP_HOST | — | For email | SMTP server |
| SMTP_PORT | 587 | No | SMTP port |
| SMTP_USER | — | For email | SMTP username |
| SMTP_PASSWORD | — | For email | SMTP password |
| SMTP_FROM | noreply@example.com | No | From address |
| TWILIO_SID | — | For SMS | Twilio Account SID |
| TWILIO_TOKEN | — | For SMS | Twilio Auth Token |
| TWILIO_FROM | — | For SMS | Twilio phone number |
| FCM_SERVER_KEY | — | For push | Firebase server key |

## 📄 License
MIT
