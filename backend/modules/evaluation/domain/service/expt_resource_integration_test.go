// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/pkg/ctxcache"
	"github.com/stretchr/testify/require"
)

type resourceItemStore struct {
	repo.IExptItemResultRepo
	run          *entity.ExptItemResultRunLog
	writeErr     error
	writeContext context.Context
}

func (s *resourceItemStore) GetItemRunLog(context.Context, int64, int64, int64, int64) (*entity.ExptItemResultRunLog, error) {
	return s.run, nil
}
func (s *resourceItemStore) BatchGet(context.Context, int64, int64, []int64) ([]*entity.ExptItemResult, error) {
	return nil, nil
}

type resourceDispatchStore struct {
	repo.IExptItemDispatchRepo
	item *resourceItemStore
}

func (s *resourceDispatchStore) StartReservedItem(context.Context, int64, int64, int64, int64) (bool, error) {
	if entity.ItemRunState(s.item.run.Status) != entity.ItemRunState_Queueing {
		return false, nil
	}
	s.item.run.Status = int32(entity.ItemRunState_Processing)
	s.item.run.QuotaReservationState = entity.QuotaReservationStateNone
	return true, nil
}
func (s *resourceDispatchStore) MGetDispatchObservations(context.Context, int64, int64, int64, []int64) ([]*repo.ExptDispatchObservation, error) {
	return nil, nil
}

type resourceGuard struct {
	component.ICentralReservationGuard
	ready           bool
	confirmationErr error
	finalized       []*entity.ExptItemResultRunLog
	prepareErr      error
	prepareContext  func(context.Context) context.Context
}

func (g *resourceGuard) ConfirmRunning(context.Context, string, int64, int64) (bool, error) {
	return true, nil
}
func (g *resourceGuard) ConfirmItemRunning(_ context.Context, _ *entity.Experiment, item *entity.ExptItemResultRunLog) (bool, error) {
	if item.ID != 41 {
		return false, errors.New("wrong immutable item run identity")
	}
	return g.ready, g.confirmationErr
}
func (g *resourceGuard) PrepareItemResources(ctx context.Context, _ *entity.ExptItemEvalCtx) (context.Context, error) {
	if g.prepareErr != nil {
		return nil, g.prepareErr
	}
	if g.prepareContext != nil {
		return g.prepareContext(ctx), nil
	}
	return ctx, nil
}
func (g *resourceGuard) OnItemTerminal(ctx context.Context, item *entity.ExptItemResultRunLog) error {
	deadline, ok := ctx.Deadline()
	if !ok || time.Until(deadline) > 3*time.Second {
		return errors.New("missing short callback budget")
	}
	copy := *item
	g.finalized = append(g.finalized, &copy)
	return errors.New("center unavailable")
}

func resourceFixture() (*ExptItemEventEvalServiceImpl, *resourceItemStore, *resourceGuard, *entity.ExptItemEvalEvent, context.Context) {
	item := &resourceItemStore{run: &entity.ExptItemResultRunLog{ID: 41, SpaceID: 3, ExptID: 1, ExptRunID: 2, ItemID: 4, Status: int32(entity.ItemRunState_Queueing), QuotaReservationState: entity.QuotaReservationStateReserved}}
	guard := &resourceGuard{}
	svc := &ExptItemEventEvalServiceImpl{centralGuard: guard, exptItemResultRepo: item, dispatchRepo: &resourceDispatchStore{item: item}, mutex: cancellationMutex{}, experimentRepo: &cancellationExptStore{expt: &entity.Experiment{ID: 1, SpaceID: 3, Status: entity.ExptStatus_Processing, ExptDispatchMode: entity.ExptDispatchModeEnforce, SchedulerScope: testScope}}}
	event := &entity.ExptItemEvalEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2, EvalSetItemID: 4}
	ctx := ctxcache.Init(context.Background())
	event.WithCtxCentralAdmittedExpt(ctx, &entity.Experiment{ID: 1, SpaceID: 3, ExptDispatchMode: entity.ExptDispatchModeEnforce, SchedulerScope: testScope})
	return svc, item, guard, event, ctx
}

