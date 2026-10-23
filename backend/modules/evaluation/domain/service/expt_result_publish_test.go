// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	componentmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	idmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	entitymocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity/mocks"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	experimentrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	experimentmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"
)

func publicationFixture(t *testing.T) (*lateWriteFixture, *ExptResultServiceImpl, *svcmocks.MockEvaluatorRecordService, *entity.Experiment) {
	t.Helper()
	f := newLateWriteFixture(t, "rq1")
	// Keep one legal turn0; the version-zero record remains legacy-compatible.
	require.NoError(t, f.conn.Unscoped().Where("space_id=? AND turn_id<>0", f.space).Delete(&model.ExptTurnResultRunLog{}).Error)
	require.NoError(t, f.conn.Unscoped().Where("space_id=? AND turn_id<>0", f.space).Delete(&model.ExptTurnResult{}).Error)
	require.NoError(t, f.conn.Where("space_id=?", f.space).Delete(&model.ExptTurnEvaluatorResultRef{}).Error)
	refs, err := json.Marshal(&entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: f.base + 51, Alias: "alias", RecordID: f.base + 61}}})
	require.NoError(t, err)
	require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=?", f.space).Updates(map[string]any{"status": int32(entity.TurnRunState_Success), "evaluator_result_ids": refs}).Error)
	require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Updates(map[string]any{"status": int32(entity.ItemRunState_Success), "result_state": int32(entity.ExptItemResultStateLogged)}).Error)
	require.NoError(t, f.conn.Model(&model.ExptTurnResult{}).Where("space_id=?", f.space).Update("weighted_score", 77).Error)
	ctrl := gomock.NewController(t)
	records := svcmocks.NewMockEvaluatorRecordService(ctrl)
	target := svcmocks.NewMockIEvalTargetService(ctrl)
	gen := idmocks.NewMockIIDGenerator(ctrl)
	var seq atomic.Int64
	gen.EXPECT().GenMultiIDs(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, n int) ([]int64, error) {
		out := make([]int64, n)
		for i := range out {
			out[i] = f.base + 3000 + seq.Add(1)
		}
		return out, nil
	}).AnyTimes()
	target.EXPECT().GetRecordByID(gomock.Any(), f.space, f.base+41).Return(&entity.EvalTargetRecord{ID: f.base + 41, SpaceID: f.space, ExperimentRunID: f.run, ItemID: f.item, TurnID: 0, Status: gptr.Of(entity.EvalTargetRunStatusSuccess)}, nil).AnyTimes()
	e := &ExptResultServiceImpl{ExptItemResultRepo: f.itemRepo(), ExptTurnResultRepo: f.turnRepo(), ExptStatsRepo: experimentrepo.NewExptStatsRepo(experimentmysql.NewExptStatsDAO(f.provider)), idgen: gen, evaluatorRecordService: records, evalTargetService: target, scoreCalculator: NewEvaluatorScoreCalculator(nil, nil), resultPublishRepo: experimentrepo.NewExptResultPublishRepo(f.provider), resultPublishLocker: lateWriteLocker(t, ctrl)}
	expt := &entity.Experiment{ID: f.expt, SpaceID: f.space, LatestRunID: f.run, EvalConf: &entity.EvaluationConfiguration{}}
	return f, e, records, expt
}
func publicationRecord(f *lateWriteFixture, score *float64) *entity.EvaluatorRecord {
	return &entity.EvaluatorRecord{ID: f.base + 61, SpaceID: f.space, ExperimentID: f.expt, ExperimentRunID: f.run, ItemID: f.item, TurnID: 0, EvaluatorVersionID: f.base + 51, Alias: "alias", Status: entity.EvaluatorRunStatusSuccess, EvaluatorOutputData: &entity.EvaluatorOutputData{EvaluatorResult: &entity.EvaluatorResult{Score: score}}}
}

