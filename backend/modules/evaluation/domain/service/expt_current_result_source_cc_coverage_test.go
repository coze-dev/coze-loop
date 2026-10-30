// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	idmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
)

func TestCCCurrentSourceCoverageExecutionSource(t *testing.T) {
	require.Nil(t, executionSource(context.Background()))
	require.Nil(t, executionSource(context.WithValue(context.Background(), currentItemSourceKey{}, "not-an-execution")))
	execution := &currentItemExecution{LockLost: true}
	require.Same(t, execution, executionSource(context.WithValue(context.Background(), currentItemSourceKey{}, execution)))
}

func ccCurrentSourceCoverageIdentitySource() *entity.ExptCurrentItemRunSource {
	return &entity.ExptCurrentItemRunSource{
		Current: entity.ExptItemCurrentRequest{
			SpaceID: 3, ExptID: 5, RunID: 7, ItemID: 11, ItemVersionID: 13,
			RequestID: "rq-current", Mode: entity.EvaluationModeSubmit,
		},
		CanonicalTurns: []*entity.ExptTurnResult{{ID: 21, TurnID: 0, ItemVersionID: 13, TargetResultID: 41}},
		Turns: map[int64]*entity.ExptTurnResultRunLog{
			0: {ID: 21, TargetResultID: 41, EvaluatorResultIds: &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{51: 61}}},
		},
		CurrentRefs: []*entity.ExptTurnEvaluatorResultRef{{ID: 71}},
	}
}

func TestCCCurrentSourceCoverageSameExecutionIdentity(t *testing.T) {
	require.False(t, sameExecutionIdentity(nil, nil))
	require.False(t, sameExecutionIdentity(ccCurrentSourceCoverageIdentitySource(), nil))
	require.False(t, sameExecutionIdentity(nil, ccCurrentSourceCoverageIdentitySource()))

	for _, tc := range []struct {
		name   string
		mutate func(expected, actual *entity.ExptCurrentItemRunSource)
		want   bool
	}{
		{name: "identical", mutate: func(_, _ *entity.ExptCurrentItemRunSource) {}, want: true},
		{name: "stale_space", mutate: func(expected, _ *entity.ExptCurrentItemRunSource) { expected.Current.SpaceID = 4 }},
		{name: "stale_expt", mutate: func(expected, _ *entity.ExptCurrentItemRunSource) { expected.Current.ExptID = 6 }},
		{name: "stale_run", mutate: func(expected, _ *entity.ExptCurrentItemRunSource) { expected.Current.RunID = 8 }},
		{name: "stale_item", mutate: func(expected, _ *entity.ExptCurrentItemRunSource) { expected.Current.ItemID = 12 }},
		{name: "stale_item_version", mutate: func(expected, _ *entity.ExptCurrentItemRunSource) { expected.Current.ItemVersionID = 14 }},
		{name: "stale_request", mutate: func(expected, _ *entity.ExptCurrentItemRunSource) { expected.Current.RequestID = "rq-replacement" }},
		{name: "stale_mode", mutate: func(expected, _ *entity.ExptCurrentItemRunSource) {
			expected.Current.Mode = entity.EvaluationModeFailRetry
		}},
		{name: "turn_count", mutate: func(expected, _ *entity.ExptCurrentItemRunSource) {
			expected.Turns[1] = &entity.ExptTurnResultRunLog{ID: 22}
		}},
		{name: "canonical_turn_count", mutate: func(expected, _ *entity.ExptCurrentItemRunSource) {
			expected.CanonicalTurns = append(expected.CanonicalTurns, &entity.ExptTurnResult{ID: 22, TurnID: 1})
		}},
		{name: "canonical_turn_nil", mutate: func(expected, _ *entity.ExptCurrentItemRunSource) {
			expected.CanonicalTurns = append(expected.CanonicalTurns, nil)
		}},
		{name: "canonical_turn_id", mutate: func(expected, _ *entity.ExptCurrentItemRunSource) { expected.CanonicalTurns[0].ID = 99 }},
		{name: "canonical_turn_turn_id", mutate: func(expected, _ *entity.ExptCurrentItemRunSource) { expected.CanonicalTurns[0].TurnID = 1 }},
		{name: "canonical_turn_item_version", mutate: func(expected, _ *entity.ExptCurrentItemRunSource) { expected.CanonicalTurns[0].ItemVersionID = 14 }},
		{name: "canonical_turn_target", mutate: func(expected, _ *entity.ExptCurrentItemRunSource) { expected.CanonicalTurns[0].TargetResultID = 42 }},
		{name: "turn_log_nil", mutate: func(expected, _ *entity.ExptCurrentItemRunSource) { expected.Turns[0] = nil }},
		{name: "turn_log_missing_key", mutate: func(expected, _ *entity.ExptCurrentItemRunSource) {
			expected.Turns = map[int64]*entity.ExptTurnResultRunLog{1: {ID: 21, TargetResultID: 41}}
		}},
		{name: "turn_log_id", mutate: func(expected, _ *entity.ExptCurrentItemRunSource) { expected.Turns[0].ID = 99 }},
		{name: "turn_log_target", mutate: func(expected, _ *entity.ExptCurrentItemRunSource) { expected.Turns[0].TargetResultID = 42 }},
		{name: "turn_log_evaluator_refs", mutate: func(expected, _ *entity.ExptCurrentItemRunSource) {
			expected.Turns[0].EvaluatorResultIds = &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{51: 62}}
		}},
		{name: "nil_and_empty_evaluator_refs_equal", mutate: func(expected, actual *entity.ExptCurrentItemRunSource) {
			expected.Turns[0].EvaluatorResultIds = nil
			actual.Turns[0].EvaluatorResultIds = &entity.EvaluatorResults{}
		}, want: true},
		{name: "current_refs", mutate: func(expected, _ *entity.ExptCurrentItemRunSource) {
			expected.CurrentRefs = []*entity.ExptTurnEvaluatorResultRef{{ID: 72}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			expected := ccCurrentSourceCoverageIdentitySource()
			actual := ccCurrentSourceCoverageIdentitySource()
			tc.mutate(expected, actual)
			require.Equal(t, tc.want, sameExecutionIdentity(expected, actual))
		})
	}
}

func ccCurrentSourceCoverageEvalEvent(mode entity.ExptRunMode) *entity.ExptItemEvalEvent {
	return &entity.ExptItemEvalEvent{
		SpaceID: 3, ExptID: 5, ExptRunID: 7, EvalSetItemID: 11,
		RequestID: "rq-current", ExptRunMode: mode,
	}
}

func ccCurrentSourceCoverageRunSource() *entity.ExptCurrentItemRunSource {
	return &entity.ExptCurrentItemRunSource{Current: entity.ExptItemCurrentRequest{
		SpaceID: 3, ExptID: 5, RunID: 7, ItemID: 11, ItemVersionID: 13,
		RequestID: "rq-current", Mode: entity.EvaluationModeSubmit,
	}}
}

func TestCCCurrentSourceCoverageSourceForExecution(t *testing.T) {
	t.Run("context_cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		source, err := sourceForExecution(ctx, nil, ccCurrentSourceCoverageEvalEvent(entity.EvaluationModeSubmit))
		require.Nil(t, source)
		require.True(t, errno.NeedMQRetry(err))
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("frozen_execution_source_reused", func(t *testing.T) {
		frozen := ccCurrentSourceCoverageRunSource()
		ctx := context.WithValue(context.Background(), currentItemSourceKey{}, &currentItemExecution{Source: frozen})
		source, err := sourceForExecution(ctx, nil, ccCurrentSourceCoverageEvalEvent(entity.EvaluationModeSubmit))
		require.NoError(t, err)
		require.Same(t, frozen, source)
	})

	t.Run("execution_ownership_lost", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), currentItemSourceKey{}, &currentItemExecution{
			Source: ccCurrentSourceCoverageRunSource(), LockLost: true,
		})
		source, err := sourceForExecution(ctx, nil, ccCurrentSourceCoverageEvalEvent(entity.EvaluationModeSubmit))
		require.Nil(t, source)
		require.True(t, errno.NeedMQRetry(err))
		require.ErrorContains(t, err, "item execution ownership lost")
	})

	t.Run("port_read_error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		port := repomocks.NewMockIExptCurrentResultRepo(ctrl)
		fault := errors.New("current source read failed")
		port.EXPECT().GetCurrentItemRunSource(gomock.Any(), int64(3), int64(5), int64(7), int64(11)).Return(nil, fault)
		source, err := sourceForExecution(context.Background(), port, ccCurrentSourceCoverageEvalEvent(entity.EvaluationModeSubmit))
		require.Nil(t, source)
		require.True(t, errno.NeedMQRetry(err))
		require.ErrorIs(t, err, fault)
	})

	for _, stale := range []string{"nil", "run", "request"} {
		t.Run("stale_"+stale, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			port := repomocks.NewMockIExptCurrentResultRepo(ctrl)
			observed := ccCurrentSourceCoverageRunSource()
			switch stale {
			case "nil":
				observed = nil
			case "run":
				observed.Current.RunID = 8
			case "request":
				observed.Current.RequestID = "rq-replacement"
			}
			port.EXPECT().GetCurrentItemRunSource(gomock.Any(), int64(3), int64(5), int64(7), int64(11)).Return(observed, nil)
			source, err := sourceForExecution(context.Background(), port, ccCurrentSourceCoverageEvalEvent(entity.EvaluationModeSubmit))
			require.Nil(t, source)
			require.ErrorIs(t, err, errExptNonCurrentSource)
		})
	}

	t.Run("mode_mismatch", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		port := repomocks.NewMockIExptCurrentResultRepo(ctrl)
		port.EXPECT().GetCurrentItemRunSource(gomock.Any(), int64(3), int64(5), int64(7), int64(11)).Return(ccCurrentSourceCoverageRunSource(), nil)
		source, err := sourceForExecution(context.Background(), port, ccCurrentSourceCoverageEvalEvent(entity.EvaluationModeFailRetry))
		require.Nil(t, source)
		require.True(t, errno.NeedMQRetry(err))
		require.ErrorContains(t, err, "event source mode mismatch")
	})

	t.Run("unknown_event_mode_accepted", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		port := repomocks.NewMockIExptCurrentResultRepo(ctrl)
		observed := ccCurrentSourceCoverageRunSource()
		port.EXPECT().GetCurrentItemRunSource(gomock.Any(), int64(3), int64(5), int64(7), int64(11)).Return(observed, nil)
		source, err := sourceForExecution(context.Background(), port, ccCurrentSourceCoverageEvalEvent(entity.EvaluationModeUnknown))
		require.NoError(t, err)
		require.Same(t, observed, source)
	})

	t.Run("current_source", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		port := repomocks.NewMockIExptCurrentResultRepo(ctrl)
		observed := ccCurrentSourceCoverageRunSource()
		port.EXPECT().GetCurrentItemRunSource(gomock.Any(), int64(3), int64(5), int64(7), int64(11)).Return(observed, nil)
		source, err := sourceForExecution(context.Background(), port, ccCurrentSourceCoverageEvalEvent(entity.EvaluationModeSubmit))
		require.NoError(t, err)
		require.Same(t, observed, source)
	})
}

