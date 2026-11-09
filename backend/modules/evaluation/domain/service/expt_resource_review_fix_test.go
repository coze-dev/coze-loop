// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/stretchr/testify/require"
)

type reviewArchiveItems struct {
	repo.IExptItemResultRepo
	log  entity.ExptItemResultRunLog
	main entity.ExptItemResult
	turn entity.ExptTurnResult
}

func (s *reviewArchiveItems) ScanCurrentRequests(context.Context, int64, int64, int64, bool) ([]*entity.ExptItemCurrentRequest, error) {
	return []*entity.ExptItemCurrentRequest{{SpaceID: s.log.SpaceID, ExptID: s.log.ExptID, RunID: s.log.ExptRunID, ItemID: s.log.ItemID,
		Mode: entity.EvaluationModeSubmit, Status: entity.ItemRunState(s.log.Status), ResultState: entity.ExptItemResultState(s.log.ResultState),
		QuotaReservationState: s.log.QuotaReservationState, UpdatedAt: s.log.UpdatedAt}}, nil
}

func (s *reviewArchiveItems) ScanManualRetryFilterItems(context.Context, int64, int64, int64, int64, int) ([]int64, int64, error) {
	return nil, 0, nil
}

func (s *reviewArchiveItems) ScanItemRunLogs(_ context.Context, _, _ int64, f *entity.ExptItemRunLogFilter, _, _, _ int64) ([]*entity.ExptItemResultRunLog, int64, error) {
	selected := f.RawFilter && (s.log.Status == int32(entity.ItemRunState_Processing) || s.log.ResultState == int32(entity.ExptItemResultStateLogged))
	for _, state := range f.Status {
		selected = selected || int32(state) == s.log.Status
	}
	if !selected {
		return nil, 0, nil
	}
	copy := s.log
	return []*entity.ExptItemResultRunLog{&copy}, 0, nil
}
func (s *reviewArchiveItems) GetItemRunLog(context.Context, int64, int64, int64, int64) (*entity.ExptItemResultRunLog, error) {
	copy := s.log
	return &copy, nil
}

func (s *reviewArchiveItems) GetItemResultPublishSource(context.Context, int64, int64, int64, int64) (*entity.ExptItemResultPublishSource, error) {
	log, main, turn := s.log, s.main, s.turn
	current := entity.ExptItemCurrentRequest{SpaceID: log.SpaceID, ExptID: log.ExptID, RunID: log.ExptRunID, ItemID: log.ItemID,
		Mode: entity.EvaluationModeSubmit, Status: entity.ItemRunState(log.Status), ResultState: entity.ExptItemResultState(log.ResultState)}
	source := &entity.ExptCurrentItemRunSource{Current: current, CanonicalTurns: []*entity.ExptTurnResult{&turn},
		Turns: map[int64]*entity.ExptTurnResultRunLog{turn.TurnID: {ID: 51, SpaceID: log.SpaceID, ExptID: log.ExptID,
			ExptRunID: log.ExptRunID, ItemID: log.ItemID, TurnID: turn.TurnID, Status: entity.TurnRunState_Fail}}}
	return &entity.ExptItemResultPublishSource{Source: source, Item: &main, ItemLog: &log}, nil
}

func (s *reviewArchiveItems) IsItemResultPublishSourceCurrent(_ context.Context, source *entity.ExptItemResultPublishSource) (bool, error) {
	return source.Source.Current.RunID == s.log.ExptRunID && source.ItemLog.ResultState == s.log.ResultState, nil
}

func (s *reviewArchiveItems) CommitItemResult(_ context.Context, publication *entity.ExptItemResultPublication) (bool, error) {
	s.main.Status = entity.ItemRunState(publication.Source.ItemLog.Status)
	s.turn = *publication.Turns[0]
	s.log.ResultState = int32(entity.ExptItemResultStateResulted)
	return true, nil
}
func (s *reviewArchiveItems) BatchGet(context.Context, int64, int64, []int64) ([]*entity.ExptItemResult, error) {
	copy := s.main
	return []*entity.ExptItemResult{&copy}, nil
}
func (s *reviewArchiveItems) GetItemTurnResults(context.Context, int64, int64, int64) ([]*entity.ExptTurnResult, error) {
	copy := s.turn
	return []*entity.ExptTurnResult{&copy}, nil
}
func (s *reviewArchiveItems) UpdateItemsResult(_ context.Context, _, _ int64, _ []int64, fields map[string]any) error {
	s.main.Status = entity.ItemRunState(fields["status"].(int32))
	return nil
}
func (s *reviewArchiveItems) UpdateItemRunLog(_ context.Context, _, _ int64, _ []int64, fields map[string]any, _ int64) error {
	s.log.ResultState = fields["result_state"].(int32)
	return nil
}

type reviewArchiveTurns struct {
	repo.IExptTurnResultRepo
	items *reviewArchiveItems
}

func (s reviewArchiveTurns) GetItemTurnRunLogs(context.Context, int64, int64, int64, int64) ([]*entity.ExptTurnResultRunLog, error) {
	return []*entity.ExptTurnResultRunLog{{ID: 50, ExptID: 1, ExptRunID: 2, ItemID: 4, TurnID: 5, Status: entity.TurnRunState_Fail}}, nil
}
func (s reviewArchiveTurns) SaveTurnResults(_ context.Context, rows []*entity.ExptTurnResult) error {
	s.items.turn = *rows[0]
	return nil
}

type reviewArchiveStats struct{ repo.IExptStatsRepo }

func (reviewArchiveStats) ArithOperateCount(context.Context, int64, int64, *entity.StatsCntArithOp) error {
	return nil
}

