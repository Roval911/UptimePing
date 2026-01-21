package apikey_test

import (
	"fmt"
	"strings"
	"testing"

	"UptimePingPlatform/services/auth-service/internal/pkg/apikey"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateKeyPair(t *testing.T) {
	keyPair, err := apikey.GenerateKeyPair()
	require.NoError(t, err)
	require.NotNil(t, keyPair)
	assert.NotEmpty(t, keyPair.Key)
	assert.NotEmpty(t, keyPair.Secret)

	assert.True(t, len(keyPair.Key) >= 4+24)
	assert.True(t, len(keyPair.Secret) >= 4+32)
	assert.True(t, strings.HasPrefix(keyPair.Key, "upk_"))
	assert.True(t, strings.HasPrefix(keyPair.Secret, "sec_"))
	assert.True(t, apikey.ValidateFormat(keyPair.Key, keyPair.Secret))

	keyPair2, err := apikey.GenerateKeyPair()
	require.NoError(t, err)
	assert.NotEqual(t, keyPair.Key, keyPair2.Key)
	assert.NotEqual(t, keyPair.Secret, keyPair2.Secret)
}

func TestHashKey(t *testing.T) {
	key := "upk_testkey1234567890abcdef"

	hash, err := apikey.HashKey(key)
	assert.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, key, hash)
}

func TestValidateKey(t *testing.T) {
	key := "upk_testkey1234567890abcdef"

	hash, err := apikey.HashKey(key)
	assert.NoError(t, err)

	assert.True(t, apikey.ValidateKey(key, hash))
	assert.False(t, apikey.ValidateKey("upk_wrongkey4567890abcdefgh", hash))
	assert.False(t, apikey.ValidateKey("upk_testkey1234567890abcde", hash))
}

func TestFormatKey(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		// Ключи с префиксом upk_
		{"Короткий upk ключ (3 символа тела)", "upk_abc", "upk_abc"},   // тело < 6 символов
		{"Upk ключ из 4 символов тела", "upk_abcd", "upk_abcd"},        // тело < 6 символов
		{"Upk ключ из 5 символов тела", "upk_abcde", "upk_abcde"},      // тело < 6 символов
		{"Upk ключ из 6 символов тела", "upk_abcdef", "upk_ab***def"},  // тело = 6 символов
		{"Upk ключ из 7 символов тела", "upk_abcdefg", "upk_ab***efg"}, // тело = 7 символов
		{"Средний upk ключ", "upk_12345678", "upk_12***678"},
		{"Длинный upk ключ", "upk_abcdefghijklmnop", "upk_ab***nop"},
		{"Очень длинный upk ключ", "upk_1234567890abcdefghijklmnop", "upk_12***nop"},
		{"С нижним подчеркиванием в upk ключе", "upk_a_b_c_d_e_f", "upk_a_***e_f"},
		{"Только префикс upk", "upk_", "upk_"},

		// Ключи без префикса upk_
		{"Короткий ключ без префикса", "short", "short"},
		{"9 символов без префикса", "123456789", "123456789"},           // < 10 символов
		{"10 символов без префикса", "1234567890", "123***890"},         // = 10 символов
		{"11 символов без префикса", "12345678901", "123***901"},        // > 10 символов
		{"Ключ без подчеркивания (8 символов)", "abcdefgh", "abcdefgh"}, // < 10 символов
		{"Неправильный формат с одним подчеркиванием", "test_key_123", "tes***123"},
		{"Ключ с подчеркиванием в конце", "key_", "key_"},
		{"Ключ с несколькими подчеркиваниями", "prefix_middle_key_123", "pre***123"},

		// Edge cases
		{"Пустая строка", "", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := apikey.FormatKey(tc.input)
			assert.Equal(t, tc.expected, result, "Input: %s, Expected: %s, Got: %s", tc.input, tc.expected, result)
		})
	}
}

