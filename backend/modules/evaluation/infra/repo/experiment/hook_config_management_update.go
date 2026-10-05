// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"encoding/json"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	mysqldao "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

var _ repo.IHookConfigMetadataUpdater = (*hookConfigRepo)(nil)

func (r *hookConfigRepo) UpdateExperimentWithHookConfig(ctx context.Context, owner hookcomponent.ConfigOwner, expt *entity.Experiment, in entity.HookConfigUpdateInput) error {
	if expt == nil || owner.Kind != hookcomponent.ConfigOwnerExperiment || expt.ID != owner.ObjectID || expt.SpaceID != owner.WorkspaceID {
		return entity.ErrHookConfigStorage
	}
	po, err := convert.NewExptConverter().DO2PO(expt)
	if err != nil {
		return entity.ErrHookConfigStorage
	}
	return r.updateMetadataAndHook(ctx, owner, in, func(tx *gorm.DB) error {
		return mysqldao.NewExptDAO(hookManagementTxProvider{r.provider, tx}).Update(ctx, po)
	})
}

func (r *hookConfigRepo) UpdateTemplateWithHookConfig(ctx context.Context, owner hookcomponent.ConfigOwner, template *entity.ExptTemplate, refs []*entity.ExptTemplateEvaluatorRef, in entity.HookConfigUpdateInput) error {
	if template == nil || template.Meta == nil || owner.Kind != hookcomponent.ConfigOwnerTemplate || template.Meta.ID != owner.ObjectID || template.Meta.WorkspaceID != owner.WorkspaceID {
		return entity.ErrHookConfigStorage
	}
	if template.ExptInfo != nil && template.ExptInfo.CronActivate || template.ExptSource != nil && template.ExptSource.Scheduler != nil || managementTemplateHasScheduler(template.TemplateConf) {
		return repo.ErrHookConfigScheduleBindingRequired
	}
	ids := make([]int64, len(refs))
	seen := make(map[int64]bool, len(refs))
	pairs := make(map[[2]int64]bool, len(refs))
	copies := make([]*entity.ExptTemplateEvaluatorRef, len(refs))
	for i, ref := range refs {
		if ref == nil || ref.ID <= 0 || seen[ref.ID] || ref.SpaceID != owner.WorkspaceID || ref.ExptTemplateID != owner.ObjectID {
			return entity.ErrHookConfigStorage
		}
		pair := [2]int64{ref.EvaluatorID, ref.EvaluatorVersionID}
		if pairs[pair] {
			return entity.ErrHookConfigStorage
		}
		pairs[pair], seen[ref.ID], ids[i] = true, true, ref.ID
		copy := *ref
		copies[i] = &copy
	}
	return r.updateMetadataAndHook(ctx, owner, in, func(tx *gorm.DB) error {
		bound := hookManagementTxProvider{r.provider, tx}
		// Reuse the legacy diff/restore/soft-delete behavior on this transaction only.
		legacy := NewExptTemplateRepo(mysqldao.NewExptTemplateDAO(bound), mysqldao.NewExptTemplateEvaluatorRefDAO(bound), &hookManagementReservedIDs{ids: ids})
		return legacy.UpdateWithRefs(ctx, template, copies)
	})
}

