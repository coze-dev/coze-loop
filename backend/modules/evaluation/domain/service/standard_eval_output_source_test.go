// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	experimentrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"
)

func prepareSourceStandardFixture(t *testing.T, token string) (*lateWriteFixture, *ExptResultServiceImpl) {
	t.Helper()
	f := newLateWriteFixture(t, token)
	require.NoError(t, f.conn.Model(&model.ExptRunLog{}).Where("space_id=?", f.space).Update("status", int64(entity.ExptStatus_Success)).Error)
	require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("status", int32(entity.ItemRunState_Success)).Error)
	require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Updates(map[string]any{"status": int32(entity.ItemRunState_Success), "result_state": int32(entity.ExptItemResultStateResulted)}).Error)
	require.NoError(t, f.conn.Model(&model.ExptTurnResult{}).Where("space_id=?", f.space).Update("status", int32(entity.TurnRunState_Success)).Error)
	require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=?", f.space).Update("status", int32(entity.TurnRunState_Success)).Error)
	require.NoError(t, f.conn.Where("space_id=?", f.space).Delete(&model.ExptTurnEvaluatorResultRef{}).Error)
	for index, turn := range []int64{0, f.base + 22} {
		refs := &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: 10, Alias: "", RecordID: f.base + 60}, {VersionID: f.base + 51, Alias: "a", RecordID: f.base + 61}}, Inline: []*entity.InlineEvalResult{{InlineKey: "inline", RecordID: f.base + 62}}}
		if index == 1 {
			refs = &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{f.base + 51: f.base + 63}}
		}
		raw, err := json.Marshal(refs)
		require.NoError(t, err)
		require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND turn_id=?", f.space, turn).Update("evaluator_result_ids", raw).Error)
		for n, ref := range NewTurnEvaluatorResultRefs(0, f.expt, f.base+400+int64(index), f.space, refs) {
			require.NoError(t, f.conn.Create(&model.ExptTurnEvaluatorResultRef{ID: f.base + 1000 + int64(index*10+n), SpaceID: ref.SpaceID, ExptID: ref.ExptID, ExptTurnResultID: ref.ExptTurnResultID, EvaluatorVersionID: ref.EvaluatorVersionID, Alias_: ref.Alias, InlineKey: ref.InlineKey, SourceType: ref.SourceType, EvaluatorResultID: ref.EvaluatorResultID}).Error)
		}
	}
	svc := NewExptResultServiceWithPublication(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, experimentrepo.NewExptResultPublishRepo(f.provider), nil).(*ExptResultServiceImpl)
	return f, svc
}

func TestSourceStandardMySQLProductionPortMQShapeAndNoWrites(t *testing.T) {
	for _, token := range []string{"", "f059e038-4d6e-41c8-a20d-68f8801adeaf"} {
		t.Run(token, func(t *testing.T) {
			f, svc := prepareSourceStandardFixture(t, token)
			before := f.snapshot()
			recordReads := 0
			require.NoError(t, f.conn.Callback().Query().After("gorm:query").Register("source_standard_record_reads", func(tx *gorm.DB) {
				if tx.Statement.Table == "eval_target_record" || tx.Statement.Table == "evaluator_record" {
					recordReads++
				}
			}))
			source, err := svc.GetStandardEvalOutputSource(context.Background(), f.space, f.expt, 0, f.item)
			require.NoError(t, err)
			require.Equal(t, 0, recordReads)
			require.NoError(t, f.conn.Callback().Query().Remove("source_standard_record_reads"))
			records, err := StandardEvalOutputSourceRecords(source)
			require.NoError(t, err)
			require.Len(t, records, 2)
			require.Equal(t, "0", records[0].TurnID)
			require.Equal(t, "10", records[0].Registered[0].VersionID, "sort numerically across narrow and nineteen-digit IDs")
			require.Len(t, records[0].Inline, 1)
			copySnapshot := *source.Snapshot
			copyLog := *source.Snapshot.ItemLog
			copyLog.ResultState = int32(entity.ExptItemResultStateLogged)
			copySnapshot.ItemLog = &copyLog
			p := &entity.ExptItemResultPublication{Source: &copySnapshot, Turns: source.Snapshot.Source.CanonicalTurns, Refs: source.Snapshot.Source.CurrentRefs}
			c := source.Snapshot.Source.Current
			event, err := BuildItemCompleteEventFromSchedulerWithSource(c.SpaceID, c.ExptID, c.RunID, nil, &entity.ExptEvalItem{ItemID: c.ItemID, RequestID: c.RequestID}, nil, 0, p)
			require.NoError(t, err)
			require.Equal(t, records, event.SourceTurnRecords)
			require.Equal(t, token, *event.RequestID)
			require.Equal(t, before, f.snapshot(), "source reads preserve all eleven fixture tables")
			f.token("rq2")
			next, err := svc.GetStandardEvalOutputSource(context.Background(), f.space, f.expt, f.run, f.item)
			require.NoError(t, err)
			require.False(t, SameStandardEvalOutputSource(source, next))
		})
	}
}

