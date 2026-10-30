// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/ctxcache"
)

// These spies sit at external boundaries. EvalTurns, result loading, stage
// selection and persistence construction remain real, including on a replay.
type manualContinuationCalls struct {
	Targets    map[int64]int           `json:"targets"`
	Evaluators map[int64]map[int64]int `json:"evaluators"`
	Benefits   int                     `json:"benefits"`
	Stores     map[int64]int           `json:"stores"`
	mu         sync.Mutex
	targets    map[int64]*entity.EvalTargetRecord
	evaluators map[int64]*entity.EvaluatorRecord
	sequence   int64
}

func newManualContinuationCalls(t *testing.T, f *turnResumeFixture) *manualContinuationCalls {
	t.Helper()
	c := &manualContinuationCalls{
		Targets: map[int64]int{}, Evaluators: map[int64]map[int64]int{}, Stores: map[int64]int{},
		targets: map[int64]*entity.EvalTargetRecord{}, evaluators: map[int64]*entity.EvaluatorRecord{}, sequence: 10000,
	}
	f.item.Event.Ext = map[string]string{entity.ManualRetryExtKey: `{"run_id":103,"request_id":"RQ1","mode":5,"operator":"u","accepted_at_ms":1}`}
	f.target.EXPECT().GetRecordByID(gomock.Any(), int64(3), gomock.Any()).DoAndReturn(func(_ context.Context, _ int64, id int64) (*entity.EvalTargetRecord, error) {
		return c.targets[id], nil
	}).AnyTimes()
	f.records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), false, false).DoAndReturn(func(_ context.Context, ids []int64, _, _ bool) ([]*entity.EvaluatorRecord, error) {
		rows := make([]*entity.EvaluatorRecord, 0, len(ids))
		for _, id := range ids {
			require.Contains(t, c.evaluators, id, "fixture must contain every persisted ref")
			rows = append(rows, c.evaluators[id])
		}
		return rows, nil
	}).AnyTimes()
	f.target.EXPECT().ExecuteTarget(gomock.Any(), int64(3), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, _ int64, _ int64, _ int64, exec *entity.ExecuteTargetCtx, input *entity.EvalTargetInputData) (*entity.EvalTargetRecord, error) {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.Targets[exec.TurnID]++
		c.sequence++
		record := c.target(c.sequence, exec.TurnID)
		c.targets[record.ID] = record
		assert.NotContains(t, input.Ext, entity.ManualRetryExtKey)
		assert.Len(t, input.HistoryMessages, map[int64]int{0: 0, 20: 1, 30: 2}[exec.TurnID])
		return record, nil
	}).AnyTimes()
	f.evaluator.EXPECT().ShouldInterceptEvaluator(gomock.Any(), gomock.Any()).Return(nil, false, nil).AnyTimes()
	f.evaluator.EXPECT().RunEvaluator(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *entity.RunEvaluatorRequest) (*entity.EvaluatorRecord, error) {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.Evaluators[req.TurnID] == nil {
			c.Evaluators[req.TurnID] = map[int64]int{}
		}
		c.Evaluators[req.TurnID][req.EvaluatorVersionID]++
		c.sequence++
		record := resumeEvaluator(c.sequence, req.EvaluatorVersionID, req.TurnID, req.Alias)
		record.Ext = map[string]string{"request_id": "RQ1"}
		c.evaluators[record.ID] = record
		assert.NotContains(t, req.Ext, entity.ManualRetryExtKey)
		return record, nil
	}).AnyTimes()
	f.benefit.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, _ *benefit.CheckAndDeductEvalBenefitParams) (*benefit.CheckAndDeductEvalBenefitResult, error) {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.Benefits++
		return &benefit.CheckAndDeductEvalBenefitResult{}, nil
	}).AnyTimes()
	f.turns.EXPECT().SaveTurnRunLogs(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, rows []*entity.ExptTurnResultRunLog) error {
		require.Len(t, rows, 1)
		row := rows[0]
		c.Stores[row.TurnID]++
		assert.Equal(t, entity.TurnRunState_Success, row.Status)
		f.item.ExistItemEvalResult.TurnResultRunLogs[row.TurnID] = row
		return nil
	}).AnyTimes()
	return c
}