func TestCCCurrentSourceCoverageCheckedCurrentWrite(t *testing.T) {
	fault := errors.New("write failed")
	err := checkedCurrentWrite(context.Background(), false, fault)
	require.True(t, errno.NeedMQRetry(err))
	require.ErrorIs(t, err, fault)

	err = checkedCurrentWrite(context.Background(), false, nil)
	require.ErrorIs(t, err, errExptNonCurrentSource)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = checkedCurrentWrite(ctx, true, nil)
	require.True(t, errno.NeedMQRetry(err))
	require.ErrorIs(t, err, context.Canceled)

	// A stale write is rejected before cancellation is considered.
	require.ErrorIs(t, checkedCurrentWrite(ctx, false, nil), errExptNonCurrentSource)
	require.NoError(t, checkedCurrentWrite(context.Background(), true, nil))
}

func TestCCCurrentSourceCoverageFrozenTargetRecordIDs(t *testing.T) {
	require.Empty(t, frozenTargetRecordIDs(&entity.ExptCurrentItemRunSource{}))

	t.Run("single", func(t *testing.T) {
		source := &entity.ExptCurrentItemRunSource{Turns: map[int64]*entity.ExptTurnResultRunLog{
			0: {TurnID: 0, TargetResultID: 41},
		}}
		require.Equal(t, []int64{41}, frozenTargetRecordIDs(source))
	})

	t.Run("dedupe_and_nil_skip", func(t *testing.T) {
		source := &entity.ExptCurrentItemRunSource{Turns: map[int64]*entity.ExptTurnResultRunLog{
			0: {TurnID: 0, TargetResultID: 41},
			1: {TurnID: 1, TargetResultID: 41},
			2: {TurnID: 2, TargetResultID: 0},
			3: nil,
			4: {TurnID: 4, TargetResultID: -7},
		}}
		require.Equal(t, []int64{41}, frozenTargetRecordIDs(source))
	})

	t.Run("canonical_only_when_turn_log_missing", func(t *testing.T) {
		source := &entity.ExptCurrentItemRunSource{
			Turns: map[int64]*entity.ExptTurnResultRunLog{
				0: {TurnID: 0, TargetResultID: 41},
			},
			CanonicalTurns: []*entity.ExptTurnResult{
				{TurnID: 0, TargetResultID: 41}, // covered by run log
				{TurnID: 1, TargetResultID: 42}, // missing run log
				{TurnID: 2, TargetResultID: 41}, // duplicate of turn 0 ID
				nil,
				{TurnID: 3, TargetResultID: -1},
			},
		}
		require.ElementsMatch(t, []int64{41, 42}, frozenTargetRecordIDs(source))
		require.Len(t, frozenTargetRecordIDs(source), 2)
	})
}

func ccCurrentSourceCoverageRawTurnSource() *entity.ExptCurrentItemRunSource {
	return &entity.ExptCurrentItemRunSource{
		Current: entity.ExptItemCurrentRequest{SpaceID: 3, ExptID: 5, RunID: 7, ItemID: 11},
		CanonicalTurns: []*entity.ExptTurnResult{
			{ID: 21, SpaceID: 3, ExptID: 5, ExptRunID: 7, ItemID: 11, ItemVersionID: 13, TurnID: 0, TargetResultID: 41},
			{ID: 22, SpaceID: 3, ExptID: 5, ExptRunID: 7, ItemID: 11, ItemVersionID: 13, TurnID: 1, TargetResultID: 42},
		},
		Turns: map[int64]*entity.ExptTurnResultRunLog{
			0: {ID: 21, TurnID: 0},
		},
	}
}

func TestCCCurrentSourceCoveragePrepareMissingRawTurnLogs(t *testing.T) {
	t.Run("reuse_request_skips_preparation", func(t *testing.T) {
		source := ccCurrentSourceCoverageRawTurnSource()
		source.Current.RequestID = "rq-reused"
		write := &entity.ExptCurrentItemRunWrite{}
		require.NoError(t, prepareMissingRawTurnLogs(context.Background(), nil, source, write))
		require.Empty(t, write.MissingTurnLogs)
	})

	t.Run("retry_only_skips_preparation", func(t *testing.T) {
		write := &entity.ExptCurrentItemRunWrite{RetryOnly: true}
		require.NoError(t, prepareMissingRawTurnLogs(context.Background(), nil, ccCurrentSourceCoverageRawTurnSource(), write))
		require.Empty(t, write.MissingTurnLogs)
	})

	t.Run("no_missing_turns", func(t *testing.T) {
		source := ccCurrentSourceCoverageRawTurnSource()
		source.Turns[1] = &entity.ExptTurnResultRunLog{ID: 22, TurnID: 1}
		write := &entity.ExptCurrentItemRunWrite{}
		require.NoError(t, prepareMissingRawTurnLogs(context.Background(), nil, source, write))
		require.Empty(t, write.MissingTurnLogs)
	})

	t.Run("context_cancelled_before", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := prepareMissingRawTurnLogs(ctx, nil, ccCurrentSourceCoverageRawTurnSource(), &entity.ExptCurrentItemRunWrite{})
		require.True(t, errno.NeedMQRetry(err))
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("generator_missing", func(t *testing.T) {
		err := prepareMissingRawTurnLogs(context.Background(), nil, ccCurrentSourceCoverageRawTurnSource(), &entity.ExptCurrentItemRunWrite{})
		require.True(t, errno.NeedMQRetry(err))
		require.ErrorContains(t, err, "raw turn log ID preparation unavailable")
	})

	t.Run("generator_error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		gen := idmocks.NewMockIIDGenerator(ctrl)
		fault := errors.New("id generation failed")
		gen.EXPECT().GenMultiIDs(gomock.Any(), 1).Return(nil, fault)
		err := prepareMissingRawTurnLogs(context.Background(), gen, ccCurrentSourceCoverageRawTurnSource(), &entity.ExptCurrentItemRunWrite{})
		require.True(t, errno.NeedMQRetry(err))
		require.ErrorIs(t, err, fault)
	})

	t.Run("generator_id_count_mismatch", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		gen := idmocks.NewMockIIDGenerator(ctrl)
		gen.EXPECT().GenMultiIDs(gomock.Any(), 1).Return([]int64{}, nil)
		err := prepareMissingRawTurnLogs(context.Background(), gen, ccCurrentSourceCoverageRawTurnSource(), &entity.ExptCurrentItemRunWrite{})
		require.True(t, errno.NeedMQRetry(err))
		require.ErrorContains(t, err, "raw turn log ID preparation incomplete")
	})

	t.Run("cancelled_during_generation", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		gen := idmocks.NewMockIIDGenerator(ctrl)
		ctx, cancel := context.WithCancel(context.Background())
		gen.EXPECT().GenMultiIDs(gomock.Any(), 1).DoAndReturn(func(context.Context, int) ([]int64, error) {
			cancel()
			return []int64{901}, nil
		})
		write := &entity.ExptCurrentItemRunWrite{}
		err := prepareMissingRawTurnLogs(ctx, gen, ccCurrentSourceCoverageRawTurnSource(), write)
		require.True(t, errno.NeedMQRetry(err))
		require.ErrorIs(t, err, context.Canceled)
		require.Empty(t, write.MissingTurnLogs)
	})

	t.Run("fail_status_is_prepared", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		gen := idmocks.NewMockIIDGenerator(ctrl)
		gen.EXPECT().GenMultiIDs(gomock.Any(), 1).Return([]int64{901}, nil)
		write := &entity.ExptCurrentItemRunWrite{Status: entity.ItemRunState_Fail}
		require.NoError(t, prepareMissingRawTurnLogs(context.Background(), gen, ccCurrentSourceCoverageRawTurnSource(), write))
		require.Len(t, write.MissingTurnLogs, 1)
		log := write.MissingTurnLogs[0]
		require.Equal(t, int64(901), log.ID)
		require.Equal(t, int64(7), log.ExptRunID)
		require.Equal(t, int64(11), log.ItemID)
		require.Equal(t, int64(13), log.ItemVersionID)
		require.Equal(t, int64(1), log.TurnID)
		require.Equal(t, entity.TurnRunState_Fail, log.Status)
		require.NotEmpty(t, log.ErrMsg)
	})

	t.Run("yield_status_is_queueing", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		gen := idmocks.NewMockIIDGenerator(ctrl)
		gen.EXPECT().GenMultiIDs(gomock.Any(), 1).Return([]int64{901}, nil)
		write := &entity.ExptCurrentItemRunWrite{Yield: true}
		require.NoError(t, prepareMissingRawTurnLogs(context.Background(), gen, ccCurrentSourceCoverageRawTurnSource(), write))
		require.Len(t, write.MissingTurnLogs, 1)
		require.Equal(t, entity.TurnRunState_Queueing, write.MissingTurnLogs[0].Status)
		require.Empty(t, write.MissingTurnLogs[0].ErrMsg)
	})

	t.Run("terminal_status_overrides_yield", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		gen := idmocks.NewMockIIDGenerator(ctrl)
		gen.EXPECT().GenMultiIDs(gomock.Any(), 1).Return([]int64{901}, nil)
		write := &entity.ExptCurrentItemRunWrite{Status: entity.ItemRunState_Terminal, Yield: true}
		require.NoError(t, prepareMissingRawTurnLogs(context.Background(), gen, ccCurrentSourceCoverageRawTurnSource(), write))
		require.Len(t, write.MissingTurnLogs, 1)
		require.Equal(t, entity.TurnRunState_Terminal, write.MissingTurnLogs[0].Status)
		require.NotEmpty(t, write.MissingTurnLogs[0].ErrMsg)
	})

	t.Run("multiple_missing_turns_keep_canonical_order", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		gen := idmocks.NewMockIIDGenerator(ctrl)
		gen.EXPECT().GenMultiIDs(gomock.Any(), 2).Return([]int64{901, 902}, nil)
		source := ccCurrentSourceCoverageRawTurnSource()
		delete(source.Turns, 0)
		write := &entity.ExptCurrentItemRunWrite{}
		require.NoError(t, prepareMissingRawTurnLogs(context.Background(), gen, source, write))
		require.Len(t, write.MissingTurnLogs, 2)
		require.Equal(t, int64(901), write.MissingTurnLogs[0].ID)
		require.Equal(t, int64(0), write.MissingTurnLogs[0].TurnID)
		require.Equal(t, int64(902), write.MissingTurnLogs[1].ID)
		require.Equal(t, int64(1), write.MissingTurnLogs[1].TurnID)
	})
}