type resourceConfig struct {
	component.IConfiger
	retry int
}

func (c resourceConfig) GetErrRetryConf(context.Context, int64, error) *entity.RetryConf {
	return &entity.RetryConf{RetryTimes: c.retry}
}

type resourceMetric struct{ metrics.ExptMetric }

func (resourceMetric) EmitItemExecResult(int64, int64, bool, bool, bool, int64, int64) {}

type resourceBroker struct {
	events.ExptEventPublisher
	event *entity.ExptItemEvalEvent
	err   error
}

func (b *resourceBroker) PublishExptRecordEvalEvent(_ context.Context, event *entity.ExptItemEvalEvent, _ *time.Duration, _ func(*entity.ExptItemEvalEvent)) error {
	b.event = event
	return b.err
}
func (s *resourceItemStore) UpdateItemRunLog(ctx context.Context, _, _ int64, _ []int64, fields map[string]any, _ int64) error {
	s.writeContext = ctx
	if s.writeErr != nil {
		return s.writeErr
	}
	if state, ok := fields["status"].(int32); ok {
		s.run.Status = state
	}
	return nil
}
func (s *resourceItemStore) UpdateItemRunLogIfNotTerminal(ctx context.Context, expt, run int64, ids []int64, fields map[string]any, space int64) error {
	if entity.ItemRunState(s.run.Status) == entity.ItemRunState_Terminal {
		return nil
	}
	return s.UpdateItemRunLog(ctx, expt, run, ids, fields, space)
}
func (s *resourceItemStore) UpdateItemsResult(context.Context, int64, int64, []int64, map[string]any) error {
	return nil
}

type resourceScheduleMode struct {
	entity.ExptSchedulerMode
	guard     *resourceTickerGuard
	submitted int
}

func (*resourceScheduleMode) ExptStart(context.Context, *entity.ExptScheduleEvent, *entity.Experiment) error {
	return nil
}
func (*resourceScheduleMode) ScheduleStart(context.Context, *entity.ExptScheduleEvent, *entity.Experiment) error {
	return nil
}
func (m *resourceScheduleMode) ScanEvalItems(context.Context, *entity.ExptScheduleEvent, *entity.Experiment) ([]*entity.ExptEvalItem, []*entity.ExptEvalItem, []*entity.ExptEvalItem, error) {
	if !m.guard.replayed {
		return nil, nil, nil, errors.New("scan before handshake replay")
	}
	return []*entity.ExptEvalItem{{ItemID: 4, State: entity.ItemRunState_Queueing}}, nil, nil, nil
}
func (m *resourceScheduleMode) ExptEnd(_ context.Context, _ *entity.ExptScheduleEvent, _ *entity.Experiment, submitted, _ int) (bool, error) {
	m.submitted = submitted
	return false, nil
}

type resourceModeFactory struct {
	SchedulerModeFactory
	mode *resourceScheduleMode
}

func (f resourceModeFactory) NewSchedulerMode(entity.ExptRunMode) (entity.ExptSchedulerMode, error) {
	return f.mode, nil
}

type resourceManager struct {
	IExptManager
	expt *entity.Experiment
}

func (m resourceManager) GetDetail(context.Context, int64, int64, *entity.Session, ...entity.GetExptTupleOptionFn) (*entity.Experiment, error) {
	return m.expt, nil
}

type resourceTickerGuard struct {
	component.ICentralReservationGuard
	replayed   bool
	candidates []*entity.ExptEvalItem
	submitErr  error
}

func (g *resourceTickerGuard) ReplayPendingItems(context.Context, *entity.ExptScheduleEvent, *entity.Experiment) error {
	g.replayed = true
	return nil
}
func (g *resourceTickerGuard) SubmitItems(_ context.Context, _ *entity.ExptScheduleEvent, _ *entity.Experiment, items []*entity.ExptEvalItem) (int, error) {
	g.candidates = items
	return len(items), g.submitErr
}
func (resourceConfig) GetConsumerConf(context.Context) *entity.ExptConsumerConf {
	return &entity.ExptConsumerConf{}
}

