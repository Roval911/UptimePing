package service

// Ключи для использования в контексте
// Используются для передачи данных между middleware и обработчиками

// UserIDKey ключ для хранения ID пользователя в контексте
var UserIDKey = "user_id"

// TenantIDKey ключ для хранения ID тенанта в контексте
var TenantIDKey = "tenant_id"

// IsAdminKey ключ для хранения флага администратора в контексте
var IsAdminKey = "is_admin"