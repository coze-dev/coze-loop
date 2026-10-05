// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

type hookConfigRepo struct {
	provider db.Provider
	codec    hookcomponent.StorageCodec
}

func NewHookConfigRepo(provider db.Provider, codec hookcomponent.StorageCodec) repo.IHookConfigRepo {
	return &hookConfigRepo{provider: provider, codec: codec}
}

func (r *hookConfigRepo) GetConfig(ctx context.Context, owner hookcomponent.ConfigOwner) (*entity.HookConfigRecord, error) {
	if !validHookConfigOwner(owner) || r.codec == nil {
		return nil, entity.ErrHookConfigStorage
	}
	row, err := readHookConfig(r.provider.NewSession(ctx, db.WithMaster()), owner, false)
	if err != nil {
		return nil, hookConfigError(err)
	}
	conf, err := r.codec.DecodeConfig(ctx, owner, row.LifecycleHookConf)
	if err != nil {
		return nil, entity.ErrHookConfigStorage
	}
	return &entity.HookConfigRecord{Config: conf, Revision: hookConfigRevision(row.LifecycleHookConf)}, nil
}

func (r *hookConfigRepo) UpdateConfig(ctx context.Context, owner hookcomponent.ConfigOwner, in entity.HookConfigUpdateInput) (bool, error) {
	if in.Config == nil || (in.Config.Before == nil && in.Config.After == nil) {
		return false, nil
	}
	current, err := r.GetConfig(ctx, owner)
	if err != nil {
		return false, err
	}
	if current.Revision != in.ExpectedRevision {
		return false, entity.ErrHookStoreConflict
	}
	conf, err := entity.ResolveLifecycleHookConf(current.Config, in.Config)
	if err != nil {
		return false, err
	}
	encoded, err := r.codec.EncodeConfig(ctx, in.KeyID, owner, conf)
	if err != nil || len(encoded) == 0 {
		return false, entity.ErrHookConfigStorage
	}
	// Crypto and external providers must finish before acquiring the same row lock as Run creation.
	err = r.provider.Transaction(ctx, func(tx *gorm.DB) error {
		tx = tx.Session(&gorm.Session{Logger: logger.Discard})
		row, err := readHookConfig(tx, owner, true)
		if err != nil {
			return err
		}
		if owner.Kind == hookcomponent.ConfigOwnerExperiment {
			if row.LatestRunID != 0 {
				return entity.ErrHookConfigImmutable
			}
			var log model.ExptRunLog
			err = tx.Unscoped().Select("id").Where("space_id=? AND expt_id=?", owner.WorkspaceID, owner.ObjectID).Clauses(clause.Locking{Strength: "UPDATE"}).Take(&log).Error
			if err == nil {
				return entity.ErrHookConfigImmutable
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		if hookConfigRevision(row.LifecycleHookConf) != in.ExpectedRevision {
			return entity.ErrHookStoreConflict
		}
		return hookOneRow(tx.Table(hookConfigTable(owner)).Where("id=? AND space_id=? AND deleted_at IS NULL", owner.ObjectID, owner.WorkspaceID).UpdateColumn("lifecycle_hook_conf", encoded))
	}, db.WithMaster())
	if err != nil {
		return false, hookConfigError(err)
	}
	return true, nil
}

type hookConfigRow struct {
	LifecycleHookConf []byte
	LatestRunID       int64
}

func readHookConfig(tx *gorm.DB, owner hookcomponent.ConfigOwner, lock bool) (*hookConfigRow, error) {
	q := tx.Session(&gorm.Session{Logger: logger.Discard}).Table(hookConfigTable(owner)).Select("lifecycle_hook_conf").Where("id=? AND space_id=? AND deleted_at IS NULL", owner.ObjectID, owner.WorkspaceID)
	if owner.Kind == hookcomponent.ConfigOwnerExperiment {
		q = q.Select("lifecycle_hook_conf", "latest_run_id")
	}
	if lock {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var row hookConfigRow
	err := q.Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, entity.ErrHookStoreMissing
	}
	return &row, err
}

func hookConfigTable(owner hookcomponent.ConfigOwner) string {
	if owner.Kind == hookcomponent.ConfigOwnerTemplate {
		return model.TableNameExptTemplate
	}
	return model.TableNameExperiment
}

func validHookConfigOwner(owner hookcomponent.ConfigOwner) bool {
	if owner.WorkspaceID <= 0 || owner.ObjectID <= 0 || (owner.Kind != hookcomponent.ConfigOwnerExperiment && owner.Kind != hookcomponent.ConfigOwnerTemplate) || owner.ExecutionScope == "" || len(owner.ExecutionScope) > 128 || strings.TrimSpace(owner.ExecutionScope) != owner.ExecutionScope {
		return false
	}
	for _, c := range owner.ExecutionScope {
		if c < 33 || c > 126 {
			return false
		}
	}
	return true
}

func hookConfigRevision(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func hookConfigError(err error) error {
	for _, safe := range []error{entity.ErrHookConfigImmutable, entity.ErrHookStoreConflict, entity.ErrHookStoreMissing} {
		if errors.Is(err, safe) {
			return safe
		}
	}
	return entity.ErrHookConfigStorage
}
