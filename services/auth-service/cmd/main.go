package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// generateUserID генерирует уникальный ID пользователя
func generateUserID() string {
	// Используем timestamp + random для избежания блокировки rand.Read
	timestamp := time.Now().UnixNano()
	randomBytes := make([]byte, 8)
	rand.Read(randomBytes)
	randomNum := int64(binary.LittleEndian.Uint64(randomBytes))
	uniqueID := fmt.Sprintf("%d_%d", timestamp, randomNum)
	return "user_" + base64.URLEncoding.EncodeToString([]byte(uniqueID))[:8]
}

// generateTenantID генерирует уникальный ID тенанта в формате UUID
func generateTenantID() string {
	// Генерируем валидный UUID v4
	id := uuid.New()
	return id.String()
}

// generateJWTToken создает JWT токен для пользователя
func generateJWTToken(userID, tenantID, email string) (string, error) {
	// Создаем кастомные claims с уникальными данными пользователя
	claims := jwt.MapClaims{
		"user_id":   userID,
		"tenant_id": tenantID,
		"email":     email,
		"is_admin":  true,
		"exp":       time.Now().Add(24 * time.Hour).Unix(), // Токен на 24 часа
		"iat":       time.Now().Unix(),
		"nbf":       time.Now().Unix(),
		"sub":       userID,
	}

	// Создаем токен с подписью
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Секретный ключ (в реальном приложении должен быть в конфигурации)
	secretKey := "your-secret-key-here"

	signedToken, err := token.SignedString([]byte(secretKey))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return signedToken, nil
}

// generateRefreshToken генерирует refresh токен
func generateRefreshToken(userID string) string {
	// Используем timestamp + random для избежания блокировки rand.Read
	timestamp := time.Now().UnixNano()
	randomBytes := make([]byte, 8)
	rand.Read(randomBytes)
	randomNum := int64(binary.LittleEndian.Uint64(randomBytes))
	uniqueID := fmt.Sprintf("%d_%d", timestamp, randomNum+2)
	return "refresh_" + base64.URLEncoding.EncodeToString([]byte(uniqueID))[:16] + "_" + userID
}

func main() {
	// Initialize HTTP server with timeout
	mux := http.NewServeMux()

	// Create server with timeouts to prevent hanging
	server := &http.Server{
		Addr:         ":51051",
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Health endpoints
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "healthy",
			"service": "auth-service",
		})
	})

	// Auth endpoints
	mux.HandleFunc("/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Парсим тело запроса для получения email
		var req struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Генерируем уникальные ID для пользователя
		userID := generateUserID()
		tenantID := generateTenantID()

		// Генерируем уникальный JWT токен
		accessToken, err := generateJWTToken(userID, tenantID, req.Email)
		if err != nil {
			http.Error(w, "Failed to generate token", http.StatusInternalServerError)
			return
		}

		// Генерируем refresh токен
		refreshToken := generateRefreshToken(userID)

		// Формируем ответ
		response := map[string]string{
			"access_token":  accessToken,
			"refresh_token": refreshToken,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	})

	mux.HandleFunc("/api/v1/auth/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Парсим тело запроса
		var req struct {
			Email      string `json:"email"`
			Password   string `json:"password"`
			TenantName string `json:"tenant_name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Генерируем уникальные ID для нового пользователя
		userID := generateUserID()
		tenantID := generateTenantID()

		// Генерируем уникальный JWT токен
		accessToken, err := generateJWTToken(userID, tenantID, req.Email)
		if err != nil {
			http.Error(w, "Failed to generate token", http.StatusInternalServerError)
			return
		}

		// Генерируем refresh токен
		refreshToken := generateRefreshToken(userID)

		// Формируем ответ
		response := map[string]string{
			"access_token":  accessToken,
			"refresh_token": refreshToken,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
	})

	mux.HandleFunc("/api/v1/auth/validate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Парсим тело запроса
		var req struct {
			AccessToken string `json:"access_token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Валидация JWT токена
		if req.AccessToken == "" {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// Парсим JWT токен
		token, err := jwt.ParseWithClaims(req.AccessToken, jwt.MapClaims{}, func(token *jwt.Token) (interface{}, error) {
			// Проверяем метод подписи
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			// Секретный ключ (должен совпадать с тем что используется для генерации)
			return []byte("your-secret-key-here"), nil
		})

		if err != nil {
			http.Error(w, "Invalid token: "+err.Error(), http.StatusUnauthorized)
			return
		}

		// Получаем claims из токена
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok || !token.Valid {
			http.Error(w, "Invalid token claims", http.StatusUnauthorized)
			return
		}

		// Извлекаем данные пользователя из claims
		userID, _ := claims["user_id"].(string)
		tenantID, _ := claims["tenant_id"].(string)
		email, _ := claims["email"].(string)
		isAdmin, _ := claims["is_admin"].(bool)
		exp, _ := claims["exp"].(float64)

		// Формируем ответ с реальными данными из токена
		userInfo := map[string]interface{}{
			"user_id":    userID,
			"tenant_id":  tenantID,
			"is_admin":   isAdmin,
			"email":      email,
			"expires_at": int64(exp),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(userInfo)
	})

	log.Println("Auth Service starting on port 51051...")
	log.Fatal(server.ListenAndServe())
}
