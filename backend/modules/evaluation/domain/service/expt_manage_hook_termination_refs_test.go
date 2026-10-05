// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"testing"

	"errors"
	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	evalconvert "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/convertor"
	evalmodel "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	targetmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target/mysql"
	targetconvert "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target/mysql/convertor"
	targetmodel "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"time"
)

type activeReferenceIDs struct{ idgen.IIDGenerator }

func persistActiveTerminationTarget(t *testing.T, f *finalizationManagerFixture, record *entity.EvalTargetRecord) {
	t.Helper()
	po, err := targetconvert.EvalTargetRecordDO2PO(record)
	require.NoError(t, err)
	require.NoError(t, f.sql.Create(po).Error)
	t.Cleanup(func() {
		require.NoError(t, f.sql.Unscoped().Where("id=?", record.ID).Delete(&targetmodel.TargetRecord{}).Error)
	})
}

func TestHookActiveTerminationRecordAndReferenceRollbackMySQL(t *testing.T) {
	f, m, ids, cleaner := activeTerminationReferenceFixture(t)
	injected := errors.New("ref table unavailable")
	require.NoError(t, f.sql.Callback().Create().Before("gorm:create").Register("active-ref-failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "expt_turn_evaluator_result_ref" {
			tx.AddError(injected)
		}
	}))
	ctx := context.Background()
	require.ErrorIs(t, f.manager.Kill(ctx, f.expt, &f.key.RunID, f.space, "cancel", nil), injected)
	require.NoError(t, f.sql.Callback().Create().Remove("active-ref-failure"))
	require.False(t, finalizationRead(t, f).State.After.Activated)
	var item model.ExptItemResultRunLog
	require.NoError(t, f.sql.First(&item, m.ItemRunLogID).Error)
	require.NotEqual(t, int32(entity.ExptItemResultStateResulted), gptr.Indirect(item.ResultState))
	var tr model.ExptTurnResult
	require.NoError(t, f.sql.First(&tr, m.Turns[0].ResultID).Error)
	require.Equal(t, int32(entity.TurnRunState_Processing), tr.Status, "projection and refs roll back together")
	require.NoError(t, f.manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}))
	require.Equal(t, 2, cleaner.calls)
	var record evalmodel.EvaluatorRecord
	require.NoError(t, f.sql.First(&record, ids[2]).Error)
	var output map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(gptr.Indirect(record.OutputData), &output))
	require.JSONEq(t, `{"keep":true}`, string(output["custom"]))
	require.Contains(t, output, "evaluator_run_error")
}

type activeCommitReferenceProbe struct {
	repo.IHookRepo
	before func()
}

func (p activeCommitReferenceProbe) AcceptTermination(ctx context.Context, in entity.HookFinalizeInput) (entity.HookStoreResult, error) {
	return p.IHookRepo.(repo.IHookTerminationRepo).AcceptTermination(ctx, in)
}
func (p activeCommitReferenceProbe) CommitFinalize(ctx context.Context, in entity.HookFinalizeInput) (entity.HookStoreResult, error) {
	p.before()
	return p.IHookRepo.CommitFinalize(ctx, in)
}

