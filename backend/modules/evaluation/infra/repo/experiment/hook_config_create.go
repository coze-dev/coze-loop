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
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
)

var _ repo.IHookConfigCreator = (*hookConfigRepo)(nil)

func (r *hookConfigRepo) CreateExperimentWithHookConfig(ctx context.Context, expt *entity.Experiment, refs []*entity.ExptEvaluatorRef, in repo.HookConfigCreateInput) error {
	if expt == nil || expt.SpaceID != in.WorkspaceID {
		return entity.ErrHookConfigStorage
	}
	seen := make(map[int64]bool, len(refs))
	for _, ref := range refs {
		if ref == nil || ref.ID <= 0 || seen[ref.ID] || ref.SpaceID != expt.SpaceID || ref.ExptID != expt.ID {
			return entity.ErrHookConfigStorage
		}
		seen[ref.ID] = true
	}
	owner := hookcomponent.ConfigOwner{WorkspaceID: in.WorkspaceID, ObjectID: expt.ID, Kind: hookcomponent.ConfigOwnerExperiment, ExecutionScope: in.ExecutionScope}
	encoded, err := r.encodeCreateConfig(ctx, owner, in)
	if err != nil {
		return err
	}
	po, err := convert.NewExptConverter().DO2PO(expt)
	if err != nil {
		return entity.ErrHookConfigStorage
	}
	po.LifecycleHookConf = &encoded
	refPOs := convert.NewExptEvaluatorRefConverter().DO2PO(refs)
	err = r.provider.Transaction(ctx, func(tx *gorm.DB) error {
		tx = tx.Session(&gorm.Session{Logger: logger.Discard})
		if err := tx.Create(po).Error; err != nil {
			return err
		}
		if len(refPOs) > 0 {
			return tx.Create(refPOs).Error
		}
		return nil
	}, db.WithMaster())
	if err != nil {
		return entity.ErrHookConfigStorage
	}
	return nil
}

func (r *hookConfigRepo) CreateTemplateWithHookConfig(ctx context.Context, template *entity.ExptTemplate, refs []*entity.ExptTemplateEvaluatorRef, in repo.HookConfigCreateInput) error {
	if template == nil || template.Meta == nil || template.Meta.WorkspaceID != in.WorkspaceID {
		return entity.ErrHookConfigStorage
	}
	seen := make(map[int64]bool, len(refs))
	for _, ref := range refs {
		if ref == nil || ref.ID <= 0 || seen[ref.ID] || ref.SpaceID != template.Meta.WorkspaceID || ref.ExptTemplateID != template.Meta.ID {
			return entity.ErrHookConfigStorage
		}
		seen[ref.ID] = true
	}
	owner := hookcomponent.ConfigOwner{WorkspaceID: in.WorkspaceID, ObjectID: template.Meta.ID, Kind: hookcomponent.ConfigOwnerTemplate, ExecutionScope: in.ExecutionScope}
	encoded, err := r.encodeCreateConfig(ctx, owner, in)
	if err != nil {
		return err
	}
	po, err := convert.NewExptTemplateConverter().DO2PO(template)
	if err != nil {
		return entity.ErrHookConfigStorage
	}
	po.LifecycleHookConf = &encoded
	// A copied configuration is not a new authorization to run as the source schedule owner.
	po.ScheduleRunBinding = nil
	refPOs := convert.NewExptTemplateEvaluatorRefConverter().DO2PO(refs)
	err = r.provider.Transaction(ctx, func(tx *gorm.DB) error {
		tx = tx.Session(&gorm.Session{Logger: logger.Discard})
		if err := tx.Create(po).Error; err != nil {
			return err
		}
		if len(refPOs) > 0 {
			return tx.Create(refPOs).Error
		}
		return nil
	}, db.WithMaster())
	if err != nil {
		return entity.ErrHookConfigStorage
	}
	return nil
}

func (r *hookConfigRepo) encodeCreateConfig(ctx context.Context, owner hookcomponent.ConfigOwner, in repo.HookConfigCreateInput) ([]byte, error) {
	if r == nil || r.provider == nil || r.codec == nil || !validHookConfigOwner(owner) || in.Config == nil || (in.Config.Before == nil && in.Config.After == nil) {
		return nil, entity.ErrHookConfigStorage
	}
	encoded, err := r.codec.EncodeConfig(ctx, in.KeyID, owner, in.Config)
	if err != nil || len(encoded) == 0 {
		return nil, entity.ErrHookConfigStorage
	}
	return encoded, nil
}
