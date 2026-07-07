// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

const defaultWebhookDeliveryPageSize int32 = 20

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

// GetByDeliveryID 供 retry consumer / OApi 明细回显；delivery_id 命中 uk_delivery_id 索引。
func (r *WebhookDeliveryRepoImpl) GetByDeliveryID(ctx context.Context, deliveryID string) (*entity.WebhookDelivery, error) {
	var po POWebhookDelivery
	if err := r.provider.NewSession(ctx).WithContext(ctx).Where("delivery_id = ?", deliveryID).First(&po).Error; err != nil {
		return nil, errorx.Wrapf(err, "get webhook_delivery fail, delivery_id: %s", deliveryID)
	}
	return poToEntityWebhookDelivery(&po), nil
}

// UpdateStatus 选择性更新 7 个字段 + first_sent_at COALESCE 保底（首投时间只落一次）。
func (r *WebhookDeliveryRepoImpl) UpdateStatus(ctx context.Context, req *repo.UpdateWebhookDeliveryStatusRequest) error {
	ufields := map[string]any{
		"status":             int32(req.Status),
		"attempt_count":      req.AttemptCount,
		"last_response_code": req.LastResponseCode,
		"last_error":         req.LastError,
		"last_sent_at":       req.LastSentAt,
		"next_retry_at":      req.NextRetryAt,
	}
	if req.LastSentAt != nil {
		ufields["first_sent_at"] = gorm.Expr("COALESCE(first_sent_at, ?)", req.LastSentAt)
	}
	if err := r.provider.NewSession(ctx, db.WithMaster()).WithContext(ctx).
		Model(&POWebhookDelivery{}).Where("delivery_id = ?", req.DeliveryID).Updates(ufields).Error; err != nil {
		return errorx.Wrapf(err, "update webhook_delivery status fail, req: %v", json.Jsonify(req))
	}
	return nil
}

// ListByExperimentID 分页倒序，page_token 为 base64("created_at_ns|id") 游标（对齐 test_case 26）。
func (r *WebhookDeliveryRepoImpl) ListByExperimentID(ctx context.Context, experimentID int64, pageSize int32, pageToken string) ([]*entity.WebhookDelivery, string, error) {
	if pageSize <= 0 {
		pageSize = defaultWebhookDeliveryPageSize
	}
	stmt := r.provider.NewSession(ctx).WithContext(ctx).Model(&POWebhookDelivery{}).
		Where("experiment_id = ?", experimentID)
	if pageToken != "" {
		createdAtNs, id, err := decodeWebhookDeliveryPageToken(pageToken)
		if err != nil {
			return nil, "", errorx.Wrapf(err, "invalid page_token: %s", pageToken)
		}
		cursor := time.Unix(0, createdAtNs)
		stmt = stmt.Where("(created_at < ? OR (created_at = ? AND id < ?))", cursor, cursor, id)
	}
	var pos []*POWebhookDelivery
	if err := stmt.Order("created_at DESC").Order("id DESC").Limit(int(pageSize)).Find(&pos).Error; err != nil {
		return nil, "", errorx.Wrapf(err, "list webhook_delivery fail, experiment_id: %d", experimentID)
	}
	deliveries := make([]*entity.WebhookDelivery, 0, len(pos))
	for _, po := range pos {
		deliveries = append(deliveries, poToEntityWebhookDelivery(po))
	}
	var next string
	if int32(len(pos)) == pageSize && len(pos) > 0 {
		last := pos[len(pos)-1]
		next = encodeWebhookDeliveryPageToken(last.CreatedAt.UnixNano(), last.ID)
	}
	return deliveries, next, nil
}

func entityToPOWebhookDelivery(e *entity.WebhookDelivery) *POWebhookDelivery {
	return &POWebhookDelivery{
		ID: e.ID, DeliveryID: e.DeliveryID, WorkspaceID: e.WorkspaceID, ExperimentID: e.ExperimentID,
		Event: int32(e.Event), URL: e.URL, Status: int32(e.Status), AttemptCount: e.AttemptCount,
		LastResponseCode: e.LastResponseCode, LastError: e.LastError, RequestBody: e.RequestBody,
		FirstSentAt: e.FirstSentAt, LastSentAt: e.LastSentAt, NextRetryAt: e.NextRetryAt,
	}
}

func poToEntityWebhookDelivery(po *POWebhookDelivery) *entity.WebhookDelivery {
	return &entity.WebhookDelivery{
		ID: po.ID, DeliveryID: po.DeliveryID, WorkspaceID: po.WorkspaceID, ExperimentID: po.ExperimentID,
		Event: entity.NotificationTrigger(po.Event), URL: po.URL, Status: entity.DeliveryStatus(po.Status),
		AttemptCount: po.AttemptCount, LastResponseCode: po.LastResponseCode, LastError: po.LastError,
		RequestBody: po.RequestBody, FirstSentAt: po.FirstSentAt, LastSentAt: po.LastSentAt, NextRetryAt: po.NextRetryAt,
	}
}

func encodeWebhookDeliveryPageToken(createdAtNs, id int64) string {
	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%d|%d", createdAtNs, id)))
}

func decodeWebhookDeliveryPageToken(token string) (int64, int64, error) {
	raw, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return 0, 0, err
	}
	var createdAtNs, id int64
	if _, err := fmt.Sscanf(string(raw), "%d|%d", &createdAtNs, &id); err != nil {
		return 0, 0, errors.New("webhook_delivery page_token malformed")
	}
	return createdAtNs, id, nil
}
