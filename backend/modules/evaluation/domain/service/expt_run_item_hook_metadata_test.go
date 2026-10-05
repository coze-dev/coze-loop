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
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

func newHookMetadataFixture(t *testing.T, managed bool, turn int64) *executionHookFixture {
	t.Helper()
	f := newExecutionHookFixture(t, false, false, false)
	admission := newItemHookFixture(t, true, false)
	admission.item.ItemVersionID = 7
	f.etec.ExistItemEvalResult.ItemResultRunLog = admission.item
	f.etec.EvalSetItem.ItemVersionID = gptr.Of(int64(7))
	row := f.etec.GetExistTurnResultRunLog(1)
	row.TurnID = turn
	row.ItemVersionID = 7
	f.etec.Turn.ID = turn
	f.etec.ExistItemEvalResult.TurnResultRunLogs = map[int64]*entity.ExptTurnResultRunLog{turn: row}
	f.etec.Event.Ext = map[string]string{"item_version_id": "999"}
	if managed {
		admission.ports.held = true
		svc := admission.svc.(*ExptItemEventEvalServiceImpl)
		svc.hookAdmission.progress = f.progress
		require.NoError(t, svc.handleHookItemAdmission(func(ctx context.Context, _ *entity.ExptItemEvalEvent) error { f.ctx = ctx; return nil })(context.WithValue(f.ctx, itemHookManagedKey{}, true), admission.event))
	}
	return f
}

func TestHookMetadataRealPreEvalFrozenVersion(t *testing.T) {
	for _, managed := range []bool{false, true} {
		for _, retry := range []bool{false, true} {
			t.Run(fmt.Sprintf("managed=%v/retry=%v", managed, retry), func(t *testing.T) {
				f := newHookMetadataFixture(t, managed, 0)
				f.etec.ExistItemEvalResult.TurnResultRunLogs = map[int64]*entity.ExptTurnResultRunLog{}
				f.etec.EvalSetItem.Turns = []*entity.Turn{{ID: 0}, {ID: 1}}
				ctrl := gomock.NewController(t)
				turns := repomocks.NewMockIExptTurnResultRepo(ctrl)
				ids := idmocks.NewMockIIDGenerator(ctrl)
				ids.EXPECT().GenMultiIDs(gomock.Any(), 2).Return([]int64{20, 21}, nil)
				if !retry {
					turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), int64(2), int64(3), int64(4), int64(1)).Return(nil, nil)
				}
				turns.EXPECT().BatchCreateNXRunLog(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, rows []*entity.ExptTurnResultRunLog) error {
					require.Len(t, rows, 2)
					for i, row := range rows {
						want := int64(0)
						if managed {
							want = 7
						}
						assert.Equal(t, want, row.ItemVersionID)
						assert.Equal(t, int64(i), row.TurnID)
						assert.Equal(t, int64(3), row.ExptRunID)
					}
					return nil
				})
				var err error
				if retry {
					err = (&ExptRecordEvalModeRetryIgnoreResult{exptTurnResultRepo: turns, idgen: ids}).PreEval(f.ctx, f.etec.ExptItemEvalCtx)
				} else {
					err = (&ExptRecordEvalModeSubmit{exptTurnResultRepo: turns, idgen: ids}).PreEval(f.ctx, f.etec.ExptItemEvalCtx)
				}
				require.NoError(t, err)
			})
		}
	}
}

func TestHookMetadataFailRetryKeepsLawfulSourceResults(t *testing.T) {
	f := newHookMetadataFixture(t, true, 1)
	prepareHookContinuationRecords(f, false, false)
	f.etec.Event.ExptRunMode = entity.EvaluationModeFailRetry
	f.etec.Event.HookControlContinuation = true
	f.etec.ExptTurnRunResult.TargetResult.ExperimentRunID = 2
	for _, r := range f.etec.ExptTurnRunResult.EvaluatorResults {
		r.ExperimentRunID = 2
	}
	result := f.turnEval.Eval(f.ctx, f.etec)
	require.NoError(t, result.EvalErr)
	assert.Same(t, f.etec.ExptTurnRunResult.TargetResult, result.TargetResult)
	assert.Equal(t, f.etec.ExptTurnRunResult.EvaluatorResults, result.EvaluatorResults)
}

func TestHookMetadataRealFailRetryPreEvalFrozenVersion(t *testing.T) {
	f := newHookMetadataFixture(t, true, 0)
	f.etec.ExistItemEvalResult.TurnResultRunLogs = map[int64]*entity.ExptTurnResultRunLog{}
	f.etec.Expt.TargetVersionID = 0
	ctrl := gomock.NewController(t)
	results := svcmocks.NewMockExptResultService(ctrl)
	turns := repomocks.NewMockIExptTurnResultRepo(ctrl)
	ids := idmocks.NewMockIIDGenerator(ctrl)
	results.EXPECT().GetExptItemTurnResults(gomock.Any(), int64(2), int64(4), int64(1), gomock.Any()).Return([]*entity.ExptTurnResult{{ID: 10, SpaceID: 1, ExptID: 2, ExptRunID: 2, ItemID: 4, TurnID: 0}}, nil)
	turns.EXPECT().BatchGetTurnEvaluatorResultRef(gomock.Any(), int64(1), []int64{10}).Return(nil, nil)
	ids.EXPECT().GenMultiIDs(gomock.Any(), 1).Return([]int64{20}, nil)
	turns.EXPECT().BatchCreateNXRunLog(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, rows []*entity.ExptTurnResultRunLog) error {
		require.Len(t, rows, 1)
		assert.Equal(t, int64(7), rows[0].ItemVersionID)
		assert.Equal(t, int64(3), rows[0].ExptRunID)
		return nil
	})
	require.NoError(t, (&ExptRecordEvalModeFailRetry{resultSvc: results, exptTurnResultRepo: turns, idgen: ids}).PreEval(f.ctx, f.etec.ExptItemEvalCtx))
}

func TestHookMetadataIdentityRejectsMismatches(t *testing.T) {
	f := newHookMetadataFixture(t, true, 0)
	ctx, err := bindItemHookTurnIdentity(f.ctx, f.etec)
	require.NoError(t, err)
	for _, field := range []string{"space", "expt", "run", "item", "turn"} {
		space, expt, run, item, turn := int64(1), int64(2), int64(3), int64(4), int64(0)
		switch field {
		case "space":
			space++
		case "expt":
			expt++
		case "run":
			run++
		case "item":
			item++
		case "turn":
			turn++
		}
		_, err := hookRecordItemVersion(ctx, space, expt, run, item, turn)
		var control itemHookControlError
		require.ErrorAs(t, err, &control)
	}
	f.etec.GetExistTurnResultRunLog(0).ItemVersionID = 8
	_, err = bindItemHookTurnIdentity(f.ctx, f.etec)
	require.Error(t, err)
}