func TestCCCurrentSourceCoveragePrepareMissingRawTurnLogsWrapper(t *testing.T) {
	svc := &ExptItemEventEvalServiceImpl{}
	source := &entity.ExptCurrentItemRunSource{Current: entity.ExptItemCurrentRequest{RequestID: "rq-reused"}}
	write := &entity.ExptCurrentItemRunWrite{}
	require.NoError(t, svc.prepareMissingRawTurnLogs(context.Background(), source, write))
	require.Empty(t, write.MissingTurnLogs)
}

type ccCurrentSourceCoverageResumeFixture struct {
	svc     *ExptItemEventEvalServiceImpl
	turns   *repomocks.MockIExptTurnResultRepo
	target  *svcmocks.MockIEvalTargetService
	records *svcmocks.MockEvaluatorRecordService
	manager *svcmocks.MockIExptManager
	event   *entity.ExptItemEvalEvent
	current *entity.ExptItemCurrentRequest
}

func ccCurrentSourceCoverageNewResumeFixture(t *testing.T, kind string) *ccCurrentSourceCoverageResumeFixture {
	t.Helper()
	ctrl := gomock.NewController(t)
	turns := repomocks.NewMockIExptTurnResultRepo(ctrl)
	target := svcmocks.NewMockIEvalTargetService(ctrl)
	records := svcmocks.NewMockEvaluatorRecordService(ctrl)
	manager := svcmocks.NewMockIExptManager(ctrl)
	return &ccCurrentSourceCoverageResumeFixture{
		svc: &ExptItemEventEvalServiceImpl{
			exptTurnResultRepo: turns, evaTargetService: target,
			evaluatorRecordService: records, manager: manager,
		},
		turns: turns, target: target, records: records, manager: manager,
		event: &entity.ExptItemEvalEvent{
			SpaceID: 3, ExptID: 5, ExptRunID: 7, EvalSetItemID: 11,
			ResumeSource: &entity.ExptResumeSource{Kind: kind, TurnID: 0, RecordID: 61, InvokeID: "61"},
		},
		current: &entity.ExptItemCurrentRequest{SpaceID: 3, ExptID: 5, RunID: 7, ItemID: 11, ItemVersionID: 13, Mode: entity.EvaluationModeSubmit},
	}
}

func ccCurrentSourceCoverageResumeTurn() *entity.ExptTurnResultRunLog {
	return &entity.ExptTurnResultRunLog{
		SpaceID: 3, ExptID: 5, ExptRunID: 7, ItemID: 11, ItemVersionID: 13,
		TurnID: 0, TargetResultID: 61, Status: entity.TurnRunState_Processing,
	}
}

func ccCurrentSourceCoverageExpectTurn(f *ccCurrentSourceCoverageResumeFixture, logs []*entity.ExptTurnResultRunLog, err error) {
	f.turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), int64(5), int64(7), int64(11), int64(3)).Return(logs, err)
}

func ccCurrentSourceCoverageExpectEvaluator(f *ccCurrentSourceCoverageResumeFixture, turn *entity.ExptTurnResultRunLog, record *entity.EvaluatorRecord, err error) {
	ccCurrentSourceCoverageExpectTurn(f, []*entity.ExptTurnResultRunLog{turn}, nil)
	f.records.EXPECT().GetEvaluatorRecord(gomock.Any(), int64(61), false).Return(record, err)
}

func ccCurrentSourceCoverageExpectTarget(f *ccCurrentSourceCoverageResumeFixture, turn *entity.ExptTurnResultRunLog, space int64, record *entity.EvalTargetRecord, err error) {
	ccCurrentSourceCoverageExpectTurn(f, []*entity.ExptTurnResultRunLog{turn}, nil)
	f.target.EXPECT().GetRecordByID(gomock.Any(), space, int64(61)).Return(record, err)
}

func ccCurrentSourceCoverageTargetRecord(status entity.EvalTargetRunStatus) *entity.EvalTargetRecord {
	return &entity.EvalTargetRecord{
		ID: 61, SpaceID: 3, ExperimentRunID: 7, ItemID: 11,
		ItemVersionID: 13, TurnID: 0, Status: &status,
	}
}

func ccCurrentSourceCoverageEvaluatorRecord() *entity.EvaluatorRecord {
	return &entity.EvaluatorRecord{
		ID: 61, SpaceID: 3, ExperimentID: 5, ExperimentRunID: 7,
		ItemID: 11, ItemVersionID: 13, TurnID: 0,
		EvaluatorVersionID: 51, Alias: "a",
		SourceType: entity.EvaluatorRecordSourceTypeBuiltin,
		Status:     entity.EvaluatorRunStatusSuccess,
	}
}

func ccCurrentSourceCoverageEvaluatorRefs() *entity.EvaluatorResults {
	return &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: 51, Alias: "a", RecordID: 61}}}
}