func TestHookActiveTerminationCommitRechecksLateReferencesMySQL(t *testing.T) {
	f, m, ids, _ := activeTerminationReferenceFixture(t)
	added := finalizationTestIDs.Add(1)
	require.NoError(t, f.sql.Create(&evalmodel.EvaluatorRecord{ID: added, SpaceID: f.space, ExperimentID: gptr.Of(f.expt), ExperimentRunID: f.key.RunID, ItemID: m.Frozen.ItemID, ItemVersionID: m.Frozen.ItemVersionID, Status: 1, EvaluatorVersionID: 93, Alias_: "late", SourceType: 1}).Error)
	t.Cleanup(func() {
		require.NoError(t, f.sql.Unscoped().Where("id=?", added).Delete(&evalmodel.EvaluatorRecord{}).Error)
	})
	f.deps.Runs = activeCommitReferenceProbe{IHookRepo: f.repo, before: func() {
		var row model.ExptTurnResultRunLog
		require.NoError(t, f.sql.Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).First(&row).Error)
		refs := &entity.EvaluatorResults{}
		require.NoError(t, json.Unmarshal(gptr.Indirect(row.EvaluatorResultIds), refs))
		refs.Registered = append(refs.Registered, &entity.RegisteredEvalResult{VersionID: 93, Alias: "late", RecordID: added})
		raw, err := json.Marshal(refs)
		require.NoError(t, err)
		require.NoError(t, f.sql.Model(&model.ExptTurnResultRunLog{}).Where("id=?", row.ID).UpdateColumn("evaluator_result_ids", raw).Error)
	}}
	finalizationRecreate(t, f)
	ctx := context.Background()
	require.Error(t, f.manager.Kill(ctx, f.expt, &f.key.RunID, f.space, "cancel", nil))
	require.False(t, finalizationRead(t, f).State.After.Activated)
	var oldRefs []model.ExptTurnEvaluatorResultRef
	require.NoError(t, f.sql.Where("space_id=? AND expt_id=?", f.space, f.expt).Find(&oldRefs).Error)
	require.Len(t, oldRefs, 3)
	f.deps.Runs = f.repo
	finalizationRecreate(t, f)
	require.NoError(t, f.manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}))
	var newRefs []model.ExptTurnEvaluatorResultRef
	require.NoError(t, f.sql.Where("space_id=? AND expt_id=?", f.space, f.expt).Find(&newRefs).Error)
	require.Len(t, newRefs, 4)
	byID := map[int64]int64{}
	for _, ref := range newRefs {
		byID[ref.EvaluatorResultID] = ref.ID
	}
	for _, ref := range oldRefs {
		require.Equal(t, ref.ID, byID[ref.EvaluatorResultID])
	}
	require.NotZero(t, byID[ids[1]])
	require.NotZero(t, byID[added])
}

func TestHookActiveTerminationNormalArchiveThenCancelMySQL(t *testing.T) {
	f, m, _, _ := activeTerminationReferenceFixture(t)
	require.NoError(t, f.sql.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).UpdateColumn("status", int32(entity.TurnRunState_Success)).Error)
	require.NoError(t, f.sql.Model(&model.ExptItemResultRunLog{}).Where("id=?", m.ItemRunLogID).UpdateColumn("status", int32(entity.ItemRunState_Success)).Error)
	require.NoError(t, f.sql.Model(&model.ExptStats{}).Where("space_id=? AND expt_id=?", f.space, f.expt).UpdateColumns(map[string]any{"pending_cnt": 0, "processing_cnt": 1}).Error)
	base := f.base.(*ExptMangerImpl).exptResultService.(*ExptResultServiceImpl)
	base.idgen = activeReferenceIDs{}
	svc, err := base.WithHookArchive(f.deps.Repository.(repo.IHookItemArchiveRepo), "local")
	require.NoError(t, err)
	ctx := context.Background()
	source, err := f.deps.Repository.ReadFinalizationSource(ctx, f.key)
	require.NoError(t, err)
	refs, err := svc.RecordItemRunLogs(ctx, f.expt, f.key.RunID, m.Frozen.ItemID, f.space, source.Experiment)
	require.NoError(t, err)
	require.Len(t, refs, 3)
	var stats model.ExptStats
	require.NoError(t, f.sql.Where("space_id=? AND expt_id=?", f.space, f.expt).First(&stats).Error)
	require.Equal(t, int32(1), stats.SuccessCnt)
	require.Zero(t, stats.ProcessingCnt)
	var before model.ExptTurnResult
	require.NoError(t, f.sql.First(&before, m.Turns[0].ResultID).Error)
	require.Equal(t, 0.75, gptr.Indirect(before.WeightedScore))
	require.NoError(t, f.manager.Kill(ctx, f.expt, &f.key.RunID, f.space, "cancel", nil))
	var after model.ExptTurnResult
	require.NoError(t, f.sql.First(&after, m.Turns[0].ResultID).Error)
	require.Equal(t, before, after, "completed read-side result must stay intact")
}

