package output

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v2"
)

// FormatType представляет тип форматирования вывода
type FormatType string

const (
	FormatTable FormatType = "table"
	FormatJSON  FormatType = "json"
	FormatYAML  FormatType = "yaml"
)

// Formatter интерфейс для форматирования вывода
type Formatter interface {
	Format(data interface{}) (string, error)
}

// TableFormatter форматирует данные в виде таблицы
type TableFormatter struct{}

func NewTableFormatter() *TableFormatter {
	return &TableFormatter{}
}

func (f *TableFormatter) Format(data interface{}) (string, error) {
	switch v := data.(type) {
	case *TableData:
		return f.formatTable(v), nil
	case *TableRow:
		return f.formatSingleRow(v), nil
	default:
		return fmt.Sprintf("%v", v), nil
	}
}

func (f *TableFormatter) formatTable(data *TableData) string {
	if len(data.Rows) == 0 {
		return "No data found"
	}

	var builder strings.Builder
	
	// Формируем заголовок
	builder.WriteString(strings.Join(data.Headers, "\t") + "\n")
	
	// Формируем разделитель
	separators := make([]string, len(data.Headers))
	for i := range separators {
		separators[i] = strings.Repeat("-", len(data.Headers[i]))
	}
	builder.WriteString(strings.Join(separators, "\t") + "\n")
	
	// Формируем строки данных
	for _, row := range data.Rows {
		builder.WriteString(strings.Join(row.Cells, "\t") + "\n")
	}
	
	return builder.String()
}

func (f *TableFormatter) formatSingleRow(row *TableRow) string {
	return strings.Join(row.Cells, "\t")
}

// JSONFormatter форматирует данные в JSON
type JSONFormatter struct {
	Pretty bool
}

func NewJSONFormatter(pretty bool) *JSONFormatter {
	return &JSONFormatter{Pretty: pretty}
}

func (f *JSONFormatter) Format(data interface{}) (string, error) {
	var output []byte
	var err error
	
	if f.Pretty {
		output, err = json.MarshalIndent(data, "", "  ")
	} else {
		output, err = json.Marshal(data)
	}
	
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}
	
	return string(output), nil
}

// YAMLFormatter форматирует данные в YAML
type YAMLFormatter struct{}

func NewYAMLFormatter() *YAMLFormatter {
	return &YAMLFormatter{}
}

func (f *YAMLFormatter) Format(data interface{}) (string, error) {
	output, err := yaml.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal YAML: %w", err)
	}
	
	return string(output), nil
}

// ColorFormatter добавляет цветовое форматирование
type ColorFormatter struct {
	Formatter Formatter
	UseColors bool
}

func NewColorFormatter(formatter Formatter, useColors bool) *ColorFormatter {
	return &ColorFormatter{
		Formatter: formatter,
		UseColors: useColors,
	}
}

func (f *ColorFormatter) Format(data interface{}) (string, error) {
	output, err := f.Formatter.Format(data)
	if err != nil {
		return "", err
	}
	
	if !f.UseColors {
		return output, nil
	}
	
	return f.applyColors(output), nil
}

func (f *ColorFormatter) applyColors(output string) string {
	lines := strings.Split(output, "\n")
	var result []string
	
	for i, line := range lines {
		if i == 0 {
			// Заголовок - синий цвет
			result = append(result, fmt.Sprintf("\033[1;34m%s\033[0m", line))
		} else if strings.Contains(line, "---") {
			// Разделитель - серый цвет
			result = append(result, fmt.Sprintf("\033[1;90m%s\033[0m", line))
		} else if strings.Contains(line, "✓") || strings.Contains(line, "Success") {
			// Успех - зеленый цвет
			result = append(result, fmt.Sprintf("\033[1;32m%s\033[0m", line))
		} else if strings.Contains(line, "✗") || strings.Contains(line, "Error") || strings.Contains(line, "Failed") {
			// Ошибка - красный цвет
			result = append(result, fmt.Sprintf("\033[1;31m%s\033[0m", line))
		} else if strings.Contains(line, "⚠") || strings.Contains(line, "Warning") {
			// Предупреждение - желтый цвет
			result = append(result, fmt.Sprintf("\033[1;33m%s\033[0m", line))
		} else {
			result = append(result, line)
		}
	}
	
	return strings.Join(result, "\n")
}

// GetFormatter возвращает подходящий форматировщик
func GetFormatter(format FormatType, pretty bool, useColors bool) Formatter {
	var baseFormatter Formatter
	
	switch format {
	case FormatJSON:
		baseFormatter = NewJSONFormatter(pretty)
	case FormatYAML:
		baseFormatter = NewYAMLFormatter()
	case FormatTable:
		fallthrough
	default:
		baseFormatter = NewTableFormatter()
	}
	
	if useColors && format == FormatTable {
		return NewColorFormatter(baseFormatter, useColors)
	}
	
	return baseFormatter
}

// DetectFormat определяет формат из переменных окружения или аргументов
func DetectFormat(args []string) FormatType {
	// Проверяем аргументы командной строки
	for _, arg := range args {
		switch strings.ToLower(arg) {
		case "--json", "-j":
			return FormatJSON
		case "--yaml", "-y":
			return FormatYAML
		case "--table", "-t":
			return FormatTable
		}
	}
	
	// Проверяем переменные окружения
	if format := os.Getenv("UPTIMEPING_FORMAT"); format != "" {
		switch strings.ToLower(format) {
		case "json":
			return FormatJSON
		case "yaml":
			return FormatYAML
		case "table":
			return FormatTable
		}
	}
	
	// По умолчанию - таблица
	return FormatTable
}

// DetectPretty определяет нужно ли форматировать вывод
func DetectPretty(args []string) bool {
	for _, arg := range args {
		switch arg {
		case "--pretty", "-p":
			return true
		case "--no-pretty":
			return false
		}
	}
	
	// Проверяем переменные окружения
	if pretty := os.Getenv("UPTIMEPING_PRETTY"); pretty != "" {
		return strings.ToLower(pretty) == "true"
	}
	
	// По умолчанию для JSON и YAML - pretty, для таблицы - нет
	format := DetectFormat(args)
	return format == FormatJSON || format == FormatYAML
}

// DetectColors определяет нужно ли использовать цвета
func DetectColors() bool {
	// Проверяем переменные окружения
	if colors := os.Getenv("UPTIMEPING_COLORS"); colors != "" {
		return strings.ToLower(colors) == "true"
	}
	
	// Проверяем, что вывод идет в терминал
	return os.Stdout != nil && isTerminal()
}

// isTerminal проверяет, что вывод идет в терминал
func isTerminal() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	
	return (fi.Mode() & os.ModeCharDevice) != 0
}