func TestCCCurrentSourceCoverageValidateResumeWithoutSource(t *testing.T) {
	for _, tc := range []struct {
		name   string
		async  bool
		asyncE bool
		want   bool
		retry  bool
	}{
		{name: "plain_legacy_event", want: true},
		{name: "async_report_without_identity", async: true, retry: true},
		{name: "async_evaluator_report_without_identity", asyncE: true, retry: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := ccCurrentSourceCoverageNewResumeFixture(t, "target")
			f.event.ResumeSource = nil
			f.event.AsyncReportTrigger = tc.async
			f.event.AsyncEvaluatorReportTrigger = tc.asyncE
			valid, err := f.svc.validateResumeSource(context.Background(), f.event, f.current)
			require.Equal(t, tc.want, valid)
			if tc.retry {
				require.True(t, errno.NeedMQRetry(err))
				require.ErrorContains(t, err, "legacy async event lacks original recoverable invocation identity")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCCCurrentSourceCoverageValidateResumeSourceShape(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*entity.ExptResumeSource)
	}{
		{name: "record_id_zero", mutate: func(s *entity.ExptResumeSource) { s.RecordID = 0 }},
		{name: "record_id_negative", mutate: func(s *entity.ExptResumeSource) { s.RecordID = -1 }},
		{name: "turn_id_negative", mutate: func(s *entity.ExptResumeSource) { s.TurnID = -1 }},
		{name: "invoke_id_mismatch", mutate: func(s *entity.ExptResumeSource) { s.InvokeID = "other" }},
		{name: "kind_unknown", mutate: func(s *entity.ExptResumeSource) { s.Kind = "other" }},
		{name: "kind_empty", mutate: func(s *entity.ExptResumeSource) { s.Kind = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := ccCurrentSourceCoverageNewResumeFixture(t, "target")
			tc.mutate(f.event.ResumeSource)
			valid, err := f.svc.validateResumeSource(context.Background(), f.event, f.current)
			require.False(t, valid)
			require.True(t, errno.NeedMQRetry(err))
			require.ErrorContains(t, err, "invalid original resume source")
		})
	}
}

func TestCCCurrentSourceCoverageValidateResumeTurnBranches(t *testing.T) {
	t.Run("turn_reader_missing", func(t *testing.T) {
		f := ccCurrentSourceCoverageNewResumeFixture(t, "target")
		f.svc.exptTurnResultRepo = nil
		valid, err := f.svc.validateResumeSource(context.Background(), f.event, f.current)
		require.False(t, valid)
		require.True(t, errno.NeedMQRetry(err))
		require.ErrorContains(t, err, "turn source reader missing")
	})

	t.Run("turn_read_error", func(t *testing.T) {
		f := ccCurrentSourceCoverageNewResumeFixture(t, "target")
		fault := errors.New("turn source read failed")
		ccCurrentSourceCoverageExpectTurn(f, nil, fault)
		valid, err := f.svc.validateResumeSource(context.Background(), f.event, f.current)
		require.False(t, valid)
		require.True(t, errno.NeedMQRetry(err))
		require.ErrorIs(t, err, fault)
	})

	t.Run("nil_turn_log", func(t *testing.T) {
		f := ccCurrentSourceCoverageNewResumeFixture(t, "target")
		ccCurrentSourceCoverageExpectTurn(f, []*entity.ExptTurnResultRunLog{nil}, nil)
		valid, err := f.svc.validateResumeSource(context.Background(), f.event, f.current)
		require.False(t, valid)
		require.True(t, errno.NeedMQRetry(err))
		require.ErrorContains(t, err, "incomplete current turn source")
	})

	t.Run("duplicate_turn", func(t *testing.T) {
		f := ccCurrentSourceCoverageNewResumeFixture(t, "target")
		turn := ccCurrentSourceCoverageResumeTurn()
		ccCurrentSourceCoverageExpectTurn(f, []*entity.ExptTurnResultRunLog{turn, turn}, nil)
		valid, err := f.svc.validateResumeSource(context.Background(), f.event, f.current)
		require.False(t, valid)
		require.True(t, errno.NeedMQRetry(err))
		require.ErrorContains(t, err, "duplicate current source turn")
	})

	t.Run("missing_turn", func(t *testing.T) {
		f := ccCurrentSourceCoverageNewResumeFixture(t, "target")
		turn := ccCurrentSourceCoverageResumeTurn()
		turn.TurnID = 9
		ccCurrentSourceCoverageExpectTurn(f, []*entity.ExptTurnResultRunLog{turn}, nil)
		valid, err := f.svc.validateResumeSource(context.Background(), f.event, f.current)
		require.False(t, valid)
		require.True(t, errno.NeedMQRetry(err))
		require.ErrorContains(t, err, "original source turn missing")
	})

	for _, tc := range []struct {
		name   string
		mutate func(*entity.ExptTurnResultRunLog)
	}{
		{name: "foreign_space", mutate: func(l *entity.ExptTurnResultRunLog) { l.SpaceID = 4 }},
		{name: "foreign_expt", mutate: func(l *entity.ExptTurnResultRunLog) { l.ExptID = 6 }},
		{name: "foreign_run", mutate: func(l *entity.ExptTurnResultRunLog) { l.ExptRunID = 8 }},
		{name: "foreign_item", mutate: func(l *entity.ExptTurnResultRunLog) { l.ItemID = 12 }},
		{name: "foreign_item_version", mutate: func(l *entity.ExptTurnResultRunLog) { l.ItemVersionID = 14 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := ccCurrentSourceCoverageNewResumeFixture(t, "target")
			turn := ccCurrentSourceCoverageResumeTurn()
			tc.mutate(turn)
			ccCurrentSourceCoverageExpectTurn(f, []*entity.ExptTurnResultRunLog{turn}, nil)
			valid, err := f.svc.validateResumeSource(context.Background(), f.event, f.current)
			require.False(t, valid)
			require.True(t, errno.NeedMQRetry(err))
			require.ErrorContains(t, err, "foreign or incomplete source turn")
		})
	}

	for _, tc := range []struct {
		name   string
		status entity.TurnRunState
	}{
		{name: "below_queueing", status: entity.TurnRunState(-1)},
		{name: "above_terminal", status: entity.TurnRunState(5)},
	} {
		t.Run("state_"+tc.name, func(t *testing.T) {
			f := ccCurrentSourceCoverageNewResumeFixture(t, "target")
			turn := ccCurrentSourceCoverageResumeTurn()
			turn.Status = tc.status
			ccCurrentSourceCoverageExpectTurn(f, []*entity.ExptTurnResultRunLog{turn}, nil)
			valid, err := f.svc.validateResumeSource(context.Background(), f.event, f.current)
			require.False(t, valid)
			require.True(t, errno.NeedMQRetry(err))
			require.ErrorContains(t, err, "invalid source turn state")
		})
	}

	for _, tc := range []struct {
		name   string
		status entity.TurnRunState
	}{
		{name: "queueing", status: entity.TurnRunState_Queueing},
		{name: "terminal", status: entity.TurnRunState_Terminal},
	} {
		t.Run("legal_boundary_state_"+tc.name, func(t *testing.T) {
			f := ccCurrentSourceCoverageNewResumeFixture(t, "target")
			f.svc.manager = nil
			turn := ccCurrentSourceCoverageResumeTurn()
			turn.Status = tc.status
			ccCurrentSourceCoverageExpectTarget(f, turn, int64(3), ccCurrentSourceCoverageTargetRecord(entity.EvalTargetRunStatusSuccess), nil)
			valid, err := f.svc.validateResumeSource(context.Background(), f.event, f.current)
			require.NoError(t, err)
			require.True(t, valid)
		})
	}

	t.Run("frozen_execution_turns_reused", func(t *testing.T) {
		f := ccCurrentSourceCoverageNewResumeFixture(t, "target")
		// The reader is required as an interface even though the frozen source
		// supplies the turns; it must not be queried.
		f.svc.manager = nil
		turn := ccCurrentSourceCoverageResumeTurn()
		ctx := context.WithValue(context.Background(), currentItemSourceKey{}, &currentItemExecution{
			Source: &entity.ExptCurrentItemRunSource{Turns: map[int64]*entity.ExptTurnResultRunLog{0: turn}},
		})
		f.target.EXPECT().GetRecordByID(gomock.Any(), int64(3), int64(61)).Return(ccCurrentSourceCoverageTargetRecord(entity.EvalTargetRunStatusSuccess), nil)
		valid, err := f.svc.validateResumeSource(ctx, f.event, f.current)
		require.NoError(t, err)
		require.True(t, valid)
	})
}

func TestCCCurrentSourceCoverageValidateResumeTarget(t *testing.T) {
	t.Run("target_result_id_mismatch_short_circuits", func(t *testing.T) {
		f := ccCurrentSourceCoverageNewResumeFixture(t, "target")
		f.svc.evaTargetService = nil
		f.svc.manager = nil
		turn := ccCurrentSourceCoverageResumeTurn()
		turn.TargetResultID = 42
		ccCurrentSourceCoverageExpectTurn(f, []*entity.ExptTurnResultRunLog{turn}, nil)
		valid, err := f.svc.validateResumeSource(context.Background(), f.event, f.current)
		require.NoError(t, err)
		require.False(t, valid)
	})

	t.Run("target_reader_missing", func(t *testing.T) {
		f := ccCurrentSourceCoverageNewResumeFixture(t, "target")
		f.svc.evaTargetService = nil
		f.svc.manager = nil
		ccCurrentSourceCoverageExpectTurn(f, []*entity.ExptTurnResultRunLog{ccCurrentSourceCoverageResumeTurn()}, nil)
		valid, err := f.svc.validateResumeSource(context.Background(), f.event, f.current)
		require.False(t, valid)
		require.True(t, errno.NeedMQRetry(err))
		require.ErrorContains(t, err, "target source reader missing")
	})

	t.Run("target_detail_error", func(t *testing.T) {
		f := ccCurrentSourceCoverageNewResumeFixture(t, "target")
		fault := errors.New("experiment detail read failed")
		ccCurrentSourceCoverageExpectTurn(f, []*entity.ExptTurnResultRunLog{ccCurrentSourceCoverageResumeTurn()}, nil)
		f.manager.EXPECT().GetDetail(gomock.Any(), int64(5), int64(3), gomock.Any()).Return(nil, fault)
		valid, err := f.svc.validateResumeSource(context.Background(), f.event, f.current)
		require.False(t, valid)
		require.True(t, errno.NeedMQRetry(err))
		require.ErrorIs(t, err, fault)
	})

	t.Run("target_detail_missing", func(t *testing.T) {
		f := ccCurrentSourceCoverageNewResumeFixture(t, "target")
		ccCurrentSourceCoverageExpectTurn(f, []*entity.ExptTurnResultRunLog{ccCurrentSourceCoverageResumeTurn()}, nil)
		f.manager.EXPECT().GetDetail(gomock.Any(), int64(5), int64(3), gomock.Any()).Return(nil, nil)
		valid, err := f.svc.validateResumeSource(context.Background(), f.event, f.current)
		require.False(t, valid)
		require.True(t, errno.NeedMQRetry(err))
		require.ErrorContains(t, err, "original target resource source missing")
	})

	t.Run("target_space_resolved_from_experiment", func(t *testing.T) {
		f := ccCurrentSourceCoverageNewResumeFixture(t, "target")
		ccCurrentSourceCoverageExpectTurn(f, []*entity.ExptTurnResultRunLog{ccCurrentSourceCoverageResumeTurn()}, nil)
		f.manager.EXPECT().GetDetail(gomock.Any(), int64(5), int64(3), gomock.Any()).Return(&entity.Experiment{ID: 5, SpaceID: 3, TargetSpaceID: 99}, nil)
		f.target.EXPECT().GetRecordByID(gomock.Any(), int64(99), int64(61)).Return(ccCurrentSourceCoverageTargetRecord(entity.EvalTargetRunStatusSuccess), nil)
		valid, err := f.svc.validateResumeSource(context.Background(), f.event, f.current)
		require.NoError(t, err)
		require.True(t, valid)
	})

	t.Run("target_space_falls_back_to_event", func(t *testing.T) {
		f := ccCurrentSourceCoverageNewResumeFixture(t, "target")
		f.svc.manager = nil
		ccCurrentSourceCoverageExpectTarget(f, ccCurrentSourceCoverageResumeTurn(), int64(3), ccCurrentSourceCoverageTargetRecord(entity.EvalTargetRunStatusSuccess), nil)
		valid, err := f.svc.validateResumeSource(context.Background(), f.event, f.current)
		require.NoError(t, err)
		require.True(t, valid)
	})

	t.Run("target_record_read_error", func(t *testing.T) {
		f := ccCurrentSourceCoverageNewResumeFixture(t, "target")
		f.svc.manager = nil
		fault := errors.New("target record read failed")
		ccCurrentSourceCoverageExpectTarget(f, ccCurrentSourceCoverageResumeTurn(), int64(3), nil, fault)
		valid, err := f.svc.validateResumeSource(context.Background(), f.event, f.current)
		require.False(t, valid)
		require.True(t, errno.NeedMQRetry(err))
		require.ErrorIs(t, err, fault)
	})

	t.Run("target_record_missing", func(t *testing.T) {
		f := ccCurrentSourceCoverageNewResumeFixture(t, "target")
		f.svc.manager = nil
		ccCurrentSourceCoverageExpectTarget(f, ccCurrentSourceCoverageResumeTurn(), int64(3), nil, nil)
		valid, err := f.svc.validateResumeSource(context.Background(), f.event, f.current)
		require.False(t, valid)
		require.True(t, errno.NeedMQRetry(err))
		require.ErrorContains(t, err, "original target record missing")
	})

	for _, tc := range []struct {
		name   string
		mutate func(*entity.EvalTargetRecord)
	}{
		{name: "id_zero", mutate: func(r *entity.EvalTargetRecord) { r.ID = 0 }},
		{name: "space_zero", mutate: func(r *entity.EvalTargetRecord) { r.SpaceID = 0 }},
		{name: "run_zero", mutate: func(r *entity.EvalTargetRecord) { r.ExperimentRunID = 0 }},
		{name: "item_zero", mutate: func(r *entity.EvalTargetRecord) { r.ItemID = 0 }},
		{name: "turn_negative", mutate: func(r *entity.EvalTargetRecord) { r.TurnID = -1 }},
		{name: "status_nil", mutate: func(r *entity.EvalTargetRecord) { r.Status = nil }},
		{name: "status_below_success", mutate: func(r *entity.EvalTargetRecord) {
			s := entity.EvalTargetRunStatusUnknown
			r.Status = &s
		}},
		{name: "status_above_async", mutate: func(r *entity.EvalTargetRecord) {
			s := entity.EvalTargetRunStatus(4)
			r.Status = &s
		}},
	} {
		t.Run("incomplete_"+tc.name, func(t *testing.T) {
			f := ccCurrentSourceCoverageNewResumeFixture(t, "target")
			f.svc.manager = nil
			record := ccCurrentSourceCoverageTargetRecord(entity.EvalTargetRunStatusSuccess)
			tc.mutate(record)
			ccCurrentSourceCoverageExpectTarget(f, ccCurrentSourceCoverageResumeTurn(), int64(3), record, nil)
			valid, err := f.svc.validateResumeSource(context.Background(), f.event, f.current)
			require.False(t, valid)
			require.True(t, errno.NeedMQRetry(err))
			require.ErrorContains(t, err, "incomplete original target record")
		})
	}

	for _, tc := range []struct {
		name   string
		mutate func(*entity.EvalTargetRecord)
	}{
		{name: "record_id", mutate: func(r *entity.EvalTargetRecord) { r.ID = 62 }},
		{name: "run_id", mutate: func(r *entity.EvalTargetRecord) { r.ExperimentRunID = 8 }},
		{name: "item_id", mutate: func(r *entity.EvalTargetRecord) { r.ItemID = 12 }},
		{name: "item_version", mutate: func(r *entity.EvalTargetRecord) { r.ItemVersionID = 14 }},
		{name: "turn_id", mutate: func(r *entity.EvalTargetRecord) { r.TurnID = 1 }},
	} {
		t.Run("identity_"+tc.name, func(t *testing.T) {
			f := ccCurrentSourceCoverageNewResumeFixture(t, "target")
			f.svc.manager = nil
			record := ccCurrentSourceCoverageTargetRecord(entity.EvalTargetRunStatusSuccess)
			tc.mutate(record)
			ccCurrentSourceCoverageExpectTarget(f, ccCurrentSourceCoverageResumeTurn(), int64(3), record, nil)
			valid, err := f.svc.validateResumeSource(context.Background(), f.event, f.current)
			require.NoError(t, err)
			require.False(t, valid)
		})
	}

	for _, tc := range []struct {
		name   string
		status entity.EvalTargetRunStatus
	}{
		{name: "success", status: entity.EvalTargetRunStatusSuccess},
		{name: "async_invoking", status: entity.EvalTargetRunStatusAsyncInvoking},
	} {
		t.Run("legal_status_"+tc.name, func(t *testing.T) {
			f := ccCurrentSourceCoverageNewResumeFixture(t, "target")
			f.svc.manager = nil
			ccCurrentSourceCoverageExpectTarget(f, ccCurrentSourceCoverageResumeTurn(), int64(3), ccCurrentSourceCoverageTargetRecord(tc.status), nil)
			valid, err := f.svc.validateResumeSource(context.Background(), f.event, f.current)
			require.NoError(t, err)
			require.True(t, valid)
		})
	}

	t.Run("legacy_item_version_zero_allowed", func(t *testing.T) {
		f := ccCurrentSourceCoverageNewResumeFixture(t, "target")
		f.svc.manager = nil
		record := ccCurrentSourceCoverageTargetRecord(entity.EvalTargetRunStatusSuccess)
		record.ItemVersionID = 0
		ccCurrentSourceCoverageExpectTarget(f, ccCurrentSourceCoverageResumeTurn(), int64(3), record, nil)
		valid, err := f.svc.validateResumeSource(context.Background(), f.event, f.current)
		require.NoError(t, err)
		require.True(t, valid)
	})
}

func TestCCCurrentSourceCoverageValidateResumeEvaluator(t *testing.T) {
	t.Run("evaluator_reader_missing", func(t *testing.T) {
		f := ccCurrentSourceCoverageNewResumeFixture(t, "evaluator")
		f.svc.evaluatorRecordService = nil
		turn := ccCurrentSourceCoverageResumeTurn()
		turn.EvaluatorResultIds = ccCurrentSourceCoverageEvaluatorRefs()
		ccCurrentSourceCoverageExpectTurn(f, []*entity.ExptTurnResultRunLog{turn}, nil)
		valid, err := f.svc.validateResumeSource(context.Background(), f.event, f.current)
		require.False(t, valid)
		require.True(t, errno.NeedMQRetry(err))
		require.ErrorContains(t, err, "evaluator source reader missing")
	})

	t.Run("evaluator_read_error", func(t *testing.T) {
		f := ccCurrentSourceCoverageNewResumeFixture(t, "evaluator")
		fault := errors.New("evaluator record read failed")
		turn := ccCurrentSourceCoverageResumeTurn()
		ccCurrentSourceCoverageExpectEvaluator(f, turn, nil, fault)
		valid, err := f.svc.validateResumeSource(context.Background(), f.event, f.current)
		require.False(t, valid)
		require.True(t, errno.NeedMQRetry(err))
		require.ErrorIs(t, err, fault)
	})

	t.Run("evaluator_record_missing", func(t *testing.T) {
		f := ccCurrentSourceCoverageNewResumeFixture(t, "evaluator")
		turn := ccCurrentSourceCoverageResumeTurn()
		ccCurrentSourceCoverageExpectEvaluator(f, turn, nil, nil)
		valid, err := f.svc.validateResumeSource(context.Background(), f.event, f.current)
		require.False(t, valid)
		require.True(t, errno.NeedMQRetry(err))
		require.ErrorContains(t, err, "original evaluator record missing")
	})

	for _, tc := range []struct {
		name   string
		mutate func(*entity.EvaluatorRecord)
	}{
		{name: "id_zero", mutate: func(r *entity.EvaluatorRecord) { r.ID = 0 }},
		{name: "space_zero", mutate: func(r *entity.EvaluatorRecord) { r.SpaceID = 0 }},
		{name: "experiment_zero", mutate: func(r *entity.EvaluatorRecord) { r.ExperimentID = 0 }},
		{name: "run_zero", mutate: func(r *entity.EvaluatorRecord) { r.ExperimentRunID = 0 }},
		{name: "item_zero", mutate: func(r *entity.EvaluatorRecord) { r.ItemID = 0 }},
		{name: "turn_negative", mutate: func(r *entity.EvaluatorRecord) { r.TurnID = -1 }},
		{name: "status_below_success", mutate: func(r *entity.EvaluatorRecord) { r.Status = entity.EvaluatorRunStatusUnknown }},
		{name: "status_above_skipped", mutate: func(r *entity.EvaluatorRecord) { r.Status = entity.EvaluatorRunStatus(5) }},
		{name: "source_type_negative", mutate: func(r *entity.EvaluatorRecord) { r.SourceType = entity.EvaluatorRecordSourceType(-1) }},
		{name: "source_type_above_inline", mutate: func(r *entity.EvaluatorRecord) { r.SourceType = entity.EvaluatorRecordSourceType(3) }},
	} {
		t.Run("incomplete_"+tc.name, func(t *testing.T) {
			f := ccCurrentSourceCoverageNewResumeFixture(t, "evaluator")
			turn := ccCurrentSourceCoverageResumeTurn()
			turn.EvaluatorResultIds = ccCurrentSourceCoverageEvaluatorRefs()
			record := ccCurrentSourceCoverageEvaluatorRecord()
			tc.mutate(record)
			ccCurrentSourceCoverageExpectEvaluator(f, turn, record, nil)
			valid, err := f.svc.validateResumeSource(context.Background(), f.event, f.current)
			require.False(t, valid)
			require.True(t, errno.NeedMQRetry(err))
			require.ErrorContains(t, err, "incomplete original evaluator record")
		})
	}

	for _, tc := range []struct {
		name   string
		mutate func(*entity.EvaluatorRecord)
	}{
		{name: "record_id", mutate: func(r *entity.EvaluatorRecord) { r.ID = 62 }},
		{name: "experiment_id", mutate: func(r *entity.EvaluatorRecord) { r.ExperimentID = 6 }},
		{name: "run_id", mutate: func(r *entity.EvaluatorRecord) { r.ExperimentRunID = 8 }},
		{name: "item_id", mutate: func(r *entity.EvaluatorRecord) { r.ItemID = 12 }},
		{name: "item_version", mutate: func(r *entity.EvaluatorRecord) { r.ItemVersionID = 14 }},
		{name: "turn_id", mutate: func(r *entity.EvaluatorRecord) { r.TurnID = 1 }},
	} {
		t.Run("identity_"+tc.name, func(t *testing.T) {
			f := ccCurrentSourceCoverageNewResumeFixture(t, "evaluator")
			turn := ccCurrentSourceCoverageResumeTurn()
			turn.EvaluatorResultIds = ccCurrentSourceCoverageEvaluatorRefs()
			record := ccCurrentSourceCoverageEvaluatorRecord()
			tc.mutate(record)
			ccCurrentSourceCoverageExpectEvaluator(f, turn, record, nil)
			valid, err := f.svc.validateResumeSource(context.Background(), f.event, f.current)
			require.NoError(t, err)
			require.False(t, valid)
		})
	}

	t.Run("target_record_reference_mismatch", func(t *testing.T) {
		f := ccCurrentSourceCoverageNewResumeFixture(t, "evaluator")
		turn := ccCurrentSourceCoverageResumeTurn()
		turn.EvaluatorResultIds = ccCurrentSourceCoverageEvaluatorRefs()
		record := ccCurrentSourceCoverageEvaluatorRecord()
		record.TargetRecordID = 42
		ccCurrentSourceCoverageExpectEvaluator(f, turn, record, nil)
		valid, err := f.svc.validateResumeSource(context.Background(), f.event, f.current)
		require.NoError(t, err)
		require.False(t, valid)
	})

	t.Run("evaluator_refs_missing", func(t *testing.T) {
		f := ccCurrentSourceCoverageNewResumeFixture(t, "evaluator")
		turn := ccCurrentSourceCoverageResumeTurn()
		ccCurrentSourceCoverageExpectEvaluator(f, turn, ccCurrentSourceCoverageEvaluatorRecord(), nil)
		valid, err := f.svc.validateResumeSource(context.Background(), f.event, f.current)
		require.False(t, valid)
		require.True(t, errno.NeedMQRetry(err))
		require.ErrorContains(t, err, "current evaluator source references missing")
	})

	t.Run("inline_key_missing", func(t *testing.T) {
		f := ccCurrentSourceCoverageNewResumeFixture(t, "evaluator")
		turn := ccCurrentSourceCoverageResumeTurn()
		turn.EvaluatorResultIds = &entity.EvaluatorResults{Inline: []*entity.InlineEvalResult{{InlineKey: "k", RecordID: 61}}}
		record := ccCurrentSourceCoverageEvaluatorRecord()
		record.SourceType = entity.EvaluatorRecordSourceTypeInline
		record.EvaluatorVersionID = 0
		ccCurrentSourceCoverageExpectEvaluator(f, turn, record, nil)
		valid, err := f.svc.validateResumeSource(context.Background(), f.event, f.current)
		require.False(t, valid)
		require.True(t, errno.NeedMQRetry(err))
		require.ErrorContains(t, err, "incomplete original inline evaluator source")
	})

	t.Run("inline_nil_reference", func(t *testing.T) {
		f := ccCurrentSourceCoverageNewResumeFixture(t, "evaluator")
		turn := ccCurrentSourceCoverageResumeTurn()
		turn.EvaluatorResultIds = &entity.EvaluatorResults{Inline: []*entity.InlineEvalResult{nil}}
		record := ccCurrentSourceCoverageEvaluatorRecord()
		record.SourceType = entity.EvaluatorRecordSourceTypeInline
		record.EvaluatorVersionID = 0
		record.InlineKey = "k"
		ccCurrentSourceCoverageExpectEvaluator(f, turn, record, nil)
		valid, err := f.svc.validateResumeSource(context.Background(), f.event, f.current)
		require.False(t, valid)
		require.True(t, errno.NeedMQRetry(err))
		require.ErrorContains(t, err, "incomplete inline source reference")
	})

	t.Run("inline_match", func(t *testing.T) {
		f := ccCurrentSourceCoverageNewResumeFixture(t, "evaluator")
		turn := ccCurrentSourceCoverageResumeTurn()
		turn.EvaluatorResultIds = &entity.EvaluatorResults{Inline: []*entity.InlineEvalResult{{InlineKey: "k", RecordID: 61}}}
		record := ccCurrentSourceCoverageEvaluatorRecord()
		record.SourceType = entity.EvaluatorRecordSourceTypeInline
		record.EvaluatorVersionID = 0
		record.InlineKey = "k"
		ccCurrentSourceCoverageExpectEvaluator(f, turn, record, nil)
		valid, err := f.svc.validateResumeSource(context.Background(), f.event, f.current)
		require.NoError(t, err)
		require.True(t, valid)
	})

	t.Run("inline_record_mismatch", func(t *testing.T) {
		f := ccCurrentSourceCoverageNewResumeFixture(t, "evaluator")
		turn := ccCurrentSourceCoverageResumeTurn()
		turn.EvaluatorResultIds = &entity.EvaluatorResults{Inline: []*entity.InlineEvalResult{{InlineKey: "k", RecordID: 62}}}
		record := ccCurrentSourceCoverageEvaluatorRecord()
		record.SourceType = entity.EvaluatorRecordSourceTypeInline
		record.EvaluatorVersionID = 0
		record.InlineKey = "k"
		ccCurrentSourceCoverageExpectEvaluator(f, turn, record, nil)
		valid, err := f.svc.validateResumeSource(context.Background(), f.event, f.current)
		require.NoError(t, err)
		require.False(t, valid)
	})

	t.Run("inline_key_not_found", func(t *testing.T) {
		f := ccCurrentSourceCoverageNewResumeFixture(t, "evaluator")
		turn := ccCurrentSourceCoverageResumeTurn()
		turn.EvaluatorResultIds = &entity.EvaluatorResults{Inline: []*entity.InlineEvalResult{{InlineKey: "other", RecordID: 61}}}
		record := ccCurrentSourceCoverageEvaluatorRecord()
		record.SourceType = entity.EvaluatorRecordSourceTypeInline
		record.EvaluatorVersionID = 0
		record.InlineKey = "k"
		ccCurrentSourceCoverageExpectEvaluator(f, turn, record, nil)
		valid, err := f.svc.validateResumeSource(context.Background(), f.event, f.current)
		require.NoError(t, err)
		require.False(t, valid)
	})

	t.Run("registered_version_missing", func(t *testing.T) {
		f := ccCurrentSourceCoverageNewResumeFixture(t, "evaluator")
		turn := ccCurrentSourceCoverageResumeTurn()
		turn.EvaluatorResultIds = ccCurrentSourceCoverageEvaluatorRefs()
		record := ccCurrentSourceCoverageEvaluatorRecord()
		record.EvaluatorVersionID = 0
		ccCurrentSourceCoverageExpectEvaluator(f, turn, record, nil)
		valid, err := f.svc.validateResumeSource(context.Background(), f.event, f.current)
		require.False(t, valid)
		require.True(t, errno.NeedMQRetry(err))
		require.ErrorContains(t, err, "original evaluator version missing")
	})

	t.Run("registered_nil_reference", func(t *testing.T) {
		f := ccCurrentSourceCoverageNewResumeFixture(t, "evaluator")
		turn := ccCurrentSourceCoverageResumeTurn()
		turn.EvaluatorResultIds = &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{nil}}
		ccCurrentSourceCoverageExpectEvaluator(f, turn, ccCurrentSourceCoverageEvaluatorRecord(), nil)
		valid, err := f.svc.validateResumeSource(context.Background(), f.event, f.current)
		require.False(t, valid)
		require.True(t, errno.NeedMQRetry(err))
		require.ErrorContains(t, err, "incomplete registered source reference")
	})

	t.Run("registered_match", func(t *testing.T) {
		f := ccCurrentSourceCoverageNewResumeFixture(t, "evaluator")
		turn := ccCurrentSourceCoverageResumeTurn()
		turn.EvaluatorResultIds = ccCurrentSourceCoverageEvaluatorRefs()
		ccCurrentSourceCoverageExpectEvaluator(f, turn, ccCurrentSourceCoverageEvaluatorRecord(), nil)
		valid, err := f.svc.validateResumeSource(context.Background(), f.event, f.current)
		require.NoError(t, err)
		require.True(t, valid)
	})

	t.Run("registered_record_mismatch", func(t *testing.T) {
		f := ccCurrentSourceCoverageNewResumeFixture(t, "evaluator")
		turn := ccCurrentSourceCoverageResumeTurn()
		turn.EvaluatorResultIds = &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: 51, Alias: "a", RecordID: 62}}}
		ccCurrentSourceCoverageExpectEvaluator(f, turn, ccCurrentSourceCoverageEvaluatorRecord(), nil)
		valid, err := f.svc.validateResumeSource(context.Background(), f.event, f.current)
		require.NoError(t, err)
		require.False(t, valid)
	})

	t.Run("legacy_fallback_match", func(t *testing.T) {
		f := ccCurrentSourceCoverageNewResumeFixture(t, "evaluator")
		turn := ccCurrentSourceCoverageResumeTurn()
		turn.EvaluatorResultIds = &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{51: 61}}
		record := ccCurrentSourceCoverageEvaluatorRecord()
		record.Alias = ""
		ccCurrentSourceCoverageExpectEvaluator(f, turn, record, nil)
		valid, err := f.svc.validateResumeSource(context.Background(), f.event, f.current)
		require.NoError(t, err)
		require.True(t, valid)
	})

	t.Run("legacy_fallback_requires_empty_alias", func(t *testing.T) {
		f := ccCurrentSourceCoverageNewResumeFixture(t, "evaluator")
		turn := ccCurrentSourceCoverageResumeTurn()
		turn.EvaluatorResultIds = &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{51: 61}}
		ccCurrentSourceCoverageExpectEvaluator(f, turn, ccCurrentSourceCoverageEvaluatorRecord(), nil)
		valid, err := f.svc.validateResumeSource(context.Background(), f.event, f.current)
		require.NoError(t, err)
		require.False(t, valid)
	})

	t.Run("legacy_fallback_mismatch", func(t *testing.T) {
		f := ccCurrentSourceCoverageNewResumeFixture(t, "evaluator")
		turn := ccCurrentSourceCoverageResumeTurn()
		turn.EvaluatorResultIds = &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{51: 62}}
		record := ccCurrentSourceCoverageEvaluatorRecord()
		record.Alias = ""
		ccCurrentSourceCoverageExpectEvaluator(f, turn, record, nil)
		valid, err := f.svc.validateResumeSource(context.Background(), f.event, f.current)
		require.NoError(t, err)
		require.False(t, valid)
	})

	t.Run("legacy_item_version_zero_allowed", func(t *testing.T) {
		f := ccCurrentSourceCoverageNewResumeFixture(t, "evaluator")
		turn := ccCurrentSourceCoverageResumeTurn()
		turn.EvaluatorResultIds = ccCurrentSourceCoverageEvaluatorRefs()
		record := ccCurrentSourceCoverageEvaluatorRecord()
		record.ItemVersionID = 0
		ccCurrentSourceCoverageExpectEvaluator(f, turn, record, nil)
		valid, err := f.svc.validateResumeSource(context.Background(), f.event, f.current)
		require.NoError(t, err)
		require.True(t, valid)
	})

	t.Run("matching_target_reference_allowed", func(t *testing.T) {
		f := ccCurrentSourceCoverageNewResumeFixture(t, "evaluator")
		turn := ccCurrentSourceCoverageResumeTurn()
		turn.EvaluatorResultIds = ccCurrentSourceCoverageEvaluatorRefs()
		record := ccCurrentSourceCoverageEvaluatorRecord()
		record.TargetRecordID = turn.TargetResultID
		ccCurrentSourceCoverageExpectEvaluator(f, turn, record, nil)
		valid, err := f.svc.validateResumeSource(context.Background(), f.event, f.current)
		require.NoError(t, err)
		require.True(t, valid)
	})
}