func TestHookActiveTerminationRejectsExtraProjectionMySQL(t *testing.T) {
	f, _ := activeTerminationFixture(t, entity.ItemRunState_Queueing)
	require.NoError(t, f.sql.Create(&model.ExptTurnResult{ID: finalizationTestIDs.Add(1), SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ItemID: finalizationTestIDs.Add(1), Status: int32(entity.TurnRunState_Success)}).Error)
	require.ErrorIs(t, f.manager.Kill(context.Background(), f.expt, &f.key.RunID, f.space, "cancel", nil), entity.ErrHookStoreCorrupt)
	require.False(t, finalizationRead(t, f).State.After.Activated)
}

func TestHookActiveTerminationRejectsUnownedExistingReferenceMySQL(t *testing.T) {
	f, ms := activeTerminationFixtureDB(t, "tx", entity.ItemRunState_Queueing)
	ref := &model.ExptTurnEvaluatorResultRef{ID: finalizationTestIDs.Add(1), SpaceID: f.space, ExptID: f.expt, ExptTurnResultID: ms[0].Turns[0].ResultID, EvaluatorVersionID: 93, EvaluatorResultID: 999}
	require.NoError(t, f.sql.Create(ref).Error)
	t.Cleanup(func() {
		require.NoError(t, f.sql.Unscoped().Where("id=?", ref.ID).Delete(&model.ExptTurnEvaluatorResultRef{}).Error)
	})
	require.ErrorIs(t, f.manager.Kill(context.Background(), f.expt, &f.key.RunID, f.space, "cancel", nil), entity.ErrHookStoreCorrupt)
	var item model.ExptItemResultRunLog
	require.NoError(t, f.sql.First(&item, ms[0].ItemRunLogID).Error)
	require.NotEqual(t, int32(entity.ExptItemResultStateResulted), gptr.Indirect(item.ResultState))
}
func (activeReferenceIDs) GenMultiIDs(_ context.Context, n int) ([]int64, error) {
	ids := make([]int64, n)
	for i := range ids {
		ids[i] = finalizationTestIDs.Add(1)
	}
	return ids, nil
}

type activeStoredEvaluatorReader struct {
	EvaluatorRecordService
	sql *gorm.DB
}