func TestResultPublishDirectRealRollback(t *testing.T) {
	for _, table := range []string{"expt_item_result", "expt_item_result_run_log", "expt_stats"} {
		t.Run(table, func(t *testing.T) {
			f, svc, records, expt := publicationFixture(t)
			records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), false, false).Return([]*entity.EvaluatorRecord{publicationRecord(f, gptr.Of(0.0))}, nil).AnyTimes()
			before := f.snapshot()
			fault := errors.New("publication write fault")
			hits := 0
			hook := "publish_fault_" + table
			require.NoError(t, f.conn.Callback().Update().Before("gorm:update").Register(hook, func(tx *gorm.DB) {
				if tx.Statement.Table == table {
					hits++
					_ = tx.AddError(fault)
				}
			}))
			_, err := svc.RecordItemRunLogs(context.Background(), f.expt, f.run, f.item, f.space, expt)
			require.NoError(t, f.conn.Callback().Update().Remove(hook))
			require.Positive(t, hits)
			require.ErrorIs(t, err, fault)
			require.Equal(t, before, f.snapshot(), "publication must roll back refs, score, item/resulted and stats together")
		})
	}
}
func TestResultPublishDirectStalePreparation(t *testing.T) {
	f, svc, records, expt := publicationFixture(t)
	var afterMutation map[string]string
	records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), false, false).DoAndReturn(func(context.Context, []int64, bool, bool) ([]*entity.EvaluatorRecord, error) {
		f.token("rq2")
		afterMutation = f.snapshot()
		return []*entity.EvaluatorRecord{publicationRecord(f, gptr.Of(0.0))}, nil
	}).AnyTimes()
	_, err := svc.RecordItemRunLogs(context.Background(), f.expt, f.run, f.item, f.space, expt)
	require.NoError(t, err)
	require.Equal(t, afterMutation, f.snapshot(), "old prepared record must not overwrite new request")
}
func TestResultPublishDirectNoOldScore(t *testing.T) {
	for _, kind := range []string{"no_score", "read_error", "missing_record"} {
		t.Run(kind, func(t *testing.T) {
			f, svc, records, expt := publicationFixture(t)
			before := f.snapshot()
			var result []*entity.EvaluatorRecord
			var fault error
			if kind == "read_error" {
				fault = errors.New("evaluator storage unavailable")
			} else if kind == "no_score" {
				result = []*entity.EvaluatorRecord{publicationRecord(f, nil)}
			}
			records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), false, false).Return(result, fault).AnyTimes()
			_, err := svc.RecordItemRunLogs(context.Background(), f.expt, f.run, f.item, f.space, expt)
			if kind != "no_score" {
				require.Error(t, err)
				require.Equal(t, before, f.snapshot())
			} else {
				require.NoError(t, err)
				var turn model.ExptTurnResult
				require.NoError(t, f.conn.Where("space_id=?", f.space).Take(&turn).Error)
				assert.Nil(t, turn.WeightedScore, "no new score writes SQL NULL, never old77")
			}
		})
	}
}

