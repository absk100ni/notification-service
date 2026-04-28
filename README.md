# 🔔 Notification Service — Scalable, Real-Time, Multi-Channel

A production-ready, independently deployable notification microservice built in **Go**. Supports **Email, SMS, and Push** notifications with **Kafka-based async processing**, **priority queues**, **circuit breakers**, **retry with exponential backoff**, and **dead-letter queues**.

## 🏗 Architecture

```
Client → API Gateway → Notification API (Gin)
                            ↓
                      MongoDB (persist TRIGGERED)
                            ↓
                      Kafka (priority topics)
                            ↓
                      Worker (consumers)
                            ↓
                ┌───────────┼───────────┐
                ↓           ↓           ↓
          EmailAdapter  SMSAdapter  PushAdapter
          (SMTP)        (Twilio)    (FCM)
                            ↓
                      MongoDB (update SENT/FAILED)
                      Redis (cache status)
```

## 🚀 Quick Start

### Prerequisites
- Go 1.22+
- MongoDB (`brew services start mongodb-community`)
- Redis (`brew services start redis`)
- Kafka (`docker-compose up kafka` or `brew install kafka`)

### Run with Docker Compose (Recommended)
```bash
docker-compose up --build
```

### Run Locally (3 terminals)

**Terminal 1 — Kafka** (via Docker):
```bash
docker-compose up kafka
```

**Terminal 2 — API Server**:
```bash
go mod tidy
go run ./cmd/api/ -mode=api
```

**Terminal 3 — Worker**:
```bash
go run ./cmd/api/ -mode=worker
```

**Or run both in one process:**
```bash
go run ./cmd/api/ -mode=both
```

## 📡 API Reference

### POST /notification/send
Send a single notification.
```json
{
  "type": "EMAIL",
  "recipient": "user@example.com",
  "metadata": {
    "subject": "Welcome!",
    "body": "Thanks for signing up."
  },
  "trigger_type": "INSTANT",
  "priority": "HIGH"
}
```
Response: `202 Accepted`
```json
{
  "notification_id": "uuid",
  "status": "TRIGGERED"
}
```

### POST /notification/bulk
Send bulk campaign notifications.
```json
{
  "campaign_id": "welcome-campaign-2024",
  "type": "EMAIL",
  "recipients": ["a@x.com", "b@x.com", "c@x.com"],
  "metadata": {
    "subject": "Special Offer!",
    "body": "50% off today only."
  },
  "priority": "LOW"
}
```
Response: `202 Accepted`
```json
{
  "campaign_id": "welcome-campaign-2024",
  "total": 3,
  "accepted": 3,
  "status": "PROCESSING"
}
```

### GET /notification/status/:id
Get notification delivery status.
```json
{
  "notification_id": "uuid",
  "recipient": "user@example.com",
  "type": "EMAIL",
  "status": "SENT",
  "priority": "HIGH",
  "retry_count": 0,
  "created_at": "2024-01-01T00:00:00Z",
  "sent_at": "2024-01-01T00:00:02Z"
}
```

## 🗂 Project Structure

```
notification-service/
├── cmd/api/main.go              # Entry point (api/worker/both modes)
├── internal/
│   ├── config/                  # Environment configuration
│   ├── models/                  # Notification entity, request/response types
│   ├── handler/                 # HTTP API handlers (send, bulk, status)
│   ├── queue/                   # Kafka producer (multi-topic, batched)
│   ├── worker/                  # Kafka consumers, routing, retry logic
│   ├── adapter/                 # Channel adapter interface
│   │   ├── email/               # SMTP email adapter
│   │   ├── sms/                 # Twilio SMS adapter
│   │   └── push/                # FCM push adapter
│   ├── store/                   # MongoDB + Redis data layer
│   └── circuit/                 # Circuit breaker pattern
├── docker-compose.yml           # Kafka + MongoDB + Redis + Service
├── Dockerfile
└── README.md
```

## 🔑 Key Design Decisions

### Priority-Based Kafka Topics
- `notification-high` — OTP, payment confirmations (500ms poll)
- `notification-medium` — Account updates (1s poll)
- `notification-low` — Marketing, bulk campaigns (2s poll)

### Retry with Exponential Backoff
- 3 retries max (configurable)
- Backoff: 2s → 4s → 8s
- After max retries → DLQ

### Circuit Breaker (per channel)
- Opens after 5 consecutive failures
- Half-open after 30s
- 2 successes to close

### Fallback Strategy
- PUSH fails → automatically tries SMS

### Deduplication
- Redis-based dedup cache (5-minute TTL)
- Prevents duplicate sends for same recipient+type

### Idempotency
- Each notification gets a UUID
- Status tracked: TRIGGERED → PROCESSING → SENT/FAILED/DEAD_LETTERED

## ⚙️ Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `9090` | API port |
| `MONGO_URI` | `mongodb://localhost:27017/notifications` | MongoDB |
| `REDIS_ADDR` | `localhost:6379` | Redis |
| `KAFKA_BROKERS` | `localhost:9092` | Kafka brokers |
| `SMTP_HOST` | — | SMTP server (empty = mock) |
| `SMTP_PORT` | `587` | SMTP port |
| `SMTP_USER` | — | SMTP username |
| `SMTP_PASSWORD` | — | SMTP password |
| `SMTP_FROM` | `noreply@example.com` | From address |
| `TWILIO_SID` | — | Twilio Account SID (empty = mock) |
| `TWILIO_TOKEN` | — | Twilio Auth Token |
| `TWILIO_FROM` | — | Twilio phone number |
| `FCM_SERVER_KEY` | — | Firebase Cloud Messaging key (empty = mock) |

> **Dev mode:** All adapters mock when credentials aren't configured — they log the notification instead of actually sending.

## 🧪 Testing

```bash
# Health check
curl http://localhost:9090/health

# Send email notification
curl -X POST http://localhost:9090/notification/send \
  -H "Content-Type: application/json" \
  -d '{
    "type": "EMAIL",
    "recipient": "test@example.com",
    "metadata": {"subject": "Test", "body": "Hello!"},
    "priority": "HIGH"
  }'

# Send SMS
curl -X POST http://localhost:9090/notification/send \
  -H "Content-Type: application/json" \
  -d '{
    "type": "SMS",
    "recipient": "+919876543210",
    "metadata": {"body": "Your OTP is 123456"},
    "priority": "HIGH"
  }'

# Bulk campaign
curl -X POST http://localhost:9090/notification/bulk \
  -H "Content-Type: application/json" \
  -d '{
    "campaign_id": "test-campaign",
    "type": "EMAIL",
    "recipients": ["a@x.com", "b@x.com"],
    "metadata": {"subject": "Promo", "body": "50% off!"},
    "priority": "LOW"
  }'

# Check status
curl http://localhost:9090/notification/status/{notification_id}
```

## 📄 License
MIT
