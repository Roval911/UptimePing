package domain

import (
	"time"
)

// User представляет пользователя системы
// Пароли хранятся с использованием bcrypt (cost 10)
// Email должен быть уникальным в рамках tenant
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"password_hash"`
	TenantID     string    `json:"tenant_id"`
	IsActive     bool      `json:"is_active"`
	IsAdmin      bool      `json:"is_admin"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Tenant представляет клиента/организацию в системе
// Каждый tenant изолирован от других
type Tenant struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Slug      string                 `json:"slug"`
	Settings  map[string]interface{} `json:"settings"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// APIKey представляет API ключ для доступа к системе
// API ключи: key (публичный, в БД), secret (приватный, только при создании)
// KeyHash используется для поиска ключа по публичной части
// SecretHash используется для проверки приватной части (аналогично паролям)
type APIKey struct {
	ID         string    `json:"id"`
	TenantID   string    `json:"tenant_id"`
	KeyHash    string    `json:"key_hash"`
	SecretHash string    `json:"secret_hash"`
	Name       string    `json:"name"`
	IsActive   bool      `json:"is_active"`
	ExpiresAt  time.Time `json:"expires_at"`
	CreatedAt  time.Time `json:"created_at"`
}

// Session представляет сессию пользователя
// JWT токены: access (15 мин), refresh (7 дней)
// Refresh токены хранятся в Redis для возможности отзыва
// Access и Refresh токены хэшируются перед сохранением
type Session struct {
	ID               string    `json:"id"`
	UserID           string    `json:"user_id"`
	AccessTokenHash  string    `json:"access_token_hash"`
	RefreshTokenHash string    `json:"refresh_token_hash"`
	ExpiresAt        time.Time `json:"expires_at"`
	CreatedAt        time.Time `json:"created_at"`
}