func TestResultPublishSchedulerStaleNoNotification(t *testing.T) {
	f, svc, records, expt := publicationFixture(t)
	ctrl := gomock.NewController(t)
	expt.EvalSet = &entity.EvaluationSet{ID: f.base + 70, EvaluationSetVersion: &entity.EvaluationSetVersion{ID: f.base + 71}}
	dataset := svcmocks.NewMockEvaluationSetItemService(ctrl)
	dataset.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).Return([]*entity.EvaluationSetItem{{ItemID: f.item, EvaluationSetID: f.base + 70}}, nil).AnyTimes()
	publisher := &stubItemCompletePublisher{}
	mode := entitymocks.NewMockExptSchedulerMode(ctrl)
	mode.EXPECT().PublishResult(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	refsService := &publicationSchedulerResult{ExptResultServiceImpl: svc}
	eventPublisher := eventmocks.NewMockExptEventPublisher(ctrl)
	eventPublisher.EXPECT().PublishExptTurnResultFilterEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	// Preparing from source rq1 must not send a success notification after rq2 changes it.
	var afterMutation map[string]string
	records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), false, false).DoAndReturn(func(context.Context, []int64, bool, bool) ([]*entity.EvaluatorRecord, error) {
		f.token("rq2")
		afterMutation = f.snapshot()
		return []*entity.EvaluatorRecord{publicationRecord(f, gptr.Of(0.0))}, nil
	}).AnyTimes()
	scheduler := &ExptSchedulerImpl{ResultSvc: refsService, ExptItemResultRepo: f.itemRepo(), itemCompletePublisher: publisher, evaluationSetItemService: dataset, Publisher: eventPublisher}
	err := scheduler.recordEvalItemRunLogs(context.Background(), &entity.ExptScheduleEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run}, []*entity.ExptEvalItem{{ItemID: f.item, State: entity.ItemRunState_Success, RequestID: "rq1"}}, mode, expt)
	require.NoError(t, err)
	require.Empty(t, publisher.events, "stale prepared result cannot publish success")
	require.Equal(t, afterMutation, f.snapshot())
}

// Publication stays real; the unrelated filter projection is outside task4.1.
type publicationSchedulerResult struct{ *ExptResultServiceImpl }

func (*publicationSchedulerResult) UpsertExptTurnResultFilter(context.Context, int64, int64, []int64) error {
	return nil
}

func TestResultPublishPreparedRealWritepoints(t *testing.T) {
	for _, point := range []string{"delete_ref", "insert_ref", "turn", "item", "resulted", "stats"} {
		t.Run(point, func(t *testing.T) {
			f, svc, records, expt := publicationFixture(t)
			records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), false, false).Return([]*entity.EvaluatorRecord{publicationRecord(f, gptr.Of(0.0))}, nil)
			source, err := svc.GetItemResultPublishSource(context.Background(), f.space, f.expt, f.run, f.item)
			require.NoError(t, err)
			publication, err := svc.PrepareItemResultPublication(context.Background(), source, expt)
			require.NoError(t, err)
			before := f.snapshot()
			fault := errors.New("fault " + point)
			hits := 0
			hook := "publication_point_" + point
			callback := func(tx *gorm.DB) {
				table := tx.Statement.Table
				if point == "delete_ref" && table == "expt_turn_evaluator_result_ref" || point == "insert_ref" && table == "expt_turn_evaluator_result_ref" || point == "turn" && table == "expt_turn_result" || point == "item" && table == "expt_item_result" || point == "resulted" && table == "expt_item_result_run_log" || point == "stats" && table == "expt_stats" {
					hits++
					_ = tx.AddError(fault)
				}
			}
			if point == "delete_ref" {
				require.NoError(t, f.conn.Callback().Delete().Before("gorm:delete").Register(hook, callback))
			} else if point == "insert_ref" {
				require.NoError(t, f.conn.Callback().Create().Before("gorm:create").Register(hook, callback))
			} else {
				require.NoError(t, f.conn.Callback().Update().Before("gorm:update").Register(hook, callback))
			}
			applied, err := svc.PublishPreparedItemResult(context.Background(), publication, nil)
			if point == "delete_ref" {
				require.NoError(t, f.conn.Callback().Delete().Remove(hook))
			} else if point == "insert_ref" {
				require.NoError(t, f.conn.Callback().Create().Remove(hook))
			} else {
				require.NoError(t, f.conn.Callback().Update().Remove(hook))
			}
			require.Positive(t, hits)
			require.ErrorIs(t, err, fault)
			require.False(t, applied)
			require.Equal(t, before, f.snapshot(), "all eleven tables including receipt must roll back")
		})
	}
}

