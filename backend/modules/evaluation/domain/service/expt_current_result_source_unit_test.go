// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	idmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	componentmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// currentUnitRepo serves the item-result repository and the narrower
// source-conditional writers in one value.
type currentUnitRepo struct {
	*repomocks.MockIExptItemResultRepo
	*repomocks.MockIExptCurrentResultRepo
}

func currentUnitSource() *entity.ExptCurrentItemRunSource {
	return &entity.ExptCurrentItemRunSource{
		Current: entity.ExptItemCurrentRequest{
			SpaceID: 3, ExptID: 1, RunID: 2, ItemID: 9, ItemVersionID: 7,
			RequestID: "req", Mode: entity.EvaluationModeFailRetry, Status: entity.ItemRunState_Processing,
		},
		Turns: map[int64]*entity.ExptTurnResultRunLog{},
	}
}

func currentUnitEvent() *entity.ExptItemEvalEvent {
	return &entity.ExptItemEvalEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2, EvalSetItemID: 9, RequestID: "req", ExptRunMode: entity.EvaluationModeFailRetry}
}

type currentUnitDeps struct {
	ctrl      *gomock.Controller
	items     *currentUnitRepo
	turns     *repomocks.MockIExptTurnResultRepo
	expts     *repomocks.MockIExperimentRepo
	config    *componentmocks.MockIConfiger
	publisher *eventmocks.MockExptEventPublisher
	records   *svcmocks.MockEvaluatorRecordService
	targets   *svcmocks.MockIEvalTargetService
	manager   *svcmocks.MockIExptManager
	locker    *lockmocks.MockILocker
}

func newCurrentUnitDeps(t *testing.T) *currentUnitDeps {
	t.Helper()
	ctrl := gomock.NewController(t)
	return &currentUnitDeps{
		ctrl:      ctrl,
		items:     &currentUnitRepo{MockIExptItemResultRepo: repomocks.NewMockIExptItemResultRepo(ctrl), MockIExptCurrentResultRepo: repomocks.NewMockIExptCurrentResultRepo(ctrl)},
		turns:     repomocks.NewMockIExptTurnResultRepo(ctrl),
		expts:     repomocks.NewMockIExperimentRepo(ctrl),
		config:    componentmocks.NewMockIConfiger(ctrl),
		publisher: eventmocks.NewMockExptEventPublisher(ctrl),
		records:   svcmocks.NewMockEvaluatorRecordService(ctrl),
		targets:   svcmocks.NewMockIEvalTargetService(ctrl),
		manager:   svcmocks.NewMockIExptManager(ctrl),
		locker:    lockmocks.NewMockILocker(ctrl),
	}
}

func (d *currentUnitDeps) eventService() *ExptItemEventEvalServiceImpl {
	return &ExptItemEventEvalServiceImpl{
		manager:                d.manager,
		publisher:              d.publisher,
		exptItemResultRepo:     d.items.MockIExptItemResultRepo,
		exptTurnResultRepo:     d.turns,
		experimentRepo:         d.expts,
		configer:               d.config,
		itemLocker:             func() lock.ILocker { return d.locker },
		evaluatorRecordService: d.records,
		evaTargetService:       d.targets,
	}
}

func (d *currentUnitDeps) scheduler() *ExptSchedulerImpl {
	return &ExptSchedulerImpl{
		ExptItemResultRepo: d.items,
		ExptTurnResultRepo: d.turns,
		itemLocker:         func() lock.ILocker { return d.locker },
		evalTargetService:  d.targets,
	}
}

func TestSameExecutionIdentityUnit(t *testing.T) {
	require.False(t, sameExecutionIdentity(nil, nil))
	require.False(t, sameExecutionIdentity(currentUnitSource(), nil))

	base := currentUnitSource()
	require.True(t, sameExecutionIdentity(base, currentUnitSource()))

	changed := currentUnitSource()
	changed.Current.RequestID = "other"
	require.False(t, sameExecutionIdentity(base, changed))

	changed = currentUnitSource()
	changed.CanonicalTurns = []*entity.ExptTurnResult{{ID: 1, TurnID: 0}}
	require.False(t, sameExecutionIdentity(base, changed))

	left := currentUnitSource()
	left.CanonicalTurns = []*entity.ExptTurnResult{{ID: 1, TurnID: 0, TargetResultID: 5}}
	right := currentUnitSource()
	right.CanonicalTurns = []*entity.ExptTurnResult{{ID: 1, TurnID: 0, TargetResultID: 6}}
	require.False(t, sameExecutionIdentity(left, right))

	left.Turns = map[int64]*entity.ExptTurnResultRunLog{0: {ID: 1, TargetResultID: 5}}
	right.Turns = map[int64]*entity.ExptTurnResultRunLog{0: {ID: 1, TargetResultID: 5}}
	right.CanonicalTurns[0].TargetResultID = 5
	require.True(t, sameExecutionIdentity(left, right))
}