func TestSourceStandardMySQLNotReadyAndMismatch(t *testing.T) {
	for _, scenario := range []string{"queueing", "processing", "logged", "failed", "canonical_mismatch", "wrong_ref", "latest_moved", "read_error"} {
		t.Run(scenario, func(t *testing.T) {
			f, svc := prepareSourceStandardFixture(t, "rq1")
			want := int32(errno.StandardEvalOutputSourceNotReadyCode)
			switch scenario {
			case "queueing", "processing", "failed":
				status := entity.ItemRunState_Queueing
				if scenario == "processing" {
					status = entity.ItemRunState_Processing
				}
				if scenario == "failed" {
					status = entity.ItemRunState_Fail
				}
				require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("status", int32(status)).Error)
				require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("status", int32(status)).Error)
			case "logged":
				require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("result_state", int32(entity.ExptItemResultStateLogged)).Error)
			case "canonical_mismatch":
				want = errno.StandardEvalOutputSourceMismatchCode
				require.NoError(t, f.conn.Model(&model.ExptTurnResult{}).Where("space_id=? AND turn_id=0", f.space).Update("target_result_id", f.base+99).Error)
			case "wrong_ref":
				want = errno.StandardEvalOutputSourceMismatchCode
				require.NoError(t, f.conn.Model(&model.ExptTurnEvaluatorResultRef{}).Where("space_id=?", f.space).Update("evaluator_result_id", f.base+99).Error)
			case "latest_moved":
				want = errno.StandardEvalOutputSourceMismatchCode
				require.NoError(t, f.conn.Model(&model.Experiment{}).Where("space_id=?", f.space).Update("latest_run_id", f.run+1).Error)
			case "read_error":
				fault := errors.New("primary reader failure")
				require.NoError(t, f.conn.Callback().Query().Before("gorm:query").Register("source_standard_fault", func(tx *gorm.DB) { _ = tx.AddError(fault) }))
				source, err := svc.GetStandardEvalOutputSource(context.Background(), f.space, f.expt, f.run, f.item)
				require.NoError(t, f.conn.Callback().Query().Remove("source_standard_fault"))
				require.Nil(t, source)
				require.ErrorIs(t, err, fault)
				return
			}
			source, err := svc.GetStandardEvalOutputSource(context.Background(), f.space, f.expt, f.run, f.item)
			require.Nil(t, source)
			status, ok := errorx.FromStatusError(err)
			require.True(t, ok)
			require.Equal(t, want, status.Code())
		})
	}
}

