// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

const tableNameWebhookDelivery = "webhook_delivery"

// POWebhookDelivery 手写 GORM PO(缺 gorm.io/gen 迁移前的桥接);列定义对齐
// test_case 26 IDL 14 字段 + created_at/updated_at + uk_delivery_id 幂等索引。
type POWebhookDelivery struct {
	ID               int64      `gorm:"column:id;primaryKey;autoIncrement"`
	DeliveryID       string     `gorm:"column:delivery_id;type:varchar(64);not null;uniqueIndex:uk_delivery_id"`
	WorkspaceID      int64      `gorm:"column:workspace_id;not null;index:idx_status_next_retry,priority:1"`
	ExperimentID     int64      `gorm:"column:experiment_id;not null;index:idx_experiment_id_created_at,priority:1"`
	Event            int32      `gorm:"column:event;not null"`
	URL              string     `gorm:"column:url;type:varchar(1024);not null"`
	Status           int32      `gorm:"column:status;not null;index:idx_status_next_retry,priority:2"`
	AttemptCount     int32      `gorm:"column:attempt_count;not null;default:0"`
	LastResponseCode int32      `gorm:"column:last_response_code;not null;default:0"`
	LastError        string     `gorm:"column:last_error;type:varchar(512);not null;default:''"`
	RequestBody      string     `gorm:"column:request_body;type:mediumtext"`
	FirstSentAt      *time.Time `gorm:"column:first_sent_at"`
	LastSentAt       *time.Time `gorm:"column:last_sent_at"`
	NextRetryAt      *time.Time `gorm:"column:next_retry_at;index:idx_status_next_retry,priority:3"`
	CreatedAt        time.Time  `gorm:"column:created_at;not null;autoCreateTime;index:idx_experiment_id_created_at,priority:2"`
	UpdatedAt        time.Time  `gorm:"column:updated_at;not null;autoUpdateTime"`
}

// TableName 显式定 表名,避免 GORM 复数化推断。
func (*POWebhookDelivery) TableName() string { return tableNameWebhookDelivery }

// WebhookDeliveryRepoImpl 落 repo.IWebhookDeliveryRepo;当前仅实现 Create 幂等入口,
// GetByDeliveryID / UpdateStatus / ListByExperimentID 拆到后续 attempt。
type WebhookDeliveryRepoImpl struct {
	provider db.Provider
}

// NewWebhookDeliveryRepo dev_orchestrator 侧 Wire DI 挂点。
func NewWebhookDeliveryRepo(provider db.Provider) repo.IWebhookDeliveryRepo {
	return &WebhookDeliveryRepoImpl{provider: provider}
}

// Create 首投前落 pending 行;uk_delivery_id 冲突按 test_case 27 幂等短路(不覆盖既有行状态)。
func (r *WebhookDeliveryRepoImpl) Create(ctx context.Context, delivery *entity.WebhookDelivery) error {
	po := entityToPOWebhookDelivery(delivery)
	if err := r.provider.NewSession(ctx, db.WithMaster()).WithContext(ctx).Create(po).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil
		}
		return errorx.Wrapf(err, "create webhook_delivery fail, po: %v", json.Jsonify(po))
	}
	delivery.ID = po.ID
	return nil
}

// GetByDeliveryID 待下轮 GORM Where+First 落地,当前 stub 返 not-implemented 让 iface 满足。
func (r *WebhookDeliveryRepoImpl) GetByDeliveryID(ctx context.Context, deliveryID string) (*entity.WebhookDelivery, error) {
	return nil, errors.New("webhook_delivery.GetByDeliveryID not implemented yet")
}

// UpdateStatus 待下轮 GORM Updates+where 落地。
func (r *WebhookDeliveryRepoImpl) UpdateStatus(ctx context.Context, req *repo.UpdateWebhookDeliveryStatusRequest) error {
	return errors.New("webhook_delivery.UpdateStatus not implemented yet")
}

// ListByExperimentID 待下轮 分页倒序 + page_token 落地。
func (r *WebhookDeliveryRepoImpl) ListByExperimentID(ctx context.Context, experimentID int64, pageSize int32, pageToken string) ([]*entity.WebhookDelivery, string, error) {
	return nil, "", errors.New("webhook_delivery.ListByExperimentID not implemented yet")
}

func entityToPOWebhookDelivery(e *entity.WebhookDelivery) *POWebhookDelivery {
	return &POWebhookDelivery{
		ID: e.ID, DeliveryID: e.DeliveryID, WorkspaceID: e.WorkspaceID, ExperimentID: e.ExperimentID,
		Event: int32(e.Event), URL: e.URL, Status: int32(e.Status), AttemptCount: e.AttemptCount,
		LastResponseCode: e.LastResponseCode, LastError: e.LastError, RequestBody: e.RequestBody,
		FirstSentAt: e.FirstSentAt, LastSentAt: e.LastSentAt, NextRetryAt: e.NextRetryAt,
	}
}
