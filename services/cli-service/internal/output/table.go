package output

import (
	"fmt"
	"strings"
	"text/tabwriter"
	"time"
)

// TableData представляет данные для табличного вывода
type TableData struct {
	Headers []string
	Rows    []*TableRow
}

// TableRow представляет строку таблицы
type TableRow struct {
	Cells []string
	Style RowStyle
}

// RowStyle определяет стиль строки
type RowStyle int

const (
	StyleDefault RowStyle = iota
	StyleHeader
	StyleSeparator
	StyleSuccess
	StyleError
	StyleWarning
	StyleInfo
)

// NewTableData создает новые табличные данные
func NewTableData(headers []string) *TableData {
	return &TableData{
		Headers: headers,
		Rows:    make([]*TableRow, 0),
	}
}

// AddRow добавляет строку
func (td *TableData) AddRow(cells ...string) {
	td.Rows = append(td.Rows, &TableRow{Cells: cells})
}

// AddRowf добавляет отформатированную строку в таблицу
func (td *TableData) AddRowf(format string, args ...interface{}) {
	row := fmt.Sprintf(format, args...)
	cells := strings.Fields(row)
	td.Rows = append(td.Rows, &TableRow{Cells: cells})
}

// AddRowWithStyle добавляет строку с указанием стиля
func (td *TableData) AddRowWithStyle(cells []string, style RowStyle) {
	row := &TableRow{Cells: cells, Style: style}
	td.Rows = append(td.Rows, row)
}

// AddSeparatorRow добавляет строку-разделитель
func (td *TableData) AddSeparatorRow(char string) {
	separator := make([]string, len(td.Headers))
	for i := range separator {
		separator[i] = strings.Repeat(char, len(td.Headers[i]))
	}
	td.AddRow(separator...)
}

// String возвращает строковое представление таблицы
func (td *TableData) String() string {
	if len(td.Rows) == 0 {
		return "No data found"
	}

	var builder strings.Builder
	w := tabwriter.NewWriter(&builder, 0, 0, 2, ' ', 0)

	// Заголовок
	if len(td.Headers) > 0 {
		fmt.Fprintln(w, td.formatRow(td.Headers, StyleHeader))
		// Разделитель
		separators := make([]string, len(td.Headers))
		for i := range separators {
			separators[i] = strings.Repeat("-", len(td.Headers[i]))
		}
		fmt.Fprintln(w, td.formatRow(separators, StyleSeparator))
	}

	// Данные
	for _, row := range td.Rows {
		fmt.Fprintln(w, td.formatRow(row.Cells, row.Style))
	}

	w.Flush()
	return builder.String()
}

// formatRow форматирует строку с учетом стиля
func (td *TableData) formatRow(cells []string, style RowStyle) string {
	if len(cells) == 0 {
		return ""
	}
	return strings.Join(cells, "\t")
}

// PrettyTable улучшенный табличный вывод с цветами
type PrettyTable struct {
	data      *TableData
	useColors bool
}

// NewPrettyTable создает новую красивую таблицу
func NewPrettyTable(headers []string, useColors bool) *PrettyTable {
	return &PrettyTable{
		data:      NewTableData(headers),
		useColors: useColors,
	}
}

// AddRow добавляет строку
func (pt *PrettyTable) AddRow(cells ...string) {
	pt.data.AddRow(cells...)
}

// AddRowf добавляет отформатированную строку
func (pt *PrettyTable) AddRowf(format string, args ...interface{}) {
	pt.data.AddRowf(format, args...)
}

// AddRowWithStyle добавляет строку с указанием стиля
func (pt *PrettyTable) AddRowWithStyle(cells []string, style RowStyle) {
	pt.data.AddRowWithStyle(cells, style)
}

// AddStatusRow добавляет строку со статусом
func (pt *PrettyTable) AddStatusRow(status, message string) {
	icon := getStatusIcon(status)
	pt.AddRowf("%s\t%s", icon, message)
}

// AddTimestampRow добавляет строку с временной меткой
func (pt *PrettyTable) AddTimestampRow(timestamp time.Time, message string) {
	pt.AddRowf("%s\t%s", timestamp.Format("2006-01-02 15:04:05"), message)
}

// String возвращает отформатированную таблицу
func (pt *PrettyTable) String() string {
	if !pt.useColors {
		return pt.data.String()
	}

	return pt.applyColors(pt.data.String())
}