func (r *hookConfigRepo) updateMetadataAndHook(ctx context.Context, owner hookcomponent.ConfigOwner, in entity.HookConfigUpdateInput, update func(*gorm.DB) error) error {
	if r == nil || r.provider == nil || r.codec == nil || !validHookConfigOwner(owner) || in.Config == nil || (in.Config.Before == nil && in.Config.After == nil) {
		return entity.ErrHookConfigStorage
	}
	current, err := r.GetConfig(ctx, owner)
	if err != nil {
		return err
	}
	if current.Revision != in.ExpectedRevision {
		return entity.ErrHookStoreConflict
	}
	conf, err := entity.ResolveLifecycleHookConf(current.Config, in.Config)
	if err != nil {
		return err
	}
	encoded, err := r.codec.EncodeConfig(ctx, in.KeyID, owner, conf)
	if err != nil || len(encoded) == 0 {
		return entity.ErrHookConfigStorage
	}
	err = r.provider.Transaction(ctx, func(tx *gorm.DB) error {
		tx = tx.Session(&gorm.Session{Logger: logger.Discard})
		row, err := lockManagementHookConfig(tx, owner)
		if err != nil {
			return err
		}
		if hookConfigRevision(row.LifecycleHookConf) != in.ExpectedRevision {
			return entity.ErrHookStoreConflict
		}
		if err := update(tx); err != nil {
			return err
		}
		return hookOneRow(tx.Table(hookConfigTable(owner)).Where("id=? AND space_id=? AND deleted_at IS NULL", owner.ObjectID, owner.WorkspaceID).UpdateColumn("lifecycle_hook_conf", encoded))
	}, db.WithMaster())
	if errors.Is(err, repo.ErrHookConfigScheduleBindingRequired) {
		return repo.ErrHookConfigScheduleBindingRequired
	}
	if err != nil {
		return hookConfigError(err)
	}
	return nil
}

func lockManagementHookConfig(tx *gorm.DB, owner hookcomponent.ConfigOwner) (*hookConfigRow, error) {
	if owner.Kind == hookcomponent.ConfigOwnerExperiment {
		row, err := readHookConfig(tx, owner, true)
		if err != nil {
			return nil, err
		}
		if row.LatestRunID != 0 {
			return nil, entity.ErrHookConfigImmutable
		}
		var log model.ExptRunLog
		err = tx.Unscoped().Select("id").Where("space_id=? AND expt_id=?", owner.WorkspaceID, owner.ObjectID).Clauses(clause.Locking{Strength: "UPDATE"}).Take(&log).Error
		if err == nil {
			return nil, entity.ErrHookConfigImmutable
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		return row, nil
	}
	var row struct {
		LifecycleHookConf  []byte
		CronActivate       bool
		ScheduleRunBinding []byte
		TemplateConf       []byte
	}
	err := tx.Table(model.TableNameExptTemplate).Select("lifecycle_hook_conf", "cron_activate", "schedule_run_binding", "template_conf").
		Where("id=? AND space_id=? AND deleted_at IS NULL", owner.ObjectID, owner.WorkspaceID).Clauses(clause.Locking{Strength: "UPDATE"}).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, entity.ErrHookStoreMissing
	}
	if err != nil {
		return nil, err
	}
	if row.CronActivate || len(row.ScheduleRunBinding) > 0 {
		return nil, repo.ErrHookConfigScheduleBindingRequired
	}
	if len(row.TemplateConf) > 0 {
		var conf entity.ExptTemplateConfiguration
		if json.Unmarshal(row.TemplateConf, &conf) != nil {
			return nil, entity.ErrHookConfigStorage
		}
		if managementTemplateHasScheduler(&conf) {
			return nil, repo.ErrHookConfigScheduleBindingRequired
		}
	}
	return &hookConfigRow{LifecycleHookConf: row.LifecycleHookConf}, nil
}

func managementTemplateHasScheduler(conf *entity.ExptTemplateConfiguration) bool {
	return conf != nil && conf.ExptSource != nil && conf.ExptSource.Scheduler != nil
}

type hookManagementTxProvider struct {
	db.Provider
	tx *gorm.DB
}

func (p hookManagementTxProvider) NewSession(ctx context.Context, opts ...db.Option) *gorm.DB {
	return p.Provider.NewSession(ctx, append(opts, db.WithTransaction(p.tx))...)
}

// The legacy repo may consume fewer IDs after reading existing refs; no generator I/O occurs under lock.
type hookManagementReservedIDs struct{ ids []int64 }

func (g *hookManagementReservedIDs) GenID(ctx context.Context) (int64, error) {
	ids, err := g.GenMultiIDs(ctx, 1)
	if err != nil {
		return 0, err
	}
	return ids[0], nil
}
func (g *hookManagementReservedIDs) GenMultiIDs(_ context.Context, n int) ([]int64, error) {
	if n < 0 || n > len(g.ids) {
		return nil, entity.ErrHookConfigStorage
	}
	out := append([]int64(nil), g.ids[:n]...)
	g.ids = g.ids[n:]
	return out, nil
}
