package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"

	kafkaHandlers "kafka-order-service/internal/delivery/kafka"
	kafkaInfra "kafka-order-service/internal/infrastructure/kafka"
	"kafka-order-service/internal/infrastructure/postgres"
	"kafka-order-service/internal/usecase"
	"kafka-order-service/pkg/config"
	"kafka-order-service/pkg/logger"
)

func main() {
	// Load environment variables
	_ = godotenv.Load()

	// Initialize logger
	log, err := logger.New("info", true)
	if err != nil {
		fmt.Printf("Logger init error: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	log.Info("Kafka consumer starting")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Config load error", "error", err)
	}

	// Connect to database
	db, err := connectDatabase(cfg.Database.DSN())
	if err != nil {
		log.Fatal("DB connect error", "error", err)
	}
	defer db.Close()

	// Initialize repository and producer (for event chaining)
	orderRepo := postgres.NewOrderRepository(db)
	producer := kafkaInfra.NewProducer(kafkaInfra.ProducerConfig{
		Brokers:      cfg.Kafka.Brokers,
		Topic:        cfg.Kafka.Topic,
		BatchSize:    100,
		BatchTimeout: 10 * time.Millisecond,
	})
	defer producer.Close()

	// Initialize use cases
	updateUC := usecase.NewUpdateOrderStatusUseCase(orderRepo, producer, log)
	getUC := usecase.NewGetOrderUseCase(orderRepo, log)

	// Initialize Kafka event handler
	handler := kafkaHandlers.NewOrderEventHandler(updateUC, getUC, log)

	// Initialize Kafka consumer
	consumer := kafkaInfra.NewConsumer(kafkaInfra.ConsumerConfig{
		Brokers:        cfg.Kafka.Brokers,
		Topic:          cfg.Kafka.Topic,
		GroupID:        cfg.Kafka.GroupID,
		MinBytes:       1,
		MaxBytes:       10e6,
		CommitInterval: 1 * time.Second,
	}, handler)
	defer consumer.Close()

	// Run consumer with graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		log.Info("Consumer running...")
		if err := consumer.Start(ctx); err != nil && err != context.Canceled {
			log.Error("Consumer error", "error", err)
		}
	}()

	// Wait for termination signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Consumer shutting down...")
	cancel()
	time.Sleep(5 * time.Second)
	log.Info("Consumer stopped")
}

// connectDatabase attempts to connect with retries
func connectDatabase(dsn string) (*sql.DB, error) {
	var db *sql.DB
	var err error
	for i := 0; i < 5; i++ {
		db, err = sql.Open("postgres", dsn)
		if err == nil && db.Ping() == nil {
			db.SetMaxOpenConns(25)
			db.SetMaxIdleConns(5)
			db.SetConnMaxLifetime(5 * time.Minute)
			return db, nil
		}
		time.Sleep(time.Duration(i+1) * time.Second)
	}
	return nil, fmt.Errorf("DB connect failed: %w", err)
}
