package errors

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestError_ToGRPCErr_WithDetails(t *testing.T) {
	tests := []struct {
		name        string
		error       *Error
		expectCode  codes.Code
		expectMsg   string
		expectDetails string
	}{
		{
			name: "error with details",
			error: &Error{
				Code:    ErrValidation,
				Message: "validation failed",
				Details: "field 'email' is required",
			},
			expectCode:     codes.InvalidArgument,
			expectMsg:      "validation failed",
			expectDetails:  "field 'email' is required",
		},
		{
			name: "error with details and context",
			error: &Error{
				Code:    ErrInternal,
				Message: "internal error",
				Details: "database connection failed",
				Context: context.WithValue(context.Background(), "trace_id", "trace-123"),
			},
			expectCode:     codes.Internal,
			expectMsg:      "internal error",
			expectDetails:  "database connection failed (trace_id: trace-123)",
		},
		{
			name: "error without details",
			error: &Error{
				Code:    ErrNotFound,
				Message: "resource not found",
			},
			expectCode:     codes.NotFound,
			expectMsg:      "resource not found",
			expectDetails:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			grpcErr := tt.error.ToGRPCErr()
			
			// Проверяем gRPC статус
			grpcStatus, ok := status.FromError(grpcErr)
			assert.True(t, ok, "error should be gRPC status")
			assert.Equal(t, tt.expectCode, grpcStatus.Code())
			assert.Equal(t, tt.expectMsg, grpcStatus.Message())
			
			// Проверяем детали
			if tt.expectDetails != "" {
				details := grpcStatus.Details()
				assert.NotEmpty(t, details, "should have details")
				if len(details) > 0 {
					errorDetails, ok := details[0].(*ErrorDetails)
					assert.True(t, ok, "detail should be ErrorDetails")
					if ok {
						assert.Equal(t, tt.expectDetails, errorDetails.Details)
					}
				}
			} else {
				assert.Empty(t, grpcStatus.Details(), "should not have details")
			}
		})
	}
}

func TestExtractErrorDetails(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectDetails  string
	}{
		{
			name: "gRPC error with details",
			err: func() error {
				errorDetails := &ErrorDetails{Details: "field 'email' is required"}
				status, _ := status.New(codes.InvalidArgument, "validation failed").WithDetails(errorDetails)
				return status.Err()
			}(),
			expectDetails: "field 'email' is required",
		},
		{
			name: "gRPC error without details",
			err: status.Error(codes.NotFound, "resource not found"),
			expectDetails: "",
		},
		{
			name:          "non-gRPC error",
			err:           assert.AnError,
			expectDetails: "",
		},
		{
			name:          "nil error",
			err:           nil,
			expectDetails: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			details := ExtractErrorDetails(tt.err)
			assert.Equal(t, tt.expectDetails, details)
		})
	}
}

func TestFromGRPCErr_WithDetails(t *testing.T) {
	// Создаем gRPC ошибку с деталями
	errorDetails := &ErrorDetails{Details: "field 'email' is required"}
	grpcStatus, _ := status.New(codes.InvalidArgument, "validation failed").WithDetails(errorDetails)
	grpcErr := grpcStatus.Err()

	// Конвертируем в нашу ошибку
	customErr := FromGRPCErr(grpcErr)

	// Проверяем базовые поля
	assert.NotNil(t, customErr)
	assert.Equal(t, ErrValidation, customErr.Code)
	assert.Equal(t, "validation failed", customErr.Message)

	// Проверяем, что детали можно извлечь обратно
	details := ExtractErrorDetails(grpcErr)
	assert.Equal(t, "field 'email' is required", details)
}

func TestErrorDetails_Structure(t *testing.T) {
	errorDetails := &ErrorDetails{
		Details: "test error details",
	}

	assert.Equal(t, "test error details", errorDetails.Details)
}
