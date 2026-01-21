package password_test

import (
	"strings"
	"testing"

	"UptimePingPlatform/services/auth-service/internal/pkg/password"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBcryptHasher_Hash(t *testing.T) {
	hasher := password.NewBcryptHasher(10)
	testPassword := "TestPassword123"

	hash, err := hasher.Hash(testPassword)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, testPassword, hash)
}

func TestBcryptHasher_Check(t *testing.T) {
	hasher := password.NewBcryptHasher(10)
	testPassword := "TestPassword123"

	hash, err := hasher.Hash(testPassword)
	require.NoError(t, err)

	result := hasher.Check(testPassword, hash)
	assert.True(t, result)

	result = hasher.Check("WrongPassword123", hash)
	assert.False(t, result)
}

func TestBcryptHasher_Validate_Valid(t *testing.T) {
	hasher := password.NewBcryptHasher(10)

	validPasswords := []string{
		"Password123",
		"SecurePass456",
		"MyPassw0rd!@#",
		"Test1234",
		"HelloWorld42",
		"P@ssw0rd1",    // Добавили цифру
		"My$Password1", // Добавили цифру
	}

	for _, pwd := range validPasswords {
		t.Run(pwd, func(t *testing.T) {
			result := hasher.Validate(pwd)
			assert.True(t, result, "Password %s should be valid", pwd)
		})
	}
}

func TestBcryptHasher_Validate_Invalid(t *testing.T) {
	hasher := password.NewBcryptHasher(10)

	invalidPasswords := []struct {
		password string
		reason   string
	}{
		{"pass", "Слишком короткий"},
		{"password", "Нет цифр и заглавных букв"},
		{"PASSWORD", "Нет цифр и строчных букв"},
		{"Password", "Нет цифр"},
		{"password123", "Нет заглавных букв"},
		{"PASSWORD123", "Нет строчных букв"},
		{"Pass1", "Слишком короткий"},
		{"", "Пустой пароль"},
		{"12345678", "Нет букв"},
		{"abcdefgh", "Нет цифр и заглавных букв"},
		{"ABCDEFGH", "Нет цифр и строчных букв"},
		{"!@#$%^&*", "Нет букв и цифр"},
		{"My$Password", "Нет цифр"}, // Теперь это будет правильно
	}

	for _, testCase := range invalidPasswords {
		t.Run(testCase.reason, func(t *testing.T) {
			result := hasher.Validate(testCase.password)
			assert.False(t, result, "Password '%s' should be invalid: %s", testCase.password, testCase.reason)
		})
	}
}

func TestBcryptHasher_Hash_Strength(t *testing.T) {
	hasher := password.NewBcryptHasher(10)
	testPassword := "TestPassword123"

	hash1, err := hasher.Hash(testPassword)
	require.NoError(t, err)

	hash2, err := hasher.Hash(testPassword)
	require.NoError(t, err)

	assert.NotEqual(t, hash1, hash2)
}

func TestBcryptHasher_Check_Empty(t *testing.T) {
	hasher := password.NewBcryptHasher(10)

	assert.False(t, hasher.Check("", ""))
	assert.False(t, hasher.Check("password", ""))
	assert.False(t, hasher.Check("", "$2a$10$N9qo8uLOickgx2ZMRZoMyO$u5p9gDa/SqWZtMGF10K$)"))
}

func TestBcryptHasher_Check_WrongHashFormat(t *testing.T) {
	hasher := password.NewBcryptHasher(10)

	assert.False(t, hasher.Check("password", "invalid-hash-format"))
	assert.False(t, hasher.Check("password", "short"))
}

func TestBcryptHasher_DifferentCosts(t *testing.T) {
	costs := []int{4, 8, 10, 12}

	for _, cost := range costs {
		t.Run("Cost", func(t *testing.T) {
			hasher := password.NewBcryptHasher(cost)
			testPassword := "TestPassword123"

			hash, err := hasher.Hash(testPassword)
			require.NoError(t, err)

			result := hasher.Check(testPassword, hash)
			assert.True(t, result)
		})
	}
}

func TestBcryptHasher_InterfaceCompliance(t *testing.T) {
	var _ password.Hasher = &password.BcryptHasher{}
}

func TestDeprecated_HashPassword(t *testing.T) {
	testPassword := "TestPassword123"

	hash, err := password.HashPassword(testPassword)
	assert.NoError(t, err)
	assert.NotEmpty(t, hash)

	result := password.CheckPasswordHash(testPassword, hash)
	assert.True(t, result)
}

func TestDeprecated_CheckPasswordHash(t *testing.T) {
	testPassword := "TestPassword123"

	hash, err := password.HashPassword(testPassword)
	assert.NoError(t, err)

	assert.True(t, password.CheckPasswordHash(testPassword, hash))
	assert.False(t, password.CheckPasswordHash("WrongPassword", hash))
}