func TestSourceForExecutionUnit(t *testing.T) {
	d := newCurrentUnitDeps(t)
	event := currentUnitEvent()

	t.Run("cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := sourceForExecution(ctx, d.items.MockIExptCurrentResultRepo, event)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("execution_lock_lost", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), currentItemSourceKey{}, &currentItemExecution{Source: currentUnitSource(), LockLost: true})
		_, err := sourceForExecution(ctx, d.items.MockIExptCurrentResultRepo, event)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("execution_source", func(t *testing.T) {
		source := currentUnitSource()
		ctx := context.WithValue(context.Background(), currentItemSourceKey{}, &currentItemExecution{Source: source})
		got, err := sourceForExecution(ctx, d.items.MockIExptCurrentResultRepo, event)
		require.NoError(t, err)
		require.Same(t, source, got)
	})
	t.Run("read_error", func(t *testing.T) {
		d.items.MockIExptCurrentResultRepo.EXPECT().GetCurrentItemRunSource(gomock.Any(), int64(3), int64(1), int64(2), int64(9)).Return(nil, errors.New("read"))
		_, err := sourceForExecution(context.Background(), d.items.MockIExptCurrentResultRepo, event)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("non_current", func(t *testing.T) {
		source := currentUnitSource()
		source.Current.RequestID = "other"
		d.items.MockIExptCurrentResultRepo.EXPECT().GetCurrentItemRunSource(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(source, nil)
		_, err := sourceForExecution(context.Background(), d.items.MockIExptCurrentResultRepo, event)
		require.ErrorIs(t, err, errExptNonCurrentSource)
	})
	t.Run("mode_mismatch", func(t *testing.T) {
		source := currentUnitSource()
		source.Current.Mode = entity.EvaluationModeRetryAll
		d.items.MockIExptCurrentResultRepo.EXPECT().GetCurrentItemRunSource(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(source, nil)
		_, err := sourceForExecution(context.Background(), d.items.MockIExptCurrentResultRepo, event)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("unknown_mode_allowed", func(t *testing.T) {
		source := currentUnitSource()
		source.Current.Mode = entity.EvaluationModeRetryAll
		event := currentUnitEvent()
		event.ExptRunMode = entity.EvaluationModeUnknown
		d.items.MockIExptCurrentResultRepo.EXPECT().GetCurrentItemRunSource(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(source, nil)
		got, err := sourceForExecution(context.Background(), d.items.MockIExptCurrentResultRepo, event)
		require.NoError(t, err)
		require.Same(t, source, got)
	})
}

func TestCheckedCurrentWriteUnit(t *testing.T) {
	require.True(t, errno.NeedMQRetry(checkedCurrentWrite(context.Background(), true, errors.New("write"))))
	require.ErrorIs(t, checkedCurrentWrite(context.Background(), false, nil), errExptNonCurrentSource)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.True(t, errno.NeedMQRetry(checkedCurrentWrite(ctx, true, nil)))
	require.NoError(t, checkedCurrentWrite(context.Background(), true, nil))
}

func TestFrozenTargetRecordIDsUnit(t *testing.T) {
	require.Empty(t, frozenTargetRecordIDs(&entity.ExptCurrentItemRunSource{}))
	source := &entity.ExptCurrentItemRunSource{
		Turns:          map[int64]*entity.ExptTurnResultRunLog{0: {TargetResultID: 5}, 1: {TargetResultID: 5}},
		CanonicalTurns: []*entity.ExptTurnResult{{TurnID: 2, TargetResultID: 6}},
	}
	require.Equal(t, []int64{5, 6}, frozenTargetRecordIDs(source))
}

func TestPrepareMissingRawTurnLogsUnit(t *testing.T) {
	t.Run("cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		require.True(t, errno.NeedMQRetry(prepareMissingRawTurnLogs(ctx, nil, currentUnitSource(), &entity.ExptCurrentItemRunWrite{})))
	})
	t.Run("request_id_present_skips", func(t *testing.T) {
		require.NoError(t, prepareMissingRawTurnLogs(context.Background(), nil, currentUnitSource(), &entity.ExptCurrentItemRunWrite{}))
	})
	t.Run("retry_only_skips", func(t *testing.T) {
		source := currentUnitSource()
		source.Current.RequestID = ""
		source.CanonicalTurns = []*entity.ExptTurnResult{{ID: 1, TurnID: 0}}
		require.NoError(t, prepareMissingRawTurnLogs(context.Background(), nil, source, &entity.ExptCurrentItemRunWrite{RetryOnly: true}))
	})
	t.Run("no_missing_skips", func(t *testing.T) {
		source := currentUnitSource()
		source.Current.RequestID = ""
		source.CanonicalTurns = []*entity.ExptTurnResult{{ID: 1, TurnID: 0}}
		source.Turns = map[int64]*entity.ExptTurnResultRunLog{0: {TurnID: 0}}
		require.NoError(t, prepareMissingRawTurnLogs(context.Background(), nil, source, &entity.ExptCurrentItemRunWrite{}))
	})
	t.Run("generator_missing", func(t *testing.T) {
		source := currentUnitSource()
		source.Current.RequestID = ""
		source.CanonicalTurns = []*entity.ExptTurnResult{{ID: 1, TurnID: 0}}
		err := prepareMissingRawTurnLogs(context.Background(), nil, source, &entity.ExptCurrentItemRunWrite{})
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("generator_error", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		gen := idmocks.NewMockIIDGenerator(d.ctrl)
		source := currentUnitSource()
		source.Current.RequestID = ""
		source.CanonicalTurns = []*entity.ExptTurnResult{{ID: 1, TurnID: 0}}
		gen.EXPECT().GenMultiIDs(gomock.Any(), 1).Return(nil, errors.New("idgen"))
		err := prepareMissingRawTurnLogs(context.Background(), gen, source, &entity.ExptCurrentItemRunWrite{})
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("yield_and_terminal_states", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		gen := idmocks.NewMockIIDGenerator(d.ctrl)
		source := currentUnitSource()
		source.Current.RequestID = ""
		source.CanonicalTurns = []*entity.ExptTurnResult{{ID: 1, TurnID: 0, ExptRunID: 2}}
		gen.EXPECT().GenMultiIDs(gomock.Any(), 1).Return([]int64{700}, nil)
		write := &entity.ExptCurrentItemRunWrite{Yield: true}
		require.NoError(t, prepareMissingRawTurnLogs(context.Background(), gen, source, write))
		require.Len(t, write.MissingTurnLogs, 1)
		require.Equal(t, entity.TurnRunState_Queueing, write.MissingTurnLogs[0].Status)
		require.Equal(t, int64(700), write.MissingTurnLogs[0].ID)

		gen.EXPECT().GenMultiIDs(gomock.Any(), 1).Return([]int64{701}, nil)
		write = &entity.ExptCurrentItemRunWrite{Status: entity.ItemRunState_Terminal}
		require.NoError(t, prepareMissingRawTurnLogs(context.Background(), gen, source, write))
		require.Equal(t, entity.TurnRunState_Terminal, write.MissingTurnLogs[0].Status)
	})
}

func TestValidateResumeSourceUnit(t *testing.T) {
	eventWith := func(source *entity.ExptResumeSource) *entity.ExptItemEvalEvent {
		event := currentUnitEvent()
		event.ResumeSource = source
		return event
	}
	current := &entity.ExptItemCurrentRequest{ItemVersionID: 7}
	validSource := &entity.ExptResumeSource{RecordID: 61, TurnID: 0, InvokeID: "61", Kind: "evaluator"}

	t.Run("nil_source_async_rejected", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		event := currentUnitEvent()
		event.AsyncReportTrigger = true
		_, err := d.eventService().validateResumeSource(context.Background(), event, current)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("nil_source_plain_allowed", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		valid, err := d.eventService().validateResumeSource(context.Background(), currentUnitEvent(), current)
		require.NoError(t, err)
		require.True(t, valid)
	})
	for _, bad := range []*entity.ExptResumeSource{
		{RecordID: 0, TurnID: 0, InvokeID: "0", Kind: "evaluator"},
		{RecordID: 61, TurnID: -1, InvokeID: "61", Kind: "evaluator"},
		{RecordID: 61, TurnID: 0, InvokeID: "62", Kind: "evaluator"},
		{RecordID: 61, TurnID: 0, InvokeID: "61", Kind: "other"},
	} {
		t.Run("invalid_source", func(t *testing.T) {
			d := newCurrentUnitDeps(t)
			_, err := d.eventService().validateResumeSource(context.Background(), eventWith(bad), current)
			require.True(t, errno.NeedMQRetry(err))
		})
	}
	t.Run("turn_reader_missing", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		svc := d.eventService()
		svc.exptTurnResultRepo = nil
		_, err := svc.validateResumeSource(context.Background(), eventWith(validSource), current)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("execution_source_used", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		source := currentUnitSource()
		source.Turns = map[int64]*entity.ExptTurnResultRunLog{0: {ID: 1, SpaceID: 3, ExptID: 1, ExptRunID: 2, ItemID: 9, ItemVersionID: 7, TurnID: 0, Status: entity.TurnRunState_Success, TargetResultID: 61}}
		ctx := context.WithValue(context.Background(), currentItemSourceKey{}, &currentItemExecution{Source: source})
		svc := d.eventService()
		svc.manager = nil
		d.targets.EXPECT().GetRecordByID(gomock.Any(), int64(3), int64(61)).Return(&entity.EvalTargetRecord{ID: 61, SpaceID: 3, ExperimentRunID: 2, ItemID: 9, TurnID: 0, Status: gptr.Of(entity.EvalTargetRunStatusSuccess)}, nil)
		targetSource := &entity.ExptResumeSource{RecordID: 61, TurnID: 0, InvokeID: "61", Kind: "target"}
		valid, err := svc.validateResumeSource(ctx, eventWith(targetSource), current)
		require.NoError(t, err)
		require.True(t, valid)
	})
	t.Run("turn_read_error", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		d.turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), int64(1), int64(2), int64(9), int64(3)).Return(nil, errors.New("read"))
		_, err := d.eventService().validateResumeSource(context.Background(), eventWith(validSource), current)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("nil_turn_log", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		d.turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultRunLog{nil}, nil)
		_, err := d.eventService().validateResumeSource(context.Background(), eventWith(validSource), current)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("duplicate_turn", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		log := &entity.ExptTurnResultRunLog{TurnID: 0}
		d.turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultRunLog{log, log}, nil)
		_, err := d.eventService().validateResumeSource(context.Background(), eventWith(validSource), current)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("turn_missing", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		d.turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultRunLog{{TurnID: 1}}, nil)
		_, err := d.eventService().validateResumeSource(context.Background(), eventWith(validSource), current)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("foreign_turn", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		d.turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultRunLog{{SpaceID: 99, TurnID: 0}}, nil)
		_, err := d.eventService().validateResumeSource(context.Background(), eventWith(validSource), current)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("invalid_turn_state", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		d.turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultRunLog{{SpaceID: 3, ExptID: 1, ExptRunID: 2, ItemID: 9, ItemVersionID: 7, TurnID: 0, Status: entity.TurnRunState(99)}}, nil)
		_, err := d.eventService().validateResumeSource(context.Background(), eventWith(validSource), current)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("target_record_mismatch", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		d.turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultRunLog{{SpaceID: 3, ExptID: 1, ExptRunID: 2, ItemID: 9, ItemVersionID: 7, TurnID: 0, Status: entity.TurnRunState_Success, TargetResultID: 62}}, nil)
		targetSource := &entity.ExptResumeSource{RecordID: 61, TurnID: 0, InvokeID: "61", Kind: "target"}
		valid, err := d.eventService().validateResumeSource(context.Background(), eventWith(targetSource), current)
		require.NoError(t, err)
		require.False(t, valid)
	})
	t.Run("target_reader_missing", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		svc := d.eventService()
		svc.evaTargetService = nil
		d.turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultRunLog{{SpaceID: 3, ExptID: 1, ExptRunID: 2, ItemID: 9, ItemVersionID: 7, TurnID: 0, Status: entity.TurnRunState_Success, TargetResultID: 61}}, nil)
		targetSource := &entity.ExptResumeSource{RecordID: 61, TurnID: 0, InvokeID: "61", Kind: "target"}
		_, err := svc.validateResumeSource(context.Background(), eventWith(targetSource), current)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("detail_error", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		d.turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultRunLog{{SpaceID: 3, ExptID: 1, ExptRunID: 2, ItemID: 9, ItemVersionID: 7, TurnID: 0, Status: entity.TurnRunState_Success, TargetResultID: 61}}, nil)
		d.manager.EXPECT().GetDetail(gomock.Any(), int64(1), int64(3), gomock.Any()).Return(nil, errors.New("detail"))
		targetSource := &entity.ExptResumeSource{RecordID: 61, TurnID: 0, InvokeID: "61", Kind: "target"}
		_, err := d.eventService().validateResumeSource(context.Background(), eventWith(targetSource), current)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("detail_missing", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		d.turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultRunLog{{SpaceID: 3, ExptID: 1, ExptRunID: 2, ItemID: 9, ItemVersionID: 7, TurnID: 0, Status: entity.TurnRunState_Success, TargetResultID: 61}}, nil)
		d.manager.EXPECT().GetDetail(gomock.Any(), int64(1), int64(3), gomock.Any()).Return(nil, nil)
		targetSource := &entity.ExptResumeSource{RecordID: 61, TurnID: 0, InvokeID: "61", Kind: "target"}
		_, err := d.eventService().validateResumeSource(context.Background(), eventWith(targetSource), current)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("target_record_error", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		d.turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultRunLog{{SpaceID: 3, ExptID: 1, ExptRunID: 2, ItemID: 9, ItemVersionID: 7, TurnID: 0, Status: entity.TurnRunState_Success, TargetResultID: 61}}, nil)
		d.manager.EXPECT().GetDetail(gomock.Any(), int64(1), int64(3), gomock.Any()).Return(&entity.Experiment{ID: 1, SpaceID: 3, TargetSpaceID: 4}, nil)
		d.targets.EXPECT().GetRecordByID(gomock.Any(), int64(4), int64(61)).Return(nil, errors.New("read"))
		targetSource := &entity.ExptResumeSource{RecordID: 61, TurnID: 0, InvokeID: "61", Kind: "target"}
		_, err := d.eventService().validateResumeSource(context.Background(), eventWith(targetSource), current)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("target_record_missing", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		d.turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultRunLog{{SpaceID: 3, ExptID: 1, ExptRunID: 2, ItemID: 9, ItemVersionID: 7, TurnID: 0, Status: entity.TurnRunState_Success, TargetResultID: 61}}, nil)
		d.manager.EXPECT().GetDetail(gomock.Any(), int64(1), int64(3), gomock.Any()).Return(&entity.Experiment{ID: 1, SpaceID: 3}, nil)
		d.targets.EXPECT().GetRecordByID(gomock.Any(), int64(3), int64(61)).Return(nil, nil)
		targetSource := &entity.ExptResumeSource{RecordID: 61, TurnID: 0, InvokeID: "61", Kind: "target"}
		_, err := d.eventService().validateResumeSource(context.Background(), eventWith(targetSource), current)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("target_record_incomplete", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		d.turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultRunLog{{SpaceID: 3, ExptID: 1, ExptRunID: 2, ItemID: 9, ItemVersionID: 7, TurnID: 0, Status: entity.TurnRunState_Success, TargetResultID: 61}}, nil)
		d.manager.EXPECT().GetDetail(gomock.Any(), int64(1), int64(3), gomock.Any()).Return(&entity.Experiment{ID: 1, SpaceID: 3}, nil)
		d.targets.EXPECT().GetRecordByID(gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.EvalTargetRecord{ID: 61}, nil)
		targetSource := &entity.ExptResumeSource{RecordID: 61, TurnID: 0, InvokeID: "61", Kind: "target"}
		_, err := d.eventService().validateResumeSource(context.Background(), eventWith(targetSource), current)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("target_record_identity", func(t *testing.T) {
		for _, expected := range []bool{true, false} {
			d := newCurrentUnitDeps(t)
			d.turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultRunLog{{SpaceID: 3, ExptID: 1, ExptRunID: 2, ItemID: 9, ItemVersionID: 7, TurnID: 0, Status: entity.TurnRunState_Success, TargetResultID: 61}}, nil)
			d.manager.EXPECT().GetDetail(gomock.Any(), int64(1), int64(3), gomock.Any()).Return(&entity.Experiment{ID: 1, SpaceID: 3}, nil)
			record := &entity.EvalTargetRecord{ID: 61, SpaceID: 3, ExperimentRunID: 2, ItemID: 9, TurnID: 0, Status: gptr.Of(entity.EvalTargetRunStatusSuccess)}
			if !expected {
				record.ItemVersionID = 8
			}
			d.targets.EXPECT().GetRecordByID(gomock.Any(), gomock.Any(), gomock.Any()).Return(record, nil)
			targetSource := &entity.ExptResumeSource{RecordID: 61, TurnID: 0, InvokeID: "61", Kind: "target"}
			valid, err := d.eventService().validateResumeSource(context.Background(), eventWith(targetSource), current)
			require.NoError(t, err)
			require.Equal(t, expected, valid)
		}
	})
	t.Run("evaluator_reader_missing", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		svc := d.eventService()
		svc.evaluatorRecordService = nil
		d.turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultRunLog{{SpaceID: 3, ExptID: 1, ExptRunID: 2, ItemID: 9, ItemVersionID: 7, TurnID: 0, Status: entity.TurnRunState_Success}}, nil)
		_, err := svc.validateResumeSource(context.Background(), eventWith(validSource), current)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("evaluator_read_error", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		d.turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultRunLog{{SpaceID: 3, ExptID: 1, ExptRunID: 2, ItemID: 9, ItemVersionID: 7, TurnID: 0, Status: entity.TurnRunState_Success}}, nil)
		d.records.EXPECT().GetEvaluatorRecord(gomock.Any(), int64(61), false).Return(nil, errors.New("read"))
		_, err := d.eventService().validateResumeSource(context.Background(), eventWith(validSource), current)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("evaluator_record_missing", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		d.turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultRunLog{{SpaceID: 3, ExptID: 1, ExptRunID: 2, ItemID: 9, ItemVersionID: 7, TurnID: 0, Status: entity.TurnRunState_Success}}, nil)
		d.records.EXPECT().GetEvaluatorRecord(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
		_, err := d.eventService().validateResumeSource(context.Background(), eventWith(validSource), current)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("evaluator_record_incomplete", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		d.turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultRunLog{{SpaceID: 3, ExptID: 1, ExptRunID: 2, ItemID: 9, ItemVersionID: 7, TurnID: 0, Status: entity.TurnRunState_Success}}, nil)
		d.records.EXPECT().GetEvaluatorRecord(gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.EvaluatorRecord{ID: 61}, nil)
		_, err := d.eventService().validateResumeSource(context.Background(), eventWith(validSource), current)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("evaluator_record_identity", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		log := &entity.ExptTurnResultRunLog{SpaceID: 3, ExptID: 1, ExptRunID: 2, ItemID: 9, ItemVersionID: 7, TurnID: 0, Status: entity.TurnRunState_Success, EvaluatorResultIds: &entity.EvaluatorResults{}}
		d.turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultRunLog{log}, nil)
		d.records.EXPECT().GetEvaluatorRecord(gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.EvaluatorRecord{ID: 61, SpaceID: 3, ExperimentID: 99, ExperimentRunID: 2, ItemID: 9, TurnID: 0, Status: entity.EvaluatorRunStatusSuccess}, nil)
		valid, err := d.eventService().validateResumeSource(context.Background(), eventWith(validSource), current)
		require.NoError(t, err)
		require.False(t, valid)
	})
	t.Run("evaluator_target_record_mismatch", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		d.turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultRunLog{{SpaceID: 3, ExptID: 1, ExptRunID: 2, ItemID: 9, ItemVersionID: 7, TurnID: 0, Status: entity.TurnRunState_Success, TargetResultID: 61}}, nil)
		d.records.EXPECT().GetEvaluatorRecord(gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.EvaluatorRecord{ID: 61, SpaceID: 3, ExperimentID: 1, ExperimentRunID: 2, ItemID: 9, TurnID: 0, Status: entity.EvaluatorRunStatusSuccess, TargetRecordID: 62}, nil)
		valid, err := d.eventService().validateResumeSource(context.Background(), eventWith(validSource), current)
		require.NoError(t, err)
		require.False(t, valid)
	})
	t.Run("evaluator_refs_missing", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		d.turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultRunLog{{SpaceID: 3, ExptID: 1, ExptRunID: 2, ItemID: 9, ItemVersionID: 7, TurnID: 0, Status: entity.TurnRunState_Success}}, nil)
		d.records.EXPECT().GetEvaluatorRecord(gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.EvaluatorRecord{ID: 61, SpaceID: 3, ExperimentID: 1, ExperimentRunID: 2, ItemID: 9, TurnID: 0, Status: entity.EvaluatorRunStatusSuccess}, nil)
		_, err := d.eventService().validateResumeSource(context.Background(), eventWith(validSource), current)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("inline_valid_and_missing", func(t *testing.T) {
		for _, expected := range []bool{true, false} {
			d := newCurrentUnitDeps(t)
			log := &entity.ExptTurnResultRunLog{
				SpaceID: 3, ExptID: 1, ExptRunID: 2, ItemID: 9, ItemVersionID: 7, TurnID: 0, Status: entity.TurnRunState_Success,
				EvaluatorResultIds: &entity.EvaluatorResults{Inline: []*entity.InlineEvalResult{{InlineKey: "k", RecordID: 61}}},
			}
			d.turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultRunLog{log}, nil)
			record := &entity.EvaluatorRecord{ID: 61, SpaceID: 3, ExperimentID: 1, ExperimentRunID: 2, ItemID: 9, TurnID: 0, Status: entity.EvaluatorRunStatusSuccess, SourceType: entity.EvaluatorRecordSourceTypeInline, InlineKey: "k"}
			if !expected {
				record.InlineKey = "other"
			}
			d.records.EXPECT().GetEvaluatorRecord(gomock.Any(), gomock.Any(), gomock.Any()).Return(record, nil)
			valid, err := d.eventService().validateResumeSource(context.Background(), eventWith(validSource), current)
			require.NoError(t, err)
			require.Equal(t, expected, valid)
		}
	})
	t.Run("inline_key_empty", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		log := &entity.ExptTurnResultRunLog{
			SpaceID: 3, ExptID: 1, ExptRunID: 2, ItemID: 9, ItemVersionID: 7, TurnID: 0, Status: entity.TurnRunState_Success,
			EvaluatorResultIds: &entity.EvaluatorResults{Inline: []*entity.InlineEvalResult{{InlineKey: "k", RecordID: 61}}},
		}
		d.turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultRunLog{log}, nil)
		d.records.EXPECT().GetEvaluatorRecord(gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.EvaluatorRecord{ID: 61, SpaceID: 3, ExperimentID: 1, ExperimentRunID: 2, ItemID: 9, TurnID: 0, Status: entity.EvaluatorRunStatusSuccess, SourceType: entity.EvaluatorRecordSourceTypeInline}, nil)
		_, err := d.eventService().validateResumeSource(context.Background(), eventWith(validSource), current)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("registered_valid", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		log := &entity.ExptTurnResultRunLog{
			SpaceID: 3, ExptID: 1, ExptRunID: 2, ItemID: 9, ItemVersionID: 7, TurnID: 0, Status: entity.TurnRunState_Success,
			EvaluatorResultIds: &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: 51, Alias: "a", RecordID: 61}}},
		}
		d.turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultRunLog{log}, nil)
		d.records.EXPECT().GetEvaluatorRecord(gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.EvaluatorRecord{ID: 61, SpaceID: 3, ExperimentID: 1, ExperimentRunID: 2, ItemID: 9, TurnID: 0, Status: entity.EvaluatorRunStatusSuccess, EvaluatorVersionID: 51, Alias: "a"}, nil)
		valid, err := d.eventService().validateResumeSource(context.Background(), eventWith(validSource), current)
		require.NoError(t, err)
		require.True(t, valid)
	})
	t.Run("registered_version_missing", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		log := &entity.ExptTurnResultRunLog{
			SpaceID: 3, ExptID: 1, ExptRunID: 2, ItemID: 9, ItemVersionID: 7, TurnID: 0, Status: entity.TurnRunState_Success,
			EvaluatorResultIds: &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: 51, Alias: "a", RecordID: 61}}},
		}
		d.turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultRunLog{log}, nil)
		d.records.EXPECT().GetEvaluatorRecord(gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.EvaluatorRecord{ID: 61, SpaceID: 3, ExperimentID: 1, ExperimentRunID: 2, ItemID: 9, TurnID: 0, Status: entity.EvaluatorRunStatusSuccess}, nil)
		_, err := d.eventService().validateResumeSource(context.Background(), eventWith(validSource), current)
		require.True(t, errno.NeedMQRetry(err))
	})
}