func (r activeStoredEvaluatorReader) BatchGetEvaluatorRecord(ctx context.Context, ids []int64, deleted, full bool) ([]*entity.EvaluatorRecord, error) {
	var rows []evalmodel.EvaluatorRecord
	if err := r.sql.WithContext(ctx).Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	var out []*entity.EvaluatorRecord
	for _, row := range rows {
		record, err := evalconvert.ConvertEvaluatorRecordPO2DO(&row)
		if err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	return out, nil
}

type activeStoredTargetCleaner struct {
	IEvalTargetService
	dao   targetmysql.EvalTargetRecordDAO
	fail  bool
	calls int
	after func(context.Context)
}

func (c *activeStoredTargetCleaner) GetRecordByID(ctx context.Context, space, id int64) (*entity.EvalTargetRecord, error) {
	po, err := c.dao.GetByIDAndSpaceID(ctx, id, space)
	if err != nil {
		return nil, err
	}
	return targetconvert.EvalTargetRecordPO2DO(po)
}
func (c *activeStoredTargetCleaner) CleanupHookTargetSandboxes(ctx context.Context, key entity.HookRunKey, records []*entity.EvalTargetRecord) error {
	c.calls++
	if c.after != nil {
		c.after(ctx)
	}
	if c.fail {
		return entity.ErrHookFinalizationUnsettled
	}
	return nil
}

type activeCancelableLease struct {
	lock.ILocker
	cancel context.CancelFunc
}

func (l *activeCancelableLease) LockWithRenew(ctx context.Context, key string, ttl, max time.Duration) (bool, context.Context, func(), error) {
	ok, held, cancel, err := l.ILocker.LockWithRenew(ctx, key, ttl, max)
	l.cancel = cancel
	return ok, held, cancel, err
}

func TestHookActiveTerminationLeaseLossLeavesPendingMySQL(t *testing.T) {
	f, m, _, cleaner := activeTerminationReferenceFixture(t)
	var lease *activeCancelableLease
	f.manager.finalization.NewItemLocker = func() lock.ILocker {
		lease = &activeCancelableLease{ILocker: lock.NewRedisLocker(f.redis)}
		return lease
	}
	cleaner.after = func(context.Context) { lease.cancel() }
	require.ErrorIs(t, f.manager.Kill(context.Background(), f.expt, &f.key.RunID, f.space, "cancel", nil), context.Canceled)
	require.False(t, finalizationRead(t, f).State.After.Activated)
	var item model.ExptItemResultRunLog
	require.NoError(t, f.sql.First(&item, m.ItemRunLogID).Error)
	require.NotEqual(t, int32(entity.ExptItemResultStateResulted), gptr.Indirect(item.ResultState))
	cleaner.after = nil
	require.NoError(t, f.manager.FinalizeRun(context.Background(), f.key, entity.HookTerminalIntent{}))
	require.True(t, finalizationRead(t, f).State.After.Activated)
}

func activeTerminationReferenceFixture(t *testing.T) (*finalizationManagerFixture, entity.HookExecutionManifest, []int64, *activeStoredTargetCleaner) {
	t.Helper()
	f, ms := activeTerminationFixtureDB(t, "tx", entity.ItemRunState_Processing)
	m := ms[0]
	ids := []int64{finalizationTestIDs.Add(1), finalizationTestIDs.Add(1), finalizationTestIDs.Add(1), finalizationTestIDs.Add(1)}
	target := targetmodel.TargetRecord{ID: ids[0], SpaceID: f.space + 500, TargetID: 91, TargetVersionID: 92, ExperimentRunID: f.key.RunID, ItemID: m.Frozen.ItemID, ItemVersionID: m.Frozen.ItemVersionID, Status: 3, Ext: []byte(`{"sandbox_execute_id":"owned-execute"}`)}
	require.NoError(t, f.sql.Create(&target).Error)
	t.Cleanup(func() {
		require.NoError(t, f.sql.Unscoped().Where("id=?", ids[0]).Delete(&targetmodel.TargetRecord{}).Error)
		require.NoError(t, f.sql.Unscoped().Where("id IN ?", ids[1:]).Delete(&evalmodel.EvaluatorRecord{}).Error)
		require.NoError(t, f.sql.Unscoped().Where("space_id=? AND expt_id=?", f.space, f.expt).Delete(&model.ExptTurnEvaluatorResultRef{}).Error)
	})
	for i, id := range ids[1:] {
		r := evalmodel.EvaluatorRecord{ID: id, SpaceID: f.space, ExperimentID: gptr.Of(f.expt), ExperimentRunID: f.key.RunID, ItemID: m.Frozen.ItemID, ItemVersionID: m.Frozen.ItemVersionID, EvaluatorVersionID: 93, Status: 1, Score: gptr.Of(0.75), SourceType: 1, Alias_: "a"}
		r.OutputData = gptr.Of([]byte(`{"evaluator_result":{"score":0.75},"ext":{"evidence":"preserved"},"custom":{"keep":true}}`))
		if i == 1 {
			r.Alias_ = "b"
			r.Status = 3
		}
		if i == 2 {
			r.Alias_ = ""
			r.SourceType = 2
			r.InlineKey = "quality"
			r.EvaluatorVersionID = 0
			r.TargetRecordID = ids[0]
		}
		require.NoError(t, f.sql.Create(&r).Error)
	}
	refs := &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: 93, Alias: "a", RecordID: ids[1]}, {VersionID: 93, Alias: "b", RecordID: ids[2]}}, Inline: []*entity.InlineEvalResult{{InlineKey: "quality", RecordID: ids[3]}}}
	raw, err := json.Marshal(refs)
	require.NoError(t, err)
	require.NoError(t, f.sql.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).UpdateColumns(map[string]any{"target_result_id": ids[0], "evaluator_result_ids": raw}).Error)
	require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumns(map[string]any{"target_id": 91, "target_version_id": 92, "target_space_id": target.SpaceID}).Error)
	cleaner := &activeStoredTargetCleaner{dao: targetmysql.NewEvalTargetRecordDAO(f.p)}
	f.base.(*ExptMangerImpl).evalTargetService = cleaner
	f.base.(*ExptMangerImpl).idgenerator = activeReferenceIDs{}
	f.base.(*ExptMangerImpl).exptResultService = &ExptResultServiceImpl{evaluatorRecordService: activeStoredEvaluatorReader{sql: f.sql}, scoreCalculator: NewEvaluatorScoreCalculator(nil, nil)}
	finalizationRecreate(t, f)
	return f, m, ids, cleaner
}

