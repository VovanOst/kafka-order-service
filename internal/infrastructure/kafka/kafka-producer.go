package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
	"kafka-order-service/internal/domain/entities"
)

// ProducerConfig конфигурация для Kafka Producer
type ProducerConfig struct {
	Brokers      []string      `json:"brokers"`
	Topic        string        `json:"topic"`
	BatchSize    int           `json:"batch_size"`
	BatchTimeout time.Duration `json:"batch_timeout"`
}

// Producer представляет Kafka producer
type Producer struct {
	writer *kafka.Writer
	config ProducerConfig
}

// NewProducer создает новый Kafka producer
func NewProducer(config ProducerConfig) *Producer {
	writer := &kafka.Writer{
		Addr:                   kafka.TCP(config.Brokers...),
		Topic:                  config.Topic,
		Balancer:               &kafka.Hash{}, // Балансировка по ключу
		AllowAutoTopicCreation: true,
		BatchSize:              config.BatchSize,
		BatchTimeout:           config.BatchTimeout,
		RequiredAcks:           kafka.RequireOne, // Гарантия доставки
		Async:                  false,            // Синхронная отправка
	}

	return &Producer{
		writer: writer,
		config: config,
	}
}

// PublishOrderEvent публикует событие заказа в Kafka
func (p *Producer) PublishOrderEvent(ctx context.Context, event *entities.OrderEvent) error {
	// Сериализация события в JSON
	eventData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Создание сообщения Kafka
	message := kafka.Message{
		Key:   []byte(event.OrderID.String()), // Ключ - ID заказа для партиционирования
		Value: eventData,
		Headers: []kafka.Header{
			{Key: "event-type", Value: []byte(event.EventType)},
			{Key: "event-id", Value: []byte(event.EventID.String())},
			{Key: "customer-id", Value: []byte(event.CustomerID.String())},
			{Key: "content-type", Value: []byte("application/json")},
			{Key: "timestamp", Value: []byte(event.Timestamp.Format(time.RFC3339))},
		},
		Time: event.Timestamp,
	}

	// Отправка сообщения
	if err := p.writer.WriteMessages(ctx, message); err != nil {
		return fmt.Errorf("failed to write message to kafka: %w", err)
	}

	return nil
}

// PublishOrderEvents публикует множественные события заказов (batch)
func (p *Producer) PublishOrderEvents(ctx context.Context, events []*entities.OrderEvent) error {
	if len(events) == 0 {
		return nil
	}

	messages := make([]kafka.Message, 0, len(events))

	for _, event := range events {
		eventData, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("failed to marshal event %s: %w", event.EventID, err)
		}

		message := kafka.Message{
			Key:   []byte(event.OrderID.String()),
			Value: eventData,
			Headers: []kafka.Header{
				{Key: "event-type", Value: []byte(event.EventType)},
				{Key: "event-id", Value: []byte(event.EventID.String())},
				{Key: "customer-id", Value: []byte(event.CustomerID.String())},
				{Key: "content-type", Value: []byte("application/json")},
				{Key: "timestamp", Value: []byte(event.Timestamp.Format(time.RFC3339))},
			},
			Time: event.Timestamp,
		}

		messages = append(messages, message)
	}

	// Отправка batch сообщений
	if err := p.writer.WriteMessages(ctx, messages...); err != nil {
		return fmt.Errorf("failed to write batch messages to kafka: %w", err)
	}

	return nil
}

// Close закрывает producer
func (p *Producer) Close() error {
	return p.writer.Close()
}

// Stats возвращает статистику producer
func (p *Producer) Stats() kafka.WriterStats {
	return p.writer.Stats()
}

// PublishGenericMessage публикует произвольное сообщение
func (p *Producer) PublishGenericMessage(ctx context.Context, key string, data interface{}, headers map[string]string) error {
	// Сериализация данных
	messageData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal message data: %w", err)
	}

	// Подготовка заголовков
	kafkaHeaders := make([]kafka.Header, 0, len(headers)+2)
	kafkaHeaders = append(kafkaHeaders, 
		kafka.Header{Key: "content-type", Value: []byte("application/json")},
		kafka.Header{Key: "timestamp", Value: []byte(time.Now().Format(time.RFC3339))},
	)

	for k, v := range headers {
		kafkaHeaders = append(kafkaHeaders, kafka.Header{
			Key:   k,
			Value: []byte(v),
		})
	}

	// Создание сообщения
	message := kafka.Message{
		Key:     []byte(key),
		Value:   messageData,
		Headers: kafkaHeaders,
		Time:    time.Now(),
	}

	// Отправка сообщения
	if err := p.writer.WriteMessages(ctx, message); err != nil {
		return fmt.Errorf("failed to write generic message to kafka: %w", err)
	}

	return nil
}

// GetTopicMetadata получает метаданные топика
func (p *Producer) GetTopicMetadata(ctx context.Context) ([]kafka.Partition, error) {
	conn, err := kafka.DialContext(ctx, "tcp", p.config.Brokers[0])
	if err != nil {
		return nil, fmt.Errorf("failed to dial kafka: %w", err)
	}
	defer conn.Close()

	partitions, err := conn.ReadPartitions(p.config.Topic)
	if err != nil {
		return nil, fmt.Errorf("failed to read partitions: %w", err)
	}

	return partitions, nil
}

// CreateTopic создает топик если он не существует
func (p *Producer) CreateTopic(ctx context.Context, numPartitions int, replicationFactor int) error {
	conn, err := kafka.DialContext(ctx, "tcp", p.config.Brokers[0])
	if err != nil {
		return fmt.Errorf("failed to dial kafka: %w", err)
	}
	defer conn.Close()

	controller, err := conn.Controller()
	if err != nil {
		return fmt.Errorf("failed to get controller: %w", err)
	}

	controllerConn, err := kafka.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", controller.Host, controller.Port))
	if err != nil {
		return fmt.Errorf("failed to dial controller: %w", err)
	}
	defer controllerConn.Close()

	topicConfig := kafka.TopicConfig{
		Topic:             p.config.Topic,
		NumPartitions:     numPartitions,
		ReplicationFactor: replicationFactor,
		ConfigEntries: []kafka.ConfigEntry{
			{ConfigName: "cleanup.policy", ConfigValue: "delete"},
			{ConfigName: "retention.ms", ConfigValue: "604800000"}, // 7 дней
		},
	}

	if err := controllerConn.CreateTopics(topicConfig); err != nil {
		return fmt.Errorf("failed to create topic: %w", err)
	}

	return nil
}