func TestResourceTickerReplaysAndCountsOriginalCandidates(t *testing.T) {
	guard := &resourceTickerGuard{}
	mode := &resourceScheduleMode{guard: guard}
	svc := &ExptSchedulerImpl{Manager: resourceManager{expt: &entity.Experiment{ID: 1, SpaceID: 3, Status: entity.ExptStatus_Processing, ExptDispatchMode: entity.ExptDispatchModeEnforce}}, centralGuard: guard, schedulerModeFactory: resourceModeFactory{mode: mode}, Configer: resourceConfig{}}
	svc.Endpoints = svc.HandleEventEndpoint(func(context.Context, *entity.ExptScheduleEvent) error { return nil })
	require.NoError(t, svc.Schedule(context.Background(), &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3}))
	require.Len(t, guard.candidates, 1)
	require.Equal(t, int64(4), guard.candidates[0].ItemID)
	require.Equal(t, 1, mode.submitted, "the last selected batch must keep the experiment alive")
}

func TestResourceItemAdmissionFailureDoesNotFailClaim(t *testing.T) {
	svc, item, guard, event, ctx := resourceFixture()
	guard.confirmationErr = errors.New("handshake primary unavailable")
	svc.configer = resourceConfig{}
	svc.metric = resourceMetric{}
	svc.endpoints = svc.HandleEventErr(svc.HandleCentralReservation(func(context.Context, *entity.ExptItemEvalEvent) error { return nil }))
	require.ErrorIs(t, svc.Eval(ctx, event), guard.confirmationErr)
	require.Equal(t, int32(entity.ItemRunState_Queueing), item.run.Status)
	require.Equal(t, entity.QuotaReservationStateReserved, item.run.QuotaReservationState)
	require.Empty(t, guard.finalized)
}

func TestResourceItemTerminalWriteFailureRetainsBudgetAndRetries(t *testing.T) {
	svc, item, guard, event, ctx := resourceFixture()
	item.run.Status = int32(entity.ItemRunState_Processing)
	item.writeErr = errors.New("primary write unavailable")
	svc.configer = resourceConfig{}
	svc.metric = resourceMetric{}
	svc.endpoints = svc.HandleEventErr(func(context.Context, *entity.ExptItemEvalEvent) error { return errors.New("fatal sandbox error") })
	require.ErrorIs(t, svc.Eval(ctx, event), item.writeErr)
	require.Equal(t, int32(entity.ItemRunState_Processing), item.run.Status)
	require.Empty(t, guard.finalized)
}

func TestResourceItemRetryKeepsOriginalBudgetAndRun(t *testing.T) {
	svc, item, guard, event, ctx := resourceFixture()
	item.run.Status = int32(entity.ItemRunState_Processing)
	item.run.QuotaReservationState = entity.QuotaReservationStateNone
	event.Ext = map[string]string{entity.RetryYieldExtKey: "true"}
	broker := &resourceBroker{err: errors.New("broker unavailable")}
	svc.publisher = broker
	svc.configer = resourceConfig{retry: 3}
	svc.metric = resourceMetric{}
	svc.endpoints = svc.HandleEventErr(func(context.Context, *entity.ExptItemEvalEvent) error { return errors.New("transient sandbox error") })
	require.ErrorIs(t, svc.Eval(ctx, event), broker.err)
	require.Equal(t, int32(entity.ItemRunState_Processing), item.run.Status)
	require.Empty(t, guard.finalized)
	require.NotNil(t, broker.event)
	require.Equal(t, event.ExptRunID, broker.event.ExptRunID)
	require.Equal(t, 1, broker.event.RetryTimes)
}

func TestResourceItemTerminalNotificationDoesNotRollbackBusinessState(t *testing.T) {
	svc, item, guard, event, _ := resourceFixture()
	svc.endpoints = func(context.Context, *entity.ExptItemEvalEvent) error {
		item.run.Status = int32(entity.ItemRunState_Fail)
		return nil
	}
	require.NoError(t, svc.Eval(context.Background(), event))
	require.Len(t, guard.finalized, 1)
	require.Equal(t, int32(entity.ItemRunState_Fail), guard.finalized[0].Status)
	require.Equal(t, int32(entity.ItemRunState_Fail), item.run.Status)
}