func (c *manualContinuationCalls) target(id, turnID int64) *entity.EvalTargetRecord {
	record := resumeTarget(id, turnID)
	record.SpaceID, record.ExperimentRunID, record.ItemID = 3, 103, 10
	record.Ext = map[string]string{"request_id": "RQ1"}
	return record
}

func (c *manualContinuationCalls) prepare(f *turnResumeFixture, turnID int64, status entity.TurnRunState, failedE2 bool) {
	log := f.item.ExistItemEvalResult.TurnResultRunLogs[turnID]
	log.Status, log.TargetResultID = status, 201+turnID
	log.Ext = f.item.Event.Ext
	c.targets[log.TargetResultID] = c.target(log.TargetResultID, turnID)
	log.EvaluatorResultIds = &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{
		{VersionID: 401, RecordID: 301 + turnID}, {VersionID: 402, RecordID: 302 + turnID},
	}}
	c.evaluators[301+turnID] = resumeEvaluator(301+turnID, 401, turnID, "")
	c.evaluators[301+turnID].Ext = map[string]string{"request_id": "RQ1"}
	e2 := resumeEvaluator(302+turnID, 402, turnID, "")
	e2.Ext = map[string]string{"request_id": "RQ1"}
	if failedE2 {
		e2.Status = entity.EvaluatorRunStatusFail
	}
	c.evaluators[e2.ID] = e2
}

func setManualContinuationSource(f *turnResumeFixture, kind string, turnID int64) {
	recordID := 201 + turnID
	if kind == "evaluator" {
		recordID = 302 + turnID
	}
	f.item.Event.AsyncReportTrigger = kind == "target"
	f.item.Event.AsyncEvaluatorReportTrigger = kind == "evaluator"
	f.item.Event.ResumeSource = &entity.ExptResumeSource{Kind: kind, TurnID: turnID, RecordID: recordID, InvokeID: strconv.FormatInt(recordID, 10)}
}

func (c *manualContinuationCalls) log(t *testing.T, phase string) {
	t.Helper()
	raw, err := json.Marshal(c)
	require.NoError(t, err)
	t.Logf("actual_call_counts %s %s", phase, raw)
}

// Catches dropping ResumeSource on a non-source clone: RQ1's completed turn
// would be treated as another user-confirmed manual invocation and billed again.
func TestRunningRetryManualContinuation_EvalTurnsSource20DoesNotRerunHistory(t *testing.T) {
	for _, kind := range []string{"target", "evaluator"} {
		t.Run(kind, func(t *testing.T) {
			f := newTurnResumeFixture(t, entity.EvaluationModeRetryItems, 0, 20, 30)
			calls := newManualContinuationCalls(t, f)
			calls.prepare(f, 0, entity.TurnRunState_Success, false)
			calls.prepare(f, 20, entity.TurnRunState_Processing, false)
			setManualContinuationSource(f, kind, 20)
			originalSource := *f.item.Event.ResumeSource
			first := f.item.ExistItemEvalResult.TurnResultRunLogs[0]
			for _, phase := range []string{"resume", "duplicate callback"} {
				abort, err := f.executor.EvalTurns(ctxcache.Init(context.Background()), f.item)
				require.NoError(t, err)
				assert.False(t, abort)
				calls.log(t, phase)
				assert.Equal(t, map[int64]int{30: 1}, calls.Targets)
				assert.Equal(t, map[int64]map[int64]int{30: {401: 1, 402: 1}}, calls.Evaluators)
				assert.Equal(t, 2, calls.Benefits)
				assert.Equal(t, map[int64]int{20: 1, 30: 1}, calls.Stores)
				assert.Same(t, first, f.item.ExistItemEvalResult.TurnResultRunLogs[0], "completed history must not be saved again")
				assert.Equal(t, originalSource, *f.item.Event.ResumeSource)
			}
		})
	}
}