func TestSourceStandardRawVersionTargetTurnZero(t *testing.T) {
	f, svc := prepareSourceStandardFixture(t, "")
	for _, table := range []string{"expt_item_result", "expt_item_result_run_log", "expt_turn_result", "expt_turn_result_run_log", "expt_item_ref"} {
		require.NoError(t, f.conn.Table(table).Where("space_id=?", f.space).Update("item_version_id", 0).Error)
	}
	for _, table := range []string{"expt_turn_result", "expt_turn_result_run_log"} {
		require.NoError(t, f.conn.Table(table).Where("space_id=?", f.space).Update("target_result_id", 0).Error)
	}
	source, err := svc.GetStandardEvalOutputSource(context.Background(), f.space, f.expt, f.run, f.item)
	require.NoError(t, err)
	records, err := StandardEvalOutputSourceRecords(source)
	require.NoError(t, err)
	require.Equal(t, "0", records[0].TurnID)
	require.Equal(t, "0", records[0].TargetRecordID)
	require.Equal(t, int64(0), source.Snapshot.Source.Current.ItemVersionID)
}

func TestSourceStandardResultBuilderPreservesAliasesAndInline(t *testing.T) {
	ctrl := gomock.NewController(t)
	f, svc := prepareSourceStandardFixture(t, "rq1")
	frozen, err := svc.GetStandardEvalOutputSource(context.Background(), f.space, f.expt, f.run, f.item)
	require.NoError(t, err)
	refs := frozen.Snapshot.Source.CurrentRefs
	records := svcmocks.NewMockEvaluatorRecordService(ctrl)
	all := make([]*entity.EvaluatorRecord, 0, len(refs))
	for _, ref := range refs {
		all = append(all, &entity.EvaluatorRecord{ID: ref.EvaluatorResultID, EvaluatorVersionID: ref.EvaluatorVersionID, Alias: ref.Alias, InlineKey: ref.InlineKey, SourceType: entity.EvaluatorRecordSourceType(ref.SourceType)})
	}
	// Two aliases on one version and multiple Inline version-zero records are legal distinct sources.
	all = append(all, &entity.EvaluatorRecord{ID: f.base + 69, EvaluatorVersionID: f.base + 51, Alias: "z"})
	records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), false, false).Return(all, nil)
	for _, item := range all[len(all)-1:] {
		ref := &entity.ExptTurnEvaluatorResultRef{ID: f.base + 1099, SpaceID: f.space, ExptID: f.expt, ExptTurnResultID: f.base + 400, EvaluatorVersionID: item.EvaluatorVersionID, Alias: item.Alias, EvaluatorResultID: item.ID}
		refs = append(refs, ref)
		require.NoError(t, f.conn.Create(&model.ExptTurnEvaluatorResultRef{ID: ref.ID, SpaceID: ref.SpaceID, ExptID: ref.ExptID, ExptTurnResultID: ref.ExptTurnResultID, EvaluatorVersionID: ref.EvaluatorVersionID, Alias_: ref.Alias, EvaluatorResultID: ref.EvaluatorResultID}).Error)
	}
	builder := &ExptResultBuilder{SpaceID: f.space, ExptTurnResultRepo: f.turnRepo(), evaluatorRecordService: records, turnResultDO: frozen.Snapshot.Source.CanonicalTurns}
	ctx := entity.WithExptStandardEvalOutputSource(context.Background(), frozen)
	require.NoError(t, builder.buildEvaluatorResult(ctx))
	require.Len(t, builder.turnResultID2EvaluatorVersionID2Result[f.base+400], 4, "standard source result reader must not collapse aliases by version ID")
}

func TestSourceStandardChangedRequestBeforeResultedIsMismatch(t *testing.T) {
	f, svc := prepareSourceStandardFixture(t, "rq2")
	require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Updates(map[string]any{"status": int32(entity.ItemRunState_Processing), "result_state": int32(entity.ExptItemResultStateDefault)}).Error)
	expected := "rq1"
	source, err := svc.GetStandardEvalOutputSource(context.Background(), f.space, f.expt, f.run, f.item, &expected)
	require.Nil(t, source)
	status, ok := errorx.FromStatusError(err)
	require.True(t, ok)
	require.Equal(t, int32(errno.StandardEvalOutputSourceMismatchCode), status.Code())
}
