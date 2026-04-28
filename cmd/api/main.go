package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"notification-service/internal/config"
	"notification-service/internal/handler"
	"notification-service/internal/queue"
	"notification-service/internal/store"
	"notification-service/internal/worker"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	mode := flag.String("mode", "api", "Run mode: api, worker, or both")
	flag.Parse()

	cfg := config.Load()

	// MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		log.Fatalf("MongoDB connect failed: %v", err)
	}
	defer mongoClient.Disconnect(context.Background())
	if err := mongoClient.Ping(ctx, nil); err != nil {
		log.Fatalf("MongoDB ping failed: %v", err)
	}
	log.Println("✅ Connected to MongoDB")

	db := mongoClient.Database(cfg.MongoDB)

	// Redis
	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("Redis connect failed: %v", err)
	}
	log.Println("✅ Connected to Redis")

	// Initialize stores and producer
	mongoStore := store.NewMongoStore(db)
	redisCache := store.NewRedisCache(rdb)
	producer := queue.NewProducer(cfg)
	defer producer.Close()

	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())
	defer shutdownCancel()

	switch *mode {
	case "worker":
		runWorker(shutdownCtx, shutdownCancel, cfg, mongoStore, redisCache, producer)
	case "both":
		go func() {
			w := worker.NewWorker(cfg, mongoStore, redisCache, producer)
			w.Start(shutdownCtx)
		}()
		runAPI(shutdownCtx, shutdownCancel, cfg, mongoStore, redisCache, producer)
	default:
		runAPI(shutdownCtx, shutdownCancel, cfg, mongoStore, redisCache, producer)
	}
}

func runAPI(ctx context.Context, cancel context.CancelFunc, cfg *config.Config, s *store.MongoStore, c *store.RedisCache, p *queue.Producer) {
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Health
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "notification-service", "time": time.Now().Format(time.RFC3339)})
	})

	h := handler.NewHandler(s, c, p, cfg)

	api := r.Group("/notification")
	{
		api.POST("/send", h.Send)
		api.POST("/bulk", h.Bulk)
		api.GET("/status/:id", h.Status)
	}

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("🚀 Notification API starting on port %s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down API...")
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	srv.Shutdown(shutdownCtx)
	log.Println("API stopped")
}

func runWorker(ctx context.Context, cancel context.CancelFunc, cfg *config.Config, s *store.MongoStore, c *store.RedisCache, p *queue.Producer) {
	w := worker.NewWorker(cfg, s, c, p)
	go w.Start(ctx)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("\nShutting down worker...")
	cancel()
	time.Sleep(2 * time.Second)
	log.Println("Worker stopped")
}