func TestExtractKeyFromHeader(t *testing.T) {
	testCases := []struct {
		name       string
		header     string
		wantKey    string
		wantSecret string
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "Bearer с правильным форматом",
			header:     "Bearer upk_key123:sec_secret456",
			wantKey:    "upk_key123",
			wantSecret: "sec_secret456",
			wantErr:    false,
		},
		{
			name:       "ApiKey с правильным форматом",
			header:     "ApiKey upk_key123:sec_secret456",
			wantKey:    "upk_key123",
			wantSecret: "sec_secret456",
			wantErr:    false,
		},
		{
			name:       "Нечувствительность к регистру Bearer",
			header:     "BEARER upk_key123:sec_secret456",
			wantKey:    "upk_key123",
			wantSecret: "sec_secret456",
			wantErr:    false,
		},
		{
			name:       "Нечувствительность к регистру ApiKey",
			header:     "APIKEY upk_key123:sec_secret456",
			wantKey:    "upk_key123",
			wantSecret: "sec_secret456",
			wantErr:    false,
		},
		{
			name:       "С пробелами в начале и конце",
			header:     "  Bearer   upk_key123:sec_secret456  ",
			wantKey:    "upk_key123",
			wantSecret: "sec_secret456",
			wantErr:    false,
		},
		{
			name:    "Неподдерживаемый тип аутентификации",
			header:  "Basic dXNlcjpwYXNz",
			wantErr: true,
			errMsg:  "unsupported authorization type",
		},
		{
			name:    "Отсутствует секрет",
			header:  "Bearer upk_key123",
			wantErr: true,
			errMsg:  "invalid key format",
		},
		{
			name:       "Лишние двоеточия - только первое двоеточие разделяет",
			header:     "Bearer upk_key123:sec_secret456:extra",
			wantKey:    "upk_key123",
			wantSecret: "sec_secret456:extra",
			wantErr:    false,
		},
		{
			name:    "Пустой заголовок",
			header:  "",
			wantErr: true,
			errMsg:  "empty authorization header",
		},
		{
			name:    "Только тип аутентификации",
			header:  "Bearer",
			wantErr: true,
			errMsg:  "invalid authorization header format",
		},
		{
			name:    "Только тип аутентификации с пробелами",
			header:  "Bearer  ",
			wantErr: true,
			errMsg:  "invalid authorization header format",
		},
		{
			name:    "Пустой ключ или секрет",
			header:  "Bearer :",
			wantErr: true,
			errMsg:  "empty key or secret",
		},
		{
			name:    "Пустой ключ",
			header:  "Bearer :sec_secret",
			wantErr: true,
			errMsg:  "empty key or secret",
		},
		{
			name:    "Пустой секрет",
			header:  "Bearer upk_key:",
			wantErr: true,
			errMsg:  "empty key or secret",
		},
		{
			name:       "Секрет с двоеточиями",
			header:     "Bearer upk_key123:sec_secret:with:colons",
			wantKey:    "upk_key123",
			wantSecret: "sec_secret:with:colons",
			wantErr:    false,
		},
		{
			name:       "С пробелами вокруг двоеточия",
			header:     "Bearer upk_key123 : sec_secret456",
			wantKey:    "upk_key123",
			wantSecret: "sec_secret456",
			wantErr:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			key, secret, err := apikey.ExtractKeyFromHeader(tc.header)

			if tc.wantErr {
				require.Error(t, err)
				if tc.errMsg != "" {
					assert.Contains(t, err.Error(), tc.errMsg)
				}
				assert.Empty(t, key)
				assert.Empty(t, secret)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.wantKey, key)
				assert.Equal(t, tc.wantSecret, secret)
			}
		})
	}
}

