package errors

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Error представляет кастомную ошибку с дополнительной информацией
type Error struct {
	Code    ErrorCode       `json:"code"`
	Message string          `json:"message"`
	Details string          `json:"details,omitempty"`
	Cause   error           `json:"-"`
	Context context.Context `json:"-"`
}

// ErrorCode представляет код ошибки
type ErrorCode string

// Определение кодов ошибок
const (
	ErrNotFound     ErrorCode = "NOT_FOUND"
	ErrValidation   ErrorCode = "VALIDATION_ERROR"
	ErrUnauthorized ErrorCode = "UNAUTHORIZED"
	ErrForbidden    ErrorCode = "FORBIDDEN"
	ErrInternal     ErrorCode = "INTERNAL_ERROR"
	ErrConflict     ErrorCode = "CONFLICT"
)

// Error возвращает сообщение об ошибке
func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

// Unwrap возвращает причину ошибки
func (e *Error) Unwrap() error {
	return e.Cause
}

// Is проверяет, является ли ошибка указанного типа
func (e *Error) Is(target error) bool {
	if targetError, ok := target.(*Error); ok {
		return e.Code == targetError.Code
	}
	return false
}

// New создает новую кастомную ошибку
func New(code ErrorCode, message string) *Error {
	return &Error{
		Code:    code,
		Message: message,
	}
}

// Wrap оборачивает существующую ошибку в кастомную
func Wrap(err error, code ErrorCode, message string) *Error {
	if err == nil {
		return nil
	}
	return &Error{
		Code:    code,
		Message: message,
		Cause:   err,
	}
}

// WithDetails добавляет детали к ошибке
func (e *Error) WithDetails(details string) *Error {
	if e == nil {
		return nil
	}
	return &Error{
		Code:    e.Code,
		Message: e.Message,
		Details: details,
		Cause:   e.Cause,
		Context: e.Context,
	}
}

// WithContext добавляет контекст к ошибке
func (e *Error) WithContext(ctx context.Context) *Error {
	if e == nil {
		return nil
	}
	return &Error{
		Code:    e.Code,
		Message: e.Message,
		Details: e.Details,
		Cause:   e.Cause,
		Context: ctx,
	}
}

// ToGRPCErr переводит кастомную ошибку в gRPC статус
func (e *Error) ToGRPCErr() error {
	if e == nil {
		return nil
	}

	// Преобразуем код ошибки в gRPC код
	var grpcCode codes.Code
	switch e.Code {
	case ErrNotFound:
		grpcCode = codes.NotFound
	case ErrValidation:
		grpcCode = codes.InvalidArgument
	case ErrUnauthorized:
		grpcCode = codes.Unauthenticated
	case ErrForbidden:
		grpcCode = codes.PermissionDenied
	case ErrConflict:
		grpcCode = codes.AlreadyExists
	case ErrInternal:
		grpcCode = codes.Internal
	default:
		grpcCode = codes.Unknown
	}

	// Создаем gRPC статус
	status := status.New(grpcCode, e.Message)

	// Добавляем детали, если есть
	//TODO В реальной реализации нужно реализовать proper proto message handling
	// if e.Details != "" {
	// 	withDetails, err := status.WithDetails(&ErrorDetails{
	// 		Details: e.Details,
	// 	})
	// 	if err == nil {
	// 		status = withDetails
	// 	}
	// }

	return status.Err()
}

// FromGRPCErr преобразует gRPC ошибку в кастомную ошибку
func FromGRPCErr(err error) *Error {
	if err == nil {
		return nil
	}

	// Проверяем, является ли ошибка gRPC статусом
	if grpcStatus, ok := status.FromError(err); ok {
		// Преобразуем gRPC код в наш код ошибки
		var code ErrorCode
		switch grpcStatus.Code() {
		case codes.NotFound:
			code = ErrNotFound
		case codes.InvalidArgument:
			code = ErrValidation
		case codes.Unauthenticated:
			code = ErrUnauthorized
		case codes.PermissionDenied:
			code = ErrForbidden
		case codes.AlreadyExists:
			code = ErrConflict
		case codes.Internal, codes.Unknown:
			code = ErrInternal
		default:
			code = ErrInternal
		}

		return &Error{
			Code:    code,
			Message: grpcStatus.Message(),
		}
	}

	// Если это не gRPC ошибка, оборачиваем как внутреннюю ошибку
	return Wrap(err, ErrInternal, "internal error")
}

// HTTPStatus возвращает соответствующий HTTP статус для ошибки
func (e *Error) HTTPStatus() int {
	if e == nil {
		return http.StatusOK
	}

	switch e.Code {
	case ErrNotFound:
		return http.StatusNotFound
	case ErrValidation:
		return http.StatusBadRequest
	case ErrUnauthorized:
		return http.StatusUnauthorized
	case ErrForbidden:
		return http.StatusForbidden
	case ErrConflict:
		return http.StatusConflict
	case ErrInternal:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// ErrorDetails представляет детали ошибки для gRPC
//
//go:generate protoc -I=. --go_out=. --go_opt=paths=source_relative error_details.proto
type ErrorDetails struct {
	Details string `protobuf:"bytes,1,opt,name=details,proto3" json:"details,omitempty"`
}

// GetUserMessage возвращает пользовательское сообщение об ошибке
// В реальной реализации здесь будет интеграция с системой локализации
func (e *Error) GetUserMessage() string {
	if e == nil {
		return ""
	}

	// В реальном приложении здесь будет локализация сообщений
	// Например, через сервис перевода или файлы локализации
	switch e.Code {
	case ErrNotFound:
		return "Ресурс не найден"
	case ErrValidation:
		return "Ошибка валидации данных"
	case ErrUnauthorized:
		return "Не авторизован"
	case ErrForbidden:
		return "Доступ запрещен"
	case ErrConflict:
		return "Конфликт данных (например, дубликат)"
	case ErrInternal:
		return "Внутренняя ошибка сервера"
	default:
		return "Произошла ошибка"
	}
}

// Middleware обрабатывает ошибки в HTTP запросах
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Создаем обертку для ResponseWriter для перехвата статуса
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Выполняем следующий обработчик
		next.ServeHTTP(wrapped, r)

		// Если статус уже установлен ошибочный, ничего не делаем
		if wrapped.statusCode < 400 {
			return
		}

		// Если есть ошибка в контексте, используем ее
		if err, ok := r.Context().Value(errorContextKey{}).(*Error); ok {
			// Отправляем JSON ответ с ошибкой
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(err.HTTPStatus())

			// Формируем ответ
			response := map[string]interface{}{
				"error": map[string]interface{}{
					"code":    err.Code,
					"message": err.GetUserMessage(),
					"details": err.Details,
				},
			}

			// Отправляем ответ
			jsonData, _ := json.Marshal(response)
			w.Write(jsonData)
		}
	})
}

// errorContextKey ключ для хранения ошибки в контексте
type errorContextKey struct{}

// WithError добавляет ошибку в контекст
func WithError(ctx context.Context, err *Error) context.Context {
	return context.WithValue(ctx, errorContextKey{}, err)
}

// GetError извлекает ошибку из контекста
func GetError(ctx context.Context) *Error {
	if err, ok := ctx.Value(errorContextKey{}).(*Error); ok {
		return err
	}
	return nil
}

// responseWriter обертка для перехвата статуса ответа
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader перехватывает установку статуса
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
