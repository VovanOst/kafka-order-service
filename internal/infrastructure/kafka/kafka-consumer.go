package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
	"kafka-order-service/internal/domain/entities"
)

// ConsumerConfig конфигурация для Kafka Consumer
type ConsumerConfig struct {
	Brokers       []string      `json:"brokers"`
	Topic         string        `json:"topic"`
	GroupID       string        `json:"group_id"`
	MinBytes      int           `json:"min_bytes"`
	MaxBytes      int           `json:"max_bytes"`
	CommitInterval time.Duration `json:"commit_interval"`
}

// MessageHandler интерфейс для обработки сообщений
type MessageHandler interface {
	HandleOrderCreated(ctx context.Context, event *entities.OrderEvent) error
	HandleOrderConfirmed(ctx context.Context, event *entities.OrderEvent) error
	HandleOrderCancelled(ctx context.Context, event *entities.OrderEvent) error
	HandleOrderShipped(ctx context.Context, event *entities.OrderEvent) error
	HandleOrderDelivered(ctx context.Context, event *entities.OrderEvent) error
	HandleOrderRefunded(ctx context.Context, event *entities.OrderEvent) error
	HandleGenericMessage(ctx context.Context, message kafka.Message) error
}

// Consumer представляет Kafka consumer
type Consumer struct {
	reader  *kafka.Reader
	config  ConsumerConfig
	handler MessageHandler
}

// NewConsumer создает новый Kafka consumer
func NewConsumer(config ConsumerConfig, handler MessageHandler) *Consumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        config.Brokers,
		Topic:          config.Topic,
		GroupID:        config.GroupID,
		MinBytes:       config.MinBytes,
		MaxBytes:       config.MaxBytes,
		CommitInterval: config.CommitInterval,
		StartOffset:    kafka.LastOffset, // Читаем только новые сообщения
		ErrorLogger:    kafka.LoggerFunc(func(msg string, args ...interface{}) {
			fmt.Printf("Kafka consumer error: "+msg+"\n", args...)
		}),
	})

	return &Consumer{
		reader:  reader,
		config:  config,
		handler: handler,
	}
}

// Start запускает consumer и начинает обработку сообщений
func (c *Consumer) Start(ctx context.Context) error {
	fmt.Printf("Starting Kafka consumer for topic: %s, group: %s\n", c.config.Topic, c.config.GroupID)

	for {
		select {
		case <-ctx.Done():
			fmt.Println("Kafka consumer context cancelled, stopping...")
			return ctx.Err()
		default:
			// Чтение сообщения с таймаутом
			message, err := c.reader.FetchMessage(ctx)
			if err != nil {
				fmt.Printf("Error reading message: %v\n", err)
				continue
			}

			// Обработка сообщения
			if err := c.processMessage(ctx, message); err != nil {
				fmt.Printf("Error processing message: %v, key: %s\n", err, string(message.Key))
				// В реальном приложении здесь может быть retry логика или отправка в DLQ
			} else {
				// Подтверждение обработки сообщения
				if err := c.reader.CommitMessages(ctx, message); err != nil {
					fmt.Printf("Error committing message: %v\n", err)
				}
			}
		}
	}
}

// processMessage обрабатывает одно сообщение
func (c *Consumer) processMessage(ctx context.Context, message kafka.Message) error {
	fmt.Printf("Processing message: partition=%d, offset=%d, key=%s\n", 
		message.Partition, message.Offset, string(message.Key))

	// Получаем тип события из заголовков
	eventType := c.getHeaderValue(message.Headers, "event-type")
	if eventType == "" {
		// Если нет типа события, передаем на обработку как generic сообщение
		return c.handler.HandleGenericMessage(ctx, message)
	}

	// Парсим событие заказа
	var orderEvent entities.OrderEvent
	if err := json.Unmarshal(message.Value, &orderEvent); err != nil {
		return fmt.Errorf("failed to unmarshal order event: %w", err)
	}

	// Добавляем метаданные из Kafka message
	if orderEvent.Data == nil {
		orderEvent.Data = make(map[string]interface{})
	}
	orderEvent.Data["kafka_partition"] = message.Partition
	orderEvent.Data["kafka_offset"] = message.Offset
	orderEvent.Data["kafka_timestamp"] = message.Time

	// Обрабатываем в зависимости от типа события
	switch eventType {
	case entities.EventOrderCreated:
		return c.handler.HandleOrderCreated(ctx, &orderEvent)
	case entities.EventOrderConfirmed:
		return c.handler.HandleOrderConfirmed(ctx, &orderEvent)
	case entities.EventOrderCancelled:
		return c.handler.HandleOrderCancelled(ctx, &orderEvent)
	case entities.EventOrderShipped:
		return c.handler.HandleOrderShipped(ctx, &orderEvent)
	case entities.EventOrderDelivered:
		return c.handler.HandleOrderDelivered(ctx, &orderEvent)
	case entities.EventOrderRefunded:
		return c.handler.HandleOrderRefunded(ctx, &orderEvent)
	default:
		fmt.Printf("Unknown event type: %s, processing as generic\n", eventType)
		return c.handler.HandleGenericMessage(ctx, message)
	}
}

// getHeaderValue получает значение заголовка по ключу
func (c *Consumer) getHeaderValue(headers []kafka.Header, key string) string {
	for _, header := range headers {
		if header.Key == key {
			return string(header.Value)
		}
	}
	return ""
}