func TestWithCurrentScannedItemUnit(t *testing.T) {
	event := &entity.ExptScheduleEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2}
	now := time.Now()
	item := func() *entity.ExptEvalItem {
		return &entity.ExptEvalItem{ItemID: 9, State: entity.ItemRunState_Processing, Source: currentUnitSource()}
	}

	t.Run("cancelled", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := d.scheduler().withCurrentScannedItem(ctx, event, item(), nil)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("missing_source", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		_, err := d.scheduler().withCurrentScannedItem(context.Background(), event, &entity.ExptEvalItem{}, nil)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("locker_missing", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		scheduler := d.scheduler()
		scheduler.itemLocker = nil
		_, err := scheduler.withCurrentScannedItem(context.Background(), event, item(), nil)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("locker_nil", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		scheduler := d.scheduler()
		scheduler.itemLocker = func() lock.ILocker { return nil }
		_, err := scheduler.withCurrentScannedItem(context.Background(), event, item(), nil)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("lock_error", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(false, nil, nil, errors.New("lock"))
		_, err := d.scheduler().withCurrentScannedItem(context.Background(), event, item(), nil)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("lock_busy", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(false, context.Background(), func() {}, nil)
		applied, err := d.scheduler().withCurrentScannedItem(context.Background(), event, item(), nil)
		require.NoError(t, err)
		require.False(t, applied)
	})
	t.Run("source_read_error", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		d.locker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
		d.items.MockIExptCurrentResultRepo.EXPECT().GetCurrentItemRunSource(gomock.Any(), int64(3), int64(1), int64(2), int64(9)).Return(nil, errors.New("read"))
		_, err := d.scheduler().withCurrentScannedItem(context.Background(), event, item(), nil)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("identity_changed", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		d.locker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
		other := currentUnitSource()
		other.Current.RequestID = "other"
		d.items.MockIExptCurrentResultRepo.EXPECT().GetCurrentItemRunSource(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(other, nil)
		applied, err := d.scheduler().withCurrentScannedItem(context.Background(), event, item(), nil)
		require.NoError(t, err)
		require.False(t, applied)
	})
	t.Run("updated_at_changed", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		d.locker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
		other := currentUnitSource()
		other.Current.UpdatedAt = &now
		d.items.MockIExptCurrentResultRepo.EXPECT().GetCurrentItemRunSource(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(other, nil)
		scanned := item()
		later := now.Add(time.Second)
		scanned.Source.Current.UpdatedAt = &later
		applied, err := d.scheduler().withCurrentScannedItem(context.Background(), event, scanned, func(context.Context, repo.IExptCurrentResultRepo) (bool, error) {
			t.Fatal("stale scan reached callback")
			return false, nil
		})
		require.NoError(t, err)
		require.False(t, applied)
	})
	t.Run("callback_error", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		d.locker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
		currentSource := currentUnitSource()
		currentSource.Current.UpdatedAt = &now
		d.items.MockIExptCurrentResultRepo.EXPECT().GetCurrentItemRunSource(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(currentSource, nil)
		scanned := item()
		scanned.Source.Current.UpdatedAt = &now
		_, err := d.scheduler().withCurrentScannedItem(context.Background(), event, scanned, func(context.Context, repo.IExptCurrentResultRepo) (bool, error) {
			return false, errors.New("callback")
		})
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("success", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		d.locker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
		currentSource := currentUnitSource()
		currentSource.Current.UpdatedAt = &now
		d.items.MockIExptCurrentResultRepo.EXPECT().GetCurrentItemRunSource(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(currentSource, nil)
		scanned := item()
		scanned.Source.Current.UpdatedAt = &now
		applied, err := d.scheduler().withCurrentScannedItem(context.Background(), event, scanned, func(context.Context, repo.IExptCurrentResultRepo) (bool, error) {
			return true, nil
		})
		require.NoError(t, err)
		require.True(t, applied)
	})
}

type currentUnitTerminator struct {
	*svcmocks.MockIExptManager
	called bool
	err    error
}

func (m *currentUnitTerminator) TerminateCurrentRun(context.Context, int64, int64, int64, *entity.Session, *entity.ExptCurrentItemRunSource, ...entity.CompleteExptOptionFn) (bool, error) {
	m.called = true
	return true, m.err
}

func TestHandleCurrentEventErrUnit(t *testing.T) {
	newExec := func() *currentItemExecution {
		return &currentItemExecution{Source: currentUnitSource()}
	}
	lockOK := func(d *currentUnitDeps) {
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		d.locker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
	}
	sameSource := func(d *currentUnitDeps) {
		d.items.MockIExptCurrentResultRepo.EXPECT().GetCurrentItemRunSource(gomock.Any(), int64(3), int64(1), int64(2), int64(9)).Return(currentUnitSource(), nil)
	}

	t.Run("source_missing", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		err := d.eventService().handleCurrentEventErr(context.Background(), currentUnitEvent(), errors.New("eval"), d.items.MockIExptCurrentResultRepo, &currentItemExecution{})
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("lock_lost", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		err := d.eventService().handleCurrentEventErr(context.Background(), currentUnitEvent(), errors.New("eval"), d.items.MockIExptCurrentResultRepo, &currentItemExecution{Source: currentUnitSource(), LockLost: true})
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("item_locker_missing", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		svc := d.eventService()
		svc.itemLocker = nil
		err := svc.handleCurrentEventErr(context.Background(), currentUnitEvent(), errors.New("eval"), d.items.MockIExptCurrentResultRepo, newExec())
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("factory_nil", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		svc := d.eventService()
		svc.itemLocker = func() lock.ILocker { return nil }
		err := svc.handleCurrentEventErr(context.Background(), currentUnitEvent(), errors.New("eval"), d.items.MockIExptCurrentResultRepo, newExec())
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("lock_error", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(false, nil, nil, errors.New("lock"))
		err := d.eventService().handleCurrentEventErr(context.Background(), currentUnitEvent(), errors.New("eval"), d.items.MockIExptCurrentResultRepo, newExec())
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("busy", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(false, context.Background(), func() {}, nil)
		err := d.eventService().handleCurrentEventErr(context.Background(), currentUnitEvent(), errors.New("eval"), d.items.MockIExptCurrentResultRepo, newExec())
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("read_error", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		lockOK(d)
		d.items.MockIExptCurrentResultRepo.EXPECT().GetCurrentItemRunSource(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("read"))
		err := d.eventService().handleCurrentEventErr(context.Background(), currentUnitEvent(), errors.New("eval"), d.items.MockIExptCurrentResultRepo, newExec())
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("identity_changed", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		lockOK(d)
		other := currentUnitSource()
		other.Current.RequestID = "other"
		d.items.MockIExptCurrentResultRepo.EXPECT().GetCurrentItemRunSource(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(other, nil)
		require.NoError(t, d.eventService().handleCurrentEventErr(context.Background(), currentUnitEvent(), errors.New("eval"), d.items.MockIExptCurrentResultRepo, newExec()))
	})
	t.Run("finished_terminal", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		lockOK(d)
		current := currentUnitSource()
		current.Current.Status = entity.ItemRunState_Success
		d.items.MockIExptCurrentResultRepo.EXPECT().GetCurrentItemRunSource(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(current, nil)
		require.NoError(t, d.eventService().handleCurrentEventErr(context.Background(), currentUnitEvent(), errors.New("eval"), d.items.MockIExptCurrentResultRepo, newExec()))
	})
	t.Run("resume_invalid", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		lockOK(d)
		sameSource(d)
		event := currentUnitEvent()
		event.ResumeSource = &entity.ExptResumeSource{RecordID: 0, Kind: "evaluator"}
		err := d.eventService().handleCurrentEventErr(context.Background(), event, errors.New("eval"), d.items.MockIExptCurrentResultRepo, newExec())
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("experiment_reader_missing", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		lockOK(d)
		sameSource(d)
		svc := d.eventService()
		svc.experimentRepo = nil
		err := svc.handleCurrentEventErr(context.Background(), currentUnitEvent(), errors.New("eval"), d.items.MockIExptCurrentResultRepo, newExec())
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("experiment_read_error", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		lockOK(d)
		sameSource(d)
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(nil, errors.New("read"))
		err := d.eventService().handleCurrentEventErr(context.Background(), currentUnitEvent(), errors.New("eval"), d.items.MockIExptCurrentResultRepo, newExec())
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("debt_without_terminator", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		lockOK(d)
		sameSource(d)
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3}, nil)
		d.config.EXPECT().GetErrRetryConf(gomock.Any(), int64(3), gomock.Any()).Return(&entity.RetryConf{IsInDebt: true})
		err := d.eventService().handleCurrentEventErr(context.Background(), currentUnitEvent(), errors.New("eval"), d.items.MockIExptCurrentResultRepo, newExec())
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("debt_with_terminator", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		lockOK(d)
		sameSource(d)
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3}, nil)
		d.config.EXPECT().GetErrRetryConf(gomock.Any(), int64(3), gomock.Any()).Return(&entity.RetryConf{IsInDebt: true})
		svc := d.eventService()
		terminator := &currentUnitTerminator{MockIExptManager: d.manager}
		svc.manager = terminator
		require.NoError(t, svc.handleCurrentEventErr(context.Background(), currentUnitEvent(), errors.New("eval"), d.items.MockIExptCurrentResultRepo, newExec()))
		require.True(t, terminator.called)
	})
	t.Run("retry_publish", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		lockOK(d)
		sameSource(d)
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3}, nil)
		d.config.EXPECT().GetErrRetryConf(gomock.Any(), int64(3), gomock.Any()).Return(&entity.RetryConf{RetryTimes: 1, RetryIntervalSecond: 1})
		d.items.MockIExptCurrentResultRepo.EXPECT().CompleteCurrentItemRun(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, _ *entity.ExptCurrentItemRunSource, write *entity.ExptCurrentItemRunWrite) (bool, error) {
			require.True(t, write.RetryOnly)
			require.Equal(t, int32(1), write.RetryTimes)
			return true, nil
		})
		d.publisher.EXPECT().PublishExptRecordEvalEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		require.NoError(t, d.eventService().handleCurrentEventErr(context.Background(), currentUnitEvent(), errors.New("eval"), d.items.MockIExptCurrentResultRepo, newExec()))
	})
	t.Run("retry_yield", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		lockOK(d)
		sameSource(d)
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3}, nil)
		d.config.EXPECT().GetErrRetryConf(gomock.Any(), int64(3), gomock.Any()).Return(&entity.RetryConf{RetryTimes: 1})
		d.items.MockIExptCurrentResultRepo.EXPECT().CompleteCurrentItemRun(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
		event := currentUnitEvent()
		event.Ext = map[string]string{entity.RetryYieldExtKey: "true"}
		require.NoError(t, d.eventService().handleCurrentEventErr(context.Background(), event, errors.New("eval"), d.items.MockIExptCurrentResultRepo, newExec()))
	})
	t.Run("unretriable_success", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		lockOK(d)
		sameSource(d)
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3}, nil)
		d.config.EXPECT().GetErrRetryConf(gomock.Any(), int64(3), gomock.Any()).Return(&entity.RetryConf{RetryTimes: 0})
		d.items.MockIExptCurrentResultRepo.EXPECT().CompleteCurrentItemRun(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
		require.NoError(t, d.eventService().handleCurrentEventErr(context.Background(), currentUnitEvent(), errors.New("eval"), d.items.MockIExptCurrentResultRepo, newExec()))
	})
	t.Run("write_not_applied", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		lockOK(d)
		sameSource(d)
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3}, nil)
		d.config.EXPECT().GetErrRetryConf(gomock.Any(), int64(3), gomock.Any()).Return(&entity.RetryConf{RetryTimes: 0})
		d.items.MockIExptCurrentResultRepo.EXPECT().CompleteCurrentItemRun(gomock.Any(), gomock.Any(), gomock.Any()).Return(false, nil)
		require.NoError(t, d.eventService().handleCurrentEventErr(context.Background(), currentUnitEvent(), errors.New("eval"), d.items.MockIExptCurrentResultRepo, newExec()))
	})
	t.Run("write_error", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		lockOK(d)
		sameSource(d)
		d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3}, nil)
		d.config.EXPECT().GetErrRetryConf(gomock.Any(), int64(3), gomock.Any()).Return(&entity.RetryConf{RetryTimes: 0})
		d.items.MockIExptCurrentResultRepo.EXPECT().CompleteCurrentItemRun(gomock.Any(), gomock.Any(), gomock.Any()).Return(false, errors.New("write"))
		require.True(t, errno.NeedMQRetry(d.eventService().handleCurrentEventErr(context.Background(), currentUnitEvent(), errors.New("eval"), d.items.MockIExptCurrentResultRepo, newExec())))
	})
}

