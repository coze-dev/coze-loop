// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"

	"github.com/bytedance/gg/gptr"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

type hookFinalizationRepo struct {
	provider db.Provider
	binding  *boundHookExecution
}

func NewHookFinalizationRepo(p db.Provider) repo.IHookFinalizationRepo {
	return &hookFinalizationRepo{provider: p}
}

func (r *hookFinalizationRepo) read(ctx context.Context, fn func(*gorm.DB) error) error {
	if ctx == nil || r == nil || r.provider == nil {
		return entity.ErrHookStoreCorrupt
	}
	return readHookSnapshot(ctx, r.provider, fn)
}

func (r *hookFinalizationRepo) ReadFinalizationSource(ctx context.Context, key entity.HookRunKey) (*entity.HookFinalizationSource, error) {
	if ctx == nil || r == nil || r.provider == nil || key.WorkspaceID <= 0 || key.ExperimentID <= 0 || key.RunID < 0 {
		return nil, entity.ErrHookStoreCorrupt
	}
	if r.binding == nil {
		source, err := r.readLegacySource(ctx, key)
		if err != nil || source != nil {
			return source, err
		}
	}
	var out *entity.HookFinalizationSource
	err := r.read(ctx, func(tx *gorm.DB) error {
		e, err := readHookRecoveryExperiment(tx, key)
		if err != nil {
			return err
		}
		if key.RunID == 0 {
			key.RunID = e.LatestRunID
		}
		if key.RunID <= 0 {
			return entity.ErrHookStoreMissing
		}
		if err := checkBoundFinalizationRun(tx, key, r.boundScope(), r.binding); err != nil {
			return err
		}
		var log model.ExptRunLog
		if err := tx.Unscoped().Where("id=?", key.RunID).First(&log).Error; err != nil {
			return err
		}
		if log.SpaceID != key.WorkspaceID || log.ExptID != key.ExperimentID || log.ExptRunID != key.RunID || log.DeletedAt.Valid || log.Status == nil || log.Mode == nil {
			return entity.ErrHookStoreCorrupt
		}
		marker := gptr.Indirect(log.LifecycleHookVersion)
		if marker != 0 && marker != 1 {
			return entity.ErrHookStoreCorrupt
		}
		expt, err := convert.NewExptConverter().PO2DO(e, nil)
		if err != nil {
			return err
		}
		if err := r.restoreExecution(expt, nil); err != nil {
			return err
		}
		rl, err := convert.NewExptRunLogConvertor().PO2DO(&log)
		if err != nil {
			return err
		}
		out = &entity.HookFinalizationSource{Key: key, Managed: marker == 1, Experiment: expt, RunLog: rl}
		return nil
	})
	return out, err
}

type hookLegacySourceRow struct {
	Experiment model.Experiment `gorm:"embedded"`
	Log        model.ExptRunLog `gorm:"embedded;embeddedPrefix:run_"`
}

const hookLegacySourceColumns = `e.*,l.id AS run_id,l.space_id AS run_space_id,l.expt_id AS run_expt_id,l.expt_run_id AS run_expt_run_id,
l.lifecycle_hook_version AS run_lifecycle_hook_version,l.deleted_at AS run_deleted_at,l.status AS run_status,l.mode AS run_mode,
l.created_by AS run_created_by,l.item_ids AS run_item_ids,l.pending_cnt AS run_pending_cnt,l.success_cnt AS run_success_cnt,l.fail_cnt AS run_fail_cnt,
l.processing_cnt AS run_processing_cnt,l.terminated_cnt AS run_terminated_cnt,l.credit_cost AS run_credit_cost,l.token_cost AS run_token_cost,
l.status_message AS run_status_message,l.created_at AS run_created_at,l.updated_at AS run_updated_at`

func (r *hookFinalizationRepo) readLegacySource(ctx context.Context, key entity.HookRunKey) (*entity.HookFinalizationSource, error) {
	var rows []hookLegacySourceRow
	q := r.provider.NewSession(ctx, db.WithMaster()).Session(&gorm.Session{Logger: logger.Discard}).
		Unscoped().Table("experiment AS e").Select(hookLegacySourceColumns)
	if key.RunID == 0 {
		q = q.Joins("LEFT JOIN expt_run_log AS l ON l.id=e.latest_run_id")
	} else {
		q = q.Joins("LEFT JOIN expt_run_log AS l ON l.id=?", key.RunID)
	}
	if err := q.Where("e.id=? AND e.space_id=?", key.ExperimentID, key.WorkspaceID).Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) != 1 {
		return nil, entity.ErrHookStoreMissing
	}
	e, log := &rows[0].Experiment, &rows[0].Log
	if key.RunID == 0 {
		key.RunID = e.LatestRunID
	}
	if key.RunID <= 0 || log.ID == 0 {
		return nil, entity.ErrHookStoreMissing
	}
	if e.ID != key.ExperimentID || e.SpaceID != key.WorkspaceID || log.ID != key.RunID || log.SpaceID != key.WorkspaceID || log.ExptID != key.ExperimentID || log.ExptRunID != key.RunID || log.DeletedAt.Valid || log.Status == nil || log.Mode == nil {
		return nil, entity.ErrHookStoreCorrupt
	}
	switch gptr.Indirect(log.LifecycleHookVersion) {
	case 1:
		// Bound/deleted recovery and all managed reads retain the original RR path.
		return nil, nil
	case 0:
		if e.DeletedAt.Valid {
			return nil, entity.ErrHookStoreMissing
		}
	default:
		return nil, entity.ErrHookStoreCorrupt
	}
	expt, err := convert.NewExptConverter().PO2DO(e, nil)
	if err != nil {
		return nil, err
	}
	rl, err := convert.NewExptRunLogConvertor().PO2DO(log)
	if err != nil {
		return nil, err
	}
	return &entity.HookFinalizationSource{Key: key, Experiment: expt, RunLog: rl}, nil
}