func TestDeprecated_ValidatePassword(t *testing.T) {
	assert.True(t, password.ValidatePassword("Password123"))
	assert.False(t, password.ValidatePassword("pass"))
	assert.False(t, password.ValidatePassword("password"))
	assert.False(t, password.ValidatePassword("PASSWORD"))
	assert.False(t, password.ValidatePassword("12345678"))
}

func TestBcryptHasher_Performance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	hasher := password.NewBcryptHasher(12)
	testPassword := "VerySecurePassword123"

	hash, err := hasher.Hash(testPassword)
	require.NoError(t, err)

	result := hasher.Check(testPassword, hash)
	assert.True(t, result)
}

func TestBcryptHasher_EdgeCases(t *testing.T) {
	hasher := password.NewBcryptHasher(10)

	// Очень длинный пароль, но в пределах 72 символов
	longPassword := "A" + strings.Repeat("a", 60) + "1" // 62 символа
	assert.True(t, hasher.Validate(longPassword))

	hash, err := hasher.Hash(longPassword)
	require.NoError(t, err)

	result := hasher.Check(longPassword, hash)
	assert.True(t, result)

	// Пароль с Unicode символами
	unicodePassword := "Pässwörd123"
	assert.True(t, hasher.Validate(unicodePassword))

	// Пароль с пробелами
	spacePassword := "Password 123"
	assert.True(t, hasher.Validate(spacePassword))

	// Пароль со специальными символами
	specialPassword := "P@ssw0rd!"
	assert.True(t, hasher.Validate(specialPassword))
}

func TestBcryptHasher_Bcrypt72ByteLimit(t *testing.T) {
	hasher := password.NewBcryptHasher(10)

	// Тест граничного значения (72 символа)
	exact72CharPassword := strings.Repeat("A", 70) + "a1" // 72 символа
	hash, err := hasher.Hash(exact72CharPassword)
	require.NoError(t, err)

	result := hasher.Check(exact72CharPassword, hash)
	assert.True(t, result)

	// Тест превышения лимита (73 символа)
	over72CharPassword := strings.Repeat("A", 71) + "a1" // 73 символа
	_, err = hasher.Hash(over72CharPassword)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "password length exceeds 72 bytes")
}

func TestBcryptHasher_PasswordWithSpecialCharacters(t *testing.T) {
	hasher := password.NewBcryptHasher(10)

	specialPasswords := []struct {
		password string
		expected bool
	}{
		{"P@ssw0rd!", true},
		{"Test#1234", true},
		{"My$Password1", true}, // Добавили цифру
		{"Secret%123", true},
		{"Hello^World1", true},
		{"Pass&Word2", true},
		{"Test*Test3", true},
		{"(Password123)", true},
		{"[Secure456]", true},
		{"{Lock789}", true},
		{"My$Password", false}, // Нет цифры
		{"Test#Test", false},   // Нет цифры
	}

	for _, testCase := range specialPasswords {
		t.Run(testCase.password, func(t *testing.T) {
			assert.Equal(t, testCase.expected, hasher.Validate(testCase.password))

			if testCase.expected {
				hash, err := hasher.Hash(testCase.password)
				require.NoError(t, err)

				result := hasher.Check(testCase.password, hash)
				assert.True(t, result)
			}
		})
	}
}

func TestBcryptHasher_InternationalCharacters(t *testing.T) {
	hasher := password.NewBcryptHasher(10)

	internationalPasswords := []struct {
		password string
		expected bool
	}{
		{"Pässwörd123", true},   // Немецкие умлауты
		{"Motdepasse123", true}, // Французский
		{"Пароль123", true},     // Кириллица с цифрами
		{"ПАРОЛЬ123", false},    // Только заглавные кириллица (нет строчных латинских)
		{"пароль123", false},    // Только строчные кириллица (нет заглавных)
		{"Senha123", true},      // Португальский
		{"Contraseña123", true}, // Испанский
	}

	for _, testCase := range internationalPasswords {
		t.Run(testCase.password, func(t *testing.T) {
			assert.Equal(t, testCase.expected, hasher.Validate(testCase.password))

			if testCase.expected {
				hash, err := hasher.Hash(testCase.password)
				require.NoError(t, err)

				result := hasher.Check(testCase.password, hash)
				assert.True(t, result)
			}
		})
	}
}

func TestBcryptHasher_ValidateCustomRules(t *testing.T) {
	hasher := password.NewBcryptHasher(10)

	testCases := []struct {
		name     string
		password string
		expected bool
	}{
		{"Минимальная длина", "Aa1", false},
		{"С цифрой", "Password", false},
		{"С заглавной", "password1", false},
		{"Со строчной", "PASSWORD1", false},
		{"Все требования", "Password1", true},
		{"Спецсимволы", "P@ssw0rd", true},
		{"Кириллица с цифрой", "Пароль1", true},
		{"С пробелом", "Pass word1", true},
		{"Длинный пароль", "VeryLongPasswordThatIsStillValid123", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := hasher.Validate(tc.password)
			assert.Equal(t, tc.expected, result)
		})
	}
}