func TestHandleCurrentZombiesUnit(t *testing.T) {
	now := time.Now()
	old := now.Add(-10 * time.Second)
	source := func() *entity.ExptCurrentItemRunSource {
		s := currentUnitSource()
		s.Current.UpdatedAt = &old
		return s
	}
	item := func(state entity.ItemRunState, updated *time.Time) *entity.ExptEvalItem {
		return &entity.ExptEvalItem{ItemID: 9, State: state, UpdatedAt: updated, Source: source()}
	}

	t.Run("skips_nil_and_unfinished_and_recent", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		alives, zombies, err := d.scheduler().handleCurrentZombies(context.Background(), &entity.ExptScheduleEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2}, []*entity.ExptEvalItem{nil, item(entity.ItemRunState_Success, &old), item(entity.ItemRunState_Processing, &now), item(entity.ItemRunState_Processing, nil)}, nil, 1, false)
		require.NoError(t, err)
		require.Len(t, zombies, 0)
		require.Len(t, alives, 2)
	})
	t.Run("callback_error", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(false, nil, nil, errors.New("lock"))
		_, _, err := d.scheduler().handleCurrentZombies(context.Background(), &entity.ExptScheduleEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2}, []*entity.ExptEvalItem{item(entity.ItemRunState_Processing, &old)}, nil, 1, false)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("applied_and_not_applied", func(t *testing.T) {
		for _, applied := range []bool{true, false} {
			d := newCurrentUnitDeps(t)
			scanned := item(entity.ItemRunState_Processing, &old)
			current := source()
			current.Current.UpdatedAt = &old
			d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
			d.locker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
			d.items.MockIExptCurrentResultRepo.EXPECT().GetCurrentItemRunSource(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(current, nil)
			d.items.MockIExptCurrentResultRepo.EXPECT().CompleteCurrentItemRun(gomock.Any(), gomock.Any(), gomock.Any()).Return(applied, nil)
			alives, zombies, err := d.scheduler().handleCurrentZombies(context.Background(), &entity.ExptScheduleEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2}, []*entity.ExptEvalItem{scanned}, nil, 1, false)
			require.NoError(t, err)
			if applied {
				require.Len(t, zombies, 1)
			} else {
				require.Len(t, alives, 1)
			}
		}
	})
}

