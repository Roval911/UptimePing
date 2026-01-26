package logger

import (
	"context"
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger представляет интерфейс для логирования
type Logger interface {
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)
	With(fields ...Field) Logger
	Sync() error
}

// Field представляет поле лога
type Field struct {
	zap.Field
}

// LoggerImpl реализация логгера на основе zap
type LoggerImpl struct {
	zapLogger *zap.Logger
}

// NewLogger создает новый логгер с заданными параметрами
//
// Параметры:
// - environment: окружение (dev, staging, prod)
// - level: уровень логирования
// - serviceName: имя сервиса для контекста
// - enableLoki: включить интеграцию с Loki
func NewLogger(environment, level, serviceName string, enableLoki bool) (Logger, error) {
	// Определяем уровень логирования
	var zapLevel zapcore.Level
	switch level {
	case "debug":
		zapLevel = zap.DebugLevel
	case "info":
		zapLevel = zap.InfoLevel
	case "warn":
		zapLevel = zap.WarnLevel
	case "error":
		zapLevel = zap.ErrorLevel
	default:
		zapLevel = zap.InfoLevel
	}

	// Определяем настройки кодирования в зависимости от окружения
	var encoderConfig zapcore.EncoderConfig
	var encoder zapcore.Encoder

	if environment == "dev" {
		// Для разработки используем читаемый формат
		encoderConfig = zap.NewDevelopmentEncoderConfig()
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	} else {
		// Для продакшена используем JSON формат
		encoderConfig = zap.NewProductionEncoderConfig()
		encoderConfig.TimeKey = "time"
		encoderConfig.LevelKey = "level"
		encoderConfig.NameKey = "logger"
		encoderConfig.CallerKey = "caller"
		encoderConfig.MessageKey = "msg"
		encoderConfig.StacktraceKey = "stacktrace"
		encoderConfig.LineEnding = zapcore.DefaultLineEnding
		encoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		encoderConfig.EncodeDuration = zapcore.SecondsDurationEncoder
		encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	// Создаем core для zap
	core := zapcore.NewCore(
		encoder,
		zapcore.AddSync(os.Stdout),
		zap.NewAtomicLevelAt(zapLevel),
	)

	// Создаем логгер
	zapLogger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zap.ErrorLevel))
	defer zapLogger.Sync()

	// Добавляем поля по умолчанию
	zapLogger = zapLogger.With(
		zap.String("service", serviceName),
		zap.String("environment", environment),
	)

	// Если включена интеграция с Loki, добавляем дополнительные настройки
	// В реальной реализации здесь будет настройка отправки логов в Loki
	if enableLoki {
		// TODO: Добавить интеграцию с Loki
		// Например, настройка Loki через promtail или прямое API
		zapLogger.Info("Loki integration enabled")
	}

	return &LoggerImpl{zapLogger: zapLogger}, nil
}

// Debug записывает отладочное сообщение
func (l *LoggerImpl) Debug(msg string, fields ...Field) {
	zapFields := make([]zap.Field, len(fields))
	for i, field := range fields {
		zapFields[i] = field.Field
	}
	l.zapLogger.Debug(msg, zapFields...)
}

// Info записывает информационное сообщение
func (l *LoggerImpl) Info(msg string, fields ...Field) {
	zapFields := make([]zap.Field, len(fields))
	for i, field := range fields {
		zapFields[i] = field.Field
	}
	l.zapLogger.Info(msg, zapFields...)
}

// Warn записывает предупреждение
func (l *LoggerImpl) Warn(msg string, fields ...Field) {
	zapFields := make([]zap.Field, len(fields))
	for i, field := range fields {
		zapFields[i] = field.Field
	}
	l.zapLogger.Warn(msg, zapFields...)
}

// Error записывает ошибку
func (l *LoggerImpl) Error(msg string, fields ...Field) {
	zapFields := make([]zap.Field, len(fields))
	for i, field := range fields {
		zapFields[i] = field.Field
	}
	l.zapLogger.Error(msg, zapFields...)
}

// With добавляет поля к логгеру и возвращает новый логгер
func (l *LoggerImpl) With(fields ...Field) Logger {
	zapFields := make([]zap.Field, len(fields))
	for i, field := range fields {
		zapFields[i] = field.Field
	}
	return &LoggerImpl{zapLogger: l.zapLogger.With(zapFields...)}
}

// Sync синхронизирует буферы логгера
func (l *LoggerImpl) Sync() error {
	return l.zapLogger.Sync()
}

// CtxField возвращает поле с trace_id из контекста
func CtxField(ctx context.Context) Field {
	if traceID, ok := ctx.Value("trace_id").(string); ok {
		return Field{zap.String("trace_id", traceID)}
	}
	return Field{zap.String("trace_id", "unknown")}
}

// String создает поле со строковым значением
func String(key, val string) Field {
	return Field{zap.String(key, val)}
}

// Int создает поле с целочисленным значением
func Int(key string, val int) Field {
	return Field{zap.Int(key, val)}
}

// Int32 создает поле с int32 значением
func Int32(key string, val int32) Field {
	return Field{zap.Int32(key, val)}
}

// Float64 создает поле с значением типа float64
func Float64(key string, val float64) Field {
	return Field{zap.Float64(key, val)}
}

// Bool создает поле с булевым значением
func Bool(key string, val bool) Field {
	return Field{zap.Bool(key, val)}
}

// Error создает поле с ошибкой
func Error(err error) Field {
	if err == nil {
		return Field{zap.String("error", "nil")}
	}
	return Field{zap.String("error", err.Error())}
}

// Int64 создает поле с int64 значением
func Int64(key string, val int64) Field {
	return Field{zap.Int64(key, val)}
}

// Duration создает поле с duration значением
func Duration(key string, val time.Duration) Field {
	return Field{zap.Duration(key, val)}
}

// Any создает поле с любым значением
func Any(key string, val interface{}) Field {
	return Field{zap.Any(key, val)}
}