func TestResultPublishCurrentOnceAndFirstFailure(t *testing.T) {
	for _, kind := range []string{"canonical_processing", "canonical_success", "first_failed", "registered_inline", "old_map"} {
		t.Run(kind, func(t *testing.T) {
			f, svc, records, expt := publicationFixture(t)
			if kind == "canonical_success" {
				require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("status", int32(entity.ItemRunState_Success)).Error)
				require.NoError(t, f.conn.Model(&model.ExptStats{}).Where("space_id=?", f.space).Updates(map[string]any{"processing_cnt": 1, "success_cnt": 1}).Error)
			}
			if kind == "first_failed" {
				require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("status", int32(entity.ItemRunState_Fail)).Error)
				require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=?", f.space).Updates(map[string]any{"status": int32(entity.TurnRunState_Fail), "evaluator_result_ids": []byte(`null`)}).Error)
				require.NoError(t, f.conn.Create(&model.ExptTurnResult{ID: f.base + 420, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ItemID: f.item, ItemVersionID: f.version, TurnID: f.base + 22, Status: int32(entity.TurnRunState_Processing), WeightedScore: gptr.Of(88.0)}).Error)
				require.NoError(t, f.conn.Create(&model.ExptTurnResultRunLog{ID: f.base + 520, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ItemID: f.item, ItemVersionID: f.version, TurnID: f.base + 22, Status: int32(entity.TurnRunState_Queueing)}).Error)
			} else if kind == "old_map" {
				raw, _ := json.Marshal(&entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{f.base + 51: f.base + 61}})
				require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=?", f.space).Update("evaluator_result_ids", raw).Error)
			}
			result := publicationRecord(f, gptr.Of(0.0))
			if kind == "old_map" {
				result.Alias = ""
			}
			if kind == "registered_inline" {
				raw, _ := json.Marshal(&entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: f.base + 51, Alias: "alias", RecordID: f.base + 61}}, Inline: []*entity.InlineEvalResult{{InlineKey: "inline", RecordID: f.base + 62}}})
				require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=?", f.space).Update("evaluator_result_ids", raw).Error)
				inline := *result
				inline.ID = f.base + 62
				inline.SourceType = entity.EvaluatorRecordSourceTypeInline
				inline.InlineKey = "inline"
				inline.EvaluatorVersionID = 0
				inline.Alias = ""
				inline.EvaluatorOutputData = &entity.EvaluatorOutputData{EvaluatorResult: &entity.EvaluatorResult{Score: gptr.Of(100.0)}}
				records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), false, false).Return([]*entity.EvaluatorRecord{result, &inline}, nil).Times(1)
			} else if kind != "first_failed" {
				records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), false, false).Return([]*entity.EvaluatorRecord{result}, nil).Times(1)
			}
			before := f.snapshot()
			refs, err := svc.RecordItemRunLogs(context.Background(), f.expt, f.run, f.item, f.space, expt)
			require.NoError(t, err)
			stats, err := svc.ExptStatsRepo.Get(context.Background(), f.expt, f.space)
			require.NoError(t, err)
			require.Equal(t, int32(1), stats.ProcessingItemCnt)
			if kind == "first_failed" {
				require.Equal(t, int32(1), stats.FailItemCnt)
			} else {
				require.Equal(t, int32(1), stats.SuccessItemCnt)
			}
			after := f.snapshot()
			for _, table := range []string{"experiment", "expt_run_log", "eval_target_record", "evaluator_record", "expt_item_ref", "expt_turn_result_run_log"} {
				require.Equal(t, before[table], after[table], table)
			}
			turns, err := svc.ExptItemResultRepo.GetItemTurnResults(context.Background(), f.space, f.expt, f.item)
			require.NoError(t, err)
			for _, turn := range turns {
				if kind == "first_failed" {
					require.Nil(t, turn.WeightedScore)
				} else {
					require.Equal(t, 0.0, gptr.Indirect(turn.WeightedScore))
				}
			}
			if kind == "registered_inline" {
				require.Len(t, refs, 2)
				var inline bool
				for _, ref := range refs {
					inline = inline || ref.InlineKey == "inline"
				}
				require.True(t, inline)
			}
			_, err = svc.RecordItemRunLogs(context.Background(), f.expt, f.run, f.item, f.space, expt)
			require.NoError(t, err)
			require.Equal(t, after, f.snapshot(), "Resulted repetition must make no writes, records rereads or counters")
		})
	}
}

