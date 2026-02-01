package domain

import (
	"time"
)

// User представляет пользователя системы
// Пароли хранятся с использованием bcrypt (cost 10)
// Email должен быть уникальным в рамках tenant
type User struct {
	ID           string    `json:"id" db:"id"`
	Email        string    `json:"email" db:"email"`
	PasswordHash string    `json:"password_hash" db:"password_hash"`
	TenantID     string    `json:"tenant_id" db:"tenant_id"`
	FirstName    string    `json:"first_name" db:"first_name"`
	LastName     string    `json:"last_name" db:"last_name"`
	IsActive     bool      `json:"is_active" db:"is_active"`
	IsVerified   bool      `json:"is_verified" db:"is_verified"`
	IsAdmin      bool      `json:"is_admin" db:"is_admin"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// Tenant представляет клиента/организацию в системе
// Каждый tenant изолирован от других
type Tenant struct {
	ID        string                 `json:"id" db:"id"`
	Name      string                 `json:"name" db:"name"`
	Slug      string                 `json:"slug" db:"slug"`
	Plan      string                 `json:"plan" db:"plan"`
	Status    string                 `json:"status" db:"status"`
	Settings  map[string]interface{} `json:"settings" db:"settings"`
	CreatedAt time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt time.Time              `json:"updated_at" db:"updated_at"`
}

// APIKey представляет API ключ для доступа к системе
// API ключи: key (публичный, в БД), secret (приватный, только при создании)
// KeyHash используется для поиска ключа по публичной части
// SecretHash используется для проверки приватной части (аналогично паролям)
type APIKey struct {
	ID          string                 `json:"id" db:"id"`
	TenantID    string                 `json:"tenant_id" db:"tenant_id"`
	KeyHash     string                 `json:"key_hash" db:"key_hash"`
	KeyPrefix   string                 `json:"key_prefix" db:"key_prefix"`
	SecretHash  string                 `json:"secret_hash"` // Для внутренней валидации
	Name        string                 `json:"name" db:"name"`
	Permissions map[string]interface{} `json:"permissions" db:"permissions"`
	IsActive    bool                   `json:"is_active" db:"is_active"`
	ExpiresAt   *time.Time             `json:"expires_at" db:"expires_at"`
	CreatedAt   time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at" db:"updated_at"`
}

// Session представляет сессию пользователя
// JWT токены: access (15 мин), refresh (7 дней)
// Refresh токены хранятся в Redis для возможности отзыва
// Access и Refresh токены хэшируются перед сохранением
type Session struct {
	ID               string     `json:"id" db:"id"`
	UserID           string     `json:"user_id" db:"user_id"`
	RefreshTokenHash string     `json:"refresh_token_hash" db:"refresh_token_hash"`
	ExpiresAt        *time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt        time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at" db:"updated_at"`
}
