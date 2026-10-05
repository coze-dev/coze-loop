// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func NewHookRunInitializationRepo(provider db.Provider) repo.IHookRunInitializationRepo {
	return &hookRunRepo{provider: provider}
}

func (r *hookRunRepo) AppendHookRunItems(ctx context.Context, in entity.HookAppendRunItemsInput) error {
	if err := in.HookStoreGuard.Validate(); err != nil {
		return err
	}
	seen := make(map[int64]bool, len(in.ItemIDs))
	for _, id := range in.ItemIDs {
		if id <= 0 || seen[id] {
			return entity.ErrHookStoreConflict
		}
		seen[id] = true
	}
	err := r.provider.Transaction(ctx, func(tx *gorm.DB) error {
		tx = tx.Session(&gorm.Session{Logger: logger.Discard})
		expt, err := lockHookExperiment(tx, in.Key)
		if err != nil {
			return err
		}
		s, err := loadHookRun(tx, in.Key)
		if err != nil {
			return err
		}
		if expt.DeletedAt.Valid || expt.LatestRunID != in.Key.RunID || s.life.ExecutionScope != in.ExecutionScope || s.life.Version != in.ExpectedVersion {
			return entity.ErrHookStoreConflict
		}
		if s.view.State.Gate == entity.HookGateClosed || s.view.State.Finalize != entity.HookFinalizeNone || (s.view.State.Status != entity.ExptStatus_Pending && s.view.State.Status != entity.ExptStatus_Processing) {
			return entity.ErrHookAdmissionDenied
		}
		log, err := convert.NewExptRunLogConvertor().PO2DO(&s.log)
		if err != nil {
			return err
		}
		if err = log.AppendItemIDs(in.ItemIDs); err != nil {
			return err
		}
		po, err := convert.NewExptRunLogConvertor().DO2PO(log)
		if err != nil {
			return err
		}
		now, err := hookDBNow(tx)
		if err != nil {
			return err
		}
		if err = hookOneRow(hookRunScope(tx.Model(&model.ExptRunLog{}), in.Key).Where("id=? AND lifecycle_hook_version=1 AND status=?", s.log.ID, *s.log.Status).UpdateColumns(map[string]any{"item_ids": po.ItemIds, "updated_at": now})); err != nil {
			return err
		}
		return updateHookLifecycle(tx, in.Key, s.life.Version, now, map[string]any{})
	}, db.WithMaster())
	if err != nil {
		return hookInitializationError(err)
	}
	return nil
}

// Only the writer-owned public projection is read here. Full configuration
// authentication remains the StorageCodec's responsibility on the enabled path.
func initializationHooksEnabled(raw []byte) (bool, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return false, nil
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return false, entity.ErrHookConfigStorage
	}
	if len(fields) == 0 {
		return false, nil
	}
	var projection struct {
		Version    int    `json:"version"`
		Purpose    string `json:"purpose"`
		Projection *struct {
			Before *struct {
				Enabled *bool `json:"enabled"`
			} `json:"before"`
			After *struct {
				Enabled *bool `json:"enabled"`
			} `json:"after"`
		} `json:"projection"`
	}
	if json.Unmarshal(raw, &projection) != nil || projection.Version != 1 || projection.Purpose != "configuration" || projection.Projection == nil {
		return false, entity.ErrHookConfigStorage
	}
	p := projection.Projection
	if p.Before == nil && p.After == nil || p.Before != nil && p.Before.Enabled == nil || p.After != nil && p.After.Enabled == nil {
		return false, entity.ErrHookConfigStorage
	}
	return p.Before != nil && *p.Before.Enabled || p.After != nil && *p.After.Enabled, nil
}