func TestResultPublishPreparedSourceChange(t *testing.T) {
	for _, kind := range []string{"request", "targetref", "evaluatorref", "canonical", "missing_turn", "corrupt_json", "cancelled"} {
		t.Run(kind, func(t *testing.T) {
			f, svc, records, expt := publicationFixture(t)
			records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), false, false).Return([]*entity.EvaluatorRecord{publicationRecord(f, gptr.Of(0.0))}, nil)
			source, err := svc.GetItemResultPublishSource(context.Background(), f.space, f.expt, f.run, f.item)
			require.NoError(t, err)
			publication, err := svc.PrepareItemResultPublication(context.Background(), source, expt)
			require.NoError(t, err)
			ctx := context.Background()
			switch kind {
			case "request":
				f.token("rq2")
			case "targetref":
				require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=?", f.space).Update("target_result_id", f.base+42).Error)
			case "evaluatorref":
				require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=?", f.space).Update("evaluator_result_ids", []byte(`{"registered":[{"version_id":51,"alias":"new","record_id":62}]}`)).Error)
			case "canonical":
				require.NoError(t, f.conn.Model(&model.ExptTurnResult{}).Where("space_id=?", f.space).Update("weighted_score", 66).Error)
			case "missing_turn":
				require.NoError(t, f.conn.Where("space_id=?", f.space).Delete(&model.ExptTurnResultRunLog{}).Error)
			case "corrupt_json":
				require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=?", f.space).Update("evaluator_result_ids", []byte(`{"registered":`)).Error)
			case "cancelled":
				child, cancel := context.WithCancel(ctx)
				cancel()
				ctx = child
			}
			before := f.snapshot()
			sent := false
			applied, err := svc.PublishPreparedItemResult(ctx, publication, func(context.Context, *entity.ExptItemResultPublishSource) error { sent = true; return nil })
			if kind == "missing_turn" || kind == "corrupt_json" || kind == "cancelled" {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.False(t, applied)
			require.False(t, sent)
			require.Equal(t, before, f.snapshot())
		})
	}
}

func TestResultPublishCanonicalTerminalAbsorbsLateLog(t *testing.T) {
	f, svc, records, expt := publicationFixture(t)
	require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("status", int32(entity.ItemRunState_Terminal)).Error)
	before := f.snapshot()
	records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), false, false).Return([]*entity.EvaluatorRecord{publicationRecord(f, gptr.Of(0.0))}, nil).AnyTimes()
	_, err := svc.RecordItemRunLogs(context.Background(), f.expt, f.run, f.item, f.space, expt)
	require.NoError(t, err)
	require.Equal(t, before, f.snapshot(), "late successful log cannot reverse canonical Terminal")
}