type resourceGrantContextKey struct{}
type resourceEvalSetItems struct{ EvaluationSetItemService }

func (resourceEvalSetItems) BatchGetEvaluationSetItems(context.Context, *entity.BatchGetEvaluationSetItemsParam) ([]*entity.EvaluationSetItem, error) {
	return []*entity.EvaluationSetItem{{ID: 4}}, nil
}

type resourceEvalTurnStore struct{ repo.IExptTurnResultRepo }

func (resourceEvalTurnStore) GetItemTurnRunLogs(context.Context, int64, int64, int64, int64) ([]*entity.ExptTurnResultRunLog, error) {
	return []*entity.ExptTurnResultRunLog{{ID: 50, TurnID: 5}}, nil
}
func resourceExecutionFixture() (*ExptItemEventEvalServiceImpl, *resourceItemStore, *resourceGuard, *entity.ExptItemEvalEvent, context.Context) {
	svc, items, guard, event, ctx := resourceFixture()
	event.ExptRunMode = entity.EvaluationModeSubmit
	items.run.Status = int32(entity.ItemRunState_Processing)
	svc.manager = resourceManager{expt: &entity.Experiment{ID: 1, SpaceID: 3, EvalSet: &entity.EvaluationSet{ID: 7, EvaluationSetVersion: &entity.EvaluationSetVersion{ID: 8, EvaluationSetID: 7}}}}
	svc.evaluationSetItemService = resourceEvalSetItems{}
	svc.exptTurnResultRepo = resourceEvalTurnStore{}
	svc.configer = resourceConfig{}
	svc.endpoints = svc.HandleEventExec(func(context.Context, *entity.ExptItemEvalEvent) error { return nil })
	return svc, items, guard, event, ctx
}
func TestResourceGrantedContextReachesOriginalItemEvaluation(t *testing.T) {
	svc, items, guard, event, ctx := resourceExecutionFixture()
	guard.prepareContext = func(ctx context.Context) context.Context {
		return context.WithValue(ctx, resourceGrantContextKey{}, "granted-plan")
	}
	require.NoError(t, svc.Eval(ctx, event))
	require.NotNil(t, items.writeContext)
	require.Equal(t, "granted-plan", items.writeContext.Value(resourceGrantContextKey{}), "original Item Eval must persist through the returned authorization context")
}

func TestResourceNilGrantedContextFailsClosedBeforeItemEvaluation(t *testing.T) {
	svc, items, guard, event, ctx := resourceExecutionFixture()
	guard.prepareContext = func(context.Context) context.Context { return nil }
	require.ErrorIs(t, svc.Eval(ctx, event), errCentralItemAdmission)
	require.Nil(t, items.writeContext, "nil authorization context must not enter the original Item Eval")
	require.Equal(t, int32(entity.ItemRunState_Processing), items.run.Status)
}
func TestResourceLegacyEvaluationKeepsOriginalContext(t *testing.T) {
	svc, items, _, event, _ := resourceExecutionFixture()
	svc.centralGuard = component.NewNoopCentralReservationGuard()
	ctx := context.WithValue(context.Background(), resourceGrantContextKey{}, "original-context")
	require.NoError(t, svc.Eval(ctx, event))
	require.NotNil(t, items.writeContext)
	require.Equal(t, "original-context", items.writeContext.Value(resourceGrantContextKey{}))
}

func TestResourceItemReservedWithoutReadyCannotStart(t *testing.T) {
	svc, item, _, event, ctx := resourceFixture()
	started := false
	endpoint := svc.HandleCentralReservation(func(context.Context, *entity.ExptItemEvalEvent) error { started = true; return nil })
	require.NoError(t, endpoint(ctx, event))
	require.False(t, started, "a dispatch claim is not a Task/Allocation authorization")
	require.Equal(t, int32(entity.ItemRunState_Queueing), item.run.Status)
}