func TestSweepCurrentSandboxItemsUnit(t *testing.T) {
	now := time.Now()
	old := now.Add(-10 * time.Second)
	source := func(withTarget bool) *entity.ExptCurrentItemRunSource {
		s := currentUnitSource()
		s.Current.UpdatedAt = &old
		if withTarget {
			s.CanonicalTurns = []*entity.ExptTurnResult{{ID: 71, TurnID: 0, TargetResultID: 88}}
		}
		return s
	}
	item := func(state entity.ItemRunState, withTarget bool) *entity.ExptEvalItem {
		return &entity.ExptEvalItem{ItemID: 9, State: state, UpdatedAt: &old, Source: source(withTarget)}
	}
	expt := &entity.Experiment{ID: 1, SpaceID: 3}

	t.Run("non_processing_alive", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		alives, terminated, err := d.scheduler().sweepCurrentSandboxItems(context.Background(), &entity.ExptScheduleEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2}, []*entity.ExptEvalItem{nil, item(entity.ItemRunState_Success, false)}, expt)
		require.NoError(t, err)
		require.Len(t, alives, 2)
		require.Len(t, terminated, 0)
	})
	t.Run("no_target_ids", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		d.locker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
		d.items.MockIExptCurrentResultRepo.EXPECT().GetCurrentItemRunSource(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(source(false), nil)
		alives, terminated, err := d.scheduler().sweepCurrentSandboxItems(context.Background(), &entity.ExptScheduleEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2}, []*entity.ExptEvalItem{item(entity.ItemRunState_Processing, false)}, expt)
		require.NoError(t, err)
		require.Len(t, alives, 1)
		require.Len(t, terminated, 0)
	})
	t.Run("not_stopped", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		d.locker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
		d.items.MockIExptCurrentResultRepo.EXPECT().GetCurrentItemRunSource(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(source(true), nil)
		d.targets.EXPECT().CheckSandboxTerminated(gomock.Any(), int64(3), []int64{88}).Return(nil, nil)
		alives, _, err := d.scheduler().sweepCurrentSandboxItems(context.Background(), &entity.ExptScheduleEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2}, []*entity.ExptEvalItem{item(entity.ItemRunState_Processing, true)}, expt)
		require.NoError(t, err)
		require.Len(t, alives, 1)
	})
	t.Run("stopped_terminates", func(t *testing.T) {
		d := newCurrentUnitDeps(t)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		d.locker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
		d.items.MockIExptCurrentResultRepo.EXPECT().GetCurrentItemRunSource(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(source(true), nil)
		d.targets.EXPECT().CheckSandboxTerminated(gomock.Any(), gomock.Any(), []int64{88}).Return([]int64{88}, map[int64]string{88: ""})
		d.targets.EXPECT().TerminateAsyncRecordsAndDestroySandbox(gomock.Any(), gomock.Any(), []int64{88}, gomock.Any(), gomock.Any(), false).Return(nil)
		d.items.MockIExptCurrentResultRepo.EXPECT().CompleteCurrentItemRun(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
		_, terminated, err := d.scheduler().sweepCurrentSandboxItems(context.Background(), &entity.ExptScheduleEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2}, []*entity.ExptEvalItem{item(entity.ItemRunState_Processing, true)}, expt)
		require.NoError(t, err)
		require.Len(t, terminated, 1)
	})
}