func TestResultPublishSchedulerSendCommitRetry(t *testing.T) {
	for _, kind := range []string{"send_failure", "commit_failure", "mode_failure"} {
		t.Run(kind, func(t *testing.T) {
			f, svc, records, expt := publicationFixture(t)
			ctrl := gomock.NewController(t)
			expt.EvalSet = &entity.EvaluationSet{ID: f.base + 70, EvaluationSetVersion: &entity.EvaluationSetVersion{ID: f.base + 71}}
			source, err := svc.GetItemResultPublishSource(context.Background(), f.space, f.expt, f.run, f.item)
			require.NoError(t, err)
			records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), false, false).Return([]*entity.EvaluatorRecord{publicationRecord(f, gptr.Of(0.0))}, nil).AnyTimes()
			dataset := svcmocks.NewMockEvaluationSetItemService(ctrl)
			dataset.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).Return([]*entity.EvaluationSetItem{{ItemID: f.item, EvaluationSetID: f.base + 70}}, nil).AnyTimes()
			notify := &stubItemCompletePublisher{}
			fault := errors.New("publish source fault")
			if kind == "send_failure" {
				notify.err = fault
			}
			mode := entitymocks.NewMockExptSchedulerMode(ctrl)
			modeErr := error(nil)
			if kind == "mode_failure" {
				modeErr = fault
			}
			mode.EXPECT().PublishResult(gomock.Any(), gomock.Any(), gomock.Any()).Return(modeErr).Times(1)
			events := eventmocks.NewMockExptEventPublisher(ctrl)
			events.EXPECT().PublishExptTurnResultFilterEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
			scheduler := &ExptSchedulerImpl{ResultSvc: &publicationSchedulerResult{svc}, ExptItemResultRepo: f.itemRepo(), itemCompletePublisher: notify, evaluationSetItemService: dataset, Publisher: events}
			event := &entity.ExptScheduleEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run}
			item := &entity.ExptEvalItem{ItemID: f.item, State: entity.ItemRunState_Success, RequestID: "rq1", Publication: source}
			before := f.snapshot()
			if kind == "commit_failure" {
				require.NoError(t, f.conn.Callback().Update().Before("gorm:update").Register("send_commit_fault", func(tx *gorm.DB) {
					if tx.Statement.Table == "expt_item_result_run_log" {
						_ = tx.AddError(fault)
					}
				}))
			}
			err = scheduler.recordEvalItemRunLogs(context.Background(), event, []*entity.ExptEvalItem{item}, mode, expt)
			if kind == "commit_failure" {
				require.NoError(t, f.conn.Callback().Update().Remove("send_commit_fault"))
			}
			if kind != "mode_failure" {
				require.ErrorIs(t, err, fault)
				require.Equal(t, before, f.snapshot())
				notify.err = nil
				require.NoError(t, scheduler.recordEvalItemRunLogs(context.Background(), event, []*entity.ExptEvalItem{item}, mode, expt))
				require.Len(t, notify.events, 2)
			} else {
				require.NoError(t, err)
				require.Len(t, notify.events, 1)
			}
			after := f.snapshot()
			require.NoError(t, scheduler.recordEvalItemRunLogs(context.Background(), event, []*entity.ExptEvalItem{item}, mode, expt))
			require.Equal(t, after, f.snapshot())
			require.Len(t, notify.events, map[bool]int{true: 1, false: 2}[kind == "mode_failure"], "duplicate source after Resulted must not notify")
		})
	}
}

func TestResultPublishLockCancellationNoWrites(t *testing.T) {
	for _, kind := range []string{"cancelled", "lost_lease", "busy"} {
		t.Run(kind, func(t *testing.T) {
			f, svc, records, expt := publicationFixture(t)
			records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), false, false).Return([]*entity.EvaluatorRecord{publicationRecord(f, gptr.Of(0.0))}, nil)
			source, err := svc.GetItemResultPublishSource(context.Background(), f.space, f.expt, f.run, f.item)
			require.NoError(t, err)
			p, err := svc.PrepareItemResultPublication(context.Background(), source, expt)
			require.NoError(t, err)
			ctx := context.Background()
			ctrl := gomock.NewController(t)
			locker := lockmocks.NewMockILocker(ctrl)
			if kind == "cancelled" {
				child, cancel := context.WithCancel(ctx)
				cancel()
				ctx = child
			} else {
				locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, _ string, _ time.Duration, _ time.Duration) (bool, context.Context, func(), error) {
					if kind == "busy" {
						return false, ctx, func() {}, nil
					}
					child, cancel := context.WithCancel(ctx)
					cancel()
					return true, child, cancel, nil
				})
				if kind != "busy" {
					locker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
				}
			}
			svc.resultPublishLocker = func() lock.ILocker { return locker }
			before := f.snapshot()
			sent := false
			applied, err := svc.PublishPreparedItemResult(ctx, p, func(context.Context, *entity.ExptItemResultPublishSource) error { sent = true; return nil })
			require.True(t, errno.NeedMQRetry(err))
			require.False(t, applied)
			require.False(t, sent)
			require.Equal(t, before, f.snapshot())
		})
	}
}

