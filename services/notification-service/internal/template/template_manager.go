package template

import (
	"bytes"
	"fmt"
	"text/template"

	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/notification-service/internal/domain"
)

// TemplateManager интерфейс для управления шаблонами
type TemplateManager interface {
	RenderTemplate(templateName string, data map[string]interface{}) (string, error)
	GetSubjectTemplate(eventType string) string
	GetBodyTemplate(eventType, channel string) string
}

// DefaultTemplateManager менеджер шаблонов по умолчанию
type DefaultTemplateManager struct {
	templates map[string]*template.Template
	logger    logger.Logger
}

// NewDefaultTemplateManager создает новый менеджер шаблонов
func NewDefaultTemplateManager(logger logger.Logger) *DefaultTemplateManager {
	tm := &DefaultTemplateManager{
		templates: make(map[string]*template.Template),
		logger:    logger,
	}

	// Инициализация базовых шаблонов
	tm.initializeTemplates()

	return tm
}

// initializeTemplates инициализирует базовые шаблоны
func (tm *DefaultTemplateManager) initializeTemplates() {
	// Шаблоны тем
	subjectTemplates := map[string]string{
		domain.NotificationTypeIncidentCreated: "🔴 [INCIDENT] {{.title}}",
		domain.NotificationTypeIncidentUpdated: "🟠 [INCIDENT UPDATE] {{.title}}",
		domain.NotificationTypeIncidentResolved: "🟢 [RESOLVED] {{.title}}",
		domain.NotificationTypeCheckFailed:     "🟡 [CHECK FAILED] {{.title}}",
		domain.NotificationTypeCheckRecovered:  "✅ [RECOVERED] {{.title}}",
		domain.NotificationTypeSystemAlert:     "⚠️ [SYSTEM ALERT] {{.title}}",
	}

	// Шаблоны тел для email
	emailBodyTemplates := map[string]string{
		domain.NotificationTypeIncidentCreated + ":" + domain.ChannelEmail: `
🔴 INCIDENT DETECTED

Event: {{.notification.type}}
Severity: {{.notification.severity}}
Source: {{.notification.source}}
Time: {{.notification.timestamp}}

Message:
{{.notification.message}}

Details:
Tenant ID: {{.notification.tenant_id}}
Event ID: {{.notification.event_id}}

{{if .notification.data}}Additional Information:
{{range $key, $value := .notification.data}}
- {{$key}}: {{$value}}
{{end}}{{end}}

---
This is an automated notification from UptimePing Platform.
`,
		domain.NotificationTypeIncidentResolved + ":" + domain.ChannelEmail: `
🟢 INCIDENT RESOLVED

Event: {{.notification.type}}
Severity: {{.notification.severity}}
Source: {{.notification.source}}
Time: {{.notification.timestamp}}

Message:
{{.notification.message}}

Details:
Tenant ID: {{.notification.tenant_id}}
Event ID: {{.notification.event_id}}

{{if .notification.data}}Resolution Details:
{{range $key, $value := .notification.data}}
- {{$key}}: {{$value}}
{{end}}{{end}}

---
This is an automated notification from UptimePing Platform.
`,
	}

	// Шаблоны тел для Slack
	slackBodyTemplates := map[string]string{
		domain.NotificationTypeIncidentCreated + ":" + domain.ChannelSlack: `🔴 *INCIDENT DETECTED*

*Event:* {{.notification.type}}
*Severity:* {{.notification.severity}}
*Source:* {{.notification.source}}
*Time:* {{.notification.timestamp}}

*Message:* {{.notification.message}}

{{if .notification.data}}*Details:*
{{range $key, $value := .notification.data}}
• *{{$key}}*: {{$value}}
{{end}}{{end}}`,
		domain.NotificationTypeIncidentResolved + ":" + domain.ChannelSlack: `🟢 *INCIDENT RESOLVED*

*Event:* {{.notification.type}}
*Severity:* {{.notification.severity}}
*Source:* {{.notification.source}}
*Time:* {{.notification.timestamp}}

*Message:* {{.notification.message}}

{{if .notification.data}}*Resolution Details:*
{{range $key, $value := .notification.data}}
• *{{$key}}*: {{$value}}
{{end}}{{end}}`,
	}

	// Шаблоны тел для SMS
	smsBodyTemplates := map[string]string{
		domain.NotificationTypeIncidentCreated + ":" + domain.ChannelSMS: `INCIDENT: {{.notification.title}}. Severity: {{.notification.severity}}. {{.notification.message}}`,
		domain.NotificationTypeIncidentResolved + ":" + domain.ChannelSMS: `RESOLVED: {{.notification.title}}. {{.notification.message}}`,
	}

	// Компиляция шаблонов тем
	for name, tmpl := range subjectTemplates {
		t, err := template.New(name).Parse(tmpl)
		if err != nil {
			tm.logger.Error("Failed to parse subject template",
				logger.String("name", name),
				logger.Error(err),
			)
			continue
		}
		tm.templates["subject:"+name] = t
	}

	// Компиляция шаблонов тел для email
	for name, tmpl := range emailBodyTemplates {
		t, err := template.New(name).Parse(tmpl)
		if err != nil {
			tm.logger.Error("Failed to parse email body template",
				logger.String("name", name),
				logger.Error(err),
			)
			continue
		}
		tm.templates["body:"+name] = t
	}

	// Компиляция шаблонов тел для Slack
	for name, tmpl := range slackBodyTemplates {
		t, err := template.New(name).Parse(tmpl)
		if err != nil {
			tm.logger.Error("Failed to parse slack body template",
				logger.String("name", name),
				logger.Error(err),
			)
			continue
		}
		tm.templates["body:"+name] = t
	}

	// Компиляция шаблонов тел для SMS
	for name, tmpl := range smsBodyTemplates {
		t, err := template.New(name).Parse(tmpl)
		if err != nil {
			tm.logger.Error("Failed to parse SMS body template",
				logger.String("name", name),
				logger.Error(err),
			)
			continue
		}
		tm.templates["body:"+name] = t
	}
}

