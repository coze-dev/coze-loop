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
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

type scheduledRunTriggerRepo struct{ provider db.Provider }

// NewScheduledRunTriggerRepo owns commits; provider must be a root, not transaction-bound provider.
func NewScheduledRunTriggerRepo(provider db.Provider) repo.IScheduledRunTriggerRepo {
	return &scheduledRunTriggerRepo{provider: provider}
}

func (r *scheduledRunTriggerRepo) Reserve(ctx context.Context, binding *entity.ExptTemplateScheduleBinding, instance string, ids entity.ScheduledRunTriggerIDs) (*entity.ScheduledRunTrigger, error) {
	if err := ids.Validate(); err != nil {
		return nil, err
	}
	return r.withBinding(ctx, binding, instance, func(tx *gorm.DB, current *entity.ExptTemplateScheduleBinding) (*entity.ScheduledRunTrigger, error) {
		found, err := readScheduledTrigger(tx, current, instance)
		if err != nil || found != nil {
			return found, err
		}
		row := &model.ExptTemplateTrigger{ID: ids.TriggerID, BindingID: current.BindingID, BindingVersion: current.Version, InstanceID: instance,
			SpaceID: current.SpaceID, TemplateID: current.TemplateID, ExptID: ids.ExperimentID, ExptRunID: ids.RunID, Status: entity.ScheduledRunTriggerPending}
		// The template row lock serializes this binding; the database unique key is the final guard.
		if err := tx.Create(row).Error; err != nil {
			return nil, err
		}
		return readScheduledTrigger(tx, current, instance)
	})
}

func (r *scheduledRunTriggerRepo) Commit(ctx context.Context, binding *entity.ExptTemplateScheduleBinding, instance string, submit repo.ScheduledRunSQLSubmit) (*entity.ScheduledRunTrigger, error) {
	if submit == nil {
		return nil, entity.ErrScheduledRunTriggerInvalid
	}
	return r.withBinding(ctx, binding, instance, func(tx *gorm.DB, current *entity.ExptTemplateScheduleBinding) (*entity.ScheduledRunTrigger, error) {
		trigger, err := readScheduledTrigger(tx, current, instance)
		if err != nil {
			return nil, err
		}
		if trigger == nil {
			return nil, entity.ErrScheduledRunTriggerMissing
		}
		if trigger.Status == entity.ScheduledRunTriggerSubmitted {
			return trigger, nil
		}
		if err := submit(ctx, *trigger, current.UserID, scheduledTriggerTxProvider{provider: r.provider, tx: tx}); err != nil {
			return nil, err
		}
		// Also reject local submission code that accidentally changes the binding in this transaction.
		if _, err := lockScheduledTriggerBinding(tx, *current); err != nil {
			return nil, err
		}
		if err := checkScheduledTriggerRun(tx, trigger, current.UserID); err != nil {
			return nil, err
		}
		changed := tx.Model(&model.ExptTemplateTrigger{}).Where("id=? AND binding_id=? AND binding_version=? AND instance_id=? AND space_id=? AND template_id=? AND expt_id=? AND expt_run_id=? AND status=?",
			trigger.ID, trigger.BindingID, trigger.BindingVersion, trigger.InstanceID, trigger.SpaceID, trigger.TemplateID, trigger.ExperimentID, trigger.RunID, entity.ScheduledRunTriggerPending).UpdateColumn("status", entity.ScheduledRunTriggerSubmitted)
		if changed.Error != nil {
			return nil, changed.Error
		}
		if changed.RowsAffected != 1 {
			return nil, entity.ErrScheduledRunTriggerConflict
		}
		trigger.Status = entity.ScheduledRunTriggerSubmitted
		return trigger, nil
	})
}