func TestResultPublishRealRepoMissingPortDoesNotHalfWrite(t *testing.T) {
	f, svc, records, expt := publicationFixture(t)
	records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), false, false).Return([]*entity.EvaluatorRecord{publicationRecord(f, gptr.Of(0.0))}, nil).AnyTimes()
	svc.resultPublishRepo = nil
	before := f.snapshot()
	_, err := svc.RecordItemRunLogs(context.Background(), f.expt, f.run, f.item, f.space, expt)
	require.True(t, errno.NeedMQRetry(err), "real repository without publication wiring must fail closed")
	require.Equal(t, before, f.snapshot())
}

func TestResultPublishSQLConcurrentCommitOnce(t *testing.T) {
	f, svc, records, expt := publicationFixture(t)
	records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), false, false).Return([]*entity.EvaluatorRecord{publicationRecord(f, gptr.Of(0.0))}, nil)
	source, err := svc.GetItemResultPublishSource(context.Background(), f.space, f.expt, f.run, f.item)
	require.NoError(t, err)
	p, err := svc.PrepareItemResultPublication(context.Background(), source, expt)
	require.NoError(t, err)
	start := make(chan struct{})
	var count atomic.Int32
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			applied, err := svc.resultPublishRepo.CommitItemResult(context.Background(), p)
			if applied {
				count.Add(1)
			}
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, int32(1), count.Load())
	stats, err := svc.ExptStatsRepo.Get(context.Background(), f.expt, f.space)
	require.NoError(t, err)
	require.Equal(t, int32(1), stats.SuccessItemCnt)
	require.Equal(t, int32(1), stats.ProcessingItemCnt)
	var refs []model.ExptTurnEvaluatorResultRef
	require.NoError(t, f.conn.Where("space_id=?", f.space).Find(&refs).Error)
	require.Len(t, refs, 1)
}

func TestResultPublishScanFreezesNotificationSource(t *testing.T) {
	f, svc, records, expt := publicationFixture(t)
	ctrl := gomock.NewController(t)
	config := componentmocks.NewMockIConfiger(ctrl)
	config.EXPECT().GetExptExecConf(gomock.Any(), f.space).Return(&entity.ExptExecConf{ExptItemEvalConf: &entity.ExptItemEvalConf{ConcurNum: 3}}).AnyTimes()
	expt.EvalSet = &entity.EvaluationSet{ID: f.base + 70, EvaluationSetVersion: &entity.EvaluationSetVersion{ID: f.base + 71}}
	scan := &exptBaseExec{exptItemResultRepo: f.itemRepo(), configer: config}
	event := &entity.ExptScheduleEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run}
	_, _, items, err := scan.ScanEvalItems(context.Background(), event, expt)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.True(t, items[0].PublicationObserved)
	require.NotNil(t, items[0].Publication)
	records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), false, false).Return([]*entity.EvaluatorRecord{publicationRecord(f, gptr.Of(0.0))}, nil)
	// Same request, new invocation references after the original primary scan.
	require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=?", f.space).Update("target_result_id", f.base+42).Error)
	before := f.snapshot()
	notify := &stubItemCompletePublisher{}
	dataset := svcmocks.NewMockEvaluationSetItemService(ctrl)
	dataset.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).Return([]*entity.EvaluationSetItem{{ItemID: f.item}}, nil)
	mode := entitymocks.NewMockExptSchedulerMode(ctrl)
	scheduler := &ExptSchedulerImpl{ResultSvc: &publicationSchedulerResult{svc}, ExptItemResultRepo: f.itemRepo(), itemCompletePublisher: notify, evaluationSetItemService: dataset}
	require.NoError(t, scheduler.recordEvalItemRunLogs(context.Background(), event, items, mode, expt))
	require.Empty(t, notify.events)
	require.Equal(t, before, f.snapshot())
}

