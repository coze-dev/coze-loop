// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"errors"
	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"reflect"
)

func NewHookExecutionInitializationRepo(p db.Provider) repo.IHookExecutionInitializationRepo {
	return &hookRunRepo{provider: p}
}

func hookExecutionRequired(e *model.Experiment, s *lockedHookRun) bool {
	return entity.HookExecutionInitializationRequired(s.life.BeforeEnabled || s.life.AfterEnabled, s.view.Mode, entity.ExptType(e.ExptType), entity.ExptEvalSetSourceType(e.EvalSetSourceType))
}

func (r *hookRunRepo) loadExecutionRun(tx *gorm.DB, key entity.HookRunKey, scope string) (*model.Experiment, *lockedHookRun, error) {
	if b := r.executionBinding; b != nil && (b.source.Key != key || b.source.ExecutionScope != scope) {
		return nil, nil, entity.ErrHookStoreConflict
	}
	e, err := lockHookExperiment(tx, key)
	if err != nil {
		return nil, nil, err
	}
	s, err := loadHookRun(tx, key)
	if err != nil {
		return nil, nil, err
	}
	if s.life.ExecutionScope != scope {
		return nil, nil, entity.ErrHookStoreConflict
	}
	if r.executionBinding != nil {
		if err := r.executionBinding.checkRun(tx, e, s); err != nil {
			return nil, nil, err
		}
	} else if !hookExecutionRequired(e, s) {
		return nil, nil, entity.ErrHookExecutionUnsupported
	}
	online := s.view.Mode == entity.EvaluationModeAppend && entity.ExptType(e.ExptType) == entity.ExptType_Online
	if !hookRunActive(e, s) || (entity.ExptStatus(e.Status) != entity.ExptStatus_Pending && entity.ExptStatus(e.Status) != entity.ExptStatus_Processing && !(online && entity.ExptStatus(e.Status) == entity.ExptStatus_Draining)) ||
		(s.view.State.Status != entity.ExptStatus_Pending && s.view.State.Status != entity.ExptStatus_Processing && !(online && s.view.State.Status == entity.ExptStatus_Draining)) || !s.view.PlanReady || s.view.State.Gate != entity.HookGateReady {
		return nil, nil, entity.ErrHookAdmissionDenied
	}
	for _, table := range []any{&model.ExptItemResult{}, &model.ExptTurnResult{}} {
		if r.executionBinding != nil && entity.HookBoundRetryMode(r.executionBinding.source.Mode) {
			break
		}
		var count int64
		if err = tx.Unscoped().Model(table).Where("space_id=? AND expt_id=? AND expt_run_id<>?", key.WorkspaceID, key.ExperimentID, key.RunID).Count(&count).Error; err != nil {
			return nil, nil, err
		}
		if count != 0 {
			return nil, nil, entity.ErrHookStoreConflict
		}
	}
	return e, s, nil
}

func (r *hookRunRepo) executionTransaction(ctx context.Context, fn func(*gorm.DB) error) error {
	if !hookPlanProviderAvailable(r, ctx) {
		return entity.ErrHookExecutionStorage
	}
	err := r.provider.NewSession(ctx, db.WithMaster()).Session(&gorm.Session{Logger: logger.Discard}).Transaction(fn)
	for _, safe := range []error{entity.ErrHookStoreConflict, entity.ErrHookStoreCorrupt, entity.ErrHookStoreMissing, entity.ErrHookAdmissionDenied, entity.ErrHookExecutionUnsupported, context.Canceled, context.DeadlineExceeded} {
		if errors.Is(err, safe) {
			return safe
		}
	}
	if err != nil {
		return entity.ErrHookExecutionStorage
	}
	return nil
}

func (r *hookRunRepo) ReadExecutionInitializationPage(ctx context.Context, in entity.HookExecutionInitializationReadInput) (*entity.HookExecutionInitializationPage, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}
	var out *entity.HookExecutionInitializationPage
	err := r.executionTransaction(ctx, func(tx *gorm.DB) error {
		_, s, err := r.loadExecutionRun(tx, in.Key, in.ExecutionScope)
		if err != nil {
			return err
		}
		out, _, err = r.readExecutionPage(tx, s, in.StartOrdinal, int(in.Limit))
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *hookRunRepo) WriteExecutionInitializationPage(ctx context.Context, in entity.HookExecutionInitializationWriteInput) (*entity.HookExecutionInitializationPage, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}
	var out *entity.HookExecutionInitializationPage
	err := r.executionTransaction(ctx, func(tx *gorm.DB) error {
		_, s, err := r.loadExecutionRun(tx, in.Key, in.ExecutionScope)
		if err != nil {
			return err
		}
		if s.view.PlanHash != in.PlanHash {
			return entity.ErrHookStoreConflict
		}
		page, rows, err := r.readExecutionPage(tx, s, in.StartOrdinal, len(in.Items))
		if err != nil {
			return err
		}
		if len(page.Items) != len(in.Items) {
			return entity.ErrHookStoreConflict
		}
		fresh := false
		for i, item := range page.Items {
			if in.Items[i].Frozen != item.Frozen {
				return entity.ErrHookStoreConflict
			}
			if item.Manifest != nil {
				if !reflect.DeepEqual(*item.Manifest, in.Items[i]) {
					return entity.ErrHookStoreConflict
				}
			} else {
				fresh = true
			}
		}
		if !fresh {
			out = page
			return nil
		}
		if s.life.ExecutionInitialized || s.life.ExecutionStarted || s.life.Version != in.ExpectedVersion {
			return entity.ErrHookStoreConflict
		}
		for i, item := range page.Items {
			if item.Manifest != nil {
				continue
			}
			if item.Reuse != nil {
				if err = r.writeRetryExecutionItem(tx, rows[i], in.Items[i], item.Reuse); err != nil {
					return err
				}
				continue
			}
			if err = r.writeExecutionItemRef(tx, in.Items[i]); err != nil {
				return err
			}
			if err = writeExecutionItem(tx, rows[i], in.Items[i]); err != nil {
				return err
			}
		}
		now, err := hookDBNow(tx)
		if err != nil {
			return err
		}
		if err = updateHookLifecycle(tx, in.Key, s.life.Version, now, map[string]any{}); err != nil {
			return err
		}
		s.life.Version++
		out, _, err = r.readExecutionPage(tx, s, in.StartOrdinal, len(in.Items))
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