func (r *hookRunRepo) ReadRunInitialization(ctx context.Context, key entity.HookRunKey) (*entity.HookRunInitialization, error) {
	if err := (entity.HookStoreGuard{Key: key}).Validate(); err != nil {
		return nil, err
	}
	out := new(entity.HookRunInitialization)
	err := r.provider.Transaction(ctx, func(tx *gorm.DB) error {
		tx = tx.Session(&gorm.Session{Logger: logger.Discard})
		row, err := readHookConfig(tx, hookcomponent.ConfigOwner{WorkspaceID: key.WorkspaceID, ObjectID: key.ExperimentID, Kind: hookcomponent.ConfigOwnerExperiment}, true)
		if err != nil {
			return err
		}
		out.LatestRunID, out.ConfigRevision = row.LatestRunID, hookConfigRevision(row.LifecycleHookConf)
		out.HooksEnabled, err = initializationHooksEnabled(row.LifecycleHookConf)
		if err != nil {
			return err
		}
		var log model.ExptRunLog
		err = hookRunScope(tx.Unscoped(), key).First(&log).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if log.DeletedAt.Valid || log.ID != key.RunID || log.LifecycleHookVersion != nil && *log.LifecycleHookVersion != 0 && *log.LifecycleHookVersion != 1 {
			return entity.ErrHookStoreCorrupt
		}
		out.Managed = log.LifecycleHookVersion != nil && *log.LifecycleHookVersion == 1
		out.RunLog, err = convert.NewExptRunLogConvertor().PO2DO(&log)
		return err
	}, db.WithMaster())
	if err != nil {
		return nil, hookInitializationError(err)
	}
	return out, nil
}

// The no-Hook branch uses the same parent lock and revision as UpdateConfig.
// No lifecycle tables or cryptographic providers participate in this transaction.
func (r *hookRunRepo) CreateRunWithoutHooks(ctx context.Context, log *entity.ExptRunLog, expectedLatest int64, revision string) (bool, error) {
	if log == nil || log.ID != log.ExptRunID || expectedLatest < 0 {
		return false, entity.ErrHookStoreConflict
	}
	key := entity.HookRunKey{WorkspaceID: log.SpaceID, ExperimentID: log.ExptID, RunID: log.ExptRunID}
	if err := (entity.HookStoreGuard{Key: key}).Validate(); err != nil {
		return false, err
	}
	changed := false
	err := r.provider.Transaction(ctx, func(tx *gorm.DB) error {
		tx = tx.Session(&gorm.Session{Logger: logger.Discard})
		expt, err := lockHookExperiment(tx, key)
		if err != nil {
			return err
		}
		row, err := readHookConfig(tx, hookcomponent.ConfigOwner{WorkspaceID: key.WorkspaceID, ObjectID: key.ExperimentID, Kind: hookcomponent.ConfigOwnerExperiment}, true)
		if err != nil {
			return err
		}
		if hookConfigRevision(row.LifecycleHookConf) != revision {
			return entity.ErrHookStoreConflict
		}
		enabled, err := initializationHooksEnabled(row.LifecycleHookConf)
		if err != nil {
			return err
		}
		if enabled || expt.DeletedAt.Valid {
			return entity.ErrHookStoreConflict
		}
		var existing model.ExptRunLog
		err = hookRunScope(tx.Unscoped(), key).First(&existing).Error
		if err == nil {
			if existing.ID != log.ID || existing.DeletedAt.Valid || existing.LifecycleHookVersion != nil && *existing.LifecycleHookVersion != 0 || existing.CreatedBy != log.CreatedBy || existing.Mode == nil || *existing.Mode != log.Mode {
				return entity.ErrHookStoreConflict
			}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if expt.LatestRunID != expectedLatest {
			return entity.ErrHookStoreConflict
		}
		now, err := hookDBNow(tx)
		if err != nil {
			return err
		}
		po, err := convert.NewExptRunLogConvertor().DO2PO(log)
		if err != nil {
			return err
		}
		po.CreatedAt, po.UpdatedAt = now, now
		if err = tx.Omit("lifecycle_hook_version").Create(po).Error; err != nil {
			return err
		}
		if err = hookOneRow(tx.Model(&model.Experiment{}).Where("id=? AND space_id=? AND latest_run_id=?", key.ExperimentID, key.WorkspaceID, expectedLatest).UpdateColumns(map[string]any{"latest_run_id": key.RunID, "updated_at": now})); err != nil {
			return err
		}
		changed = true
		return nil
	}, db.WithMaster())
	if err != nil {
		return false, hookInitializationError(err)
	}
	return changed, nil
}

func hookInitializationError(err error) error {
	for _, safe := range []error{entity.ErrHookStoreConflict, entity.ErrHookStoreMissing, entity.ErrHookStoreCorrupt, entity.ErrHookAdmissionDenied} {
		if errors.Is(err, safe) {
			return safe
		}
	}
	return entity.ErrHookConfigStorage
}
