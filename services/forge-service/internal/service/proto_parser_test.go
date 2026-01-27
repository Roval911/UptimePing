package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProtoParser_LoadAndValidateProtoFiles(t *testing.T) {
	// Создаем временную директорию для тестов
	tempDir := t.TempDir()

	// Создаем тестовый proto файл
	protoContent := `
syntax = "proto3";

package test.service;

option go_package = "test/service";

service TestService {
	rpc GetUser(GetUserRequest) returns (GetUserResponse);
	rpc ListUsers(ListUsersRequest) returns (stream ListUsersResponse);
	rpc CreateUser(stream CreateUserRequest) returns (CreateUserResponse);
}

message GetUserRequest {
	string user_id = 1;
}

message GetUserResponse {
	string user_id = 1;
	string name = 2;
	string email = 3;
}

message ListUsersRequest {
	int32 page = 1;
	int32 limit = 2;
}

message ListUsersResponse {
	repeated User users = 1;
}

message CreateUserRequest {
	string name = 1;
	string email = 2;
}

message CreateUserResponse {
	string user_id = 1;
}

message User {
	string id = 1;
	string name = 2;
	string email = 3;
	UserStatus status = 4;
}

enum UserStatus {
	UNKNOWN = 0;
	ACTIVE = 1;
	INACTIVE = 2;
	SUSPENDED = 3;
}
`

	// Записываем proto файл
	protoFile := filepath.Join(tempDir, "test.proto")
	err := os.WriteFile(protoFile, []byte(protoContent), 0644)
	require.NoError(t, err)

	// Создаем парсер
	parser := NewProtoParser(tempDir)

	// Загружаем и валидируем файлы
	err = parser.LoadAndValidateProtoFiles()
	assert.NoError(t, err)

	// Проверяем результаты
	services := parser.GetServices()
	assert.Len(t, services, 1)

	service := services[0]
	assert.Equal(t, "TestService", service.Name)
	assert.Equal(t, "test.service", service.Package)
	assert.Len(t, service.Methods, 3)

	// Проверяем методы
	methods := service.Methods
	assert.Equal(t, "GetUser", methods[0].Name)
	assert.Equal(t, "GetUserRequest", methods[0].InputType)
	assert.Equal(t, "GetUserResponse", methods[0].OutputType)
	assert.False(t, methods[0].ClientStreaming)
	assert.False(t, methods[0].ServerStreaming)

	assert.Equal(t, "ListUsers", methods[1].Name)
	assert.Equal(t, "ListUsersRequest", methods[1].InputType)
	assert.Equal(t, "ListUsersResponse", methods[1].OutputType)
	assert.False(t, methods[1].ClientStreaming)
	assert.True(t, methods[1].ServerStreaming)

	assert.Equal(t, "CreateUser", methods[2].Name)
	assert.Equal(t, "CreateUserRequest", methods[2].InputType)
	assert.Equal(t, "CreateUserResponse", methods[2].OutputType)
	assert.True(t, methods[2].ClientStreaming)
	assert.False(t, methods[2].ServerStreaming)

	// Проверяем сообщения
	messages := parser.GetMessages()
	assert.Len(t, messages, 7) // GetUserRequest, GetUserResponse, ListUsersRequest, ListUsersResponse, CreateUserRequest, CreateUserResponse, User

	// Проверяем конкретное сообщение
	userMessage := parser.GetMessageByName("User")
	if userMessage != nil {
		assert.Equal(t, "User", userMessage.Name)
		assert.Equal(t, "test.service", userMessage.Package)
		assert.Len(t, userMessage.Fields, 4)

		// Проверяем поля
		fields := userMessage.Fields
		assert.Equal(t, "id", fields[0].Name)
		assert.Equal(t, "string", fields[0].Type)
		assert.Equal(t, "status", fields[3].Name)
		assert.Equal(t, "UserStatus", fields[3].Type)
	}

	// Проверяем enums
	enums := parser.GetEnums()
	assert.Len(t, enums, 1)

	userStatusEnum := parser.GetEnumByName("UserStatus")
	if userStatusEnum != nil {
		assert.Equal(t, "UserStatus", userStatusEnum.Name)
		assert.Equal(t, "test.service", userStatusEnum.Package)
		assert.Len(t, userStatusEnum.Values, 4)
	}

	// Проверяем значения enum
	if userStatusEnum != nil {
		values := userStatusEnum.Values
		assert.Equal(t, "UNKNOWN", values[0].Name)
		assert.Equal(t, "ACTIVE", values[1].Name)
		assert.Equal(t, "INACTIVE", values[2].Name)
		assert.Equal(t, "SUSPENDED", values[3].Name)
	}
}

