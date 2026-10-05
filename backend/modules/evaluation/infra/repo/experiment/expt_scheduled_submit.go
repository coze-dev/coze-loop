// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"encoding/json"
	"math"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"gorm.io/gorm"
)

type scheduledExptSubmissionWriter struct{}

func NewScheduledExptSubmissionWriter() repo.IScheduledExptSubmissionWriter {
	return &scheduledExptSubmissionWriter{}
}

func (*scheduledExptSubmissionWriter) Write(ctx context.Context, p db.Provider, tr entity.ScheduledRunTrigger, user string, in *entity.PreparedScheduledExpt) error {
	if ctx == nil || p == nil || in == nil || tr.Validate() != nil || !in.Binding.Active() || in.TemplateRevision == "" || in.Experiment == nil || in.Stats == nil || in.Run.RunLog == nil || len(in.ConfigCipher) == 0 {
		return entity.ErrHookStoreConflict
	}
	b, expt, log := &in.Binding, in.Experiment, in.Run.RunLog
	key := entity.HookRunKey{WorkspaceID: tr.SpaceID, ExperimentID: tr.ExperimentID, RunID: tr.RunID}
	if user != b.UserID || tr.BindingID != b.BindingID || tr.BindingVersion != b.Version || tr.SpaceID != b.SpaceID || tr.TemplateID != b.TemplateID ||
		expt.ID != tr.ExperimentID || expt.SpaceID != tr.SpaceID || expt.CreatedBy != user || expt.LatestRunID != 0 || expt.ExptTemplateMeta == nil || expt.ExptTemplateMeta.ID != tr.TemplateID ||
		expt.Status != entity.ExptStatus_Pending || expt.TriggerType != "schedule" || in.Run.Key != key || in.Run.ExpectedLatestRunID != 0 || in.Run.SourceRunID != nil ||
		log.ID != tr.RunID || log.ExptRunID != tr.RunID || log.SpaceID != tr.SpaceID || log.ExptID != tr.ExperimentID || log.CreatedBy != user ||
		in.Stats.ID <= 0 || in.Stats.SpaceID != tr.SpaceID || in.Stats.ExptID != tr.ExperimentID {
		return entity.ErrHookStoreConflict
	}
	seen := map[int64]bool{}
	for _, ref := range in.Refs {
		if ref == nil || ref.ID <= 0 || seen[ref.ID] || ref.SpaceID != tr.SpaceID || ref.ExptID != tr.ExperimentID {
			return entity.ErrHookStoreConflict
		}
		seen[ref.ID] = true
	}
	for _, mapping := range in.Mappings {
		if mapping == nil || mapping.SpaceID != tr.SpaceID || mapping.ExptID != tr.ExperimentID {
			return entity.ErrHookStoreConflict
		}
	}
	return p.Transaction(ctx, func(tx *gorm.DB) error {
		templateKey := entity.ExptTemplateScheduleBindingKey{SpaceID: b.SpaceID, TemplateID: b.TemplateID, ExecutionScope: b.ExecutionScope}
		po, refs, err := templateScheduleRows(tx, templateKey)
		if err != nil {
			return err
		}
		if templateScheduleRevision(po, refs) != in.TemplateRevision {
			return entity.ErrHookStoreConflict
		}
		if _, err := lockScheduledTriggerBinding(tx, *b); err != nil {
			return err
		}
		experiment, err := convert.NewExptConverter().DO2PO(expt)
		if err != nil {
			return err
		}
		cipher := append([]byte(nil), in.ConfigCipher...)
		experiment.LifecycleHookConf = &cipher
		if err := tx.Create(experiment).Error; err != nil {
			return err
		}
		if err := tx.Create(convert.NewExptStatsConverter().DO2PO(in.Stats)).Error; err != nil {
			return err
		}
		if len(in.Refs) > 0 {
			if err := tx.Create(convert.NewExptEvaluatorRefConverter().DO2PO(in.Refs)).Error; err != nil {
				return err
			}
		}
		for _, mapping := range in.Mappings {
			row := convert.ExptTurnResultFilterKeyMappingDO2PO(mapping)
			row.CreatedBy = user
			if err := tx.Create(row).Error; err != nil {
				return err
			}
		}
		run := in.Run
		run.ExpectedConfigRevision = hookConfigRevision(cipher)
		bound := scheduledTriggerTxProvider{provider: p, tx: tx}
		created, err := NewHookRunRepo(bound).CreateRunWithHooks(ctx, run)
		if err != nil {
			return err
		}
		if !created.Changed || created.Run == nil || created.Run.CreatedBy != user || created.Run.State.Key != key {
			return entity.ErrHookStoreConflict
		}
		var info entity.ExptInfo
		if po.ExptInfo != nil && json.Unmarshal(*po.ExptInfo, &info) != nil {
			return entity.ErrHookStoreCorrupt
		}
		if info.CreatedExptCount < 0 || info.CreatedExptCount == math.MaxInt64 {
			return entity.ErrHookStoreCorrupt
		}
		now, err := hookDBNow(tx)
		if err != nil {
			return err
		}
		info.CreatedExptCount++
		info.LatestExptID, info.LatestExptStatus, info.LatestExptStartTime, info.CronActivate = tr.ExperimentID, entity.ExptStatus_Pending, now.UnixMilli(), po.CronActivate
		raw, err := json.Marshal(info)
		if err != nil {
			return err
		}
		if err := hookOneRow(tx.Model(&model.ExptTemplate{}).Where("id=? AND space_id=?", tr.TemplateID, tr.SpaceID).UpdateColumn("expt_info", raw)); err != nil {
			return err
		}
		return ctx.Err()
	}, db.WithMaster())
}
