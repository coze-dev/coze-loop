// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"gorm.io/gorm"
)

type manualCancellationDB struct {
	db.Provider
	tx *gorm.DB
}

func (p manualCancellationDB) NewSession(ctx context.Context, opts ...db.Option) *gorm.DB {
	return p.Provider.NewSession(ctx, append(opts, db.WithTransaction(p.tx))...)
}

func (p manualCancellationDB) Transaction(ctx context.Context, fn func(*gorm.DB) error, opts ...db.Option) error {
	return fn(p.NewSession(ctx, opts...))
}

// New Run creation locks or updates this same parent row before publishing Latest.
// The callback must finish every old-Run side effect before this transaction releases it.
func (r *hookFinalizationRepo) WithManualCancellationFence(ctx context.Context, key entity.HookRunKey, fn func(context.Context, repo.ManualCancellationRepositories) error) error {
	if r == nil || r.provider == nil || r.binding != nil || ctx == nil || fn == nil || (entity.HookStoreGuard{Key: key}).Validate() != nil {
		return entity.ErrHookStoreCorrupt
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return r.provider.Transaction(ctx, func(tx *gorm.DB) error {
		parent, err := lockHookExperiment(tx, key)
		if err != nil {
			return err
		}
		if parent.DeletedAt.Valid {
			return entity.ErrHookStoreMissing
		}
		if parent.LatestRunID != key.RunID {
			return entity.ErrHookStoreConflict
		}
		p := manualCancellationDB{Provider: r.provider, tx: tx}
		sourceRepo := &hookFinalizationRepo{provider: p}
		source, err := sourceRepo.readLegacySource(ctx, key)
		if err != nil {
			return err
		}
		if source == nil || source.Managed || source.Key != key {
			return entity.ErrHookStoreConflict
		}
		return fn(ctx, repo.ManualCancellationRepositories{
			Source:      sourceRepo,
			Experiments: NewExptRepo(mysql.NewExptDAO(p), mysql.NewExptEvaluatorRefDAO(p), nil),
			Runs:        NewExptRunLogRepo(mysql.NewExptRunLogDAO(p)),
			Items:       NewExptItemResultRepo(mysql.NewExptItemResultDAO(p)),
			Turns:       NewExptTurnResultRepo(nil, mysql.NewExptTurnResultDAO(p), mysql.NewExptTurnEvaluatorResultRefDAO(p)),
			Stats:       NewExptStatsRepo(mysql.NewExptStatsDAO(p)),
			Templates:   NewExptTemplateRepo(mysql.NewExptTemplateDAO(p), mysql.NewExptTemplateEvaluatorRefDAO(p), nil),
		})
	}, db.WithMaster())
}

var _ repo.IManualCancellationFenceRepo = (*hookFinalizationRepo)(nil)
