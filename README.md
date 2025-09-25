# Kafka Order Service

![Go Version](https://img.shields.io/badge/go-1.24-00ADD8?style=flat-square&logo=go)
![Docker](https://img.shields.io/badge/docker--compose-3.8-2496ED?style=flat-square&logo=docker)
![PostgreSQL](https://img.shields.io/badge/postgresql-15-4169E1?style=flat-square&logo=postgresql)
![Kafka](https://img.shields.io/badge/kafka-7.4-231F20?style=flat-square&logo=apache-kafka)

Микросервис для обработки заказов на базе событийной архитектуры с использованием Apache Kafka, PostgreSQL и Go.

## 📁 Структура проекта

```
kafka-order-service/
├── cmd/
│   ├── producer/           # HTTP API сервер (Producer)
│   └── consumer/           # Kafka Consumer
├── pkg/
│   ├── models/            # Модели данных
│   ├── handlers/          # HTTP handlers
│   ├── services/          # Бизнес-логика
│   ├── repository/        # Слой работы с БД
│   └── logger/            # Логирование
├── migrations/            # SQL миграции
│   ├── 001_initial_schema.up.sql
│   └── 001_initial_schema.down.sql
├── docker-compose.yml     # Docker Compose конфигурация
├── Dockerfile.producer    # Образ для Producer
├── Dockerfile.consumer    # Образ для Consumer
├── .env                   # Переменные окружения
└── go.mod                # Go модули
```

## 🚀 Быстрый старт

### Предварительные требования

- **Go 1.24+**
- **Docker & Docker Compose**
- **Git**
- **golang-migrate** (для ручного управления миграциями)

```bash
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

### 1. Клонирование и настройка

```bash
git clone https://github.com/<your-username>/kafka-order-service.git
cd kafka-order-service
```

### 2. Настройка переменных окружения

```bash
# Скопируйте пример конфигурации
cp .env.example .env
```

Пример `.env` файла:
```env
# Database
DB_HOST=localhost
DB_PORT=5432
DB_NAME=orders
DB_USER=postgres
DB_PASSWORD=postgres
DB_SSL_MODE=disable

# Kafka
KAFKA_BROKERS=localhost:9092
KAFKA_TOPIC=orders
KAFKA_GROUP_ID=order-service

# Server
HTTP_PORT=8080
LOG_LEVEL=info
```

### 3. Запуск через Docker Compose

```bash
# Запуск всех сервисов
docker-compose up -d

# Просмотр логов
docker-compose logs -f producer
docker-compose logs -f consumer

# Остановка всех сервисов
docker-compose down
```

### 4. Локальный запуск (для разработки)

```bash
# Запуск только инфраструктуры (Kafka, PostgreSQL, ZooKeeper)
docker-compose up -d zookeeper kafka postgres kafka-ui

# Применение миграций
migrate -path ./migrations -database "postgres://postgres:postgres@localhost:5432/orders?sslmode=disable" up

# Запуск Producer (HTTP API)
go run ./cmd/producer

# В другом терминале: запуск Consumer
go run ./cmd/consumer
```

## 🔧 Архитектура системы

### Producer Service (HTTP API)
- **Порт:** 8080
- **Эндпоинты:** REST API для создания заказов
- **Функции:** Прием HTTP-запросов, валидация, сохранение в БД, отправка событий в Kafka

### Consumer Service
- **Функции:** Обработка событий из Kafka, обновление статусов заказов
- **Группа:** `order-service`

### База данных PostgreSQL
- **Порт:** 5432
- **Особенности:** 
  - DEFERRABLE CONSTRAINT триггеры для целостности данных
  - Автоматический пересчет `total_amount`
  - Индексы для оптимизации запросов

### Apache Kafka
- **Broker:** 9092
- **UI:** http://localhost:8081 (Kafka UI)
- **Топики:** `orders`, `kafka.orders.created`

## 📡 API Endpoints

### Создание заказа

**POST** `/api/v1/orders`

```bash
curl -X POST http://localhost:8080/api/v1/orders \
  -H "Content-Type: application/json" \
  -H "X-Request-ID: 550e8400-e29b-41d4-a716-446655440000" \
  -d '{
    "customer_id": "123e4567-e89b-12d3-a456-426614174000",
    "email": "customer@example.com",
    "items": [
      {
        "product_id": "456e7890-e89b-12d3-a456-426614174001",
        "name": "Test Product",
        "price": 49.99,
        "quantity": 3
      }
    ],
    "currency": "USD",
    "metadata": {
      "source": "web"
    },
    "shipping_address": {
      "street": "Main St, 1",
      "city": "Minsk",
      "state": "MI",
      "country": "BY",
      "zip_code": "220030"
    },
    "billing_address": {
      "street": "Main St, 1",
      "city": "Minsk",
      "state": "MI",
      "country": "BY",
      "zip_code": "220030"
    }
  }'
```

**Ответ:**
```json
{
  "order": {
    "id": "865ca832-c5c6-44b0-b235-37273c65aa19",
    "customer_id": "123e4567-e89b-12d3-a456-426614174000",
    "email": "customer@example.com",
    "status": "pending",
    "total_amount": 149.97,
    "currency": "USD",
    "items": [...],
    "created_at": "2025-09-22T13:09:07Z",
    "updated_at": "2025-09-22T13:09:07Z"
  },
  "message": "Order created successfully"
}
```

## 🛠 Управление миграциями

### Создание новой миграции
```bash
migrate create -ext sql -dir migrations -seq add_new_table
```

### Применение миграций
```bash
migrate -path ./migrations -database "postgres://postgres:postgres@localhost:5432/orders?sslmode=disable" up
```

### Откат миграций
```bash
migrate -path ./migrations -database "postgres://postgres:postgres@localhost:5432/orders?sslmode=disable" down 1
```

### Проверка статуса
```bash
migrate -path ./migrations -database "postgres://postgres:postgres@localhost:5432/orders?sslmode=disable" version
```

## 🐞 Troubleshooting

### Проблемы с портами
```bash
# Проверка занятых портов
netstat -aon | findstr :8080
netstat -aon | findstr :9092

# Завершение процесса по PID
taskkill /F /PID <PID>
```

### Проблемы с Kafka
- **Проблема:** `failed to dial: connection refused`
- **Решение:** Проверьте, что Kafka запущена и переменная `KAFKA_BROKERS` настроена правильно
  - В Docker: `kafka:9092`
  - Локально: `localhost:9092`

### Проблемы с миграциями
- **Проблема:** `file does not exist`
- **Решение:** Убедитесь, что запуск происходит из корня проекта и папка `migrations` существует

### Проблемы с Docker
```bash
# Полная очистка контейнеров и томов
docker-compose down --volumes --remove-orphans

# Пересборка образов
docker-compose up --build

# Проверка логов конкретного сервиса
docker-compose logs <service-name>
```

## 📊 Мониторинг

### Kafka UI
- **URL:** http://localhost:8081
- **Возможности:** Просмотр топиков, сообщений, consumer groups

### PostgreSQL
```bash
# Подключение к БД
docker exec -it kafka-order-service-postgres-1 psql -U postgres -d orders

# Проверка статистики заказов
SELECT * FROM get_order_statistics();

# Просмотр заказов с аналитикой
SELECT * FROM orders_with_stats LIMIT 10;
```

## 🧪 Тестирование

### Запуск тестов
```bash
go test ./...
```

### Тестирование API
```bash
# Использование файла test-requests.http в IDE
# Или curl команды из раздела API Endpoints
```

## 🔐 Безопасность

- Email валидация на уровне БД
- UUID для всех идентификаторов
- Проверки целостности данных через DEFERRABLE триггеры
- Логирование всех операций

## 📈 Performance

### Индексы БД
- `idx_orders_customer_id` - поиск по клиенту
- `idx_orders_status` - фильтрация по статусу
- `idx_orders_created_at` - сортировка по дате
- `idx_order_items_order_id` - связь заказ-товары

### Оптимизации Kafka
- Группы потребителей для горизонтального масштабирования
- Настройка партиционирования по `customer_id`

## 🤝 Contributing

1. Форк репозитория
2. Создание feature branch (`git checkout -b feature/amazing-feature`)
3. Commit изменений (`git commit -m 'Add amazing feature'`)
4. Push в branch (`git push origin feature/amazing-feature`)
5. Создание Pull Request

## 📄 License

MIT License. См. [LICENSE](LICENSE) файл.

## 👨‍💻 Автор

Разработано с ❤️ для изучения микросервисной архитектуры и Go.