// Close закрывает consumer
func (c *Consumer) Close() error {
	fmt.Println("Closing Kafka consumer...")
	return c.reader.Close()
}

// Stats возвращает статистику consumer
func (c *Consumer) Stats() kafka.ReaderStats {
	return c.reader.Stats()
}

// Lag возвращает информацию о задержке consumer
func (c *Consumer) Lag(ctx context.Context) (int64, error) {
	stats := c.reader.Stats()
	return stats.Lag, nil
}

// CommitMessage вручную подтверждает обработку сообщения
func (c *Consumer) CommitMessage(ctx context.Context, message kafka.Message) error {
	return c.reader.CommitMessages(ctx, message)
}

// SeekToOffset устанавливает offset для чтения
func (c *Consumer) SeekToOffset(offset int64) error {
	return c.reader.SetOffset(offset)
}

// SeekToTime устанавливает время для чтения
func (c *Consumer) SeekToTime(t time.Time) error {
	return c.reader.SetOffsetAt(context.Background(), t)
}

// BatchConsumer представляет consumer для batch обработки
type BatchConsumer struct {
	reader    *kafka.Reader
	config    ConsumerConfig
	handler   MessageHandler
	batchSize int
}

// NewBatchConsumer создает новый batch consumer
func NewBatchConsumer(config ConsumerConfig, handler MessageHandler, batchSize int) *BatchConsumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        config.Brokers,
		Topic:          config.Topic,
		GroupID:        config.GroupID,
		MinBytes:       config.MinBytes,
		MaxBytes:       config.MaxBytes,
		CommitInterval: config.CommitInterval,
		StartOffset:    kafka.LastOffset,
	})

	return &BatchConsumer{
		reader:    reader,
		config:    config,
		handler:   handler,
		batchSize: batchSize,
	}
}

// StartBatch запускает batch consumer
func (c *BatchConsumer) StartBatch(ctx context.Context) error {
	fmt.Printf("Starting Kafka batch consumer (batch size: %d) for topic: %s\n", c.batchSize, c.config.Topic)

	messages := make([]kafka.Message, 0, c.batchSize)

	for {
		select {
		case <-ctx.Done():
			// Обрабатываем оставшиеся сообщения перед выходом
			if len(messages) > 0 {
				c.processBatch(ctx, messages)
			}
			return ctx.Err()
		default:
			message, err := c.reader.FetchMessage(ctx)
			if err != nil {
				fmt.Printf("Error reading message: %v\n", err)
				continue
			}

			messages = append(messages, message)

			// Обрабатываем batch когда достигли нужного размера
			if len(messages) >= c.batchSize {
				c.processBatch(ctx, messages)
				messages = messages[:0] // Очищаем slice
			}
		}
	}
}

// processBatch обрабатывает batch сообщений
func (c *BatchConsumer) processBatch(ctx context.Context, messages []kafka.Message) {
	fmt.Printf("Processing batch of %d messages\n", len(messages))

	successfulMessages := make([]kafka.Message, 0, len(messages))

	for _, message := range messages {
		if err := c.processMessage(ctx, message); err != nil {
			fmt.Printf("Error processing message in batch: %v, key: %s\n", err, string(message.Key))
		} else {
			successfulMessages = append(successfulMessages, message)
		}
	}

	// Подтверждаем только успешно обработанные сообщения
	if len(successfulMessages) > 0 {
		if err := c.reader.CommitMessages(ctx, successfulMessages...); err != nil {
			fmt.Printf("Error committing batch messages: %v\n", err)
		} else {
			fmt.Printf("Committed %d messages from batch\n", len(successfulMessages))
		}
	}
}

// processMessage для batch consumer (аналогичен обычному consumer)
func (c *BatchConsumer) processMessage(ctx context.Context, message kafka.Message) error {
	eventType := c.getHeaderValue(message.Headers, "event-type")
	if eventType == "" {
		return c.handler.HandleGenericMessage(ctx, message)
	}

	var orderEvent entities.OrderEvent
	if err := json.Unmarshal(message.Value, &orderEvent); err != nil {
		return fmt.Errorf("failed to unmarshal order event: %w", err)
	}

	switch eventType {
	case entities.EventOrderCreated:
		return c.handler.HandleOrderCreated(ctx, &orderEvent)
	case entities.EventOrderConfirmed:
		return c.handler.HandleOrderConfirmed(ctx, &orderEvent)
	case entities.EventOrderCancelled:
		return c.handler.HandleOrderCancelled(ctx, &orderEvent)
	case entities.EventOrderShipped:
		return c.handler.HandleOrderShipped(ctx, &orderEvent)
	case entities.EventOrderDelivered:
		return c.handler.HandleOrderDelivered(ctx, &orderEvent)
	case entities.EventOrderRefunded:
		return c.handler.HandleOrderRefunded(ctx, &orderEvent)
	default:
		return c.handler.HandleGenericMessage(ctx, message)
	}
}

// getHeaderValue для batch consumer
func (c *BatchConsumer) getHeaderValue(headers []kafka.Header, key string) string {
	for _, header := range headers {
		if header.Key == key {
			return string(header.Value)
		}
	}
	return ""
}

// Close закрывает batch consumer
func (c *BatchConsumer) Close() error {
	return c.reader.Close()
}