// Catches the same mode confusion for partial results in the next turn. The
// source flags do not apply there, but RQ1's successful T/E1 must still be reused.
func TestRunningRetryManualContinuation_EvalTurnsSource0OnlyInvokesNextPendingE2(t *testing.T) {
	for _, kind := range []string{"target", "evaluator"} {
		for _, format := range []string{"registered", "alias", "legacy", "inline"} {
			t.Run(kind+"/"+format, func(t *testing.T) {
				f := newTurnResumeFixture(t, entity.EvaluationModeRetryItems, 0, 20)
				calls := newManualContinuationCalls(t, f)
				calls.prepare(f, 0, entity.TurnRunState_Processing, false)
				calls.prepare(f, 20, entity.TurnRunState_Fail, true)
				setManualContinuationSource(f, kind, 0)
				next := f.item.ExistItemEvalResult.TurnResultRunLogs[20]
				switch format {
				case "alias":
					f.item.Expt.EvalSetSourceType = entity.ExptEvalSetSourceType_MultiSetConfig
					f.item.ItemConfig = &entity.ExptItemConfig{EvaluatorConfs: []*entity.ItemEvaluatorConf{{EvaluatorVersionID: 401, Alias: "a"}, {EvaluatorVersionID: 402, Alias: "b"}}}
					for _, turnID := range []int64{0, 20} {
						refs := f.item.ExistItemEvalResult.TurnResultRunLogs[turnID].EvaluatorResultIds.Registered
						refs[0].Alias, refs[1].Alias = "a", "b"
						calls.evaluators[301+turnID].Alias, calls.evaluators[302+turnID].Alias = "a", "b"
					}
				case "legacy":
					next.EvaluatorResultIds = &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{401: 321, 402: 322}}
				case "inline":
					inline := resumeEvaluator(324, 0, 20, "")
					inline.SourceType, inline.InlineKey, inline.TargetRecordID = entity.EvaluatorRecordSourceTypeInline, "quality", 221
					calls.evaluators[324] = inline
					next.EvaluatorResultIds.Inline = []*entity.InlineEvalResult{{InlineKey: "quality", RecordID: 324}}
				}
				abort, err := f.executor.EvalTurns(ctxcache.Init(context.Background()), f.item)
				require.NoError(t, err)
				assert.False(t, abort)
				calls.log(t, "resume")
				assert.Empty(t, calls.Targets)
				assert.Equal(t, map[int64]map[int64]int{20: {402: 1}}, calls.Evaluators)
				assert.Equal(t, 1, calls.Benefits)
				assert.Equal(t, map[int64]int{0: 1, 20: 1}, calls.Stores)
				saved := f.item.ExistItemEvalResult.TurnResultRunLogs[20]
				assert.Equal(t, int64(221), saved.TargetResultID)
				require.Len(t, saved.EvaluatorResultIds.Registered, 2)
				assert.Equal(t, int64(321), saved.EvaluatorResultIds.Registered[0].RecordID)
				assert.NotEqual(t, int64(322), saved.EvaluatorResultIds.Registered[1].RecordID)
				if format == "inline" {
					require.Len(t, saved.EvaluatorResultIds.Inline, 1)
					assert.Equal(t, int64(324), saved.EvaluatorResultIds.Inline[0].RecordID)
				}
			})
		}
	}
}

