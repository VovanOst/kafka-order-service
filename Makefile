.PHONY: help build run-producer run-consumer test clean docker-up docker-down

help: ## Показать справку
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

build: ## Собрать приложения
	go build -o bin/producer ./cmd/producer
	go build -o bin/consumer ./cmd/consumer

run-producer: ## Запустить producer
	go run ./cmd/producer/main.go

run-consumer: ## Запустить consumer
	go run ./cmd/consumer/main.go

test: ## Запустить тесты
	go test -v ./...

clean: ## Очистить build артефакты
	rm -rf bin/

docker-up: ## Поднять инфраструктуру (Kafka, PostgreSQL)
	docker-compose up -d

docker-down: ## Остановить инфраструктуру
	docker-compose down

docker-logs: ## Посмотреть логи
	docker-compose logs -f

mod-tidy: ## Обновить go.mod
	go mod tidy

fmt: ## Форматировать код
	go fmt ./...

vet: ## Проверить код
	go vet ./...

dev: docker-up ## Запуск в режиме разработки
	@echo "Ждем запуска Kafka..."
	sleep 10
	make run-producer & make run-consumer

kafka-create-topics:
	docker exec kafka kafka-topics --create --bootstrap-server localhost:9092 --replication-factor 1 --partitions 3 --topic kafka.orders.created
	docker exec kafka kafka-topics --create --bootstrap-server localhost:9092 --replication-factor 1 --partitions 3 --topic kafka.orders.events