// RenderTemplate рендерит шаблон с данными
func (tm *DefaultTemplateManager) RenderTemplate(templateName string, data map[string]interface{}) (string, error) {
	tmpl, exists := tm.templates[templateName]
	if !exists {
		return "", fmt.Errorf("template not found: %s", templateName)
	}

	var buf bytes.Buffer
	err := tmpl.Execute(&buf, data)
	if err != nil {
		return "", fmt.Errorf("failed to execute template %s: %w", templateName, err)
	}

	return buf.String(), nil
}

// GetSubjectTemplate возвращает имя шаблона темы
func (tm *DefaultTemplateManager) GetSubjectTemplate(eventType string) string {
	return "subject:" + eventType
}

// GetBodyTemplate возвращает имя шаблона тела
func (tm *DefaultTemplateManager) GetBodyTemplate(eventType, channel string) string {
	return "body:" + eventType + ":" + channel
}

// AddTemplate добавляет новый шаблон
func (tm *DefaultTemplateManager) AddTemplate(name, templateStr string) error {
	t, err := template.New(name).Parse(templateStr)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	tm.templates[name] = t
	tm.logger.Info("Template added",
		logger.String("name", name),
	)

	return nil
}

// ListTemplates возвращает список всех шаблонов
func (tm *DefaultTemplateManager) ListTemplates() []string {
	var names []string
	for name := range tm.templates {
		names = append(names, name)
	}
	return names
}

// RemoveTemplate удаляет шаблон
func (tm *DefaultTemplateManager) RemoveTemplate(name string) {
	delete(tm.templates, name)
	tm.logger.Info("Template removed",
		logger.String("name", name),
	)
}

// MockTemplateManager имитация менеджера шаблонов для тестов
type MockTemplateManager struct{}

// NewMockTemplateManager создает новый mock менеджер шаблонов
func NewMockTemplateManager() *MockTemplateManager {
	return &MockTemplateManager{}
}

// RenderTemplate имитирует рендеринг шаблона
func (m *MockTemplateManager) RenderTemplate(templateName string, data map[string]interface{}) (string, error) {
	// Простая имитация рендеринга
	switch templateName {
	case "subject:incident.created":
		return "🔴 [INCIDENT] Test Incident", nil
	case "body:incident.created:email":
		return "🔴 INCIDENT DETECTED\n\nMessage: Test message", nil
	default:
		return fmt.Sprintf("Mock template: %s", templateName), nil
	}
}

// GetSubjectTemplate возвращает имя шаблона темы
func (m *MockTemplateManager) GetSubjectTemplate(eventType string) string {
	return "subject:" + eventType
}

// GetBodyTemplate возвращает имя шаблона тела
func (m *MockTemplateManager) GetBodyTemplate(eventType, channel string) string {
	return "body:" + eventType + ":" + channel
}
