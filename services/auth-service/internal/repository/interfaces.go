package repository

import (
	"context"
	"time"

	"UptimePingPlatform/services/auth-service/internal/domain"
)

// UserRepository интерфейс для работы с пользователями
type UserRepository interface {
	Create(ctx context.Context, user *domain.User) error
	FindByID(ctx context.Context, id string) (*domain.User, error)
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
	Update(ctx context.Context, user *domain.User) error
	Delete(ctx context.Context, id string) error
}

// TenantRepository интерфейс для работы с тенантами
type TenantRepository interface {
	Create(ctx context.Context, tenant *domain.Tenant) error
	FindByID(ctx context.Context, id string) (*domain.Tenant, error)
	FindBySlug(ctx context.Context, slug string) (*domain.Tenant, error)
	Update(ctx context.Context, tenant *domain.Tenant) error
	Delete(ctx context.Context, id string) error
}

// APIKeyRepository интерфейс для работы с API ключами
type APIKeyRepository interface {
	Create(ctx context.Context, key *domain.APIKey) error
	FindByID(ctx context.Context, id string) (*domain.APIKey, error)
	FindByKeyHash(ctx context.Context, keyHash string) (*domain.APIKey, error)
	ListByTenant(ctx context.Context, tenantID string) ([]*domain.APIKey, error)
	Update(ctx context.Context, key *domain.APIKey) error
	Delete(ctx context.Context, id string) error
}

// SessionRepository интерфейс для работы с сессиями
type SessionRepository interface {
	Create(ctx context.Context, session *domain.Session) error
	FindByID(ctx context.Context, id string) (*domain.Session, error)
	FindByAccessTokenHash(ctx context.Context, accessTokenHash string) (*domain.Session, error)
	FindByRefreshTokenHash(ctx context.Context, refreshTokenHash string) (*domain.Session, error)
	Delete(ctx context.Context, id string) error
	DeleteByUserID(ctx context.Context, userID string) error
	CleanupExpired(ctx context.Context, before time.Time) error
}
