// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

// Package mysql / hand-written overlay PO for experiment.notifications.
//
// SchemaChange DDL (owner: 独立 SchemaChange 工单, 不在本 pipeline scope):
//
//   CREATE TABLE `experiment_notifications` (
//     `experiment_id` BIGINT NOT NULL,
//     `notifications` JSON NOT NULL,
//     `created_at`    DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
//     `updated_at`    DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
//                                          ON UPDATE CURRENT_TIMESTAMP(3),
//     PRIMARY KEY (`experiment_id`),
//     UNIQUE KEY `uk_experiment_id` (`experiment_id`)
//   ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
//
// 采用独立 overlay 表 (而不是在 experiment 表加列) 的原因:
//   - experiment 表 PO 由 gorm.io/gen 生成 (// DO NOT EDIT header), 禁手改;
//   - overlay 表以 experiment_id 为 PK 直接幂等 upsert, gorm.ErrDuplicatedKey 由 Repo 侧短路;
//   - nil Notifications 时不写行 (test_case 4 显式禁用 = []; test_case 6 老实验 = NULL).

package mysql

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	pkgjson "github.com/coze-dev/coze-loop/backend/pkg/json"
)

const tableNameExperimentNotifications = "experiment_notifications"

// POExperimentNotifications overlay PO — 以 experiment_id 为主键幂等落 notifications JSON.
type POExperimentNotifications struct {
	ExperimentID  int64     `gorm:"column:experiment_id;primaryKey;uniqueIndex:uk_experiment_id"`
	Notifications []byte    `gorm:"column:notifications;type:json;not null"`
	CreatedAt     time.Time `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt     time.Time `gorm:"column:updated_at;not null;autoUpdateTime"`
}

// TableName 显式定表名, 避免 GORM 复数化推断.
func (*POExperimentNotifications) TableName() string { return tableNameExperimentNotifications }

// POToNotifications 反序列化 overlay 行的 JSON 列到 entity 数组; nil / 空字节返 nil.
func POToNotifications(po *POExperimentNotifications) ([]entity.NotificationRule, error) {
	if po == nil || len(po.Notifications) == 0 {
		return nil, nil
	}
	var rules []entity.NotificationRule
	if err := json.Unmarshal(po.Notifications, &rules); err != nil {
		return nil, errorx.Wrapf(err, "unmarshal experiment_notifications fail, expt_id: %d", po.ExperimentID)
	}
	return rules, nil
}

// NotificationsToPO 序列化 rules 到 overlay PO; nil rules → 返 (nil, nil) 让上游跳过写入.
func NotificationsToPO(exptID int64, rules []entity.NotificationRule) (*POExperimentNotifications, error) {
	if rules == nil {
		return nil, nil
	}
	raw, err := json.Marshal(rules)
	if err != nil {
		return nil, errorx.Wrapf(err, "marshal experiment_notifications fail, expt_id: %d", exptID)
	}
	return &POExperimentNotifications{ExperimentID: exptID, Notifications: raw}, nil
}

// INotificationsOverlayDAO overlay 表访问接口; 供 experiment repo 在 Create/Update/Get 中调用.
type INotificationsOverlayDAO interface {
	Upsert(ctx context.Context, exptID int64, rules []entity.NotificationRule) error
	Get(ctx context.Context, exptID int64) ([]entity.NotificationRule, error)
}

type notificationsOverlayDAOImpl struct {
	provider db.Provider
}

// NewNotificationsOverlayDAO dev_orchestrator 侧 Wire DI 挂点.
func NewNotificationsOverlayDAO(provider db.Provider) INotificationsOverlayDAO {
	return &notificationsOverlayDAOImpl{provider: provider}
}

// Upsert 幂等写 overlay 行: nil rules 跳过 (respects test_case 4 显式禁用 / test_case 6 老实验 NULL).
// experiment_id 冲突时按 test_case 27 幂等语义, 走 ON DUPLICATE KEY UPDATE notifications=VALUES(notifications).
func (d *notificationsOverlayDAOImpl) Upsert(ctx context.Context, exptID int64, rules []entity.NotificationRule) error {
	po, err := NotificationsToPO(exptID, rules)
	if err != nil {
		return err
	}
	if po == nil {
		return nil
	}
	err = d.provider.NewSession(ctx, db.WithMaster()).WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "experiment_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"notifications", "updated_at"}),
		}).
		Create(po).Error
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil
		}
		return errorx.Wrapf(err, "upsert experiment_notifications fail, po: %v", pkgjson.Jsonify(po))
	}
	return nil
}

// Get 读取 overlay 行; 行缺失返 (nil, nil), 非幂等错误透传.
func (d *notificationsOverlayDAOImpl) Get(ctx context.Context, exptID int64) ([]entity.NotificationRule, error) {
	var po POExperimentNotifications
	err := d.provider.NewSession(ctx).WithContext(ctx).
		Where("experiment_id = ?", exptID).First(&po).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, errorx.Wrapf(err, "get experiment_notifications fail, expt_id: %d", exptID)
	}
	return POToNotifications(&po)
}