type ccCurrentSourceCoverageScanRepo struct {
	repo.IExptItemResultRepo
	repo.IExptCurrentResultRepo
}

func ccCurrentSourceCoverageScannedSource() *entity.ExptCurrentItemRunSource {
	now := time.Now()
	return &entity.ExptCurrentItemRunSource{Current: entity.ExptItemCurrentRequest{
		SpaceID: 3, ExptID: 5, RunID: 7, ItemID: 11, ItemVersionID: 13,
		RequestID: "rq-current", Mode: entity.EvaluationModeSubmit,
		Status: entity.ItemRunState_Processing, RetryTimes: 2, UpdatedAt: &now,
	}}
}

func ccCurrentSourceCoverageScannedFixture() (*entity.ExptCurrentItemRunSource, *entity.ExptEvalItem, *entity.ExptScheduleEvent) {
	source := ccCurrentSourceCoverageScannedSource()
	item := &entity.ExptEvalItem{
		ItemID: 11, ExptID: 5, State: entity.ItemRunState_Processing,
		Source: source, RetryTimes: source.Current.RetryTimes, UpdatedAt: source.Current.UpdatedAt,
	}
	event := &entity.ExptScheduleEvent{SpaceID: 3, ExptID: 5, ExptRunID: 7}
	return source, item, event
}

