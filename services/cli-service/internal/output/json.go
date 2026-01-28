package output

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// JSONOutput представляет JSON вывод с метаданными
type JSONOutput struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
	Metadata  *Metadata   `json:"metadata,omitempty"`
}

// Metadata содержит метаданные вывода
type Metadata struct {
	Command    string                 `json:"command"`
	Format     string                 `json:"format"`
	Context    map[string]interface{} `json:"context,omitempty"`
	Total      int                    `json:"total,omitempty"`
	Page       int                    `json:"page,omitempty"`
	PageSize   int                    `json:"page_size,omitempty"`
}

// NewJSONOutput создает новый JSON вывод
func NewJSONOutput(success bool, data interface{}, err error) *JSONOutput {
	return &JSONOutput{
		Success:   success,
		Data:      data,
		Error:     getErrorString(err),
		Timestamp: time.Now(),
	}
}

// WithMetadata добавляет метаданные
func (jo *JSONOutput) WithMetadata(command, format string, context map[string]interface{}, total, page, pageSize int) *JSONOutput {
	jo.Metadata = &Metadata{
		Command:  command,
		Format:   format,
		Context:  context,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}
	return jo
}

// String возвращает JSON строку
func (jo *JSONOutput) String() string {
	data, err := json.MarshalIndent(jo, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"success": false, "error": "Failed to marshal JSON: %s"}`, err.Error())
	}
	return string(data)
}

// Compact возвращает компактную JSON строку
func (jo *JSONOutput) Compact() string {
	data, err := json.Marshal(jo)
	if err != nil {
		return fmt.Sprintf(`{"success": false, "error": "Failed to marshal JSON: %s"}`, err.Error())
	}
	return string(data)
}

// getErrorString преобразует ошибку в строку
func getErrorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// JSONResponse представляет структурированный JSON ответ
type JSONResponse struct {
	Items      interface{} `json:"items"`
	Pagination *Pagination `json:"pagination,omitempty"`
}

// Pagination содержит информацию о пагинации
type Pagination struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
	HasNext    bool `json:"has_next"`
	HasPrev    bool `json:"has_prev"`
}

// NewPagination создает новый объект пагинации
func NewPagination(page, pageSize, total int) *Pagination {
	totalPages := (total + pageSize - 1) / pageSize
	return &Pagination{
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: totalPages,
		HasNext:    page < totalPages,
		HasPrev:    page > 1,
	}
}

// CreateIncidentsJSONResponse создает JSON ответ для инцидентов
func CreateIncidentsJSONResponse(incidents []interface{}, total, page, pageSize int) *JSONOutput {
	pagination := NewPagination(page, pageSize, total)
	response := &JSONResponse{
		Items:      incidents,
		Pagination: pagination,
	}
	
	return NewJSONOutput(true, response, nil).
		WithMetadata("incidents list", "json", nil, total, page, pageSize)
}

// CreateChecksJSONResponse создает JSON ответ для проверок
func CreateChecksJSONResponse(checks []interface{}, total, page, pageSize int) *JSONOutput {
	pagination := NewPagination(page, pageSize, total)
	response := &JSONResponse{
		Items:      checks,
		Pagination: pagination,
	}
	
	return NewJSONOutput(true, response, nil).
		WithMetadata("checks list", "json", nil, total, page, pageSize)
}

// CreateChannelsJSONResponse создает JSON ответ для каналов
func CreateChannelsJSONResponse(channels []interface{}, total, page, pageSize int) *JSONOutput {
	pagination := NewPagination(page, pageSize, total)
	response := &JSONResponse{
		Items:      channels,
		Pagination: pagination,
	}
	
	return NewJSONOutput(true, response, nil).
		WithMetadata("channels list", "json", nil, total, page, pageSize)
}

// CreateServicesJSONResponse создает JSON ответ для сервисов
func CreateServicesJSONResponse(services []interface{}, total, page, pageSize int) *JSONOutput {
	pagination := NewPagination(page, pageSize, total)
	response := &JSONResponse{
		Items:      services,
		Pagination: pagination,
	}
	
	return NewJSONOutput(true, response, nil).
		WithMetadata("services list", "json", nil, total, page, pageSize)
}

// CreateSingleItemJSONResponse создает JSON ответ для одного элемента
func CreateSingleItemJSONResponse(item interface{}, itemType string) *JSONOutput {
	return NewJSONOutput(true, item, nil).
		WithMetadata(fmt.Sprintf("%s get", itemType), "json", map[string]interface{}{
			"item_type": itemType,
		}, 1, 1, 1)
}

// CreateErrorJSONResponse создает JSON ответ с ошибкой
func CreateErrorJSONResponse(err error, command string) *JSONOutput {
	return NewJSONOutput(false, nil, err).
		WithMetadata(command, "json", map[string]interface{}{
			"error_type": "command_error",
		}, 0, 0, 0)
}

// CreateSuccessJSONResponse создает JSON ответ об успехе
func CreateSuccessJSONResponse(message string, command string) *JSONOutput {
	return NewJSONOutput(true, map[string]interface{}{
		"message": message,
	}, nil).
		WithMetadata(command, "json", map[string]interface{}{
			"result_type": "success",
		}, 0, 0, 0)
}

// PrintJSON выводит JSON в консоль
func PrintJSON(output *JSONOutput, pretty bool) {
	if pretty {
		fmt.Println(output.String())
	} else {
		fmt.Println(output.Compact())
	}
}

// PrintJSONToFile выводит JSON в файл
func PrintJSONToFile(output *JSONOutput, filename string, pretty bool) error {
	var data []byte
	var err error
	
	if pretty {
		data, err = json.MarshalIndent(output, "", "  ")
	} else {
		data, err = json.Marshal(output)
	}
	
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	
	return writeToFile(filename, data)
}

// writeToFile записывает данные в файл
func writeToFile(filename string, data []byte) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()
	
	_, err = file.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	
	return nil
}
