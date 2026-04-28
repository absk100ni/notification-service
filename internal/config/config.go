package config

import "os"

type Config struct {
	Port         string
	MongoURI     string
	MongoDB      string
	RedisAddr    string
	KafkaBrokers string
	Environment  string

	// Kafka Topics
	TopicHigh   string
	TopicMedium string
	TopicLow    string
	TopicDLQ    string

	// Adapter configs
	SMTPHost     string
	SMTPPort     string
	SMTPUser     string
	SMTPPassword string
	SMTPFrom     string

	TwilioSID    string
	TwilioToken  string
	TwilioFrom   string

	FCMServerKey string

	// Retry
	MaxRetries    int
	BatchSize     int
	WorkerCount   int
}

func Load() *Config {
	return &Config{
		Port:         getEnv("PORT", "9090"),
		MongoURI:     getEnv("MONGO_URI", "mongodb://localhost:27017/notifications"),
		MongoDB:      getEnv("MONGO_DB", "notifications"),
		RedisAddr:    getEnv("REDIS_ADDR", "localhost:6379"),
		KafkaBrokers: getEnv("KAFKA_BROKERS", "localhost:9092"),
		Environment:  getEnv("ENVIRONMENT", "development"),

		TopicHigh:   getEnv("KAFKA_TOPIC_HIGH", "notification-high"),
		TopicMedium: getEnv("KAFKA_TOPIC_MEDIUM", "notification-medium"),
		TopicLow:    getEnv("KAFKA_TOPIC_LOW", "notification-low"),
		TopicDLQ:    getEnv("KAFKA_TOPIC_DLQ", "notification-dlq"),

		SMTPHost:     getEnv("SMTP_HOST", ""),
		SMTPPort:     getEnv("SMTP_PORT", "587"),
		SMTPUser:     getEnv("SMTP_USER", ""),
		SMTPPassword: getEnv("SMTP_PASSWORD", ""),
		SMTPFrom:     getEnv("SMTP_FROM", "noreply@example.com"),

		TwilioSID:   getEnv("TWILIO_SID", ""),
		TwilioToken:  getEnv("TWILIO_TOKEN", ""),
		TwilioFrom:   getEnv("TWILIO_FROM", ""),

		FCMServerKey: getEnv("FCM_SERVER_KEY", ""),

		MaxRetries:  3,
		BatchSize:   500,
		WorkerCount: 5,
	}
}

func (c *Config) TopicForPriority(priority string) string {
	switch priority {
	case "HIGH":
		return c.TopicHigh
	case "MEDIUM":
		return c.TopicMedium
	default:
		return c.TopicLow
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
