package api

import "time"

// GenerateRequest представляет запрос на генерацию кода
type GenerateRequest struct {
	Input    string `json:"input"`
	Output   string `json:"output"`
	Template string `json:"template"`
	Language string `json:"language"`
	Watch    bool   `json:"watch"`
	Config   string `json:"config"`
}

// GenerateResponse представляет ответ на генерацию кода
type GenerateResponse struct {
	GeneratedFiles int       `json:"generated_files"`
	OutputPath     string    `json:"output_path"`
	GenerationTime time.Time `json:"generation_time"`
	Files          []string  `json:"files"`
}

// ValidateRequest представляет запрос на валидацию
type ValidateRequest struct {
	Input     string `json:"input"`
	ProtoPath string `json:"proto_path"`
	Lint      bool   `json:"lint"`
	Breaking  bool   `json:"breaking"`
}

// ValidationError представляет ошибку валидации
type ValidationError struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Message string `json:"message"`
}

// ValidationWarning представляет предупреждение валидации
type ValidationWarning struct {
	File    string `json:"file"`
	Message string `json:"message"`
}

// ValidateResponse представляет ответ на валидацию
type ValidateResponse struct {
	Status         string              `json:"status"`
	Valid          bool                `json:"valid"`
	FilesChecked   int                 `json:"files_checked"`
	Errors         []ValidationError   `json:"errors"`
	Warnings       []ValidationWarning `json:"warnings"`
	ValidationTime time.Time           `json:"validation_time"`
}

// InteractiveConfigRequest представляет запрос на интерактивную настройку
type InteractiveConfigRequest struct {
	ProtoFile string            `json:"proto_file"`
	Template  string            `json:"template"`
	Options   map[string]string `json:"options"`
}

// InteractiveConfigResponse представляет ответ интерактивной настройки
type InteractiveConfigResponse struct {
	Config   map[string]interface{} `json:"config"`
	Template string                 `json:"template"`
	Ready    bool                   `json:"ready"`
}

// GetTemplatesRequest представляет запрос на получение шаблонов
type GetTemplatesRequest struct {
	Type     string `json:"type"`     // http, grpc, tcp, etc.
	Language string `json:"language"` // go, java, python, etc.
}

// TemplateInfo представляет информацию о шаблоне
type TemplateInfo struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Language    string            `json:"language"`
	Description string            `json:"description"`
	Parameters  map[string]string `json:"parameters"`
	Example     string            `json:"example"`
}

// GetTemplatesResponse представляет ответ со списком шаблонов
type GetTemplatesResponse struct {
	Templates []TemplateInfo `json:"templates"`
	Total     int            `json:"total"`
}