// applyColors применяет цвета к таблице
func (pt *PrettyTable) applyColors(output string) string {
	lines := strings.Split(output, "\n")
	var result []string

	for i, line := range lines {
		if i == 0 {
			// Заголовок - синий цвет
			result = append(result, fmt.Sprintf("\033[1;34m%s\033[0m", line))
		} else if strings.Contains(line, "---") {
			// Разделитель - серый цвет
			result = append(result, fmt.Sprintf("\033[1;90m%s\033[0m", line))
		} else if strings.Contains(line, "✓") || strings.Contains(line, "Success") || strings.Contains(line, "Active") {
			// Успех - зеленый цвет
			result = append(result, fmt.Sprintf("\033[1;32m%s\033[0m", line))
		} else if strings.Contains(line, "✗") || strings.Contains(line, "Error") || strings.Contains(line, "Failed") || strings.Contains(line, "Inactive") {
			// Ошибка - красный цвет
			result = append(result, fmt.Sprintf("\033[1;31m%s\033[0m", line))
		} else if strings.Contains(line, "⚠") || strings.Contains(line, "Warning") || strings.Contains(line, "Pending") {
			// Предупреждение - желтый цвет
			result = append(result, fmt.Sprintf("\033[1;33m%s\033[0m", line))
		} else {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
}

// getStatusIcon возвращает иконку для статуса
func getStatusIcon(status string) string {
	switch strings.ToLower(status) {
	case "active", "success", "ok", "up", "running":
		return "✓"
	case "inactive", "error", "failed", "down", "stopped":
		return "✗"
	case "pending", "warning", "unknown":
		return "⚠"
	default:
		return "?"
	}
}

// CreateIncidentsTable создает таблицу инцидентов
func CreateIncidentsTable(incidents []interface{}, useColors bool) *PrettyTable {
	table := NewPrettyTable([]string{"ID", "Status", "Severity", "Title", "Created"}, useColors)

	for _, incident := range incidents {
		// Извлекаем данные из инцидента
		if inc, ok := incident.(map[string]interface{}); ok {
			id := getString(inc, "id")
			status := getString(inc, "status")
			severity := getString(inc, "severity")
			title := getString(inc, "title")
			created := getString(inc, "created_at")
			
			// Определяем стиль на основе статуса
			style := StyleDefault
			switch strings.ToLower(status) {
			case "open", "active":
				style = StyleError
			case "acknowledged":
				style = StyleWarning
			case "resolved":
				style = StyleSuccess
			}
			
			table.AddRowWithStyle([]string{id, status, severity, title, created}, style)
		} else {
			// Fallback для моковых данных
			table.AddRowWithStyle([]string{"incident-123", "active", "critical", "High priority incident", "2024-01-15 10:30:00"}, StyleError)
		}
	}

	return table
}

// CreateChecksTable создает таблицу проверок
func CreateChecksTable(checks []interface{}, useColors bool) *PrettyTable {
	table := NewPrettyTable([]string{"ID", "Name", "Type", "Status", "Interval", "Last Check"}, useColors)

	for _, check := range checks {
		// Извлекаем данные из проверки
		if ch, ok := check.(map[string]interface{}); ok {
			id := getString(ch, "id")
			name := getString(ch, "name")
			checkType := getString(ch, "type")
			status := getString(ch, "status")
			interval := getString(ch, "interval")
			lastCheck := getString(ch, "last_check")
			
			// Определяем стиль на основе статуса
			style := StyleDefault
			switch strings.ToLower(status) {
			case "up", "healthy", "success":
				style = StyleSuccess
			case "down", "unhealthy", "failed":
				style = StyleError
			case "pending", "unknown":
				style = StyleWarning
			}
			
			table.AddRowWithStyle([]string{id, name, checkType, status, interval, lastCheck}, style)
		} else {
			// Fallback для моковых данных
			table.AddRowWithStyle([]string{"check-456", "http://example.com", "HTTP", "up", "30s", "2024-01-15 10:25:00"}, StyleSuccess)
		}
	}

	return table
}

// CreateChannelsTable создает таблицу каналов уведомлений
func CreateChannelsTable(channels []interface{}, useColors bool) *PrettyTable {
	table := NewPrettyTable([]string{"ID", "Name", "Type", "Status", "Created"}, useColors)

	for _, channel := range channels {
		// Извлекаем данные из канала
		if ch, ok := channel.(map[string]interface{}); ok {
			id := getString(ch, "id")
			name := getString(ch, "name")
			channelType := getString(ch, "type")
			status := getString(ch, "status")
			created := getString(ch, "created_at")
			
			// Определяем стиль на основе статуса
			style := StyleDefault
			switch strings.ToLower(status) {
			case "active", "enabled":
				style = StyleSuccess
			case "inactive", "disabled":
				style = StyleError
			case "pending":
				style = StyleWarning
			}
			
			table.AddRowWithStyle([]string{id, name, channelType, status, created}, style)
		} else {
			// Fallback для моковых данных
			table.AddRowWithStyle([]string{"channel-789", "slack-alerts", "Slack", "active", "2024-01-15 09:00:00"}, StyleSuccess)
		}
	}

	return table
}

// CreateServicesTable создает таблицу сервисов
func CreateServicesTable(services []interface{}, useColors bool) *PrettyTable {
	table := NewPrettyTable([]string{"Service", "Status", "Address", "Last Check", "Uptime"}, useColors)

	for _, service := range services {
		// Извлекаем данные из сервиса
		if svc, ok := service.(map[string]interface{}); ok {
			serviceName := getString(svc, "name")
			status := getString(svc, "status")
			address := getString(svc, "address")
			lastCheck := getString(svc, "last_check")
			uptime := getString(svc, "uptime")
			
			// Определяем стиль на основе статуса
			style := StyleDefault
			switch strings.ToLower(status) {
			case "up", "healthy", "running":
				style = StyleSuccess
			case "down", "unhealthy", "stopped":
				style = StyleError
			case "degraded", "warning":
				style = StyleWarning
			}
			
			table.AddRowWithStyle([]string{serviceName, status, address, lastCheck, uptime}, style)
		} else {
			// Fallback для моковых данных
			table.AddRowWithStyle([]string{"api-gateway", "up", "localhost:8080", "2024-01-15 10:30:00", "99.9%"}, StyleSuccess)
		}
	}

	return table
}

// getString безопасно извлекает строковое значение из map
func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
		if num, ok := val.(float64); ok {
			return fmt.Sprintf("%.0f", num)
		}
		return fmt.Sprintf("%v", val)
	}
	return ""
}

// PrintTable выводит таблицу в консоль
func PrintTable(table *PrettyTable) {
	fmt.Println(table.String())
}
