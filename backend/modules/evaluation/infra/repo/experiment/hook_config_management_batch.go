// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
)

var _ repo.IHookConfigBatchReader = (*hookConfigRepo)(nil)

func (r *hookConfigRepo) MGetConfigs(ctx context.Context, owners []hookcomponent.ConfigOwner) ([]repo.HookConfigReadResult, error) {
	if len(owners) == 0 {
		return []repo.HookConfigReadResult{}, nil
	}
	if len(owners) > repo.MaxHookConfigBatchSize || r == nil || r.provider == nil || r.codec == nil {
		return nil, entity.ErrHookConfigStorage
	}
	ids := make([]int64, len(owners))
	wanted := make(map[int64]bool, len(owners))
	for i, owner := range owners {
		if !validHookConfigOwner(owner) || owner.Kind != owners[0].Kind || owner.WorkspaceID != owners[0].WorkspaceID || owner.ExecutionScope != owners[0].ExecutionScope || wanted[owner.ObjectID] {
			return nil, entity.ErrHookConfigStorage
		}
		ids[i], wanted[owner.ObjectID] = owner.ObjectID, true
	}
	type row struct {
		ID, SpaceID       int64
		LifecycleHookConf []byte
	}
	var rows []row
	err := r.provider.NewSession(ctx, db.WithMaster()).Session(&gorm.Session{Logger: logger.Discard}).
		Table(hookConfigTable(owners[0])).Select("id", "space_id", "lifecycle_hook_conf").
		Where("space_id=? AND id IN ? AND deleted_at IS NULL", owners[0].WorkspaceID, ids).Limit(len(ids)).Find(&rows).Error
	if err != nil {
		return nil, entity.ErrHookConfigStorage
	}
	byID := make(map[int64]row, len(rows))
	for _, v := range rows {
		_, duplicate := byID[v.ID]
		if !wanted[v.ID] || duplicate || v.SpaceID != owners[0].WorkspaceID {
			return nil, entity.ErrHookConfigStorage
		}
		byID[v.ID] = v
	}
	// Find has closed the SQL rows; crypto providers never run under a DB lock/transaction.
	results := make([]repo.HookConfigReadResult, len(owners))
	for i, owner := range owners {
		results[i].Owner = owner
		v, exists := byID[owner.ObjectID]
		if !exists {
			results[i].Err = entity.ErrHookStoreMissing
			continue
		}
		conf, err := r.codec.DecodeConfig(ctx, owner, v.LifecycleHookConf)
		if err != nil {
			results[i].Err = entity.ErrHookConfigStorage
			continue
		}
		results[i].Record = &entity.HookConfigRecord{Config: conf, Revision: hookConfigRevision(v.LifecycleHookConf)}
	}
	return results, nil
}
