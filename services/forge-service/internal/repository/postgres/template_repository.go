package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/forge-service/internal/domain"
	"github.com/lib/pq"
)

// TemplateRepository PostgreSQL репозиторий для работы с шаблонами
type TemplateRepository struct {
	db     *sql.DB
	logger logger.Logger
}

// NewTemplateRepository создает новый PostgreSQL репозиторий шаблонов
func NewTemplateRepository(db *sql.DB, logger logger.Logger) *TemplateRepository {
	return &TemplateRepository{
		db:     db,
		logger: logger,
	}
}

// Create создает новый шаблон
func (r *TemplateRepository) Create(ctx context.Context, template *domain.Template) error {
	query := `
		INSERT INTO proto_templates (id, tenant_id, name, type, content, description, version, tags, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			type = EXCLUDED.type,
			content = EXCLUDED.content,
			description = EXCLUDED.description,
			version = EXCLUDED.version,
			tags = EXCLUDED.tags,
			updated_at = EXCLUDED.updated_at
	`

	tagsJSON, _ := json.Marshal(template.Tags)
	now := time.Now()
	_, err := r.db.ExecContext(ctx, query,
		template.ID,
		template.TenantID,
		template.Name,
		template.Type,
		template.Content,
		template.Description,
		template.Version,
		tagsJSON,
		now,
		now,
	)

	if err != nil {
		r.logger.Error("Failed to create template",
			logger.String("template_id", template.ID),
			logger.String("tenant_id", template.TenantID),
			logger.Error(err),
		)
		return err
	}

	r.logger.Info("Template created successfully",
		logger.String("template_id", template.ID),
		logger.String("tenant_id", template.TenantID),
		logger.String("name", template.Name),
	)

	return nil
}

// GetByID получает шаблон по ID
func (r *TemplateRepository) GetByID(ctx context.Context, id string, tenantID string) (*domain.Template, error) {
	query := `
		SELECT id, tenant_id, name, type, content, description, version, tags, created_at, updated_at
		FROM proto_templates
		WHERE id = $1 AND tenant_id = $2
	`

	var template domain.Template
	var tagsJSON []string
	err := r.db.QueryRowContext(ctx, query, id, tenantID).Scan(
		&template.ID,
		&template.TenantID,
		&template.Name,
		&template.Type,
		&template.Content,
		&template.Description,
		&template.Version,
		&tagsJSON,
		&template.CreatedAt,
		&template.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		r.logger.Error("Failed to get template by ID",
			logger.String("template_id", id),
			logger.String("tenant_id", tenantID),
			logger.Error(err),
		)
		return nil, err
	}

	// Десериализация JSON тегов
	if len(tagsJSON) > 0 && tagsJSON[0] != "" {
		if err := json.Unmarshal([]byte(tagsJSON[0]), &template.Tags); err != nil {
			r.logger.Error("Failed to unmarshal template tags",
				logger.String("template_id", id),
				logger.Error(err),
			)
		}
	}

	return &template, nil
}

// GetByTenantID получает все шаблоны для tenant
func (r *TemplateRepository) GetByTenantID(ctx context.Context, tenantID string) ([]*domain.Template, error) {
	query := `
		SELECT id, tenant_id, name, type, content, description, version, tags, created_at, updated_at
		FROM proto_templates
		WHERE tenant_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, tenantID)
	if err != nil {
		r.logger.Error("Failed to get templates by tenant ID",
			logger.String("tenant_id", tenantID),
			logger.Error(err),
		)
		return nil, err
	}
	defer rows.Close()

	var templates []*domain.Template
	for rows.Next() {
		var template domain.Template
		var tagsJSON []string
		err := rows.Scan(
			&template.ID,
			&template.TenantID,
			&template.Name,
			&template.Type,
			&template.Content,
			&template.Description,
			&template.Version,
			&tagsJSON,
			&template.CreatedAt,
			&template.UpdatedAt,
		)
		if err != nil {
			r.logger.Error("Failed to scan template row",
				logger.Error(err),
			)
			return nil, err
		}

		// Десериализация JSON тегов
		if len(tagsJSON) > 0 && tagsJSON[0] != "" {
			if err := json.Unmarshal([]byte(tagsJSON[0]), &template.Tags); err != nil {
				r.logger.Error("Failed to unmarshal template tags",
					logger.String("template_id", template.ID),
					logger.Error(err),
				)
			}
		}

		templates = append(templates, &template)
	}

	return templates, nil
}

// GetByType получает шаблоны по типу
func (r *TemplateRepository) GetByType(ctx context.Context, tenantID, templateType string) ([]*domain.Template, error) {
	query := `
		SELECT id, tenant_id, name, type, content, description, version, tags, created_at, updated_at
		FROM proto_templates
		WHERE tenant_id = $1 AND type = $2
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, tenantID, templateType)
	if err != nil {
		r.logger.Error("Failed to get templates by type",
			logger.String("tenant_id", tenantID),
			logger.String("type", templateType),
			logger.Error(err),
		)
		return nil, err
	}
	defer rows.Close()

	var templates []*domain.Template
	for rows.Next() {
		var template domain.Template
		var tagsJSON []string
		err := rows.Scan(
			&template.ID,
			&template.TenantID,
			&template.Name,
			&template.Type,
			&template.Content,
			&template.Description,
			&template.Version,
			&tagsJSON,
			&template.CreatedAt,
			&template.UpdatedAt,
		)
		if err != nil {
			r.logger.Error("Failed to scan template row",
				logger.Error(err),
			)
			return nil, err
		}

		// Десериализация JSON тегов
		if len(tagsJSON) > 0 && tagsJSON[0] != "" {
			if err := json.Unmarshal([]byte(tagsJSON[0]), &template.Tags); err != nil {
				r.logger.Error("Failed to unmarshal template tags",
					logger.String("template_id", template.ID),
					logger.Error(err),
				)
			}
		}

		templates = append(templates, &template)
	}

	return templates, nil
}

// Update обновляет шаблон
func (r *TemplateRepository) Update(ctx context.Context, template *domain.Template) error {
	query := `
		UPDATE proto_templates
		SET name = $2, type = $3, content = $4, description = $5, version = $6, tags = $7, updated_at = $8
		WHERE id = $1 AND tenant_id = $9
	`

	tagsJSON, _ := json.Marshal(template.Tags)
	now := time.Now()
	_, err := r.db.ExecContext(ctx, query,
		template.ID,
		template.Name,
		template.Type,
		template.Content,
		template.Description,
		template.Version,
		tagsJSON,
		now,
		template.TenantID,
	)

	if err != nil {
		r.logger.Error("Failed to update template",
			logger.String("template_id", template.ID),
			logger.String("tenant_id", template.TenantID),
			logger.Error(err),
		)
		return err
	}

	r.logger.Info("Template updated successfully",
		logger.String("template_id", template.ID),
		logger.String("tenant_id", template.TenantID),
		logger.String("name", template.Name),
	)

	return nil
}

// Delete удаляет шаблон
func (r *TemplateRepository) Delete(ctx context.Context, id string, tenantID string) error {
	query := `DELETE FROM proto_templates WHERE id = $1 AND tenant_id = $2`

	_, err := r.db.ExecContext(ctx, query, id, tenantID)
	if err != nil {
		r.logger.Error("Failed to delete template",
			logger.String("template_id", id),
			logger.String("tenant_id", tenantID),
			logger.Error(err),
		)
		return err
	}

	r.logger.Info("Template deleted successfully",
		logger.String("template_id", id),
		logger.String("tenant_id", tenantID),
	)

	return nil
}
