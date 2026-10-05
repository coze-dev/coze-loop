// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"errors"
	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/db"
	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"gorm.io/gorm"
)

type hookRunRepo struct {
	provider         db.Provider
	executionBinding *boundHookExecution
}

func NewHookRunRepo(provider db.Provider) repo.IHookRepo { return &hookRunRepo{provider: provider} }

func (r *hookRunRepo) CreateRunWithHooks(ctx context.Context, in entity.HookCreateRunInput) (entity.HookStoreResult, error) {
	if err := in.Validate(); err != nil {
		return entity.HookStoreResult{}, err
	}
	var out entity.HookStoreResult
	err := r.provider.Transaction(ctx, func(tx *gorm.DB) error {
		expt, err := lockHookExperiment(tx, in.Key)
		if err != nil {
			return err
		}
		var life model.ExptLifecycleRun
		// The parent lock serializes creation; locking an absent lifecycle can deadlock unrelated experiments on a shared gap.
		err = hookRunScope(tx, in.Key).First(&life).Error
		if err == nil {
			stored, err := loadHookRun(tx, in.Key)
			if err != nil {
				return err
			}
			if stored.view.CreatedBy != in.RunLog.CreatedBy || stored.view.Mode != entity.ExptRunMode(in.RunLog.Mode) || stored.life.SnapshotHash != in.Snapshot.Hash || stored.life.ExecutionScope != in.Snapshot.ExecutionScope || !equalHookSource(stored.life.SourceRunID, in.SourceRunID) {
				return entity.ErrHookStoreConflict
			}
			if stored.life.BeforeEnabled != (in.Before != nil) || stored.life.AfterEnabled != (in.After != nil) {
				return entity.ErrHookStoreCorrupt
			}
			// Snapshot hash is supplied by the trusted protector; retain the original cipher and IDs.
			out.Run = stored.view
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if expt.DeletedAt.Valid || expt.LatestRunID != in.ExpectedLatestRunID {
			return entity.ErrHookStoreConflict
		}
		config, err := readHookConfig(tx, hookcomponent.ConfigOwner{WorkspaceID: in.Key.WorkspaceID, ObjectID: in.Key.ExperimentID, Kind: hookcomponent.ConfigOwnerExperiment}, true)
		if err != nil {
			return hookConfigError(err)
		}
		if hookConfigRevision(config.LifecycleHookConf) != in.ExpectedConfigRevision {
			return entity.ErrHookStoreConflict
		}
		for _, po := range []any{&model.ExptRunLog{}, &model.ExptLifecycleHookRun{}, &model.ExptLifecycleRunItem{}} {
			var count int64
			if err := hookRunScope(tx.Model(po).Unscoped(), in.Key).Count(&count).Error; err != nil {
				return err
			}
			if count != 0 {
				return entity.ErrHookStoreCorrupt
			}
		}
		now, err := hookDBNow(tx)
		if err != nil {
			return err
		}
		log, err := convert.NewExptRunLogConvertor().DO2PO(in.RunLog)
		if err != nil {
			return err
		}
		log.LifecycleHookVersion = gptr.Of(int32(1))
		log.CreatedAt = now
		log.UpdatedAt = now
		if err := tx.Create(log).Error; err != nil {
			return err
		}
		gate := int32(1)
		if in.Before != nil {
			gate = 0
		}
		life = model.ExptLifecycleRun{SpaceID: in.Key.WorkspaceID, ExptID: in.Key.ExperimentID, ExptRunID: in.Key.RunID, SourceRunID: in.SourceRunID,
			BeforeEnabled: in.Before != nil, AfterEnabled: in.After != nil,
			SnapshotCipher: in.Snapshot.Cipher, SnapshotKeyID: in.Snapshot.KeyID, SnapshotHash: in.Snapshot.Hash, ExecutionScope: in.Snapshot.ExecutionScope, Gate: gate, CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(&life).Error; err != nil {
			return err
		}
		for _, item := range []struct {
			phase entity.HookPhase
			seed  *entity.HookOperationSeed
		}{{entity.HookPhaseBefore, in.Before}, {entity.HookPhaseAfter, in.After}} {
			if item.seed == nil {
				continue
			}
			op := model.ExptLifecycleHookRun{ID: item.seed.ID, SpaceID: in.Key.WorkspaceID, ExptID: in.Key.ExperimentID, ExptRunID: in.Key.RunID, Phase: string(item.phase), OperationID: item.seed.OperationID, IdempotencyKey: item.seed.IdempotencyKey, Status: string(entity.HookOperationPending), ExecutionScope: in.Snapshot.ExecutionScope, UpdatedAt: now}
			if err := tx.Create(&op).Error; err != nil {
				return err
			}
		}
		projection := map[string]any{"latest_run_id": in.Key.RunID, "updated_at": now}
		if entity.ExptRunMode(in.RunLog.Mode) == entity.EvaluationModeAppend && entity.ExptType(expt.ExptType) == entity.ExptType_Online {
			projection["status"] = int32(entity.ExptStatus_Pending)
		}
		if entity.HookBoundRetryMode(entity.ExptRunMode(in.RunLog.Mode)) && in.SourceRunID != nil && *in.SourceRunID == in.ExpectedLatestRunID && in.ExpectedLatestRunID > 0 {
			projection["status"] = int32(entity.ExptStatus_Pending)
		}
		if err := hookOneRow(tx.Model(&model.Experiment{}).Where("id=? AND space_id=? AND latest_run_id=?", in.Key.ExperimentID, in.Key.WorkspaceID, in.ExpectedLatestRunID).UpdateColumns(projection)); err != nil {
			return err
		}
		stored, err := loadHookRun(tx, in.Key)
		if err != nil {
			return err
		}
		out = entity.HookStoreResult{Run: stored.view, Changed: true, LatestProjected: true}
		return nil
	}, db.WithMaster())
	if err != nil {
		return entity.HookStoreResult{}, err
	}
	return out, nil
}

func (r *hookRunRepo) GetRun(ctx context.Context, key entity.HookRunKey) (*entity.HookStoredRun, error) {
	if err := (entity.HookStoreGuard{Key: key}).Validate(); err != nil {
		return nil, err
	}
	var out *entity.HookStoredRun
	err := r.provider.Transaction(ctx, func(tx *gorm.DB) error {
		if _, err := lockHookExperiment(tx, key); err != nil {
			return err
		}
		stored, err := loadHookRun(tx, key)
		if err != nil {
			return err
		}
		out = stored.view
		return nil
	}, db.WithMaster())
	if err != nil {
		return nil, err
	}
	return out, nil
}
