package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/notification-service/internal/models"
)

// ChannelRepository репозиторий для работы с каналами уведомлений
type ChannelRepository struct {
	db     *sql.DB
	logger logger.Logger
}

// NewChannelRepository создает новый репозиторий каналов
func NewChannelRepository(db *sql.DB, logger logger.Logger) *ChannelRepository {
	return &ChannelRepository{
		db:     db,
		logger: logger,
	}
}

// Create создает новый канал
func (r *ChannelRepository) Create(channel *models.ChannelConfig) error {
	r.logger.Info("Creating notification channel",
		logger.String("tenant_id", channel.TenantID),
		logger.String("name", channel.Name),
		logger.String("type", channel.Type))

	ctx := context.Background()

	// Сериализуем конфигурацию в JSON
	configJSON, err := json.Marshal(channel.Config)
	if err != nil {
		r.logger.Error("Failed to marshal config", logger.Error(err))
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	query := `
		INSERT INTO notification_channels (id, tenant_id, name, type, config, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			type = EXCLUDED.type,
			config = EXCLUDED.config,
			is_active = EXCLUDED.is_active,
			updated_at = EXCLUDED.updated_at
	`

	_, err = r.db.ExecContext(ctx, query,
		channel.ID,
		channel.TenantID,
		channel.Name,
		channel.Type,
		string(configJSON),
		channel.IsActive,
		time.Now(),
		time.Now(),
	)

	if err != nil {
		r.logger.Error("Failed to create channel", logger.Error(err))
		return fmt.Errorf("failed to create channel: %w", err)
	}

	r.logger.Info("Channel created successfully",
		logger.String("channel_id", channel.ID))

	return nil
}

// GetByID получает канал по ID
func (r *ChannelRepository) GetByID(tenantID, channelID string) (*models.ChannelConfig, error) {
	r.logger.Debug("Getting channel by ID",
		logger.String("tenant_id", tenantID),
		logger.String("channel_id", channelID))

	ctx := context.Background()

	query := `
		SELECT id, tenant_id, name, type, config, is_active, created_at, updated_at
		FROM notification_channels
		WHERE tenant_id = $1 AND id = $2
	`

	var channel models.ChannelConfig
	var configJSON string

	err := r.db.QueryRowContext(ctx, query, tenantID, channelID).Scan(
		&channel.ID,
		&channel.TenantID,
		&channel.Name,
		&channel.Type,
		&configJSON,
		&channel.IsActive,
		&channel.CreatedAt,
		&channel.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("channel not found")
		}
		r.logger.Error("Failed to get channel", logger.Error(err))
		return nil, fmt.Errorf("failed to get channel: %w", err)
	}

	// Десериализуем конфигурацию
	if err := json.Unmarshal([]byte(configJSON), &channel.Config); err != nil {
		r.logger.Error("Failed to unmarshal config", logger.Error(err))
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &channel, nil
}

// GetAll получает все каналы тенанта
func (r *ChannelRepository) GetAll(tenantID string, channelType string) ([]*models.ChannelConfig, error) {
	r.logger.Debug("Getting all channels",
		logger.String("tenant_id", tenantID),
		logger.String("type", channelType))

	ctx := context.Background()

	query := `
		SELECT id, tenant_id, name, type, config, is_active, created_at, updated_at
		FROM notification_channels
		WHERE tenant_id = $1
	`
	args := []interface{}{tenantID}

	if channelType != "" {
		query += " AND type = $2"
		args = append(args, channelType)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		r.logger.Error("Failed to get channels", logger.Error(err))
		return nil, fmt.Errorf("failed to get channels: %w", err)
	}
	defer rows.Close()

	var channels []*models.ChannelConfig
	for rows.Next() {
		var channel models.ChannelConfig
		var configJSON string

		err := rows.Scan(
			&channel.ID,
			&channel.TenantID,
			&channel.Name,
			&channel.Type,
			&configJSON,
			&channel.IsActive,
			&channel.CreatedAt,
			&channel.UpdatedAt,
		)
		if err != nil {
			r.logger.Error("Failed to scan channel row", logger.Error(err))
			return nil, fmt.Errorf("failed to scan channel: %w", err)
		}

		// Десериализуем конфигурацию
		if err := json.Unmarshal([]byte(configJSON), &channel.Config); err != nil {
			r.logger.Error("Failed to unmarshal config", logger.Error(err))
			return nil, fmt.Errorf("failed to unmarshal config: %w", err)
		}

		channels = append(channels, &channel)
	}

	if err = rows.Err(); err != nil {
		r.logger.Error("Error iterating channels", logger.Error(err))
		return nil, fmt.Errorf("error iterating channels: %w", err)
	}

	return channels, nil
}

// GetActive получает все активные каналы тенанта
func (r *ChannelRepository) GetActive(tenantID string, channelType string) ([]*models.ChannelConfig, error) {
	r.logger.Debug("Getting active channels",
		logger.String("tenant_id", tenantID),
		logger.String("type", channelType))

	ctx := context.Background()

	query := `
		SELECT id, tenant_id, name, type, config, is_active, created_at, updated_at
		FROM notification_channels
		WHERE tenant_id = $1 AND is_active = true
	`
	args := []interface{}{tenantID}

	if channelType != "" {
		query += " AND type = $2"
		args = append(args, channelType)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		r.logger.Error("Failed to get active channels", logger.Error(err))
		return nil, fmt.Errorf("failed to get active channels: %w", err)
	}
	defer rows.Close()

	var channels []*models.ChannelConfig
	for rows.Next() {
		var channel models.ChannelConfig
		var configJSON string

		err := rows.Scan(
			&channel.ID,
			&channel.TenantID,
			&channel.Name,
			&channel.Type,
			&configJSON,
			&channel.IsActive,
			&channel.CreatedAt,
			&channel.UpdatedAt,
		)
		if err != nil {
			r.logger.Error("Failed to scan channel row", logger.Error(err))
			return nil, fmt.Errorf("failed to scan channel: %w", err)
		}

		// Десериализуем конфигурацию
		if err := json.Unmarshal([]byte(configJSON), &channel.Config); err != nil {
			r.logger.Error("Failed to unmarshal config", logger.Error(err))
			return nil, fmt.Errorf("failed to unmarshal config: %w", err)
		}

		channels = append(channels, &channel)
	}

	if err = rows.Err(); err != nil {
		r.logger.Error("Error iterating channels", logger.Error(err))
		return nil, fmt.Errorf("error iterating channels: %w", err)
	}

	return channels, nil
}

// Update обновляет канал
func (r *ChannelRepository) Update(channel *models.ChannelConfig) error {
	r.logger.Info("Updating channel",
		logger.String("channel_id", channel.ID),
		logger.String("tenant_id", channel.TenantID))

	ctx := context.Background()

	// Сериализуем конфигурацию в JSON
	configJSON, err := json.Marshal(channel.Config)
	if err != nil {
		r.logger.Error("Failed to marshal config", logger.Error(err))
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	query := `
		UPDATE notification_channels
		SET name = $1, type = $2, config = $3, is_active = $4, updated_at = $5
		WHERE tenant_id = $6 AND id = $7
	`

	_, err = r.db.ExecContext(ctx, query,
		channel.Name,
		channel.Type,
		string(configJSON),
		channel.IsActive,
		time.Now(),
		channel.TenantID,
		channel.ID,
	)

	if err != nil {
		r.logger.Error("Failed to update channel", logger.Error(err))
		return fmt.Errorf("failed to update channel: %w", err)
	}

	r.logger.Info("Channel updated successfully",
		logger.String("channel_id", channel.ID))

	return nil
}

// Delete удаляет канал
func (r *ChannelRepository) Delete(tenantID, channelID string) error {
	r.logger.Info("Deleting channel",
		logger.String("tenant_id", tenantID),
		logger.String("channel_id", channelID))

	ctx := context.Background()

	query := `DELETE FROM notification_channels WHERE tenant_id = $1 AND id = $2`

	_, err := r.db.ExecContext(ctx, query, tenantID, channelID)
	if err != nil {
		r.logger.Error("Failed to delete channel", logger.Error(err))
		return fmt.Errorf("failed to delete channel: %w", err)
	}

	r.logger.Info("Channel deleted successfully",
		logger.String("channel_id", channelID))

	return nil
}

// UpdateConfig обновляет только конфигурацию канала
func (r *ChannelRepository) UpdateConfig(tenantID, channelID string, config map[string]interface{}) error {
	r.logger.Info("Updating channel config",
		logger.String("tenant_id", tenantID),
		logger.String("channel_id", channelID))

	ctx := context.Background()

	// Сериализуем конфигурацию в JSON
	configJSON, err := json.Marshal(config)
	if err != nil {
		r.logger.Error("Failed to marshal config", logger.Error(err))
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	query := `
		UPDATE notification_channels
		SET config = $1, updated_at = $2
		WHERE tenant_id = $3 AND id = $4
	`

	_, err = r.db.ExecContext(ctx, query,
		string(configJSON),
		time.Now(),
		tenantID,
		channelID,
	)

	if err != nil {
		r.logger.Error("Failed to update channel config", logger.Error(err))
		return fmt.Errorf("failed to update channel config: %w", err)
	}

	r.logger.Info("Channel config updated successfully",
		logger.String("channel_id", channelID))

	return nil
}

// SetActive устанавливает статус активности канала
func (r *ChannelRepository) SetActive(tenantID, channelID string, isActive bool) error {
	r.logger.Info("Setting channel active status",
		logger.String("tenant_id", tenantID),
		logger.String("channel_id", channelID),
		logger.Bool("is_active", isActive))

	ctx := context.Background()

	query := `
		UPDATE notification_channels
		SET is_active = $1, updated_at = $2
		WHERE tenant_id = $3 AND id = $4
	`

	_, err := r.db.ExecContext(ctx, query,
		isActive,
		time.Now(),
		tenantID,
		channelID,
	)

	if err != nil {
		r.logger.Error("Failed to set channel active status", logger.Error(err))
		return fmt.Errorf("failed to set channel active status: %w", err)
	}

	r.logger.Info("Channel active status updated successfully",
		logger.String("channel_id", channelID))

	return nil
}

// CreateTable создает таблицу notification_channels
func (r *ChannelRepository) CreateTable() error {
	r.logger.Info("Creating notification_channels table")

	ctx := context.Background()

	query := `
		CREATE TABLE IF NOT EXISTS notification_channels (
			id VARCHAR(255) PRIMARY KEY,
			tenant_id VARCHAR(255) NOT NULL,
			name VARCHAR(255) NOT NULL,
			type VARCHAR(50) NOT NULL,
			config JSONB NOT NULL DEFAULT '{}',
			is_active BOOLEAN DEFAULT true,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_notification_channels_tenant_id ON notification_channels(tenant_id);
		CREATE INDEX IF NOT EXISTS idx_notification_channels_type ON notification_channels(type);
		CREATE INDEX IF NOT EXISTS idx_notification_channels_active ON notification_channels(is_active);

		-- Триггер для обновления updated_at
		CREATE OR REPLACE FUNCTION update_notification_channels_updated_at()
		RETURNS TRIGGER AS $$
		BEGIN
			NEW.updated_at = CURRENT_TIMESTAMP;
			RETURN NEW;
		END;
		$$ language 'plpgsql';

		DROP TRIGGER IF EXISTS update_notification_channels_updated_at_trigger ON notification_channels;
		CREATE TRIGGER update_notification_channels_updated_at_trigger
			BEFORE UPDATE ON notification_channels
			FOR EACH ROW
			EXECUTE FUNCTION update_notification_channels_updated_at();
	`

	_, err := r.db.ExecContext(ctx, query)
	if err != nil {
		r.logger.Error("Failed to create table", logger.Error(err))
		return fmt.Errorf("failed to create table: %w", err)
	}

	r.logger.Info("Table notification_channels created successfully")
	return nil
}