func TestRunningRetryManualContinuation_FirstUserSelectionExecutesAllTurns(t *testing.T) {
	f := newTurnResumeFixture(t, entity.EvaluationModeRetryItems, 0, 20, 30)
	calls := newManualContinuationCalls(t, f)
	// Admission clears every old ref/status. No callback metadata marks this
	// first user-confirmed execution, and every frozen turn must be newly invoked.
	require.Nil(t, f.item.Event.ResumeSource)
	require.False(t, f.item.Event.AsyncReportTrigger)
	require.False(t, f.item.Event.AsyncEvaluatorReportTrigger)
	abort, err := f.executor.EvalTurns(ctxcache.Init(context.Background()), f.item)
	require.NoError(t, err)
	assert.False(t, abort)
	calls.log(t, "first selection")
	assert.Equal(t, map[int64]int{0: 1, 20: 1, 30: 1}, calls.Targets)
	assert.Equal(t, map[int64]map[int64]int{0: {401: 1, 402: 1}, 20: {401: 1, 402: 1}, 30: {401: 1, 402: 1}}, calls.Evaluators)
	assert.Equal(t, 6, calls.Benefits)
	assert.Equal(t, map[int64]int{0: 1, 20: 1, 30: 1}, calls.Stores)
}

// Retaining source identity must not re-clone between CallTarget and
// CallEvaluators: a genuinely new target invalidates that turn's old evaluators.
func TestRunningRetryManualContinuation_EvalTurnsNewTargetKeepsCalledMarker(t *testing.T) {
	f := newTurnResumeFixture(t, entity.EvaluationModeRetryItems, 0, 20)
	calls := newManualContinuationCalls(t, f)
	calls.prepare(f, 0, entity.TurnRunState_Processing, false)
	calls.prepare(f, 20, entity.TurnRunState_Fail, false)
	setManualContinuationSource(f, "target", 0)
	target := calls.targets[221]
	*target.Status = entity.EvalTargetRunStatusFail
	abort, err := f.executor.EvalTurns(ctxcache.Init(context.Background()), f.item)
	require.NoError(t, err)
	assert.False(t, abort)
	calls.log(t, "new target")
	assert.Equal(t, map[int64]int{20: 1}, calls.Targets)
	assert.Equal(t, map[int64]map[int64]int{20: {401: 1, 402: 1}}, calls.Evaluators)
	assert.Equal(t, 2, calls.Benefits)
	assert.Equal(t, map[int64]int{0: 1, 20: 1}, calls.Stores)
	saved := f.item.ExistItemEvalResult.TurnResultRunLogs[20]
	assert.NotEqual(t, int64(221), saved.TargetResultID)
	require.Len(t, saved.EvaluatorResultIds.Registered, 2)
	for _, ref := range saved.EvaluatorResultIds.Registered {
		assert.NotContains(t, []int64{321, 322}, ref.RecordID)
	}
}

func TestRunningRetryManualContinuation_LegacyCallbackWithoutSourceKeepsRefs(t *testing.T) {
	for _, kind := range []string{"target", "evaluator"} {
		t.Run(kind, func(t *testing.T) {
			f := newTurnResumeFixture(t, entity.EvaluationModeRetryItems, 0)
			calls := newManualContinuationCalls(t, f)
			calls.prepare(f, 0, entity.TurnRunState_Processing, false)
			f.item.Event.AsyncReportTrigger = kind == "target"
			f.item.Event.AsyncEvaluatorReportTrigger = kind == "evaluator"
			require.Nil(t, f.item.Event.ResumeSource)
			abort, err := f.executor.EvalTurns(ctxcache.Init(context.Background()), f.item)
			require.NoError(t, err)
			assert.False(t, abort)
			calls.log(t, "legacy callback")
			assert.Empty(t, calls.Targets)
			assert.Empty(t, calls.Evaluators)
			assert.Zero(t, calls.Benefits)
			assert.Equal(t, map[int64]int{0: 1}, calls.Stores)
			assert.Equal(t, int64(201), f.item.ExistItemEvalResult.TurnResultRunLogs[0].TargetResultID)
		})
	}
}