func (r *hookFinalizationRepo) ReadFinalizationStats(ctx context.Context, key entity.HookRunKey, scope string) (*entity.HookFinalizationStats, error) {
	var out *entity.HookFinalizationStats
	err := r.read(ctx, func(tx *gorm.DB) error {
		var err error
		out, err = readHookFinalizationStats(tx, key, scope, r.binding)
		return err
	})
	return out, err
}

// Only persisted original-Run logs and the frozen execution manifest define completeness.
func readHookFinalizationStats(tx *gorm.DB, key entity.HookRunKey, scope string, bindings ...*boundHookExecution) (*entity.HookFinalizationStats, error) {
	b := firstFinalizationBinding(bindings)
	if err := checkBoundFinalizationRun(tx, key, scope, b); err != nil {
		return nil, err
	}
	if (entity.HookStoreGuard{Key: key}).Validate() != nil || !hookGateASCII(scope) {
		return nil, entity.ErrHookStoreCorrupt
	}
	var life model.ExptLifecycleRun
	if err := hookRunScope(tx, key).First(&life).Error; err != nil {
		return nil, err
	}
	if life.ExecutionScope != scope {
		return nil, entity.ErrHookStoreConflict
	}
	if b != nil && b.source.Mode == entity.EvaluationModeRetryItems {
		if err := checkRetryItemsFinalizationCoverage(tx, key, life); err != nil {
			return nil, err
		}
	}
	if hookCancellation(&life) && !life.ExecutionInitialized {
		return readHookNeverAdmittedProof(tx, key, &life, false, b)
	}
	if hookCancellation(&life) {
		return readHookActiveTerminationStats(tx, key, &life, b)
	}
	if life.PlanState != 1 || !life.ExecutionInitialized {
		return nil, entity.ErrHookFinalizationUnsupported
	}
	if life.PlanCount < 0 || life.PlanCount > math.MaxInt32 {
		return nil, entity.ErrHookStoreCorrupt
	}
	out := &entity.HookFinalizationStats{Key: key, ExecutionScope: scope}
	var turnCount int64
	digest := entity.NewHookPlanDigest()
	for start := int64(0); start < life.PlanCount; {
		var page []model.ExptLifecycleRunItem
		if err := hookRunScope(tx, key).Where("ordinal>=?", start).Order("ordinal").Limit(100).Find(&page).Error; err != nil {
			return nil, err
		}
		if len(page) == 0 || int64(len(page)) > life.PlanCount-start {
			return nil, entity.ErrHookStoreCorrupt
		}
		pageItemIDs := make([]int64, 0, len(page))
		manifests := make([]entity.HookExecutionManifest, 0, len(page))
		for _, row := range page {
			pageItemIDs = append(pageItemIDs, row.ItemID)
			m, err := hookTerminationManifest(key, row)
			if err != nil {
				return nil, err
			}
			manifests = append(manifests, m)
		}
		if err := checkBoundFinalizationRefs(tx, key, manifests, b, false); err != nil {
			return nil, err
		}
		var itemLogs []model.ExptItemResultRunLog
		if err := hookRunScope(tx.Unscoped(), key).Where("item_id IN ?", pageItemIDs).Find(&itemLogs).Error; err != nil {
			return nil, err
		}
		var turnLogs []model.ExptTurnResultRunLog
		if err := hookRunScope(tx.Unscoped(), key).Where("item_id IN ?", pageItemIDs).Find(&turnLogs).Error; err != nil {
			return nil, err
		}
		itemsByID := make(map[int64]model.ExptItemResultRunLog, len(itemLogs))
		for _, item := range itemLogs {
			if _, exists := itemsByID[item.ItemID]; exists {
				return nil, entity.ErrHookStoreCorrupt
			}
			itemsByID[item.ItemID] = item
		}
		turnsByItem := make(map[int64][]model.ExptTurnResultRunLog, len(page))
		for _, turn := range turnLogs {
			turnsByItem[turn.ItemID] = append(turnsByItem[turn.ItemID], turn)
		}
		for i, row := range page {
			if row.Ordinal != start+int64(i) || row.ExecutionManifest == nil {
				return nil, entity.ErrHookStoreCorrupt
			}
			var m entity.HookExecutionManifest
			decoder := json.NewDecoder(bytes.NewReader(*row.ExecutionManifest))
			decoder.DisallowUnknownFields()
			if decoder.Decode(&m) != nil || decoder.Decode(new(any)) != io.EOF || m.Validate() != nil || m.Key != key || m.Ordinal != row.Ordinal || m.Frozen != executionTuple(row) {
				return nil, entity.ErrHookStoreCorrupt
			}
			var err error
			digest, err = entity.AppendHookPlanDigest(digest, []entity.HookPlanItem{m.Frozen})
			if err != nil {
				return nil, err
			}
			item, exists := itemsByID[row.ItemID]
			if !exists {
				return nil, entity.ErrHookStoreCorrupt
			}
			if item.ID != m.ItemRunLogID || item.ItemVersionID != m.Frozen.ItemVersionID || item.DeletedAt.Valid {
				return nil, entity.ErrHookStoreCorrupt
			}
			if item.ResultState == nil || *item.ResultState != int32(entity.ExptItemResultStateResulted) {
				return nil, entity.ErrHookFinalizationUnsettled
			}
			if err := addHookFinalizationCount(&out.Items, item.Status); err != nil {
				return nil, err
			}
			turns := turnsByItem[row.ItemID]
			if m.NoExecutionFailure {
				// Pinned/scoped evidence must establish absence, not missing or deleted execution.
				proof, _, err := readHookArchiveItemRows(tx, key, row.ItemID, false)
				if err != nil {
					return nil, err
				}
				if !hookNoExecutionFailure(proof) {
					return nil, entity.ErrHookStoreCorrupt
				}
				expt, err := readHookRecoveryExperiment(tx, key)
				if err != nil {
					return nil, err
				}
				if expt.LatestRunID == key.RunID {
					if err := validateHookTerminationProjection(tx, key, proof); err != nil {
						return nil, err
					}
				}
				for range m.Turns {
					if err := addHookFinalizationCount(&out.Turns, int32(entity.ItemRunState_Fail)); err != nil {
						return nil, err
					}
				}
				out.ItemIDs = append(out.ItemIDs, row.ItemID)
				continue
			}
			if len(turns) != len(m.Turns) {
				return nil, entity.ErrHookFinalizationUnsettled
			}
			if m.TurnLogsInitialized != nil && !*m.TurnLogsInitialized {
				return nil, entity.ErrHookStoreCorrupt
			}
			expected := make(map[int64]entity.HookExecutionTurnManifest, len(m.Turns))
			for _, tr := range m.Turns {
				expected[tr.TurnID] = tr
			}
			for _, tr := range turns {
				frozen, ok := expected[tr.TurnID]
				// Exact IDs prove the scoped batch is the pinned set, including after a same-count replacement.
				if !ok || tr.ItemVersionID != m.Frozen.ItemVersionID || tr.DeletedAt.Valid || m.TurnLogsInitialized != nil && tr.ID != frozen.RunLogID {
					return nil, entity.ErrHookStoreCorrupt
				}
				delete(expected, tr.TurnID)
				var status entity.ItemRunState
				switch entity.TurnRunState(tr.Status) {
				case entity.TurnRunState_Success:
					status = entity.ItemRunState_Success
				case entity.TurnRunState_Fail:
					status = entity.ItemRunState_Fail
				case entity.TurnRunState_Terminal:
					status = entity.ItemRunState_Terminal
				case entity.TurnRunState_Queueing, entity.TurnRunState_Processing:
					return nil, entity.ErrHookFinalizationUnsettled
				default:
					return nil, entity.ErrHookStoreCorrupt
				}
				if err := addHookFinalizationCount(&out.Turns, int32(status)); err != nil {
					return nil, err
				}
			}
			turnCount += int64(len(turns))
			out.ItemIDs = append(out.ItemIDs, row.ItemID)
		}
		start += int64(len(page))
	}
	if digest.Hash != gptr.Indirect(life.PlanHash) {
		return nil, entity.ErrHookStoreCorrupt
	}
	for _, check := range []struct {
		table any
		n     int64
	}{{&model.ExptLifecycleRunItem{}, life.PlanCount}, {&model.ExptItemResultRunLog{}, life.PlanCount}, {&model.ExptTurnResultRunLog{}, turnCount}} {
		var n int64
		if err := hookRunScope(tx.Unscoped().Model(check.table), key).Count(&n).Error; err != nil {
			return nil, err
		}
		if n != check.n {
			return nil, entity.ErrHookStoreCorrupt
		}
	}
	return out, nil
}

func addHookFinalizationCount(c *entity.HookFinalizationCounts, status int32) error {
	var dst *int32
	switch entity.ItemRunState(status) {
	case entity.ItemRunState_Success:
		dst = &c.Success
	case entity.ItemRunState_Fail:
		dst = &c.Fail
	case entity.ItemRunState_Terminal:
		dst = &c.Terminated
	case entity.ItemRunState_Queueing, entity.ItemRunState_Processing:
		return entity.ErrHookFinalizationUnsettled
	default:
		return entity.ErrHookStoreCorrupt
	}
	if *dst == math.MaxInt32 {
		return entity.ErrHookStoreCorrupt
	}
	*dst++
	return nil
}