func ccCurrentSourceCoverageScanLocker(ctrl *gomock.Controller, locked bool, lockErr error) *lockmocks.MockILocker {
	locker := lockmocks.NewMockILocker(ctrl)
	locker.EXPECT().LockWithRenew(gomock.Any(), "expt_item_eval_run_lock:5:11", 5*time.Second, 5*time.Minute).
		DoAndReturn(func(ctx context.Context, _ string, _, _ time.Duration) (bool, context.Context, func(), error) {
			if lockErr != nil {
				return false, nil, nil, lockErr
			}
			if !locked {
				return false, nil, nil, nil
			}
			return true, ctx, func() {}, nil
		})
	if locked {
		locker.EXPECT().Unlock("expt_item_eval_run_lock:5:11").Return(true, nil)
	}
	return locker
}

func ccCurrentSourceCoverageScanPort(ctrl *gomock.Controller) (*ccCurrentSourceCoverageScanRepo, *repomocks.MockIExptCurrentResultRepo) {
	port := repomocks.NewMockIExptCurrentResultRepo(ctrl)
	return &ccCurrentSourceCoverageScanRepo{
		IExptItemResultRepo:    repomocks.NewMockIExptItemResultRepo(ctrl),
		IExptCurrentResultRepo: port,
	}, port
}

