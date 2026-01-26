package rabbitmq

import (
	"UptimePingPlatform/pkg/errors"
)

var (
	// ErrInvalidURL ошибка неверного URL
	ErrInvalidURL = errors.New(errors.ErrValidation, "invalid rabbitmq URL")
	
	// ErrInvalidExchange ошибка неверного exchange
	ErrInvalidExchange = errors.New(errors.ErrValidation, "invalid exchange name")
	
	// ErrInvalidMaxRetries ошибка неверного количества ретраев
	ErrInvalidMaxRetries = errors.New(errors.ErrValidation, "invalid max retries value")
	
	// ErrInvalidInitialDelay ошибка неверной начальной задержки
	ErrInvalidInitialDelay = errors.New(errors.ErrValidation, "invalid initial delay")
	
	// ErrInvalidMaxDelay ошибка неверной максимальной задержки
	ErrInvalidMaxDelay = errors.New(errors.ErrValidation, "invalid max delay")
	
	// ErrInvalidMultiplier ошибка неверного множителя
	ErrInvalidMultiplier = errors.New(errors.ErrValidation, "invalid multiplier value")
	
	// ErrConnectionClosed ошибка закрытого соединения
	ErrConnectionClosed = errors.New(errors.ErrInternal, "rabbitmq connection is closed")
	
	// ErrChannelClosed ошибка закрытого канала
	ErrChannelClosed = errors.New(errors.ErrInternal, "rabbitmq channel is closed")
	
	// ErrPublishFailed ошибка публикации
	ErrPublishFailed = errors.New(errors.ErrInternal, "failed to publish message")
	
	// ErrIncidentNil ошибка нулевого инцидента
	ErrIncidentNil = errors.New(errors.ErrValidation, "incident cannot be nil")
	
	// ErrEventTypeInvalid ошибка неверного типа события
	ErrEventTypeInvalid = errors.New(errors.ErrValidation, "invalid event type")
)