type reviewArchiveResults struct{ *ExptResultServiceImpl }

func (reviewArchiveResults) UpsertExptTurnResultFilter(context.Context, int64, int64, []int64) error {
	return nil
}

type reviewArchiveManager struct {
	resourceManager
	items *reviewArchiveItems
	ended bool
}

func (m *reviewArchiveManager) CompleteRun(context.Context, int64, int64, int64, *entity.Session, ...entity.CompleteExptOptionFn) error {
	return nil
}
func (m *reviewArchiveManager) CompleteExpt(context.Context, int64, *int64, int64, *entity.Session, ...entity.CompleteExptOptionFn) error {
	if m.items.main.Status != entity.ItemRunState_Fail {
		return errors.New("experiment ended before failure result was archived")
	}
	m.ended = true
	return nil
}

type reviewArchiveIdem struct {
	idem.IdempotentService
	startKey string
}

func (i reviewArchiveIdem) Exist(_ context.Context, key string) (bool, error) {
	return key == i.startKey, nil
}
func (reviewArchiveIdem) Set(context.Context, string, time.Duration) error { return nil }

type reviewArchiveConfig struct{ resourceConfig }

func (reviewArchiveConfig) GetExptExecConf(context.Context, int64) *entity.ExptExecConf {
	return &entity.ExptExecConf{ExptItemEvalConf: &entity.ExptItemEvalConf{MaxItemConcurNum: 10, ConcurNum: 1}}
}

type reviewArchivePublisher struct {
	events.ExptEventPublisher
	next *entity.ExptScheduleEvent
}

func (p *reviewArchivePublisher) PublishExptScheduleEvent(_ context.Context, event *entity.ExptScheduleEvent, _ *time.Duration) error {
	copy := *event
	p.next = &copy
	return nil
}
func (*reviewArchivePublisher) PublishExptTurnResultFilterEvent(context.Context, *entity.ExptTurnResultFilterEvent, *time.Duration) error {
	return nil
}

type reviewArchiveDispatcher struct {
	component.ICentralReservationGuard
	items *reviewArchiveItems
}

func (reviewArchiveDispatcher) ReplayPendingItems(context.Context, *entity.ExptScheduleEvent, *entity.Experiment) error {
	return nil
}
func (d reviewArchiveDispatcher) SubmitItems(_ context.Context, _ *entity.ExptScheduleEvent, _ *entity.Experiment, candidates []*entity.ExptEvalItem) (int, error) {
	if len(candidates) > 0 {
		d.items.log.Status = int32(entity.ItemRunState_Fail)
		d.items.log.ResultState = int32(entity.ExptItemResultStateLogged)
	}
	return 0, nil
}

func TestReviewLastRejectedCandidateArchivesBeforeExperimentEnd(t *testing.T) {
	ctx := context.Background()
	event := &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, ExptRunMode: entity.EvaluationModeSubmit}
	items := &reviewArchiveItems{log: entity.ExptItemResultRunLog{ID: 41, SpaceID: 3, ExptID: 1, ExptRunID: 2, ItemID: 4, Status: int32(entity.ItemRunState_Queueing)}, main: entity.ExptItemResult{ExptID: 1, ItemID: 4, Status: entity.ItemRunState_Queueing}, turn: entity.ExptTurnResult{ID: 50, ExptID: 1, ItemID: 4, TurnID: 5}}
	expt := &entity.Experiment{ID: 1, SpaceID: 3, Status: entity.ExptStatus_Processing, ExptDispatchMode: entity.ExptDispatchModeEnforce, EvalConf: &entity.EvaluationConfiguration{ItemConcurNum: gptr.Of(1)}, EvalSet: &entity.EvaluationSet{EvaluationSetVersion: &entity.EvaluationSetVersion{ID: 8}}}
	manager := &reviewArchiveManager{resourceManager: resourceManager{expt: expt}, items: items}
	config := reviewArchiveConfig{}
	pub := &reviewArchivePublisher{}
	turns := reviewArchiveTurns{items: items}
	result := reviewArchiveResults{ExptResultServiceImpl: &ExptResultServiceImpl{ExptItemResultRepo: items, ExptTurnResultRepo: turns, ExptStatsRepo: reviewArchiveStats{},
		resultPublishRepo: items, resultPublishLocker: func() lock.ILocker { return cancellationMutex{} }}}
	mode := &DefaultSchedulerModeFactory{manager: manager, exptItemResultRepo: items, idem: reviewArchiveIdem{startKey: makeStartIdemKey(event)}, configer: config, publisher: pub, resultSvc: result}
	svc := &ExptSchedulerImpl{Manager: manager, ExptItemResultRepo: items, ExptTurnResultRepo: turns, Configer: config, Publisher: pub, ResultSvc: result, schedulerModeFactory: mode, centralGuard: reviewArchiveDispatcher{items: items}}
	svc.Endpoints = svc.HandleEventEndpoint(func(context.Context, *entity.ExptScheduleEvent) error { return nil })
	require.NoError(t, svc.Schedule(ctx, event))
	require.False(t, manager.ended)
	require.NotNil(t, pub.next, "zero actual claims must still permit archiving the newly Logged failure")
	require.Equal(t, int32(entity.ExptItemResultStateLogged), items.log.ResultState)
	require.NoError(t, svc.Schedule(ctx, pub.next))
	require.True(t, manager.ended)
	results, err := items.BatchGet(ctx, 3, 1, []int64{4})
	require.NoError(t, err)
	require.Equal(t, entity.ItemRunState_Fail, results[0].Status)
	require.Equal(t, int32(entity.ExptItemResultStateResulted), items.log.ResultState)
}