func (r *scheduledRunTriggerRepo) withBinding(ctx context.Context, binding *entity.ExptTemplateScheduleBinding, instance string, apply func(*gorm.DB, *entity.ExptTemplateScheduleBinding) (*entity.ScheduledRunTrigger, error)) (*entity.ScheduledRunTrigger, error) {
	if r == nil || r.provider == nil || ctx == nil || !binding.Active() || entity.ValidateScheduledRunInstanceID(instance) != nil {
		return nil, entity.ErrScheduledRunTriggerInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	expected := *binding
	var result *entity.ScheduledRunTrigger
	err := r.provider.Transaction(ctx, func(tx *gorm.DB) error {
		tx = tx.Session(&gorm.Session{Logger: logger.Discard})
		current, err := lockScheduledTriggerBinding(tx, expected)
		if err != nil {
			return err
		}
		result, err = apply(tx, current)
		if err != nil {
			return err
		}
		return ctx.Err()
	}, db.WithMaster())
	if err != nil {
		return nil, err
	}
	return result, nil
}

func lockScheduledTriggerBinding(tx *gorm.DB, expected entity.ExptTemplateScheduleBinding) (*entity.ExptTemplateScheduleBinding, error) {
	key := entity.ExptTemplateScheduleBindingKey{SpaceID: expected.SpaceID, TemplateID: expected.TemplateID, ExecutionScope: expected.ExecutionScope}
	current, _, err := readScheduleBinding(tx, key, true)
	if err != nil {
		return nil, err
	}
	if !current.Active() || !current.BoundAt.Equal(expected.BoundAt) {
		return nil, entity.ErrScheduledRunTriggerConflict
	}
	comparable := *current
	comparable.BoundAt = expected.BoundAt
	if comparable != expected {
		return nil, entity.ErrScheduledRunTriggerConflict
	}
	var row struct{ CronActivate bool }
	if err := tx.Table(model.TableNameExptTemplate).Select("cron_activate").Where("id=? AND space_id=? AND deleted_at IS NULL", key.TemplateID, key.SpaceID).Take(&row).Error; err != nil {
		return nil, err
	}
	if !row.CronActivate {
		return nil, entity.ErrScheduledRunTriggerConflict
	}
	return current, nil
}

func readScheduledTrigger(tx *gorm.DB, binding *entity.ExptTemplateScheduleBinding, instance string) (*entity.ScheduledRunTrigger, error) {
	var row model.ExptTemplateTrigger
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("binding_id=? AND binding_version=? AND instance_id=?", binding.BindingID, binding.Version, instance).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	trigger := &entity.ScheduledRunTrigger{ID: row.ID, BindingID: row.BindingID, BindingVersion: row.BindingVersion, InstanceID: row.InstanceID, SpaceID: row.SpaceID, TemplateID: row.TemplateID,
		ExperimentID: row.ExptID, RunID: row.ExptRunID, Status: row.Status, CreatedAt: row.CreatedAt}
	if trigger.Validate() != nil || trigger.BindingID != binding.BindingID || trigger.BindingVersion != binding.Version || trigger.InstanceID != instance || trigger.SpaceID != binding.SpaceID || trigger.TemplateID != binding.TemplateID {
		return nil, entity.ErrScheduledRunTriggerConflict
	}
	return trigger, nil
}

func checkScheduledTriggerRun(tx *gorm.DB, tr *entity.ScheduledRunTrigger, user string) error {
	var expt model.Experiment
	if err := tx.Select("id", "space_id", "expt_template_id", "latest_run_id", "created_by").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", tr.ExperimentID).Take(&expt).Error; err != nil {
		return err
	}
	// Compare opaque user IDs in Go; the database's default collation is case-insensitive.
	if expt.SpaceID != tr.SpaceID || expt.ExptTemplateID != tr.TemplateID || expt.LatestRunID != tr.RunID || expt.CreatedBy != user {
		return entity.ErrScheduledRunTriggerConflict
	}
	var run model.ExptRunLog
	if err := tx.Select("id", "space_id", "expt_id", "expt_run_id", "created_by").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", tr.RunID).Take(&run).Error; err != nil {
		return err
	}
	if run.SpaceID != tr.SpaceID || run.ExptID != tr.ExperimentID || run.ExptRunID != tr.RunID || run.CreatedBy != user {
		return entity.ErrScheduledRunTriggerConflict
	}
	return nil
}

// Both entry points are pinned; inheriting Provider.Transaction would open an unrelated transaction.
type scheduledTriggerTxProvider struct {
	provider db.Provider
	tx       *gorm.DB
}

func (p scheduledTriggerTxProvider) NewSession(ctx context.Context, opts ...db.Option) *gorm.DB {
	return p.provider.NewSession(ctx, append(opts, db.WithTransaction(p.tx), db.WithMaster())...)
}

func (p scheduledTriggerTxProvider) Transaction(ctx context.Context, fn func(*gorm.DB) error, opts ...db.Option) error {
	return p.NewSession(ctx, opts...).Transaction(fn)
}
