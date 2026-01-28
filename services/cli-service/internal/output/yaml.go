package output

import (
	"fmt"
	"time"

	"gopkg.in/yaml.v2"
)

// YAMLOutput представляет YAML вывод с метаданными
type YAMLOutput struct {
	Success   bool        `yaml:"success"`
	Data      interface{} `yaml:"data,omitempty"`
	Error     string      `yaml:"error,omitempty"`
	Timestamp time.Time   `yaml:"timestamp"`
	Metadata  *Metadata   `yaml:"metadata,omitempty"`
}

// NewYAMLOutput создает новый YAML вывод
func NewYAMLOutput(success bool, data interface{}, err error) *YAMLOutput {
	return &YAMLOutput{
		Success:   success,
		Data:      data,
		Error:     getErrorString(err),
		Timestamp: time.Now(),
	}
}

// WithMetadata добавляет метаданные
func (yo *YAMLOutput) WithMetadata(command, format string, context map[string]interface{}, total, page, pageSize int) *YAMLOutput {
	yo.Metadata = &Metadata{
		Command:  command,
		Format:   format,
		Context:  context,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}
	return yo
}

// String возвращает YAML строку
func (yo *YAMLOutput) String() string {
	data, err := yaml.Marshal(yo)
	if err != nil {
		return fmt.Sprintf("success: false\nerror: Failed to marshal YAML: %s", err.Error())
	}
	return string(data)
}

// CreateIncidentsYAMLResponse создает YAML ответ для инцидентов
func CreateIncidentsYAMLResponse(incidents []interface{}, total, page, pageSize int) *YAMLOutput {
	response := map[string]interface{}{
		"items": incidents,
		"pagination": NewPagination(page, pageSize, total),
	}
	
	return NewYAMLOutput(true, response, nil).
		WithMetadata("incidents list", "yaml", nil, total, page, pageSize)
}

// CreateChecksYAMLResponse создает YAML ответ для проверок
func CreateChecksYAMLResponse(checks []interface{}, total, page, pageSize int) *YAMLOutput {
	response := map[string]interface{}{
		"items": checks,
		"pagination": NewPagination(page, pageSize, total),
	}
	
	return NewYAMLOutput(true, response, nil).
		WithMetadata("checks list", "yaml", nil, total, page, pageSize)
}

// CreateChannelsYAMLResponse создает YAML ответ для каналов
func CreateChannelsYAMLResponse(channels []interface{}, total, page, pageSize int) *YAMLOutput {
	response := map[string]interface{}{
		"items": channels,
		"pagination": NewPagination(page, pageSize, total),
	}
	
	return NewYAMLOutput(true, response, nil).
		WithMetadata("channels list", "yaml", nil, total, page, pageSize)
}

// CreateServicesYAMLResponse создает YAML ответ для сервисов
func CreateServicesYAMLResponse(services []interface{}, total, page, pageSize int) *YAMLOutput {
	response := map[string]interface{}{
		"items": services,
		"pagination": NewPagination(page, pageSize, total),
	}
	
	return NewYAMLOutput(true, response, nil).
		WithMetadata("services list", "yaml", nil, total, page, pageSize)
}

// CreateSingleItemYAMLResponse создает YAML ответ для одного элемента
func CreateSingleItemYAMLResponse(item interface{}, itemType string) *YAMLOutput {
	return NewYAMLOutput(true, item, nil).
		WithMetadata(fmt.Sprintf("%s get", itemType), "yaml", map[string]interface{}{
			"item_type": itemType,
		}, 1, 1, 1)
}

// CreateErrorYAMLResponse создает YAML ответ с ошибкой
func CreateErrorYAMLResponse(err error, command string) *YAMLOutput {
	return NewYAMLOutput(false, nil, err).
		WithMetadata(command, "yaml", map[string]interface{}{
			"error_type": "command_error",
		}, 0, 0, 0)
}

// CreateSuccessYAMLResponse создает YAML ответ об успехе
func CreateSuccessYAMLResponse(message string, command string) *YAMLOutput {
	return NewYAMLOutput(true, map[string]interface{}{
		"message": message,
	}, nil).
		WithMetadata(command, "yaml", map[string]interface{}{
			"result_type": "success",
		}, 0, 0, 0)
}

// PrintYAML выводит YAML в консоль
func PrintYAML(output *YAMLOutput) {
	fmt.Println(output.String())
}

// PrintYAMLToFile выводит YAML в файл
func PrintYAMLToFile(output *YAMLOutput, filename string) error {
	data, err := yaml.Marshal(output)
	if err != nil {
		return fmt.Errorf("failed to marshal YAML: %w", err)
	}
	
	return writeToFile(filename, data)
}
