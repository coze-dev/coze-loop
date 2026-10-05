// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	idmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	configmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	evalconvert "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/convertor"
	evalmodel "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/gorm_gen/model"
)

// The record boundary uses real converters; the controlled references are in MySQL.
func hookRecoveryReload(t *testing.T, f *executionHookFixture, records map[int64]*evalmodel.EvaluatorRecord) *entity.ExptTurnEvalCtx {
	t.Helper()
	ctrl := gomock.NewController(t)
	items := repomocks.NewMockIExptItemResultRepo(ctrl)
	items.EXPECT().BatchGet(gomock.Any(), f.etec.Event.SpaceID, f.etec.Event.ExptID, []int64{f.etec.Event.EvalSetItemID}).Return(nil, nil)
	config := configmocks.NewMockIConfiger(ctrl)
	config.EXPECT().BuildEvalExt(gomock.Any(), f.etec.Event.SpaceID, f.etec.Turn).Return(nil)
	f.targets.EXPECT().GetRecordByID(gomock.Any(), f.etec.Event.SpaceID, f.etec.ExptTurnRunResult.TargetResult.ID).Return(f.etec.ExptTurnRunResult.TargetResult, nil)
	recordSvc := svcmocks.NewMockEvaluatorRecordService(ctrl)
	recordSvc.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), false, false).DoAndReturn(func(_ context.Context, ids []int64, _, _ bool) ([]*entity.EvaluatorRecord, error) {
		out := make([]*entity.EvaluatorRecord, 0, len(ids))
		for _, id := range ids {
			require.NotNil(t, records[id])
			record, err := evalconvert.ConvertEvaluatorRecordPO2DO(records[id])
			require.NoError(t, err)
			out = append(out, record)
		}
		return out, nil
	}).AnyTimes()
	exec := &ExptItemEvalCtxExecutor{ItemResultRepo: items, Configer: config, evalTargetService: f.targets, evaluatorRecordService: recordSvc}
	loaded, err := exec.buildExptTurnEvalCtx(f.ctx, f.etec.Turn, f.etec.ExptItemEvalCtx, nil)
	require.NoError(t, err)
	return loaded
}

func TestHookRecoveryInterceptedConsumerOwnerPersistsThenContinues(t *testing.T) {
	for _, alias := range []bool{false, true} {
		t.Run(fmt.Sprintf("alias=%v", alias), func(t *testing.T) {
			f := newExecutionHookFixture(t, true, alias, false)
			prepareHookContinuationRecords(f, alias, false)
			row := f.etec.GetExistTurnResultRunLog(1)
			row.TurnID, row.EvaluatorResultIds = 0, nil
			f.etec.Turn.ID = 0
			f.etec.ExistItemEvalResult.TurnResultRunLogs = map[int64]*entity.ExptTurnResultRunLog{0: row}
			f.etec.ExptTurnRunResult.EvaluatorResults = nil
			r, _ := hookRecoveryStorage(t, f)
			for _, ev := range f.etec.Expt.Evaluators {
				ev.SpaceID = 9
			}
			ctrl := gomock.NewController(t)
			evaluators := repomocks.NewMockIEvaluatorRepo(ctrl)
			records := repomocks.NewMockIEvaluatorRecordRepo(ctrl)
			ids := idmocks.NewMockIIDGenerator(ctrl)
			source := svcmocks.NewMockEvaluatorSourceService(ctrl)
			persisted := make(map[int64]*evalmodel.EvaluatorRecord)
			records.EXPECT().CreateEvaluatorRecord(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, record *entity.EvaluatorRecord) error {
				persisted[record.ID] = evalconvert.ConvertEvaluatorRecordDO2PO(record)
				return nil
			}).Times(2)
			for i, ev := range f.etec.Expt.Evaluators {
				version := ev.GetEvaluatorVersionID()
				evaluators.EXPECT().BatchGetEvaluatorByVersionID(gomock.Any(), nil, []int64{version}, false, false).Return([]*entity.Evaluator{ev}, nil).Times(1)
				ids.EXPECT().GenID(gomock.Any()).Return(version+3000, nil).Times(1)
				first := i == 0
				source.EXPECT().ShouldIntercept(gomock.Any(), ev, gomock.Any()).DoAndReturn(func(context.Context, *entity.Evaluator, *entity.EvaluatorInputData) (*entity.EvaluatorOutputData, entity.EvaluatorRunStatus, bool) {
					if first {
						f.gate.waiting.Store(true)
					}
					return &entity.EvaluatorOutputData{EvaluatorResult: &entity.EvaluatorResult{Score: gptr.Of(0.9)}}, entity.EvaluatorRunStatusSuccess, true
				}).Times(1)
			}
			f.turnEval.evaluatorService = &EvaluatorServiceImpl{evaluatorRepo: evaluators, evaluatorRecordRepo: records, idgen: ids, evaluatorSourceServices: map[entity.EvaluatorType]EvaluatorSourceService{entity.EvaluatorTypePrompt: source}}
			paused := f.turnEval.Eval(f.ctx, f.etec)
			require.True(t, itemHookControlOnly(paused.EvalErr))
			require.Len(t, paused.EvaluatorResults, 1)
			require.NoError(t, (&ExptItemEvalCtxExecutor{}).storeTurnRunResult(f.ctx, f.etec, paused))
			stored, err := r.ReadTurnProgress(f.ctx, entity.HookTurnProgressIdentity(row))
			require.NoError(t, err)
			require.Len(t, stored.EvaluatorResultIds.Registered, 1)
			assert.Equal(t, int64(3401), stored.EvaluatorResultIds.Registered[0].RecordID)
			f.etec.Event.HookControlContinuation = true
			f.gate.waiting.Store(false)
			f.etec = hookRecoveryReload(t, f, persisted)
			require.Len(t, f.etec.ExptTurnRunResult.EvaluatorResults, 1)
			record := f.etec.ExptTurnRunResult.EvaluatorResults[0]
			assert.Equal(t, f.etec.Event.SpaceID, record.SpaceID)
			assert.Equal(t, int64(7), record.ItemVersionID)
			assert.Equal(t, int64(0), record.TurnID)
			for _, field := range []string{"space", "record", "run", "experiment", "item", "item-version", "turn", "version", "alias"} {
				t.Run(field, func(t *testing.T) {
					bad := *record
					switch field {
					case "space":
						bad.SpaceID = 987654
					case "record":
						bad.ID++
					case "run":
						bad.ExperimentRunID++
					case "experiment":
						bad.ExperimentID++
					case "item":
						bad.ItemID++
					case "item-version":
						bad.ItemVersionID++
					case "turn":
						bad.TurnID++
					case "version":
						bad.EvaluatorVersionID++
					case "alias":
						bad.Alias += "wrong"
					}
					_, err := reuseHookEvaluator(f.ctx, f.etec, stored, f.etec.ExptTurnRunResult.TargetResult, &bad, 401, record.Alias, false)
					require.Error(t, err)
				})
			}
			resumed := f.turnEval.Eval(f.ctx, f.etec)
			require.NoError(t, resumed.EvalErr)
			require.Len(t, resumed.EvaluatorResults, 2)
			assert.Equal(t, int64(3401), resumed.EvaluatorResults[0].ID)
			assert.Equal(t, 0.9, gptr.Indirect(resumed.EvaluatorResults[0].EvaluatorOutputData.EvaluatorResult.Score))
		})
	}
}
