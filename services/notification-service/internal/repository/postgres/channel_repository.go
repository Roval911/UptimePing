package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/notification-service/internal/models"
	"github.com/lib/pq"
)

// ChannelRepository PostgreSQL репозиторий для работы с каналами уведомлений
type ChannelRepository struct {
	db     *sql.DB
	logger logger.Logger
}

// NewChannelRepository создает новый PostgreSQL репозиторий каналов
func NewChannelRepository(db *sql.DB, logger logger.Logger) *ChannelRepository {
	return &ChannelRepository{
		db:     db,
		logger: logger,
	}
}

// Create создает новый канал
func (r *ChannelRepository) Create(ctx context.Context, channel *models.ChannelConfig) error {
	query := `
		INSERT INTO notification_channels (id, tenant_id, name, type, config, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			type = EXCLUDED.type,
			config = EXCLUDED.config,
			enabled = EXCLUDED.enabled,
			updated_at = EXCLUDED.updated_at
	`

	now := time.Now()
	_, err := r.db.ExecContext(ctx, query,
		channel.ID,
		channel.TenantID,
		channel.Name,
		channel.Type,
		channel.Config,
		channel.Enabled,
		now,
		now,
	)

	if err != nil {
		r.logger.Error("Failed to create notification channel",
			logger.String("channel_id", channel.ID),
			logger.String("tenant_id", channel.TenantID),
			logger.Error(err),
		)
		return err
	}

	r.logger.Info("Notification channel created successfully",
		logger.String("channel_id", channel.ID),
		logger.String("tenant_id", channel.TenantID),
		logger.String("name", channel.Name),
	)

	return nil
}

// GetByID получает канал по ID
func (r *ChannelRepository) GetByID(ctx context.Context, id string, tenantID string) (*models.ChannelConfig, error) {
	query := `
		SELECT id, tenant_id, name, type, config, enabled, created_at, updated_at
		FROM notification_channels
		WHERE id = $1 AND tenant_id = $2
	`

	var channel models.ChannelConfig
	var configJSON []string
	err := r.db.QueryRowContext(ctx, query, id, tenantID).Scan(
		&channel.ID,
		&channel.TenantID,
		&channel.Name,
		&channel.Type,
		&configJSON,
		&channel.Enabled,
		&channel.CreatedAt,
		&channel.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		r.logger.Error("Failed to get notification channel by ID",
			logger.String("channel_id", id),
			logger.String("tenant_id", tenantID),
			logger.Error(err),
		)
		return nil, err
	}

	// Десериализация JSON конфигурации
	if len(configJSON) > 0 && configJSON[0] != "" {
		if err := json.Unmarshal([]byte(configJSON[0]), &channel.Config); err != nil {
			r.logger.Error("Failed to unmarshal channel config",
				logger.String("channel_id", id),
				logger.Error(err),
			)
		}
	}

	return &channel, nil
}

// GetByTenantID получает все каналы для tenant
func (r *ChannelRepository) GetByTenantID(ctx context.Context, tenantID string) ([]*models.ChannelConfig, error) {
	query := `
		SELECT id, tenant_id, name, type, config, enabled, created_at, updated_at
		FROM notification_channels
		WHERE tenant_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, tenantID)
	if err != nil {
		r.logger.Error("Failed to get notification channels by tenant ID",
			logger.String("tenant_id", tenantID),
			logger.Error(err),
		)
		return nil, err
	}
	defer rows.Close()

	var channels []*models.ChannelConfig
	for rows.Next() {
		var channel models.ChannelConfig
		var configJSON []string
		err := rows.Scan(
			&channel.ID,
			&channel.TenantID,
			&channel.Name,
			&channel.Type,
			&configJSON,
			&channel.Enabled,
			&channel.CreatedAt,
			&channel.UpdatedAt,
		)
		if err != nil {
			r.logger.Error("Failed to scan notification channel row",
				logger.Error(err),
			)
			return nil, err
		}

		// Десериализация JSON конфигурации
		if len(configJSON) > 0 && configJSON[0] != "" {
			if err := json.Unmarshal([]byte(configJSON[0]), &channel.Config); err != nil {
				r.logger.Error("Failed to unmarshal channel config",
					logger.String("channel_id", channel.ID),
					logger.Error(err),
				)
			}
		}

		channels = append(channels, &channel)
	}

	return channels, nil
}

// Update обновляет канал
func (r *ChannelRepository) Update(ctx context.Context, channel *models.ChannelConfig) error {
	query := `
		UPDATE notification_channels
		SET name = $2, type = $3, config = $4, enabled = $5, updated_at = $6
		WHERE id = $1 AND tenant_id = $7
	`

	now := time.Now()
	_, err := r.db.ExecContext(ctx, query,
		channel.ID,
		channel.Name,
		channel.Type,
		channel.Config,
		channel.Enabled,
		now,
		channel.TenantID,
	)

	if err != nil {
		r.logger.Error("Failed to update notification channel",
			logger.String("channel_id", channel.ID),
			logger.String("tenant_id", channel.TenantID),
			logger.Error(err),
		)
		return err
	}

	r.logger.Info("Notification channel updated successfully",
		logger.String("channel_id", channel.ID),
		logger.String("tenant_id", channel.TenantID),
		logger.String("name", channel.Name),
	)

	return nil
}

// Delete удаляет канал
func (r *ChannelRepository) Delete(ctx context.Context, id string, tenantID string) error {
	query := `DELETE FROM notification_channels WHERE id = $1 AND tenant_id = $2`

	_, err := r.db.ExecContext(ctx, query, id, tenantID)
	if err != nil {
		r.logger.Error("Failed to delete notification channel",
			logger.String("channel_id", id),
			logger.String("tenant_id", tenantID),
			logger.Error(err),
		)
		return err
	}

	r.logger.Info("Notification channel deleted successfully",
		logger.String("channel_id", id),
		logger.String("tenant_id", tenantID),
	)

	return nil
}