func TestProtoParser_NonExistentDirectory(t *testing.T) {
	parser := NewProtoParser("/non/existent/directory")

	err := parser.LoadAndValidateProtoFiles()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "proto directory does not exist")
}

func TestProtoParser_EmptyDirectory(t *testing.T) {
	tempDir := t.TempDir()

	parser := NewProtoParser(tempDir)

	err := parser.LoadAndValidateProtoFiles()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no .proto files found")
}

func TestProtoParser_Validate(t *testing.T) {
	// Создаем временную директорию
	tempDir := t.TempDir()

	// Создаем proto файл с дубликатом имени сервиса
	protoContent := `
syntax = "proto3";

package test.service;

service TestService {
	rpc Method1(Request) returns (Response);
}

service TestService {
	rpc Method2(Request) returns (Response);
}

message Request {
	string field = 1;
}

message Response {
	string field = 1;
}
`

	protoFile := filepath.Join(tempDir, "test.proto")
	err := os.WriteFile(protoFile, []byte(protoContent), 0644)
	require.NoError(t, err)

	parser := NewProtoParser(tempDir)
	err = parser.LoadAndValidateProtoFiles()
	assert.NoError(t, err)

	// Валидация должна найти дубликат
	err = parser.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate service name: TestService")
}

func TestProtoParser_GetMethods(t *testing.T) {
	parser := &ProtoParser{
		services: []*ServiceInfo{
			{
				Name: "TestService",
				Methods: []*MethodInfo{
					{Name: "Method1"},
					{Name: "Method2"},
				},
			},
		},
	}

	service := parser.GetServiceByName("TestService")
	assert.NotNil(t, service)
	assert.Len(t, service.Methods, 2)
	assert.Equal(t, "Method1", service.Methods[0].Name)
	assert.Equal(t, "Method2", service.Methods[1].Name)

	// Тест несуществующего сервиса
	nonExistent := parser.GetServiceByName("NonExistent")
	assert.Nil(t, nonExistent)
}

func TestProtoParser_ExtractPackageName(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "simple package",
			content:  "package test.service;",
			expected: "test.service",
		},
		{
			name:     "package with comment",
			content:  "package test.service; // comment",
			expected: "test.service",
		},
		{
			name:     "no package",
			content:  "syntax = \"proto3\";",
			expected: "default",
		},
		{
			name:     "package with spaces",
			content:  "  package   test.service   ;",
			expected: "test.service",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractPackageName(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestProtoParser_ExtractServiceName(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected string
	}{
		{
			name:     "simple service",
			line:     "service TestService {",
			expected: "TestService",
		},
		{
			name:     "service with comment",
			line:     "service TestService { // comment",
			expected: "TestService",
		},
		{
			name:     "service with spaces",
			line:     "  service   TestService   {",
			expected: "TestService",
		},
		{
			name:     "invalid line",
			line:     "not a service line",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractServiceName(tt.line)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestProtoParser_ExtractMessageName(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected string
	}{
		{
			name:     "simple message",
			line:     "message TestMessage {",
			expected: "TestMessage",
		},
		{
			name:     "message with comment",
			line:     "message TestMessage { // comment",
			expected: "TestMessage",
		},
		{
			name:     "message with spaces",
			line:     "  message   TestMessage   {",
			expected: "TestMessage",
		},
		{
			name:     "invalid line",
			line:     "not a message line",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractMessageName(tt.line)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestProtoParser_ExtractEnumName(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected string
	}{
		{
			name:     "simple enum",
			line:     "enum TestEnum {",
			expected: "TestEnum",
		},
		{
			name:     "enum with comment",
			line:     "enum TestEnum { // comment",
			expected: "TestEnum",
		},
		{
			name:     "enum with spaces",
			line:     "  enum   TestEnum   {",
			expected: "TestEnum",
		},
		{
			name:     "invalid line",
			line:     "not an enum line",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractEnumName(tt.line)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestProtoParser_PrintSummary(t *testing.T) {
	parser := &ProtoParser{
		services: []*ServiceInfo{
			{
				Name:    "TestService",
				Package: "test.service",
				Methods: []*MethodInfo{{Name: "Method1"}},
			},
		},
		messages: []*MessageInfo{
			{
				Name:    "TestMessage",
				Package: "test.service",
				Fields:  []*FieldInfo{{Name: "field1"}},
			},
		},
		enums: []*EnumInfo{
			{
				Name:    "TestEnum",
				Package: "test.service",
				Values:  []*EnumValueInfo{{Name: "VALUE1"}},
			},
		},
	}

	// Просто проверяем, что метод не паникует
	assert.NotPanics(t, parser.PrintSummary)
}
