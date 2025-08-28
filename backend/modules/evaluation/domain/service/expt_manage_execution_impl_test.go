// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0			runMode: entity.EvaluationModeSubmit,
package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	benefitMocks "github.com/coze-dev/coze-loop/backend/infra/external/benefit/mocks"
	lockMocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	lwtMocks "github.com/coze-dev/coze-loop/backend/infra/platestwrite/mocks"
	idemMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem/mocks"
	metricsMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	componentMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventsMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	repoMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

// newTestExptManager is defined in expt_manage_impl_test.go

func TestExptMangerImpl_Run(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mgr := newTestExptManager(ctrl)
	ctx := context.Background()
	session := &entity.Session{UserID: "test_user"}

	tests := []struct {
		name     string
		exptID   int64
		runID    int64
		spaceID  int64
		runMode  entity.ExptRunMode
		ext      map[string]string
		setup    func()
		wantErr  bool
		errCheck func(error) bool
	}{
		{
			name:    "successful_run_with_normal_mode",
			exptID:  123,
			runID:   456,
			spaceID: 789,
			runMode: entity.EvaluationModeSubmit,
			ext:     map[string]string{"key": "value"},
			setup: func() {
				// Mock lwt.CheckWriteFlagByID
				mgr.lwt.(*lwtMocks.MockILatestWriteTracker).
					EXPECT().
					CheckWriteFlagByID(ctx, gomock.Any(), int64(123)).
					Return(false).AnyTimes()

				// Mock MGetByID for experiment retrieval
				mgr.exptRepo.(*repoMocks.MockIExperimentRepo).
					EXPECT().
					MGetByID(ctx, []int64{123}, int64(789)).
					Return([]*entity.Experiment{{ID: 123, SpaceID: 789}}, nil).AnyTimes()

				// Mock GetEvaluationSet
				mgr.evaluationSetService.(*svcMocks.MockIEvaluationSetService).
					EXPECT().
					GetEvaluationSet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&entity.EvaluationSet{}, nil).AnyTimes()

				// Mock MGetStats
				mgr.exptResultService.(*svcMocks.MockExptResultService).
					EXPECT().
					MGetStats(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return([]*entity.ExptStats{}, nil).AnyTimes()

				// Mock BatchGetExptAggrResultByExperimentIDs
				mgr.exptAggrResultService.(*svcMocks.MockExptAggrResultService).
					EXPECT().
					BatchGetExptAggrResultByExperimentIDs(gomock.Any(), gomock.Any(), gomock.Any()).
					Return([]*entity.ExptAggregateResult{}, nil).AnyTimes()

				mgr.quotaRepo.(*repoMocks.MockQuotaRepo).
					EXPECT().
					CreateOrUpdate(ctx, int64(789), gomock.Any(), session).
					Return(nil)
				mgr.configer.(*componentMocks.MockIConfiger).
					EXPECT().
					GetExptExecConf(ctx, int64(789)).AnyTimes().
					Return(&entity.ExptExecConf{
						SpaceExptConcurLimit: 10,
					})
				mgr.publisher.(*eventsMocks.MockExptEventPublisher).
					EXPECT().
					PublishExptScheduleEvent(ctx, gomock.Any(), gptr.Of(time.Second*3)).
					Return(nil)
			},
			wantErr: false,
		},
		{
			name:    "quota_check_failure",
			exptID:  123,
			runID:   456,
			spaceID: 789,
			runMode: entity.EvaluationModeSubmit,
			ext:     map[string]string{},
			setup: func() {
				// Mock lwt.CheckWriteFlagByID
				mgr.lwt.(*lwtMocks.MockILatestWriteTracker).
					EXPECT().
					CheckWriteFlagByID(ctx, gomock.Any(), int64(123)).
					Return(false).AnyTimes()

				// Mock MGetByID for experiment retrieval
				mgr.exptRepo.(*repoMocks.MockIExperimentRepo).
					EXPECT().
					MGetByID(ctx, []int64{123}, int64(789)).
					Return([]*entity.Experiment{{ID: 123, SpaceID: 789}}, nil).AnyTimes()

				// Mock GetEvaluationSet
				mgr.evaluationSetService.(*svcMocks.MockIEvaluationSetService).
					EXPECT().
					GetEvaluationSet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&entity.EvaluationSet{}, nil).AnyTimes()

				// Mock MGetStats
				mgr.exptResultService.(*svcMocks.MockExptResultService).
					EXPECT().
					MGetStats(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return([]*entity.ExptStats{}, nil).AnyTimes()

				// Mock BatchGetExptAggrResultByExperimentIDs
				mgr.exptAggrResultService.(*svcMocks.MockExptAggrResultService).
					EXPECT().
					BatchGetExptAggrResultByExperimentIDs(gomock.Any(), gomock.Any(), gomock.Any()).
					Return([]*entity.ExptAggregateResult{}, nil).AnyTimes()

				mgr.quotaRepo.(*repoMocks.MockQuotaRepo).
					EXPECT().
					CreateOrUpdate(ctx, int64(789), gomock.Any(), session).
					Return(errors.New("quota exceeded"))
			},
			wantErr: true,
		},
		{
			name:    "publish_event_failure",
			exptID:  123,
			runID:   456,
			spaceID: 789,
			runMode: entity.EvaluationModeFailRetry,
			ext:     map[string]string{},
			setup: func() {
				// Mock lwt.CheckWriteFlagByID
				mgr.lwt.(*lwtMocks.MockILatestWriteTracker).
					EXPECT().
					CheckWriteFlagByID(ctx, gomock.Any(), int64(123)).
					Return(false).AnyTimes()

				// Mock MGetByID for experiment retrieval
				mgr.exptRepo.(*repoMocks.MockIExperimentRepo).
					EXPECT().
					MGetByID(ctx, []int64{123}, int64(789)).
					Return([]*entity.Experiment{{ID: 123, SpaceID: 789}}, nil).AnyTimes()

				// Mock GetEvaluationSet
				mgr.evaluationSetService.(*svcMocks.MockIEvaluationSetService).
					EXPECT().
					GetEvaluationSet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&entity.EvaluationSet{}, nil).AnyTimes()

				// Mock MGetStats
				mgr.exptResultService.(*svcMocks.MockExptResultService).
					EXPECT().
					MGetStats(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return([]*entity.ExptStats{}, nil).AnyTimes()

				// Mock BatchGetExptAggrResultByExperimentIDs
				mgr.exptAggrResultService.(*svcMocks.MockExptAggrResultService).
					EXPECT().
					BatchGetExptAggrResultByExperimentIDs(gomock.Any(), gomock.Any(), gomock.Any()).
					Return([]*entity.ExptAggregateResult{}, nil).AnyTimes()

				mgr.quotaRepo.(*repoMocks.MockQuotaRepo).
					EXPECT().
					CreateOrUpdate(ctx, int64(789), gomock.Any(), session).
					Return(nil)
				mgr.configer.(*componentMocks.MockIConfiger).
					EXPECT().
					GetExptExecConf(ctx, int64(789)).AnyTimes().
					Return(&entity.ExptExecConf{
						SpaceExptConcurLimit: 10,
					})
				mgr.publisher.(*eventsMocks.MockExptEventPublisher).
					EXPECT().
					PublishExptScheduleEvent(ctx, gomock.Any(), gptr.Of(time.Second*3)).
					Return(errors.New("publish failed"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			err := mgr.Run(ctx, tt.exptID, tt.runID, tt.spaceID, session, tt.runMode, tt.ext)
			if (err != nil) != tt.wantErr {
				t.Errorf("Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.errCheck != nil && !tt.errCheck(err) {
				t.Errorf("Run() error check failed, error = %v", err)
			}
		})
	}
}

func TestExptMangerImpl_CompleteRun(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mgr := newTestExptManager(ctrl)
	ctx := context.Background()
	session := &entity.Session{UserID: "test_user"}

	tests := []struct {
		name     string
		exptID   int64
		runID    int64
		mode     entity.ExptRunMode
		spaceID  int64
		opts     []entity.CompleteExptOptionFn
		setup    func()
		wantErr  bool
		errCheck func(error) bool
	}{
		{
			name:    "successful_complete_run_without_cid",
			exptID:  123,
			runID:   456,
			mode:    entity.EvaluationModeSubmit,
			spaceID: 789,
			opts:    []entity.CompleteExptOptionFn{},
			setup: func() {
				runLog := &entity.ExptRunLog{
					ID:        456,
					ExptID:    123,
					ExptRunID: 456,
					Status:    int64(entity.ExptStatus_Processing),
				}
				mgr.runLogRepo.(*repoMocks.MockIExptRunLogRepo).
					EXPECT().
					Get(ctx, int64(123), int64(456)).
					Return(runLog, nil)

				// Mock calculateRunLogStats dependencies
				mgr.turnResultRepo.(*repoMocks.MockIExptTurnResultRepo).
					EXPECT().
					ListTurnResult(ctx, int64(789), int64(123), nil, gomock.Any(), false).
					Return([]*entity.ExptTurnResult{
						{Status: int32(entity.TurnRunState_Success)},
						{Status: int32(entity.TurnRunState_Success)},
					}, int64(2), nil)

				mgr.mutex.(*lockMocks.MockILocker).
					EXPECT().
					Unlock(gomock.Any()).
					Return(true, nil)

				mgr.runLogRepo.(*repoMocks.MockIExptRunLogRepo).
					EXPECT().
					Save(ctx, gomock.Any()).
					Return(nil)
			},
			wantErr: false,
		},
		{
			name:    "complete_run_with_cid_and_status",
			exptID:  123,
			runID:   456,
			mode:    entity.EvaluationModeSubmit,
			spaceID: 789,
			opts: []entity.CompleteExptOptionFn{
				entity.WithCID("test_cid"),
				entity.WithStatus(entity.ExptStatus_Success),
				entity.WithStatusMessage("completed successfully"),
			},
			setup: func() {
				mgr.idem.(*idemMocks.MockIdempotentService).
					EXPECT().
					Exist(ctx, "CompleteRun:test_cid").
					Return(false, nil)

				runLog := &entity.ExptRunLog{
					ID:        456,
					ExptID:    123,
					ExptRunID: 456,
					Status:    int64(entity.ExptStatus_Processing),
				}
				mgr.runLogRepo.(*repoMocks.MockIExptRunLogRepo).
					EXPECT().
					Get(ctx, int64(123), int64(456)).
					Return(runLog, nil)

				mgr.turnResultRepo.(*repoMocks.MockIExptTurnResultRepo).
					EXPECT().
					ListTurnResult(ctx, int64(789), int64(123), nil, gomock.Any(), false).
					Return([]*entity.ExptTurnResult{}, int64(0), nil)

				mgr.mutex.(*lockMocks.MockILocker).
					EXPECT().
					Unlock(gomock.Any()).
					Return(true, nil)

				mgr.runLogRepo.(*repoMocks.MockIExptRunLogRepo).
					EXPECT().
					Save(ctx, gomock.Any()).
					Return(nil)

				mgr.idem.(*idemMocks.MockIdempotentService).
					EXPECT().
					Set(ctx, "CompleteRun:test_cid", time.Second*60*3).
					Return(nil)
			},
			wantErr: false,
		},
		{
			name:    "duplicate_request_with_cid",
			exptID:  123,
			runID:   456,
			mode:    entity.EvaluationModeSubmit,
			spaceID: 789,
			opts: []entity.CompleteExptOptionFn{
				entity.WithCID("duplicate_cid"),
			},
			setup: func() {
				mgr.idem.(*idemMocks.MockIdempotentService).
					EXPECT().
					Exist(ctx, "CompleteRun:duplicate_cid").
					Return(true, nil)
			},
			wantErr: false,
		},
		{
			name:    "get_run_log_failure",
			exptID:  123,
			runID:   456,
			mode:    entity.EvaluationModeSubmit,
			spaceID: 789,
			opts:    []entity.CompleteExptOptionFn{},
			setup: func() {
				mgr.runLogRepo.(*repoMocks.MockIExptRunLogRepo).
					EXPECT().
					Get(ctx, int64(123), int64(456)).
					Return(nil, errors.New("run log not found"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			err := mgr.CompleteRun(ctx, tt.exptID, tt.runID, tt.mode, tt.spaceID, session, tt.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("CompleteRun() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExptMangerImpl_Kill(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mgr := newTestExptManager(ctrl)
	ctx := context.Background()
	session := &entity.Session{UserID: "test_user"}

	tests := []struct {
		name    string
		exptID  int64
		spaceID int64
		msg     string
		setup   func()
		wantErr bool
	}{
		{
			name:    "successful_kill",
			exptID:  123,
			spaceID: 789,
			msg:     "user terminated",
			setup: func() {
				// Mock CompleteExpt dependencies
				mgr.idem.(*idemMocks.MockIdempotentService).
					EXPECT().
					Exist(ctx, gomock.Any()).AnyTimes().
					Return(false, nil)

				mgr.exptRepo.(*repoMocks.MockIExperimentRepo).
					EXPECT().
					GetByID(ctx, int64(123), int64(789)).
					Return(&entity.Experiment{
						ID:       123,
						SpaceID:  789,
						ExptType: entity.ExptType_Offline,
						StartAt:  gptr.Of(time.Now()),
					}, nil)

				mgr.publisher.(*eventsMocks.MockExptEventPublisher).
					EXPECT().
					PublishExptAggrCalculateEvent(ctx, gomock.Any(), gptr.Of(time.Second*3)).
					Return(nil)

				mgr.exptResultService.(*svcMocks.MockExptResultService).
					EXPECT().
					CalculateStats(ctx, int64(123), int64(789), session).
					Return(&entity.ExptCalculateStats{
						SuccessItemCnt:    10,
						FailItemCnt:       0,
						PendingItemCnt:    0,
						ProcessingItemCnt: 0,
						TerminatedItemCnt: 0,
						IncompleteTurnIDs: []*entity.ItemTurnID{},
					}, nil)

				mgr.statsRepo.(*repoMocks.MockIExptStatsRepo).
					EXPECT().
					UpdateByExptID(ctx, int64(123), int64(789), gomock.Any()).
					Return(nil)

				mgr.exptRepo.(*repoMocks.MockIExperimentRepo).
					EXPECT().
					Update(ctx, gomock.Any()).
					Return(nil)

				mgr.quotaRepo.(*repoMocks.MockQuotaRepo).
					EXPECT().
					CreateOrUpdate(ctx, int64(789), gomock.Any(), session).
					Return(nil)

				mgr.configer.(*componentMocks.MockIConfiger).
					EXPECT().
					GetExptExecConf(ctx, int64(789)).AnyTimes().AnyTimes().
					Return(&entity.ExptExecConf{
						SpaceExptConcurLimit: 10,
					})

				mgr.idem.(*idemMocks.MockIdempotentService).
					EXPECT().
					Set(ctx, gomock.Any(), time.Second*60*3).AnyTimes().
					Return(nil)

				mgr.mtr.(*metricsMocks.MockExptMetric).
					EXPECT().
					EmitExptExecResult(int64(789), int64(entity.ExptType_Offline), int64(entity.ExptStatus_Terminated), gomock.Any())
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			err := mgr.Kill(ctx, tt.exptID, tt.spaceID, tt.msg, session)
			if (err != nil) != tt.wantErr {
				t.Errorf("Kill() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExptMangerImpl_RetryUnSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mgr := newTestExptManager(ctrl)
	ctx := context.Background()
	session := &entity.Session{UserID: "test_user"}

	tests := []struct {
		name    string
		exptID  int64
		runID   int64
		spaceID int64
		ext     map[string]string
		setup   func()
		wantErr bool
	}{
		{
			name:    "successful_retry",
			exptID:  123,
			runID:   456,
			spaceID: 789,
			ext:     map[string]string{"retry": "true"},
			setup: func() {
				// Mock lwt.CheckWriteFlagByID
				mgr.lwt.(*lwtMocks.MockILatestWriteTracker).
					EXPECT().
					CheckWriteFlagByID(ctx, gomock.Any(), int64(123)).
					Return(false).AnyTimes()

				// Mock MGetByID for experiment retrieval
				mgr.exptRepo.(*repoMocks.MockIExperimentRepo).
					EXPECT().
					MGetByID(ctx, []int64{123}, int64(789)).
					Return([]*entity.Experiment{{ID: 123, SpaceID: 789}}, nil).AnyTimes()

				// Mock GetEvaluationSet
				mgr.evaluationSetService.(*svcMocks.MockIEvaluationSetService).
					EXPECT().
					GetEvaluationSet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&entity.EvaluationSet{}, nil).AnyTimes()

				// Mock MGetStats
				mgr.exptResultService.(*svcMocks.MockExptResultService).
					EXPECT().
					MGetStats(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return([]*entity.ExptStats{}, nil).AnyTimes()

				// Mock BatchGetExptAggrResultByExperimentIDs
				mgr.exptAggrResultService.(*svcMocks.MockExptAggrResultService).
					EXPECT().
					BatchGetExptAggrResultByExperimentIDs(gomock.Any(), gomock.Any(), gomock.Any()).
					Return([]*entity.ExptAggregateResult{}, nil).AnyTimes()

				mgr.quotaRepo.(*repoMocks.MockQuotaRepo).
					EXPECT().
					CreateOrUpdate(ctx, int64(789), gomock.Any(), session).
					Return(nil)
				mgr.configer.(*componentMocks.MockIConfiger).
					EXPECT().
					GetExptExecConf(ctx, int64(789)).AnyTimes().
					Return(&entity.ExptExecConf{
						SpaceExptConcurLimit: 10,
					})
				mgr.publisher.(*eventsMocks.MockExptEventPublisher).
					EXPECT().
					PublishExptScheduleEvent(ctx, gomock.Any(), gptr.Of(time.Second*3)).
					Do(func(ctx context.Context, event *entity.ExptScheduleEvent, timeout *time.Duration) {
						assert.Equal(t, entity.EvaluationModeFailRetry, event.ExptRunMode)
					}).
					Return(nil)
			},
			wantErr: false,
		},
		{
			name:    "quota_check_failure_on_retry",
			exptID:  123,
			runID:   456,
			spaceID: 789,
			ext:     map[string]string{},
			setup: func() {
				// Mock lwt.CheckWriteFlagByID
				mgr.lwt.(*lwtMocks.MockILatestWriteTracker).
					EXPECT().
					CheckWriteFlagByID(ctx, gomock.Any(), int64(123)).
					Return(false).AnyTimes()

				// Mock MGetByID for experiment retrieval
				mgr.exptRepo.(*repoMocks.MockIExperimentRepo).
					EXPECT().
					MGetByID(ctx, []int64{123}, int64(789)).
					Return([]*entity.Experiment{{ID: 123, SpaceID: 789}}, nil).AnyTimes()

				// Mock GetEvaluationSet
				mgr.evaluationSetService.(*svcMocks.MockIEvaluationSetService).
					EXPECT().
					GetEvaluationSet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&entity.EvaluationSet{}, nil).AnyTimes()

				// Mock MGetStats
				mgr.exptResultService.(*svcMocks.MockExptResultService).
					EXPECT().
					MGetStats(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return([]*entity.ExptStats{}, nil).AnyTimes()

				// Mock BatchGetExptAggrResultByExperimentIDs
				mgr.exptAggrResultService.(*svcMocks.MockExptAggrResultService).
					EXPECT().
					BatchGetExptAggrResultByExperimentIDs(gomock.Any(), gomock.Any(), gomock.Any()).
					Return([]*entity.ExptAggregateResult{}, nil).AnyTimes()

				mgr.quotaRepo.(*repoMocks.MockQuotaRepo).
					EXPECT().
					CreateOrUpdate(ctx, int64(789), gomock.Any(), session).
					Return(errors.New("quota exceeded"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			err := mgr.RetryUnSuccess(ctx, tt.exptID, tt.runID, tt.spaceID, session, tt.ext)
			if (err != nil) != tt.wantErr {
				t.Errorf("RetryUnSuccess() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExptMangerImpl_LogRun(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mgr := newTestExptManager(ctrl)
	ctx := context.Background()
	session := &entity.Session{UserID: "test_user"}

	tests := []struct {
		name      string
		exptID    int64
		exptRunID int64
		mode      entity.ExptRunMode
		spaceID   int64
		setup     func()
		wantErr   bool
	}{
		{
			name:      "successful_log_run",
			exptID:    123,
			exptRunID: 456,
			mode:      entity.EvaluationModeSubmit,
			spaceID:   789,
			setup: func() {
				mgr.configer.(*componentMocks.MockIConfiger).
					EXPECT().
					GetExptExecConf(ctx, int64(789)).AnyTimes().AnyTimes().
					Return(&entity.ExptExecConf{
						ZombieIntervalSecond: 300,
					})

				mgr.mutex.(*lockMocks.MockILocker).
					EXPECT().
					LockBackoff(ctx, gomock.Any(), time.Duration(300)*time.Second, time.Second).
					Return(true, nil)

				mgr.mtr.(*metricsMocks.MockExptMetric).
					EXPECT().
					EmitExptExecRun(int64(789), int64(entity.EvaluationModeSubmit))

				mgr.runLogRepo.(*repoMocks.MockIExptRunLogRepo).
					EXPECT().
					Create(ctx, gomock.Any()).
					Do(func(ctx context.Context, runLog *entity.ExptRunLog) {
						assert.Equal(t, int64(456), runLog.ID)
						assert.Equal(t, int64(123), runLog.ExptID)
						assert.Equal(t, int64(456), runLog.ExptRunID)
						assert.Equal(t, int32(entity.EvaluationModeSubmit), runLog.Mode)
						assert.Equal(t, int64(entity.ExptStatus_Pending), runLog.Status)
						assert.Equal(t, "test_user", runLog.CreatedBy)
					}).
					Return(nil)
			},
			wantErr: false,
		},
		{
			name:      "lock_acquisition_failure",
			exptID:    123,
			exptRunID: 456,
			mode:      entity.EvaluationModeSubmit,
			spaceID:   789,
			setup: func() {
				mgr.configer.(*componentMocks.MockIConfiger).
					EXPECT().
					GetExptExecConf(ctx, int64(789)).AnyTimes().AnyTimes().
					Return(&entity.ExptExecConf{
						ZombieIntervalSecond: 300,
					})

				mgr.mutex.(*lockMocks.MockILocker).
					EXPECT().
					LockBackoff(ctx, gomock.Any(), time.Duration(300)*time.Second, time.Second).
					Return(false, nil)
			},
			wantErr: true,
		},
		{
			name:      "create_run_log_failure",
			exptID:    123,
			exptRunID: 456,
			mode:      entity.EvaluationModeSubmit,
			spaceID:   789,
			setup: func() {
				mgr.configer.(*componentMocks.MockIConfiger).
					EXPECT().
					GetExptExecConf(ctx, int64(789)).AnyTimes().AnyTimes().
					Return(&entity.ExptExecConf{
						ZombieIntervalSecond: 300,
					})

				mgr.mutex.(*lockMocks.MockILocker).
					EXPECT().
					LockBackoff(ctx, gomock.Any(), time.Duration(300)*time.Second, time.Second).
					Return(true, nil)

				mgr.mtr.(*metricsMocks.MockExptMetric).
					EXPECT().
					EmitExptExecRun(int64(789), int64(entity.EvaluationModeSubmit))

				mgr.runLogRepo.(*repoMocks.MockIExptRunLogRepo).
					EXPECT().
					Create(ctx, gomock.Any()).
					Return(errors.New("create failed"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			err := mgr.LogRun(ctx, tt.exptID, tt.exptRunID, tt.mode, tt.spaceID, session)
			if (err != nil) != tt.wantErr {
				t.Errorf("LogRun() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExptMangerImpl_GetRunLog(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mgr := newTestExptManager(ctrl)
	ctx := context.Background()
	session := &entity.Session{UserID: "test_user"}

	tests := []struct {
		name      string
		exptID    int64
		exptRunID int64
		spaceID   int64
		setup     func()
		wantErr   bool
		expected  *entity.ExptRunLog
	}{
		{
			name:      "successful_get_run_log",
			exptID:    123,
			exptRunID: 456,
			spaceID:   789,
			setup: func() {
				expectedLog := &entity.ExptRunLog{
					ID:        456,
					ExptID:    123,
					ExptRunID: 456,
					Status:    int64(entity.ExptStatus_Success),
				}
				mgr.runLogRepo.(*repoMocks.MockIExptRunLogRepo).
					EXPECT().
					Get(ctx, int64(123), int64(456)).
					Return(expectedLog, nil)
			},
			wantErr: false,
			expected: &entity.ExptRunLog{
				ID:        456,
				ExptID:    123,
				ExptRunID: 456,
				Status:    int64(entity.ExptStatus_Success),
			},
		},
		{
			name:      "get_run_log_failure",
			exptID:    123,
			exptRunID: 456,
			spaceID:   789,
			setup: func() {
				mgr.runLogRepo.(*repoMocks.MockIExptRunLogRepo).
					EXPECT().
					Get(ctx, int64(123), int64(456)).
					Return(nil, errors.New("not found"))
			},
			wantErr:  true,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			result, err := mgr.GetRunLog(ctx, tt.exptID, tt.exptRunID, tt.spaceID, session)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetRunLog() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && tt.expected != nil {
				assert.Equal(t, tt.expected.ID, result.ID)
				assert.Equal(t, tt.expected.ExptID, result.ExptID)
				assert.Equal(t, tt.expected.ExptRunID, result.ExptRunID)
				assert.Equal(t, tt.expected.Status, result.Status)
			}
		})
	}
}

func TestExptMangerImpl_CheckBenefit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mgr := newTestExptManager(ctrl)
	ctx := context.Background()
	session := &entity.Session{UserID: "test_user"}

	tests := []struct {
		name    string
		expt    *entity.Experiment
		setup   func()
		wantErr bool
	}{
		{
			name: "already_free_credit_cost",
			expt: &entity.Experiment{
				ID:         123,
				SpaceID:    789,
				CreditCost: entity.CreditCostFree,
			},
			setup:   func() {},
			wantErr: false,
		},
		{
			name: "successful_benefit_check_with_free_result",
			expt: &entity.Experiment{
				ID:         123,
				SpaceID:    789,
				CreditCost: entity.CreditCostDefault,
			},
			setup: func() {
				mgr.benefitService.(*benefitMocks.MockIBenefitService).
					EXPECT().
					CheckAndDeductEvalBenefit(ctx, gomock.Any()).
					Do(func(ctx context.Context, req *benefit.CheckAndDeductEvalBenefitParams) {
						assert.Equal(t, "test_user", req.ConnectorUID)
						assert.Equal(t, int64(789), req.SpaceID)
						assert.Equal(t, int64(123), req.ExperimentID)
					}).
					Return(&benefit.CheckAndDeductEvalBenefitResult{
						IsFreeEvaluate: gptr.Of(true),
					}, nil)

				mgr.exptRepo.(*repoMocks.MockIExperimentRepo).
					EXPECT().
					Update(ctx, gomock.Any()).
					Do(func(ctx context.Context, expt *entity.Experiment) {
						assert.Equal(t, int64(123), expt.ID)
						assert.Equal(t, int64(789), expt.SpaceID)
						assert.Equal(t, entity.CreditCostFree, expt.CreditCost)
					}).
					Return(nil)
			},
			wantErr: false,
		},
		{
			name: "benefit_service_error",
			expt: &entity.Experiment{
				ID:         123,
				SpaceID:    789,
				CreditCost: entity.CreditCostDefault,
			},
			setup: func() {
				mgr.benefitService.(*benefitMocks.MockIBenefitService).
					EXPECT().
					CheckAndDeductEvalBenefit(ctx, gomock.Any()).
					Return(nil, errors.New("benefit service error"))
			},
			wantErr: true,
		},
		{
			name: "benefit_denied",
			expt: &entity.Experiment{
				ID:         123,
				SpaceID:    789,
				CreditCost: entity.CreditCostDefault,
			},
			setup: func() {
				mgr.benefitService.(*benefitMocks.MockIBenefitService).
					EXPECT().
					CheckAndDeductEvalBenefit(ctx, gomock.Any()).
					Return(&benefit.CheckAndDeductEvalBenefitResult{
						DenyReason: gptr.Of(benefit.DenyReasonInsufficient),
					}, nil)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			err := mgr.CheckBenefit(ctx, tt.expt, session)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckBenefit() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExptMangerImpl_calculateRunLogStats(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mgr := newTestExptManager(ctrl)
	ctx := context.Background()
	session := &entity.Session{UserID: "test_user"}

	tests := []struct {
		name     string
		exptID   int64
		runID    int64
		spaceID  int64
		runLog   *entity.ExptRunLog
		setup    func()
		wantErr  bool
		validate func(*entity.ExptRunLog)
	}{
		{
			name:    "successful_stats_calculation_all_success",
			exptID:  123,
			runID:   456,
			spaceID: 789,
			runLog: &entity.ExptRunLog{
				ID:        456,
				ExptID:    123,
				ExptRunID: 456,
			},
			setup: func() {
				mgr.turnResultRepo.(*repoMocks.MockIExptTurnResultRepo).
					EXPECT().
					ListTurnResult(ctx, int64(789), int64(123), nil, gomock.Any(), false).
					Return([]*entity.ExptTurnResult{
						{Status: int32(entity.TurnRunState_Success)},
						{Status: int32(entity.TurnRunState_Success)},
						{Status: int32(entity.TurnRunState_Success)},
					}, int64(3), nil)
			},
			wantErr: false,
			validate: func(runLog *entity.ExptRunLog) {
				assert.Equal(t, int32(3), runLog.SuccessCnt)
				assert.Equal(t, int32(0), runLog.FailCnt)
				assert.Equal(t, int32(0), runLog.PendingCnt)
				assert.Equal(t, int32(0), runLog.ProcessingCnt)
				assert.Equal(t, int32(0), runLog.TerminatedCnt)
				assert.Equal(t, int64(entity.ExptStatus_Success), runLog.Status)
			},
		},
		{
			name:    "mixed_status_results",
			exptID:  123,
			runID:   456,
			spaceID: 789,
			runLog: &entity.ExptRunLog{
				ID:        456,
				ExptID:    123,
				ExptRunID: 456,
			},
			setup: func() {
				mgr.turnResultRepo.(*repoMocks.MockIExptTurnResultRepo).
					EXPECT().
					ListTurnResult(ctx, int64(789), int64(123), nil, gomock.Any(), false).
					Return([]*entity.ExptTurnResult{
						{Status: int32(entity.TurnRunState_Success)},
						{Status: int32(entity.TurnRunState_Fail)},
						{Status: int32(entity.TurnRunState_Queueing)},
						{Status: int32(entity.TurnRunState_Processing)},
						{Status: int32(entity.TurnRunState_Terminal)},
					}, int64(5), nil)
			},
			wantErr: false,
			validate: func(runLog *entity.ExptRunLog) {
				assert.Equal(t, int32(1), runLog.SuccessCnt)
				assert.Equal(t, int32(1), runLog.FailCnt)
				assert.Equal(t, int32(1), runLog.PendingCnt)
				assert.Equal(t, int32(1), runLog.ProcessingCnt)
				assert.Equal(t, int32(1), runLog.TerminatedCnt)
				assert.Equal(t, int64(entity.ExptStatus_Failed), runLog.Status)
			},
		},
		{
			name:    "list_turn_result_error",
			exptID:  123,
			runID:   456,
			spaceID: 789,
			runLog: &entity.ExptRunLog{
				ID:        456,
				ExptID:    123,
				ExptRunID: 456,
			},
			setup: func() {
				mgr.turnResultRepo.(*repoMocks.MockIExptTurnResultRepo).
					EXPECT().
					ListTurnResult(ctx, int64(789), int64(123), nil, gomock.Any(), false).
					Return(nil, int64(0), errors.New("database error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			err := mgr.calculateRunLogStats(ctx, tt.exptID, tt.runID, tt.runLog, tt.spaceID, session)
			if (err != nil) != tt.wantErr {
				t.Errorf("calculateRunLogStats() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && tt.validate != nil {
				tt.validate(tt.runLog)
			}
		})
	}
}

func TestExptMangerImpl_CheckEvaluators(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mgr := newTestExptManager(ctrl)
	ctx := context.Background()
	session := &entity.Session{UserID: "test_user"}

	tests := []struct {
		name    string
		expt    *entity.Experiment
		setup   func()
		wantErr bool
	}{
		{
			name: "valid_evaluators",
			expt: &entity.Experiment{
				ID:      123,
				SpaceID: 789,
				EvaluatorVersionRef: []*entity.ExptEvaluatorVersionRef{
					{EvaluatorID: 1, EvaluatorVersionID: 1},
					{EvaluatorID: 2, EvaluatorVersionID: 2},
				},
				Evaluators: []*entity.Evaluator{
					{ID: 1},
					{ID: 2},
				},
				EvalConf: &entity.EvaluationConfiguration{
					ConnectorConf: entity.Connector{
						TargetConf: &entity.TargetConf{},
						EvaluatorsConf: &entity.EvaluatorsConf{
							EvaluatorConf: []*entity.EvaluatorConf{
								{EvaluatorVersionID: 1},
								{EvaluatorVersionID: 2},
							},
						},
					},
				},
			},
			setup:   func() {},
			wantErr: false,
		},
		{
			name: "empty_evaluator_version_ref",
			expt: &entity.Experiment{
				ID:                  123,
				SpaceID:             789,
				EvaluatorVersionRef: []*entity.ExptEvaluatorVersionRef{},
				Evaluators:          []*entity.Evaluator{},
				EvalConf: &entity.EvaluationConfiguration{
					ConnectorConf: entity.Connector{
						TargetConf: &entity.TargetConf{},
						EvaluatorsConf: &entity.EvaluatorsConf{
							EvaluatorConf: []*entity.EvaluatorConf{
								{EvaluatorVersionID: 1},
								{EvaluatorVersionID: 2},
							},
						},
					},
				},
			},
			setup:   func() {},
			wantErr: true,
		},
		{
			name: "mismatched_evaluators_length",
			expt: &entity.Experiment{
				ID:      123,
				SpaceID: 789,
				EvaluatorVersionRef: []*entity.ExptEvaluatorVersionRef{
					{EvaluatorID: 1, EvaluatorVersionID: 1},
					{EvaluatorID: 2, EvaluatorVersionID: 2},
				},
				Evaluators: []*entity.Evaluator{
					{ID: 1},
				},
				EvalConf: &entity.EvaluationConfiguration{
					ConnectorConf: entity.Connector{
						TargetConf: &entity.TargetConf{},
						EvaluatorsConf: &entity.EvaluatorsConf{
							EvaluatorConf: []*entity.EvaluatorConf{
								{EvaluatorVersionID: 1},
								{EvaluatorVersionID: 2},
							},
						},
					},
				},
			},
			setup:   func() {},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			err := mgr.CheckEvaluators(ctx, tt.expt, session)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckEvaluators() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExptMangerImpl_CheckTarget(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mgr := newTestExptManager(ctrl)
	ctx := context.Background()
	session := &entity.Session{UserID: "test_user"}

	tests := []struct {
		name    string
		expt    *entity.Experiment
		setup   func()
		wantErr bool
	}{
		{
			name: "no_target_conf",
			expt: &entity.Experiment{
				ID:      123,
				SpaceID: 789,
				EvalConf: &entity.EvaluationConfiguration{
					ConnectorConf: entity.Connector{},
				},
			},
			setup:   func() {},
			wantErr: false,
		},
		{
			name: "no_target_conf_nil",
			expt: &entity.Experiment{
				ID:      123,
				SpaceID: 789,
				EvalConf: &entity.EvaluationConfiguration{
					ConnectorConf: entity.Connector{
						TargetConf: nil,
					},
				},
			},
			setup:   func() {},
			wantErr: false,
		},
		{
			name: "valid_target",
			expt: &entity.Experiment{
				ID:              123,
				SpaceID:         789,
				TargetID:        456,
				TargetVersionID: 789,
				Target: &entity.EvalTarget{
					ID: 456,
				},
				EvalConf: &entity.EvaluationConfiguration{
					ConnectorConf: entity.Connector{
						TargetConf: &entity.TargetConf{},
					},
				},
			},
			setup:   func() {},
			wantErr: false,
		},
		{
			name: "invalid_target_id",
			expt: &entity.Experiment{
				ID:              123,
				SpaceID:         789,
				TargetID:        0,
				TargetVersionID: 789,
				Target: &entity.EvalTarget{
					ID: 456,
				},
				EvalConf: &entity.EvaluationConfiguration{
					ConnectorConf: entity.Connector{
						TargetConf: &entity.TargetConf{},
					},
				},
			},
			setup:   func() {},
			wantErr: true,
		},
		{
			name: "invalid_target_version_id",
			expt: &entity.Experiment{
				ID:              123,
				SpaceID:         789,
				TargetID:        456,
				TargetVersionID: 0,
				Target: &entity.EvalTarget{
					ID: 456,
				},
				EvalConf: &entity.EvaluationConfiguration{
					ConnectorConf: entity.Connector{
						TargetConf: &entity.TargetConf{},
					},
				},
			},
			setup:   func() {},
			wantErr: true,
		},
		{
			name: "nil_target",
			expt: &entity.Experiment{
				ID:              123,
				SpaceID:         789,
				TargetID:        456,
				TargetVersionID: 789,
				Target:          nil,
				EvalConf: &entity.EvaluationConfiguration{
					ConnectorConf: entity.Connector{
						TargetConf: &entity.TargetConf{},
					},
				},
			},
			setup:   func() {},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			err := mgr.CheckTarget(ctx, tt.expt, session)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckTarget() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExptMangerImpl_CheckConnector(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mgr := newTestExptManager(ctrl)
	ctx := context.Background()
	session := &entity.Session{UserID: "test_user"}

	tests := []struct {
		name    string
		expt    *entity.Experiment
		setup   func()
		wantErr bool
	}{
		{
			name: "nil_eval_conf",
			expt: &entity.Experiment{
				ID:       123,
				SpaceID:  789,
				EvalConf: nil,
			},
			setup:   func() {},
			wantErr: true,
		},
		{
			name: "loop_trace_target_no_validation",
			expt: &entity.Experiment{
				ID:      123,
				SpaceID: 789,
				EvalConf: &entity.EvaluationConfiguration{
					ConnectorConf: entity.Connector{
						EvaluatorsConf: &entity.EvaluatorsConf{
							EvaluatorConf: []*entity.EvaluatorConf{
								{
									EvaluatorVersionID: 1,
									IngressConf: &entity.EvaluatorIngressConf{
										EvalSetAdapter: &entity.FieldAdapter{
											FieldConfs: []*entity.FieldConf{
												{FromField: "field1"},
											},
										},
									},
								},
							},
						},
					},
				},
				Target: &entity.EvalTarget{
					EvalTargetType: entity.EvalTargetTypeLoopTrace,
				},
				EvalSet: &entity.EvaluationSet{
					EvaluationSetVersion: &entity.EvaluationSetVersion{
						EvaluationSetSchema: &entity.EvaluationSetSchema{
							FieldSchemas: []*entity.FieldSchema{
								{Name: "field1"},
							},
						},
					},
				},
			},
			setup:   func() {},
			wantErr: false,
		},
		{
			name: "valid_target_connector",
			expt: &entity.Experiment{
				ID:      123,
				SpaceID: 789,
				EvalConf: &entity.EvaluationConfiguration{
					ConnectorConf: entity.Connector{
						TargetConf: &entity.TargetConf{
							TargetVersionID: 1,
							IngressConf: &entity.TargetIngressConf{
								EvalSetAdapter: &entity.FieldAdapter{
									FieldConfs: []*entity.FieldConf{
										{FromField: "field1"},
									},
								},
							},
						},
						EvaluatorsConf: &entity.EvaluatorsConf{
							EvaluatorConf: []*entity.EvaluatorConf{
								{
									EvaluatorVersionID: 1,
									IngressConf: &entity.EvaluatorIngressConf{
										EvalSetAdapter: &entity.FieldAdapter{
											FieldConfs: []*entity.FieldConf{
												{FromField: "field1"},
											},
										},
										TargetAdapter: &entity.FieldAdapter{
											FieldConfs: []*entity.FieldConf{},
										},
									},
								},
							},
						},
					},
				},
				Target: &entity.EvalTarget{
					EvalTargetType: entity.EvalTargetTypeLoopPrompt,
					EvalTargetVersion: &entity.EvalTargetVersion{
						OutputSchema: []*entity.ArgsSchema{},
					},
				},
				EvalSet: &entity.EvaluationSet{
					EvaluationSetVersion: &entity.EvaluationSetVersion{
						EvaluationSetSchema: &entity.EvaluationSetSchema{
							FieldSchemas: []*entity.FieldSchema{
								{Name: "field1"},
							},
						},
					},
				},
			},
			setup:   func() {},
			wantErr: false,
		},
		{
			name: "invalid_target_connector_missing_field",
			expt: &entity.Experiment{
				ID:      123,
				SpaceID: 789,
				EvalConf: &entity.EvaluationConfiguration{
					ConnectorConf: entity.Connector{
						TargetConf: &entity.TargetConf{
							IngressConf: &entity.TargetIngressConf{
								EvalSetAdapter: &entity.FieldAdapter{
									FieldConfs: []*entity.FieldConf{
										{FromField: "missing_field"},
									},
								},
							},
						},
					},
				},
				Target: &entity.EvalTarget{
					EvalTargetType: entity.EvalTargetTypeLoopPrompt,
				},
				EvalSet: &entity.EvaluationSet{
					EvaluationSetVersion: &entity.EvaluationSetVersion{
						EvaluationSetSchema: &entity.EvaluationSetSchema{
							FieldSchemas: []*entity.FieldSchema{
								{Name: "field1"},
							},
						},
					},
				},
			},
			setup:   func() {},
			wantErr: true,
		},
		{
			name: "valid_evaluators_connector",
			expt: &entity.Experiment{
				ID:      123,
				SpaceID: 789,
				EvalConf: &entity.EvaluationConfiguration{
					ConnectorConf: entity.Connector{
						EvaluatorsConf: &entity.EvaluatorsConf{
							EvaluatorConf: []*entity.EvaluatorConf{
								{
									EvaluatorVersionID: 1,
									IngressConf: &entity.EvaluatorIngressConf{
										EvalSetAdapter: &entity.FieldAdapter{
											FieldConfs: []*entity.FieldConf{
												{FromField: "field1"},
											},
										},
										TargetAdapter: &entity.FieldAdapter{ // 添加必要的TargetAdapter
											FieldConfs: []*entity.FieldConf{},
										},
									},
								},
							},
						},
					},
				},
				EvalSet: &entity.EvaluationSet{
					EvaluationSetVersion: &entity.EvaluationSetVersion{
						EvaluationSetSchema: &entity.EvaluationSetSchema{
							FieldSchemas: []*entity.FieldSchema{
								{Name: "field1"},
							},
						},
					},
				},
			},
			setup:   func() {},
			wantErr: false,
		},
		{
			name: "invalid_evaluators_connector_missing_field",
			expt: &entity.Experiment{
				ID:      123,
				SpaceID: 789,
				EvalConf: &entity.EvaluationConfiguration{
					ConnectorConf: entity.Connector{
						EvaluatorsConf: &entity.EvaluatorsConf{
							EvaluatorConf: []*entity.EvaluatorConf{
								{
									EvaluatorVersionID: 1,
									IngressConf: &entity.EvaluatorIngressConf{
										EvalSetAdapter: &entity.FieldAdapter{
											FieldConfs: []*entity.FieldConf{
												{FromField: "missing_field"},
											},
										},
										TargetAdapter: &entity.FieldAdapter{
											FieldConfs: []*entity.FieldConf{},
										},
									},
								},
							},
						},
					},
				},
				Evaluators: []*entity.Evaluator{
					{ID: 1},
				},
				EvalSet: &entity.EvaluationSet{
					EvaluationSetVersion: &entity.EvaluationSetVersion{
						EvaluationSetSchema: &entity.EvaluationSetSchema{
							FieldSchemas: []*entity.FieldSchema{
								{Name: "field1"},
							},
						},
					},
				},
			},
			setup:   func() {},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			err := mgr.CheckConnector(ctx, tt.expt, session)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckConnector() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