func TestResultPublishFailedEvaluatorCannotPublishScore(t *testing.T) {
	f, svc, records, expt := publicationFixture(t)
	record := publicationRecord(f, gptr.Of(99.0))
	record.Status = entity.EvaluatorRunStatusFail
	records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), false, false).Return([]*entity.EvaluatorRecord{record}, nil)
	_, err := svc.RecordItemRunLogs(context.Background(), f.expt, f.run, f.item, f.space, expt)
	require.NoError(t, err)
	var turn model.ExptTurnResult
	require.NoError(t, f.conn.Where("space_id=?", f.space).Take(&turn).Error)
	require.Nil(t, turn.WeightedScore, "failed evaluator's score is not a current successful result")
}

func TestResultPublishConfigSourceFrozenAndReadErrors(t *testing.T) {
	for _, kind := range []string{"config_change", "read_error", "missing_record", "space_mismatch", "unknown_status"} {
		t.Run(kind, func(t *testing.T) {
			f, svc, records, expt := publicationFixture(t)
			record := publicationRecord(f, gptr.Of(0.0))
			if kind == "space_mismatch" {
				record.SpaceID = 0
			}
			if kind == "unknown_status" {
				record.Status = entity.EvaluatorRunStatusUnknown
			}
			source, err := svc.GetItemResultPublishSource(context.Background(), f.space, f.expt, f.run, f.item)
			require.NoError(t, err)
			if kind == "read_error" || kind == "missing_record" || kind == "space_mismatch" || kind == "unknown_status" {
				fault := error(nil)
				result := []*entity.EvaluatorRecord{record}
				if kind == "read_error" {
					fault = errors.New("record source read failure")
				}
				if kind == "missing_record" {
					result = nil
				}
				records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), false, false).Return(result, fault)
				before := f.snapshot()
				_, err = svc.PrepareItemResultPublication(context.Background(), source, expt)
				require.True(t, errno.NeedMQRetry(err))
				require.Equal(t, before, f.snapshot())
				return
			}
			records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), false, false).Return([]*entity.EvaluatorRecord{record}, nil)
			p, err := svc.PrepareItemResultPublication(context.Background(), source, expt)
			require.NoError(t, err)
			require.NoError(t, f.conn.Model(&model.ExptItemRef{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("item_config", []byte(`{"target_source_space_id":99}`)).Error)
			before := f.snapshot()
			sent := false
			ok, err := svc.PublishPreparedItemResult(context.Background(), p, func(context.Context, *entity.ExptItemResultPublishSource) error { sent = true; return nil })
			require.NoError(t, err)
			require.False(t, ok)
			require.False(t, sent)
			require.Equal(t, before, f.snapshot())
		})
	}
}

func TestResultPublishPreparationStopsAfterCancelledRecordRead(t *testing.T) {
	f, svc, _, expt := publicationFixture(t)
	ctrl := gomock.NewController(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	source, err := svc.GetItemResultPublishSource(ctx, f.space, f.expt, f.run, f.item)
	require.NoError(t, err)
	target := svcmocks.NewMockIEvalTargetService(ctrl)
	evaluators := svcmocks.NewMockEvaluatorRecordService(ctrl)
	target.EXPECT().GetRecordByID(gomock.Any(), f.space, f.base+41).DoAndReturn(func(context.Context, int64, int64) (*entity.EvalTargetRecord, error) {
		cancel()
		return &entity.EvalTargetRecord{ID: f.base + 41, SpaceID: f.space, ExperimentRunID: f.run, ItemID: f.item, TurnID: 0, Status: gptr.Of(entity.EvalTargetRunStatusSuccess)}, nil
	})
	evaluators.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	svc.evalTargetService = target
	svc.evaluatorRecordService = evaluators
	before := f.snapshot()
	_, err = svc.PrepareItemResultPublication(ctx, source, expt)
	require.True(t, errno.NeedMQRetry(err))
	require.Equal(t, before, f.snapshot())
}
