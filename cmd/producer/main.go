// cmd/producer/main.go
package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/golang-migrate/migrate/v4"
	migrateDriver "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"

	httpHandlers "kafka-order-service/internal/delivery/http"
	"kafka-order-service/internal/delivery/http/middleware"
	kafkaInfra "kafka-order-service/internal/infrastructure/kafka"
	"kafka-order-service/internal/infrastructure/postgres"
	"kafka-order-service/internal/usecase"
	"kafka-order-service/pkg/config"
	"kafka-order-service/pkg/logger"
)

func main() {
	_ = godotenv.Load()

	log, err := logger.New("info", true)
	if err != nil {
		fmt.Printf("Logger init error: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Config load error", "error", err)
	}

	db, err := sql.Open("postgres", cfg.Database.DSN())
	if err != nil {
		log.Fatal("DB open error", "error", err)
	}
	defer db.Close()

	if err := runMigrationsDB(db); err != nil {
		log.Fatal("Migrations failed", "error", err)
	}

	log.Info("Migrations applied successfully")

	// Init repos and infrastructure
	orderRepo := postgres.NewOrderRepository(db)
	producer := kafkaInfra.NewProducer(kafkaInfra.ProducerConfig{
		Brokers:      cfg.Kafka.Brokers,
		Topic:        cfg.Kafka.Topic,
		BatchSize:    100,
		BatchTimeout: 10 * time.Millisecond,
	})
	defer producer.Close()

	// Init usecases
	createUC := usecase.NewCreateOrderUseCase(orderRepo, producer, log)
	updateUC := usecase.NewUpdateOrderStatusUseCase(orderRepo, producer, log)
	getUC := usecase.NewGetOrderUseCase(orderRepo, log)
	listUC := usecase.NewListOrdersUseCase(orderRepo, log)

	// Handlers
	handler := httpHandlers.NewOrderHandler(createUC, updateUC, getUC, listUC, log)

	// Router and middleware
	router := setupRouter(handler, log)

	server := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info("HTTP server starting", "port", cfg.Server.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("HTTP server error", "error", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down HTTP server...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
	log.Info("HTTP server stopped")
}

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

func runMigrationsDB(db *sql.DB) error {
	// Получаем путь к текущему файлу (main.go)
	/*_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return fmt.Errorf("unable to get caller info")
	}*/

	// Получаем корень проекта (из cmd/producer идём вверх на 2 уровня)
	//projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(filename)))
	//migrationsPath := fmt.Sprintf("file://%s", filepath.Join(projectRoot, "migrations"))
	//_migrationsPath := migrationsDirPath()
	//log.("Using migrations path", "path", migrationsPath)

	driver, err := migrateDriver.WithInstance(db, &migrateDriver.Config{})
	if err != nil {
		return fmt.Errorf("creating migrate driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://migrations",
		"postgres",
		driver,
	)
	if err != nil {
		return fmt.Errorf("initializing migrate instance: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("applying migrations: %w", err)
	}
	return nil
}

func migrationsDirPath() string {
	absPath, err := filepath.Abs("./migrations")
	if err != nil {
		panic(err)
	}
	return "file:///" + strings.ReplaceAll(absPath, "\\", "/")
}

func setupRouter(handler *httpHandlers.OrderHandler, log *logger.Logger) *mux.Router {
	r := mux.NewRouter()
	r.Use(middleware.Chain(
		middleware.Recovery(log),
		middleware.Logger(log),
		middleware.CORS(),
		middleware.Security(),
		middleware.Metrics(log),
		middleware.Timeout(30*time.Second),
	))
	api := r.PathPrefix("/api/v1").Subrouter()
	api.Use(middleware.JSONOnly())
	api.HandleFunc("/orders", handler.CreateOrder).Methods("POST")
	api.HandleFunc("/orders", handler.ListOrders).Methods("GET")
	api.HandleFunc("/orders/{id}", handler.GetOrder).Methods("GET")
	api.HandleFunc("/orders/{id}/status", handler.UpdateOrderStatus).Methods("PUT")
	r.HandleFunc("/health", handler.HealthCheck).Methods("GET")
	r.HandleFunc("/metrics", handler.Metrics).Methods("GET")
	return r
}
