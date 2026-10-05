// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

type exptTemplateScheduleBindingRepo struct{ provider db.Provider }

func NewExptTemplateScheduleBindingRepo(provider db.Provider) repo.IExptTemplateScheduleBindingRepo {
	return &exptTemplateScheduleBindingRepo{provider: provider}
}

func (r *exptTemplateScheduleBindingRepo) Get(ctx context.Context, key entity.ExptTemplateScheduleBindingKey, opts ...db.Option) (*entity.ExptTemplateScheduleBinding, error) {
	if err := r.validate(ctx, key); err != nil {
		return nil, err
	}
	session := r.provider.NewSession(ctx, append(opts, db.WithMaster())...).Session(&gorm.Session{Logger: logger.Discard})
	b, _, err := readScheduleBinding(session, key, false)
	return b, err
}

func (r *exptTemplateScheduleBindingRepo) SaveCAS(ctx context.Context, key entity.ExptTemplateScheduleBindingKey, expectedVersion int64, binding *entity.ExptTemplateScheduleBinding, opts ...db.Option) (bool, error) {
	if binding.Validate() != nil || expectedVersion < 0 || expectedVersion == math.MaxInt64 ||
		binding.Version != expectedVersion+1 || !binding.Enabled || binding.JobID != "" || !scheduleBindingMatches(binding, key) {
		return false, entity.ErrExptTemplateScheduleBindingInvalid
	}
	next := *binding
	return r.change(ctx, key, func(current *entity.ExptTemplateScheduleBinding) (*entity.ExptTemplateScheduleBinding, error) {
		if current == nil && expectedVersion != 0 || current != nil && current.Version != expectedVersion {
			return nil, entity.ErrExptTemplateScheduleBindingConflict
		}
		return &next, nil
	}, opts...)
}

func (r *exptTemplateScheduleBindingRepo) ActivateCAS(ctx context.Context, key entity.ExptTemplateScheduleBindingKey, bindingID string, version int64, receipt entity.ExptTemplateScheduleReceipt, opts ...db.Option) (bool, error) {
	if bindingID == "" || version <= 0 || receipt.Validate() != nil {
		return false, entity.ErrExptTemplateScheduleBindingInvalid
	}
	return r.change(ctx, key, func(current *entity.ExptTemplateScheduleBinding) (*entity.ExptTemplateScheduleBinding, error) {
		if current == nil || current.BindingID != bindingID || current.Version != version || !current.Enabled ||
			current.Namespace != receipt.Namespace || current.Group != receipt.Group || current.BizKey != receipt.BizKey || current.Callback != receipt.Callback {
			return nil, entity.ErrExptTemplateScheduleBindingConflict
		}
		if current.JobID != "" {
			if current.JobID != receipt.JobID {
				return nil, entity.ErrExptTemplateScheduleBindingConflict
			}
			return nil, nil
		}
		current.JobID = receipt.JobID
		return current, nil
	}, opts...)
}

func (r *exptTemplateScheduleBindingRepo) DisableCAS(ctx context.Context, key entity.ExptTemplateScheduleBindingKey, bindingID string, version int64, opts ...db.Option) (bool, error) {
	if bindingID == "" || version <= 0 || version == math.MaxInt64 {
		return false, entity.ErrExptTemplateScheduleBindingInvalid
	}
	return r.change(ctx, key, func(current *entity.ExptTemplateScheduleBinding) (*entity.ExptTemplateScheduleBinding, error) {
		if current == nil || current.BindingID != bindingID || current.Version != version {
			return nil, entity.ErrExptTemplateScheduleBindingConflict
		}
		if !current.Enabled {
			return nil, nil
		}
		current.Enabled = false
		current.Version++
		return current, nil
	}, opts...)
}

func (r *exptTemplateScheduleBindingRepo) validate(ctx context.Context, key entity.ExptTemplateScheduleBindingKey) error {
	if r == nil || r.provider == nil || ctx == nil {
		return entity.ErrExptTemplateScheduleBindingInvalid
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return key.Validate()
}

func (r *exptTemplateScheduleBindingRepo) change(ctx context.Context, key entity.ExptTemplateScheduleBindingKey, apply func(*entity.ExptTemplateScheduleBinding) (*entity.ExptTemplateScheduleBinding, error), opts ...db.Option) (bool, error) {
	if err := r.validate(ctx, key); err != nil {
		return false, err
	}
	changed := false
	err := r.provider.Transaction(ctx, func(tx *gorm.DB) error {
		tx = tx.Session(&gorm.Session{Logger: logger.Discard})
		current, raw, err := readScheduleBinding(tx, key, true)
		if err != nil {
			return err
		}
		next, err := apply(current)
		if err != nil || next == nil {
			return err
		}
		encoded, err := json.Marshal(next)
		if err != nil {
			return entity.ErrExptTemplateScheduleBindingInvalid
		}
		q := tx.Table(model.TableNameExptTemplate).Where("id=? AND space_id=? AND deleted_at IS NULL", key.TemplateID, key.SpaceID)
		if raw == nil {
			q = q.Where("schedule_run_binding IS NULL")
		} else {
			q = q.Where("schedule_run_binding = ?", *raw)
		}
		result := q.UpdateColumn("schedule_run_binding", encoded)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return entity.ErrExptTemplateScheduleBindingConflict
		}
		changed = true
		return nil
	}, append(opts, db.WithMaster())...)
	return changed && err == nil, err
}

func readScheduleBinding(session *gorm.DB, key entity.ExptTemplateScheduleBindingKey, lock bool) (*entity.ExptTemplateScheduleBinding, *[]byte, error) {
	var row struct{ ScheduleRunBinding *[]byte }
	q := session.Table(model.TableNameExptTemplate).Select("schedule_run_binding").Where("id=? AND space_id=? AND deleted_at IS NULL", key.TemplateID, key.SpaceID)
	if lock {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := q.Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, entity.ErrExptTemplateScheduleBindingMissing
		}
		return nil, nil, err
	}
	if row.ScheduleRunBinding == nil {
		return nil, nil, nil
	}
	var binding *entity.ExptTemplateScheduleBinding
	decoder := json.NewDecoder(bytes.NewReader(*row.ScheduleRunBinding))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&binding) != nil || binding.Validate() != nil || decoder.Decode(new(any)) != io.EOF {
		return nil, nil, entity.ErrExptTemplateScheduleBindingCorrupt
	}
	if !scheduleBindingMatches(binding, key) {
		return nil, nil, entity.ErrExptTemplateScheduleBindingConflict
	}
	return binding, row.ScheduleRunBinding, nil
}

func scheduleBindingMatches(b *entity.ExptTemplateScheduleBinding, key entity.ExptTemplateScheduleBindingKey) bool {
	return b.SpaceID == key.SpaceID && b.TemplateID == key.TemplateID && b.ExecutionScope == key.ExecutionScope
}