func TestHookActiveTerminationReferenceIdentityMySQL(t *testing.T) {
	for _, column := range []string{"experiment_run_id", "experiment_id", "item_id", "item_version_id", "turn_id", "evaluator_version_id", "space_id", "target_record_id"} {
		t.Run(column, func(t *testing.T) {
			f, _, ids, _ := activeTerminationReferenceFixture(t)
			id := ids[1]
			if column == "target_record_id" {
				id = ids[3]
			}
			require.NoError(t, f.sql.Model(&evalmodel.EvaluatorRecord{}).Where("id=?", id).UpdateColumn(column, 777).Error)
			require.ErrorIs(t, f.manager.Kill(context.Background(), f.expt, &f.key.RunID, f.space, "cancel", nil), entity.ErrHookStoreCorrupt)
			require.False(t, finalizationRead(t, f).State.After.Activated)
		})
	}
}

func TestHookActiveTerminationReferencesMySQL(t *testing.T) {
	f, m, ids, cleaner := activeTerminationReferenceFixture(t)
	cleaner.fail = true
	ctx := context.Background()
	require.Error(t, f.manager.Kill(ctx, f.expt, &f.key.RunID, f.space, "cancel", nil))
	cleaner.fail = false
	require.NoError(t, f.manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}))
	var refs []model.ExptTurnEvaluatorResultRef
	require.NoError(t, f.sql.Where("space_id=? AND expt_id=?", f.space, f.expt).Order("evaluator_result_id").Find(&refs).Error)
	require.Len(t, refs, 3)
	for i, ref := range refs {
		require.Equal(t, ids[i+1], ref.EvaluatorResultID)
		require.Equal(t, m.Turns[0].ResultID, ref.ExptTurnResultID)
	}
	require.Equal(t, "a", refs[0].Alias_)
	require.Equal(t, "b", refs[1].Alias_)
	require.Equal(t, "quality", refs[2].InlineKey)
	var stored model.ExptTurnResult
	require.NoError(t, f.sql.First(&stored, m.Turns[0].ResultID).Error)
	require.Equal(t, ids[0], stored.TargetResultID)
	require.NotNil(t, stored.WeightedScore, "Resulted includes required read-side score materialization")
	require.Equal(t, 0.75, gptr.Indirect(stored.WeightedScore))
	var rows []evalmodel.EvaluatorRecord
	require.NoError(t, f.sql.Where("id IN ?", ids[1:]).Order("id").Find(&rows).Error)
	require.Equal(t, 0.75, gptr.Indirect(rows[0].Score))
	require.Equal(t, int32(1), rows[0].Status)
	require.Equal(t, int32(entity.EvaluatorRunStatusFail), rows[1].Status, "unfinished evaluator record must be conditionally closed")
	var target targetmodel.TargetRecord
	require.NoError(t, f.sql.First(&target, ids[0]).Error)
	require.Equal(t, int32(entity.EvalTargetRunStatusFail), target.Status, "unfinished target must be closed without losing execute metadata")
	require.JSONEq(t, `{"sandbox_execute_id":"owned-execute"}`, string(target.Ext))
	require.NoError(t, f.manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}))
	var count int64
	require.NoError(t, f.sql.Model(&model.ExptTurnEvaluatorResultRef{}).Where("space_id=? AND expt_id=?", f.space, f.expt).Count(&count).Error)
	require.Equal(t, int64(3), count)
}