func TestCCCurrentSourceCoverageWithCurrentScannedItem(t *testing.T) {
	t.Run("context_cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		scheduler := &ExptSchedulerImpl{}
		applied, err := scheduler.withCurrentScannedItem(ctx, &entity.ExptScheduleEvent{}, nil, nil)
		require.False(t, applied)
		require.True(t, errno.NeedMQRetry(err))
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("item_missing", func(t *testing.T) {
		scheduler := &ExptSchedulerImpl{}
		applied, err := scheduler.withCurrentScannedItem(context.Background(), &entity.ExptScheduleEvent{}, nil, nil)
		require.False(t, applied)
		require.True(t, errno.NeedMQRetry(err))
		require.ErrorContains(t, err, "same-scan original item source missing")
	})

	t.Run("source_missing", func(t *testing.T) {
		scheduler := &ExptSchedulerImpl{}
		applied, err := scheduler.withCurrentScannedItem(context.Background(), &entity.ExptScheduleEvent{}, &entity.ExptEvalItem{ItemID: 11}, nil)
		require.False(t, applied)
		require.True(t, errno.NeedMQRetry(err))
		require.ErrorContains(t, err, "same-scan original item source missing")
	})

	t.Run("repo_lacks_current_port", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		_, _, event := ccCurrentSourceCoverageScannedFixture()
		scheduler := &ExptSchedulerImpl{ExptItemResultRepo: repomocks.NewMockIExptItemResultRepo(ctrl)}
		item := &entity.ExptEvalItem{ItemID: 11, Source: ccCurrentSourceCoverageScannedSource()}
		applied, err := scheduler.withCurrentScannedItem(context.Background(), event, item, nil)
		require.False(t, applied)
		require.True(t, errno.NeedMQRetry(err))
		require.ErrorContains(t, err, "source-conditional scan writer unavailable")
	})

	t.Run("locker_factory_missing", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		writer, _ := ccCurrentSourceCoverageScanPort(ctrl)
		_, item, event := ccCurrentSourceCoverageScannedFixture()
		scheduler := &ExptSchedulerImpl{ExptItemResultRepo: writer}
		applied, err := scheduler.withCurrentScannedItem(context.Background(), event, item, nil)
		require.False(t, applied)
		require.True(t, errno.NeedMQRetry(err))
		require.ErrorContains(t, err, "scan item locker unavailable")
	})

	t.Run("locker_factory_returns_nil", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		writer, _ := ccCurrentSourceCoverageScanPort(ctrl)
		_, item, event := ccCurrentSourceCoverageScannedFixture()
		scheduler := &ExptSchedulerImpl{ExptItemResultRepo: writer, itemLocker: func() lock.ILocker { return nil }}
		applied, err := scheduler.withCurrentScannedItem(context.Background(), event, item, nil)
		require.False(t, applied)
		require.True(t, errno.NeedMQRetry(err))
		require.ErrorContains(t, err, "scan item locker unavailable")
	})

	t.Run("lock_error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		writer, _ := ccCurrentSourceCoverageScanPort(ctrl)
		_, item, event := ccCurrentSourceCoverageScannedFixture()
		fault := errors.New("lock backend unavailable")
		locker := ccCurrentSourceCoverageScanLocker(ctrl, false, fault)
		scheduler := &ExptSchedulerImpl{ExptItemResultRepo: writer, itemLocker: func() lock.ILocker { return locker }}
		applied, err := scheduler.withCurrentScannedItem(context.Background(), event, item, nil)
		require.False(t, applied)
		require.True(t, errno.NeedMQRetry(err))
		require.ErrorIs(t, err, fault)
	})

	t.Run("lock_not_acquired", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		writer, _ := ccCurrentSourceCoverageScanPort(ctrl)
		_, item, event := ccCurrentSourceCoverageScannedFixture()
		locker := ccCurrentSourceCoverageScanLocker(ctrl, false, nil)
		scheduler := &ExptSchedulerImpl{ExptItemResultRepo: writer, itemLocker: func() lock.ILocker { return locker }}
		applied, err := scheduler.withCurrentScannedItem(context.Background(), event, item, nil)
		require.NoError(t, err)
		require.False(t, applied)
	})

	t.Run("current_read_error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		writer, port := ccCurrentSourceCoverageScanPort(ctrl)
		_, item, event := ccCurrentSourceCoverageScannedFixture()
		fault := errors.New("current source read failed")
		port.EXPECT().GetCurrentItemRunSource(gomock.Any(), int64(3), int64(5), int64(7), int64(11)).Return(nil, fault)
		locker := ccCurrentSourceCoverageScanLocker(ctrl, true, nil)
		scheduler := &ExptSchedulerImpl{ExptItemResultRepo: writer, itemLocker: func() lock.ILocker { return locker }}
		applied, err := scheduler.withCurrentScannedItem(context.Background(), event, item, nil)
		require.False(t, applied)
		require.True(t, errno.NeedMQRetry(err))
		require.ErrorIs(t, err, fault)
	})

	for _, tc := range []struct {
		name   string
		mutate func(*entity.ExptCurrentItemRunSource, *entity.ExptEvalItem)
	}{
		{name: "stale_run", mutate: func(current *entity.ExptCurrentItemRunSource, _ *entity.ExptEvalItem) { current.Current.RunID = 8 }},
		{name: "stale_request", mutate: func(current *entity.ExptCurrentItemRunSource, _ *entity.ExptEvalItem) {
			current.Current.RequestID = "rq-replacement"
		}},
		{name: "stale_turns", mutate: func(current *entity.ExptCurrentItemRunSource, _ *entity.ExptEvalItem) {
			current.Turns = map[int64]*entity.ExptTurnResultRunLog{0: {ID: 1}}
		}},
		{name: "not_processing", mutate: func(current *entity.ExptCurrentItemRunSource, _ *entity.ExptEvalItem) {
			current.Current.Status = entity.ItemRunState_Queueing
		}},
		{name: "terminal_item", mutate: func(current *entity.ExptCurrentItemRunSource, _ *entity.ExptEvalItem) {
			current.Current.Status = entity.ItemRunState_Terminal
		}},
		{name: "retry_count_moved", mutate: func(current *entity.ExptCurrentItemRunSource, _ *entity.ExptEvalItem) {
			current.Current.RetryTimes = 3
		}},
		{name: "scanned_updated_at_missing", mutate: func(_ *entity.ExptCurrentItemRunSource, item *entity.ExptEvalItem) {
			item.Source.Current.UpdatedAt = nil
		}},
		{name: "current_updated_at_missing", mutate: func(current *entity.ExptCurrentItemRunSource, _ *entity.ExptEvalItem) {
			current.Current.UpdatedAt = nil
		}},
		{name: "updated_at_moved", mutate: func(current *entity.ExptCurrentItemRunSource, _ *entity.ExptEvalItem) {
			moved := current.Current.UpdatedAt.Add(time.Second)
			current.Current.UpdatedAt = &moved
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			writer, port := ccCurrentSourceCoverageScanPort(ctrl)
			_, item, event := ccCurrentSourceCoverageScannedFixture()
			current := ccCurrentSourceCoverageScannedSource()
			current.Current.UpdatedAt = item.Source.Current.UpdatedAt
			tc.mutate(current, item)
			port.EXPECT().GetCurrentItemRunSource(gomock.Any(), int64(3), int64(5), int64(7), int64(11)).Return(current, nil)
			locker := ccCurrentSourceCoverageScanLocker(ctrl, true, nil)
			scheduler := &ExptSchedulerImpl{ExptItemResultRepo: writer, itemLocker: func() lock.ILocker { return locker }}
			applied, err := scheduler.withCurrentScannedItem(context.Background(), event, item, func(context.Context, repo.IExptCurrentResultRepo) (bool, error) {
				t.Fatal("stale scan must not invoke the callback")
				return false, nil
			})
			require.NoError(t, err)
			require.False(t, applied)
		})
	}

	t.Run("callback_error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		writer, port := ccCurrentSourceCoverageScanPort(ctrl)
		source, item, event := ccCurrentSourceCoverageScannedFixture()
		current := ccCurrentSourceCoverageScannedSource()
		current.Current.UpdatedAt = source.Current.UpdatedAt
		port.EXPECT().GetCurrentItemRunSource(gomock.Any(), int64(3), int64(5), int64(7), int64(11)).Return(current, nil)
		locker := ccCurrentSourceCoverageScanLocker(ctrl, true, nil)
		scheduler := &ExptSchedulerImpl{ExptItemResultRepo: writer, itemLocker: func() lock.ILocker { return locker }}
		fault := errors.New("callback failed")
		applied, err := scheduler.withCurrentScannedItem(context.Background(), event, item, func(context.Context, repo.IExptCurrentResultRepo) (bool, error) {
			return false, fault
		})
		require.False(t, applied)
		require.True(t, errno.NeedMQRetry(err))
		require.ErrorIs(t, err, fault)
	})

	t.Run("lock_context_cancelled_before_callback", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		writer, port := ccCurrentSourceCoverageScanPort(ctrl)
		source, item, event := ccCurrentSourceCoverageScannedFixture()
		current := ccCurrentSourceCoverageScannedSource()
		current.Current.UpdatedAt = source.Current.UpdatedAt
		port.EXPECT().GetCurrentItemRunSource(gomock.Any(), int64(3), int64(5), int64(7), int64(11)).Return(current, nil)
		lockCtx, cancel := context.WithCancel(context.Background())
		cancel()
		locker := lockmocks.NewMockILocker(ctrl)
		locker.EXPECT().LockWithRenew(gomock.Any(), "expt_item_eval_run_lock:5:11", 5*time.Second, 5*time.Minute).
			Return(true, lockCtx, func() {}, nil)
		locker.EXPECT().Unlock("expt_item_eval_run_lock:5:11").Return(true, nil)
		scheduler := &ExptSchedulerImpl{ExptItemResultRepo: writer, itemLocker: func() lock.ILocker { return locker }}
		applied, err := scheduler.withCurrentScannedItem(context.Background(), event, item, func(context.Context, repo.IExptCurrentResultRepo) (bool, error) {
			t.Fatal("lost lock context must not invoke the callback")
			return false, nil
		})
		require.False(t, applied)
		require.True(t, errno.NeedMQRetry(err))
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("unlock_error_does_not_discard_applied_write", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		writer, port := ccCurrentSourceCoverageScanPort(ctrl)
		source, item, event := ccCurrentSourceCoverageScannedFixture()
		current := ccCurrentSourceCoverageScannedSource()
		current.Current.UpdatedAt = source.Current.UpdatedAt
		port.EXPECT().GetCurrentItemRunSource(gomock.Any(), int64(3), int64(5), int64(7), int64(11)).Return(current, nil)
		locker := lockmocks.NewMockILocker(ctrl)
		locker.EXPECT().LockWithRenew(gomock.Any(), "expt_item_eval_run_lock:5:11", 5*time.Second, 5*time.Minute).
			DoAndReturn(func(ctx context.Context, _ string, _, _ time.Duration) (bool, context.Context, func(), error) {
				return true, ctx, func() {}, nil
			})
		locker.EXPECT().Unlock("expt_item_eval_run_lock:5:11").Return(false, errors.New("unlock failed"))
		scheduler := &ExptSchedulerImpl{ExptItemResultRepo: writer, itemLocker: func() lock.ILocker { return locker }}
		applied, err := scheduler.withCurrentScannedItem(context.Background(), event, item, func(context.Context, repo.IExptCurrentResultRepo) (bool, error) {
			return true, nil
		})
		require.NoError(t, err)
		require.True(t, applied)
	})

	t.Run("current_processing_scan_applied", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		writer, port := ccCurrentSourceCoverageScanPort(ctrl)
		source, item, event := ccCurrentSourceCoverageScannedFixture()
		current := ccCurrentSourceCoverageScannedSource()
		current.Current.UpdatedAt = source.Current.UpdatedAt
		port.EXPECT().GetCurrentItemRunSource(gomock.Any(), int64(3), int64(5), int64(7), int64(11)).Return(current, nil)
		locker := ccCurrentSourceCoverageScanLocker(ctrl, true, nil)
		scheduler := &ExptSchedulerImpl{ExptItemResultRepo: writer, itemLocker: func() lock.ILocker { return locker }}
		var observed *entity.ExptCurrentItemRunSource
		applied, err := scheduler.withCurrentScannedItem(context.Background(), event, item, func(lockCtx context.Context, got repo.IExptCurrentResultRepo) (bool, error) {
			require.NotNil(t, got)
			execution := executionSource(lockCtx)
			require.NotNil(t, execution)
			observed = execution.Source
			return true, nil
		})
		require.NoError(t, err)
		require.True(t, applied)
		require.Same(t, source, observed)
	})
}
