package password

import (
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

// Hasher интерфейс для работы с паролями
type Hasher interface {
	Hash(password string) (string, error)
	Check(password, hash string) bool
	Validate(password string) bool
}

// BcryptHasher реализация Hasher с использованием bcrypt
type BcryptHasher struct {
	cost int
}

// NewBcryptHasher создает новый BcryptHasher
func NewBcryptHasher(cost int) *BcryptHasher {
	if cost <= 0 {
		cost = bcrypt.DefaultCost
	}
	return &BcryptHasher{cost: cost}
}

// Hash хеширует пароль с использованием bcrypt
func (h *BcryptHasher) Hash(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), h.cost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// Check проверяет, соответствует ли пароль хешу
func (h *BcryptHasher) Check(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// Validate проверяет сложность пароля
// Validate проверяет сложность пароля (улучшенная версия с Unicode поддержкой)
func (h *BcryptHasher) Validate(password string) bool {
	// Минимальная длина пароля
	if len(password) < 8 {
		return false
	}

	// Проверка наличия хотя бы одной цифры
	hasDigit := false
	hasUpper := false
	hasLower := false

	for _, r := range password {
		switch {
		case r >= '0' && r <= '9':
			hasDigit = true
		case r >= 'A' && r <= 'Z':
			hasUpper = true
		case r >= 'a' && r <= 'z':
			hasLower = true
		case r >= 'А' && r <= 'Я': // Русские заглавные
			hasUpper = true
		case r >= 'а' && r <= 'я': // Русские строчные
			hasLower = true
		case unicode.IsUpper(r): // Unicode заглавные
			hasUpper = true
		case unicode.IsLower(r): // Unicode строчные
			hasLower = true
		case unicode.IsDigit(r): // Unicode цифры
			hasDigit = true
		}
	}

	return hasDigit && hasUpper && hasLower
}

// Deprecated: Используйте BcryptHasher.Hash
func HashPassword(password string) (string, error) {
	return NewBcryptHasher(bcrypt.DefaultCost).Hash(password)
}

// Deprecated: Используйте BcryptHasher.Check
func CheckPasswordHash(password, hash string) bool {
	return NewBcryptHasher(bcrypt.DefaultCost).Check(password, hash)
}

// Deprecated: Используйте BcryptHasher.Validate
func ValidatePassword(password string) bool {
	return NewBcryptHasher(bcrypt.DefaultCost).Validate(password)
}