func TestValidateFormat(t *testing.T) {
	testCases := []struct {
		name     string
		key      string
		secret   string
		expected bool
	}{
		{"Валидный формат", "upk_validkey123", "sec_validsecret456", true},
		{"Минимальная длина", "upk_a", "sec_b", true},
		{"Специальные символы base64", "upk_a-b_c", "sec_d-e_f", true},
		{"Только цифры", "upk_123456", "sec_789012", true},
		{"Смешанный регистр", "upk_AbC123", "sec_XyZ789", true},
		{"Неправильный префикс ключа", "invalidkey", "sec_validsecret", false},
		{"Неправильный префикс секрета", "upk_validkey", "invalidsecret", false},
		{"Неправильные символы в ключе", "upk_key@123", "sec_secret456", false},
		{"Неправильные символы в секрете", "upk_key123", "sec_secret@456", false},
		{"Пустые значения", "", "", false},
		{"Только префикс ключа", "upk_", "sec_valid", false},
		{"Только префикс секрета", "upk_valid", "sec_", false},
		{"Пробелы в ключе", "upk_key 123", "sec_secret456", false},
		{"Пробелы в секрете", "upk_key123", "sec_secret 456", false},
		{"Кириллица в ключе", "upk_ключ123", "sec_secret456", false},
		{"Кириллица в секрете", "upk_key123", "sec_пароль456", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := apikey.ValidateFormat(tc.key, tc.secret)
			assert.Equal(t, tc.expected, result, "For key=%s, secret=%s", tc.key, tc.secret)
		})
	}
}

func TestHashKey_Strength(t *testing.T) {
	key := "upk_testkey1234567890abcdef"

	hash1, err := apikey.HashKey(key)
	assert.NoError(t, err)

	hash2, err := apikey.HashKey(key)
	assert.NoError(t, err)

	assert.NotEqual(t, hash1, hash2, "Hashes of the same key should be different due to salt")
}

func TestValidateKey_EdgeCases(t *testing.T) {
	assert.False(t, apikey.ValidateKey("", ""))
	assert.False(t, apikey.ValidateKey("key", ""))
	assert.False(t, apikey.ValidateKey("", "$2a$10$N9qo8uLOickgx2ZMRZoMyO"))
	assert.False(t, apikey.ValidateKey("key", "invalid-hash"))
	assert.False(t, apikey.ValidateKey("upk_key123", "not-a-valid-bcrypt-hash"))
	assert.False(t, apikey.ValidateKey("upk_key123", "short"))
}

func TestGenerateKeyPair_Multiple(t *testing.T) {
	const numPairs = 50
	keys := make(map[string]bool)
	secrets := make(map[string]bool)

	for i := 0; i < numPairs; i++ {
		keyPair, err := apikey.GenerateKeyPair()
		require.NoError(t, err)

		assert.False(t, keys[keyPair.Key], "Duplicate key generated: %s", keyPair.Key)
		keys[keyPair.Key] = true

		assert.False(t, secrets[keyPair.Secret], "Duplicate secret generated: %s", keyPair.Secret)
		secrets[keyPair.Secret] = true

		assert.True(t, apikey.ValidateFormat(keyPair.Key, keyPair.Secret))
	}
}

func TestFormatKey_Consistency(t *testing.T) {
	key := "upk_1234567890abcdef"

	formatted1 := apikey.FormatKey(key)
	formatted2 := apikey.FormatKey(key)
	formatted3 := apikey.FormatKey(key)

	assert.Equal(t, formatted1, formatted2)
	assert.Equal(t, formatted2, formatted3)
	assert.Equal(t, formatted1, formatted3)
}

func TestExtractKeyFromHeader_Realistic(t *testing.T) {
	keyPair, err := apikey.GenerateKeyPair()
	require.NoError(t, err)

	authHeader := fmt.Sprintf("Bearer %s:%s", keyPair.Key, keyPair.Secret)

	extractedKey, extractedSecret, err := apikey.ExtractKeyFromHeader(authHeader)
	assert.NoError(t, err)
	assert.Equal(t, keyPair.Key, extractedKey)
	assert.Equal(t, keyPair.Secret, extractedSecret)
	assert.True(t, apikey.ValidateFormat(extractedKey, extractedSecret))
}
