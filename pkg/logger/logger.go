package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger представляет структурированный логгер
type Logger struct {
	zap *zap.Logger
}

// New создает новый логгер
func New(level string, isDevelopment bool) (*Logger, error) {
	var config zap.Config

	if isDevelopment {
		config = zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		config = zap.NewProductionConfig()
		config.EncoderConfig.TimeKey = "timestamp"
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	}

	// Установка уровня логирования
	switch level {
	case "debug":
		config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "info":
		config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	case "warn":
		config.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		config.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	default:
		config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	zapLogger, err := config.Build()
	if err != nil {
		return nil, err
	}

	return &Logger{
		zap: zapLogger,
	}, nil
}

// Info логирует информационное сообщение
func (l *Logger) Info(msg string, fields ...interface{}) {
	l.zap.Info(msg, l.parseFields(fields...)...)
}

// Error логирует ошибку
func (l *Logger) Error(msg string, fields ...interface{}) {
	l.zap.Error(msg, l.parseFields(fields...)...)
}

// Warn логирует предупреждение
func (l *Logger) Warn(msg string, fields ...interface{}) {
	l.zap.Warn(msg, l.parseFields(fields...)...)
}

// Debug логирует отладочное сообщение
func (l *Logger) Debug(msg string, fields ...interface{}) {
	l.zap.Debug(msg, l.parseFields(fields...)...)
}

// Fatal логирует критическую ошибку и завершает программу
func (l *Logger) Fatal(msg string, fields ...interface{}) {
	l.zap.Fatal(msg, l.parseFields(fields...)...)
}

// With создает новый логгер с дополнительными полями
func (l *Logger) With(fields ...interface{}) *Logger {
	return &Logger{
		zap: l.zap.With(l.parseFields(fields...)...),
	}
}

// WithError создает новый логгер с полем error
func (l *Logger) WithError(err error) *Logger {
	return &Logger{
		zap: l.zap.With(zap.Error(err)),
	}
}

// WithField создает новый логгер с одним дополнительным полем
func (l *Logger) WithField(key string, value interface{}) *Logger {
	return &Logger{
		zap: l.zap.With(l.parseField(key, value)),
	}
}

// Sync синхронизирует логгер (важно вызвать перед завершением программы)
func (l *Logger) Sync() error {
	return l.zap.Sync()
}

// parseFields парсит аргументы в zap.Field
func (l *Logger) parseFields(fields ...interface{}) []zap.Field {
	if len(fields)%2 != 0 {
		// Если нечетное количество аргументов, добавляем последний как строку
		fields = append(fields, "")
	}

	zapFields := make([]zap.Field, 0, len(fields)/2)
	for i := 0; i < len(fields); i += 2 {
		key, ok := fields[i].(string)
		if !ok {
			continue
		}
		zapFields = append(zapFields, l.parseField(key, fields[i+1]))
	}

	return zapFields
}

// parseField парсит одно поле в zap.Field
func (l *Logger) parseField(key string, value interface{}) zap.Field {
	switch v := value.(type) {
	case string:
		return zap.String(key, v)
	case int:
		return zap.Int(key, v)
	case int32:
		return zap.Int32(key, v)
	case int64:
		return zap.Int64(key, v)
	case uint:
		return zap.Uint(key, v)
	case uint32:
		return zap.Uint32(key, v)
	case uint64:
		return zap.Uint64(key, v)
	case float32:
		return zap.Float32(key, v)
	case float64:
		return zap.Float64(key, v)
	case bool:
		return zap.Bool(key, v)
	case error:
		return zap.Error(v)
	case []string:
		return zap.Strings(key, v)
	case []int:
		return zap.Ints(key, v)
	default:
		return zap.Any(key, v)
	}
}

// GetZapLogger возвращает базовый zap логгер (для интеграций)
func (l *Logger) GetZapLogger() *zap.Logger {
	return l.zap
}

// NewNoOp создает no-op логгер для тестов
func NewNoOp() *Logger {
	return &Logger{
		zap: zap.NewNop(),
	}
}

// LogLevel представляет уровень логирования
type LogLevel string

const (
	DebugLevel LogLevel = "debug"
	InfoLevel  LogLevel = "info"
	WarnLevel  LogLevel = "warn"
	ErrorLevel LogLevel = "error"
)

// Config конфигурация для логгера
type Config struct {
	Level         LogLevel `json:"level"`
	IsDevelopment bool     `json:"is_development"`
	OutputPaths   []string `json:"output_paths"`
}

// NewWithConfig создает логгер с конфигурацией
func NewWithConfig(config Config) (*Logger, error) {
	var zapConfig zap.Config

	if config.IsDevelopment {
		zapConfig = zap.NewDevelopmentConfig()
		zapConfig.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		zapConfig = zap.NewProductionConfig()
		zapConfig.EncoderConfig.TimeKey = "timestamp"
		zapConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	}

	// Установка уровня логирования
	switch config.Level {
	case DebugLevel:
		zapConfig.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	case InfoLevel:
		zapConfig.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	case WarnLevel:
		zapConfig.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
	case ErrorLevel:
		zapConfig.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	default:
		zapConfig.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	// Установка путей вывода
	if len(config.OutputPaths) > 0 {
		zapConfig.OutputPaths = config.OutputPaths
	}

	zapLogger, err := zapConfig.Build()
	if err != nil {
		return nil, err
	}

	return &Logger{
		zap: zapLogger,
	}, nil
}