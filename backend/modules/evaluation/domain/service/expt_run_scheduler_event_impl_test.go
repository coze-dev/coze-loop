// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	auditmocks "github.com/coze-dev/coze-loop/backend/infra/external/audit/mocks"
	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	idemmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	configmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	entitymocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity/mocks"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	mock_repo "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestExptSchedulerImpl_Schedule(t *testing.T) {
	testUserID := "test_user_id_123"
	mockExpt := &entity.Experiment{
		ID:                  1,
		SpaceID:             3,
		CreatedBy:           "created_by",
		Name:                "created_by",
		Description:         "description",
		EvalSetVersionID:    1,
		EvalSetID:           1,
		TargetType:          1,
		TargetVersionID:     1,
		TargetID:            1,
		EvaluatorVersionRef: []*entity.ExptEvaluatorVersionRef{{EvaluatorID: 1, EvaluatorVersionID: 1}},
		EvalConf: &entity.EvaluationConfiguration{ConnectorConf: entity.Connector{
			TargetConf: &entity.TargetConf{TargetVersionID: 1, IngressConf: &entity.TargetIngressConf{
				EvalSetAdapter: &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{{FieldName: "field_name", FromField: "from_field"}}},
			}},
			EvaluatorsConf: &entity.EvaluatorsConf{EvaluatorConcurNum: ptr.Of(1), EvaluatorConf: []*entity.EvaluatorConf{
				{
					EvaluatorVersionID: 1,
					IngressConf:        &entity.EvaluatorIngressConf{EvalSetAdapter: &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{{FieldName: "field_name", FromField: "from_field"}}}},
				},
			}},
		}},
		Target: &entity.EvalTarget{ID: 1, SpaceID: 3, SourceTargetID: "source_target_id", EvalTargetType: 1, EvalTargetVersion: &entity.EvalTargetVersion{ID: 1, OutputSchema: []*entity.ArgsSchema{{Key: ptr.Of("key")}}}, BaseInfo: &entity.BaseInfo{}},
		EvalSet: &entity.EvaluationSet{
			ID: 1, SpaceID: 3, Name: "name", Description: "description", Status: 0, Spec: nil, Features: nil, ItemCount: 0, ChangeUncommitted: false,
			EvaluationSetVersion: &entity.EvaluationSetVersion{ID: 1, AppID: 0, SpaceID: 3, EvaluationSetID: 1, Version: "version", VersionNum: 0, Description: "description", EvaluationSetSchema: nil, ItemCount: 0, BaseInfo: nil},
			LatestVersion:        "", NextVersionNum: 0, BaseInfo: nil, BizCategory: strconv.Itoa(1),
		},
		Evaluators:      []*entity.Evaluator{{}},
		Status:          0,
		StatusMessage:   "",
		LatestRunID:     0,
		CreditCost:      0,
		StartAt:         nil,
		EndAt:           nil,
		ExptType:        1,
		MaxAliveTime:    0,
		SourceType:      0,
		SourceID:        "",
		Stats:           nil,
		AggregateResult: nil,
	}

	type fields struct {
		manager              *svcmocks.MockIExptManager
		resultSvc            *svcmocks.MockExptResultService
		exptRepo             *mock_repo.MockIExperimentRepo
		exptItemResultRepo   *mock_repo.MockIExptItemResultRepo
		exptTurnResultRepo   *mock_repo.MockIExptTurnResultRepo
		exptStatsRepo        *mock_repo.MockIExptStatsRepo
		configer             *configmocks.MockIConfiger
		idGen                *idgenmocks.MockIIDGenerator
		publisher            *eventmocks.MockExptEventPublisher
		idem                 *idemmocks.MockIdempotentService
		evalSetItemSvc       *svcmocks.MockEvaluationSetItemService
		mutex                *lockmocks.MockILocker
		schedulerModeFactory *svcmocks.MockSchedulerModeFactory
	}

	type args struct {
		ctx   context.Context
		event *entity.ExptScheduleEvent
	}

	tests := []struct {
		name        string
		prepareMock func(f *fields, ctrl *gomock.Controller, args args) // Modification: add ctrl parameter
		args        args
		wantErr     bool
		assertErr   func(t *testing.T, err error)
	}{
		{
			name: "Normal flow - all success",
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event: &entity.ExptScheduleEvent{
					ExptID:      1,
					ExptRunID:   2,
					SpaceID:     3,
					ExptRunMode: 1,
					Session:     &entity.Session{UserID: testUserID},
					CreatedAt:   time.Now().Unix(),
				},
			},
			prepareMock: func(f *fields, ctrl *gomock.Controller, args args) { // Modification: add ctrl parameter
				f.configer.EXPECT().GetSchedulerAbortCtrl(gomock.Any()).Return(&entity.SchedulerAbortCtrl{}).AnyTimes()
				f.manager.EXPECT().GetDetail(gomock.Any(), int64(1), int64(3), args.event.Session).Return(mockExpt, nil).Times(1)
				f.manager.EXPECT().GetRunLog(gomock.Any(), int64(1), int64(2), int64(3), args.event.Session).Return(&entity.ExptRunLog{}, nil).Times(1)
				f.mutex.EXPECT().LockBackoffWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, args.ctx, func() {}, nil).Times(1)
				f.mutex.EXPECT().Unlock(gomock.Any()).Return(true, nil).AnyTimes()
				f.configer.EXPECT().GetExptExecConf(gomock.Any(), int64(3)).Return(&entity.ExptExecConf{
					ZombieIntervalSecond: math.MaxInt,
					ExptItemEvalConf:     &entity.ExptItemEvalConf{},
				}).AnyTimes()
				f.configer.EXPECT().GetConsumerConf(gomock.Any()).Return(&entity.ExptConsumerConf{
					ExptExecConf: &entity.ExptExecConf{
						ExptItemEvalConf: &entity.ExptItemEvalConf{},
					},
				}).AnyTimes()
				f.idGen.EXPECT().GenMultiIDs(gomock.Any(), gomock.Any()).Return([]int64{1, 2, 3}, nil).AnyTimes()
				f.publisher.EXPECT().PublishExptTurnResultFilterEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				f.resultSvc.EXPECT().UpsertExptTurnResultFilter(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				f.exptRepo.EXPECT().GetByID(gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.Experiment{Status: entity.ExptStatus_Processing}, nil).AnyTimes()
				f.publisher.EXPECT().PublishExptLifecycleEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

				mode := entitymocks.NewMockExptSchedulerMode(ctrl)
				mode.EXPECT().ExptStart(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
				mode.EXPECT().ExptEnd(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(false, nil).Times(1)
				mode.EXPECT().ScheduleStart(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
				mode.EXPECT().ScanEvalItems(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptEvalItem{}, []*entity.ExptEvalItem{}, []*entity.ExptEvalItem{}, nil).Times(1)
				mode.EXPECT().PublishResult(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				f.schedulerModeFactory.EXPECT().
					NewSchedulerMode(gomock.Any()).
					Return(mode, nil).Times(1)
				// Since mode is newed internally, interface substitution or injection is needed for actual testing
			},
			wantErr: false,
			assertErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "Experiment error",
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event: &entity.ExptScheduleEvent{
					ExptID:      1,
					ExptRunID:   2,
					SpaceID:     3,
					ExptRunMode: 1,
					Session:     &entity.Session{UserID: testUserID},
					CreatedAt:   time.Now().Unix(),
				},
			},
			prepareMock: func(f *fields, ctrl *gomock.Controller, args args) { // Modification: add ctrl parameter
				f.configer.EXPECT().GetSchedulerAbortCtrl(gomock.Any()).Return(&entity.SchedulerAbortCtrl{}).AnyTimes()
				f.manager.EXPECT().GetDetail(gomock.Any(), int64(1), int64(3), args.event.Session).Return(mockExpt, nil).Times(1)
				f.manager.EXPECT().GetRunLog(gomock.Any(), int64(1), int64(2), int64(3), args.event.Session).Return(&entity.ExptRunLog{}, nil).Times(1)
				f.mutex.EXPECT().LockBackoffWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, args.ctx, func() {}, nil).Times(1)
				f.mutex.EXPECT().Unlock(gomock.Any()).Return(true, nil).AnyTimes()
				f.configer.EXPECT().GetExptExecConf(gomock.Any(), int64(3)).Return(&entity.ExptExecConf{
					ZombieIntervalSecond: math.MaxInt,
					ExptItemEvalConf:     &entity.ExptItemEvalConf{},
				}).AnyTimes()
				f.configer.EXPECT().GetConsumerConf(gomock.Any()).Return(&entity.ExptConsumerConf{
					ExptExecConf: &entity.ExptExecConf{
						ExptItemEvalConf: &entity.ExptItemEvalConf{},
					},
				}).AnyTimes()
				f.idGen.EXPECT().GenMultiIDs(gomock.Any(), gomock.Any()).Return([]int64{1, 2, 3}, nil).AnyTimes()
				f.exptRepo.EXPECT().GetByID(gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.Experiment{Status: entity.ExptStatus_Processing}, nil).AnyTimes()
				f.publisher.EXPECT().PublishExptLifecycleEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				f.manager.EXPECT().CompleteRun(gomock.Any(), int64(1), int64(2), int64(3), args.event.Session, gomock.Any(), gomock.Any()).Return(nil).Times(1)
				f.manager.EXPECT().CompleteExpt(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
				mode := entitymocks.NewMockExptSchedulerMode(ctrl)
				mode.EXPECT().ExptStart(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
				mode.EXPECT().ScheduleStart(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
				mode.EXPECT().ScanEvalItems(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptEvalItem{}, []*entity.ExptEvalItem{}, []*entity.ExptEvalItem{}, nil).Times(1)
				mode.EXPECT().ExptEnd(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(false, errors.New("test error")).Times(1)
				mode.EXPECT().PublishResult(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				f.publisher.EXPECT().PublishExptTurnResultFilterEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				f.resultSvc.EXPECT().UpsertExptTurnResultFilter(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				f.schedulerModeFactory.EXPECT().
					NewSchedulerMode(gomock.Any()).
					Return(mode, nil).Times(1)
				// Since mode is newed internally, interface substitution or injection is needed for actual testing
			},
			wantErr: false,
			assertErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			f := &fields{
				manager:              svcmocks.NewMockIExptManager(ctrl),
				exptRepo:             mock_repo.NewMockIExperimentRepo(ctrl),
				exptItemResultRepo:   mock_repo.NewMockIExptItemResultRepo(ctrl),
				exptTurnResultRepo:   mock_repo.NewMockIExptTurnResultRepo(ctrl),
				exptStatsRepo:        mock_repo.NewMockIExptStatsRepo(ctrl),
				configer:             configmocks.NewMockIConfiger(ctrl),
				idGen:                idgenmocks.NewMockIIDGenerator(ctrl),
				publisher:            eventmocks.NewMockExptEventPublisher(ctrl),
				idem:                 idemmocks.NewMockIdempotentService(ctrl),
				evalSetItemSvc:       svcmocks.NewMockEvaluationSetItemService(ctrl),
				mutex:                lockmocks.NewMockILocker(ctrl),
				schedulerModeFactory: svcmocks.NewMockSchedulerModeFactory(ctrl),
				resultSvc:            svcmocks.NewMockExptResultService(ctrl),
			}

			if tt.prepareMock != nil {
				tt.prepareMock(f, ctrl, tt.args) // Modification point: pass ctrl
			}

			svc := &ExptSchedulerImpl{
				Manager:                  f.manager,
				ExptRepo:                 f.exptRepo,
				ExptItemResultRepo:       f.exptItemResultRepo,
				ExptTurnResultRepo:       f.exptTurnResultRepo,
				ExptStatsRepo:            f.exptStatsRepo,
				Configer:                 f.configer,
				IDGen:                    f.idGen,
				Publisher:                f.publisher,
				Idem:                     f.idem,
				evaluationSetItemService: f.evalSetItemSvc,
				Mutex:                    f.mutex,
				schedulerModeFactory:     f.schedulerModeFactory,
				ResultSvc:                f.resultSvc,
			}
			svc.Endpoints = SchedulerChain(
				svc.HandleEventErr,
				svc.SysOps,
				svc.HandleEventCheck,
				svc.HandleEventLock,
				svc.HandleEventEndpoint,
			)(func(_ context.Context, _ *entity.ExptScheduleEvent) error { return nil })

			err := svc.Schedule(tt.args.ctx, tt.args.event)
			if tt.assertErr != nil {
				tt.assertErr(t, err)
			}
		})
	}
}

func TestExptSchedulerImpl_RecordEvalItemRunLogs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	testUserID := "test_user_id_123"

	type fields struct {
		ResultSvc *svcmocks.MockExptResultService
		Publisher *eventmocks.MockExptEventPublisher
	}

	type args struct {
		ctx           context.Context
		event         *entity.ExptScheduleEvent
		completeItems []*entity.ExptEvalItem
	}

	mockMode := entitymocks.NewMockExptSchedulerMode(ctrl)

	tests := []struct {
		name        string
		prepareMock func(f *fields, ctrl *gomock.Controller, args args) // Modification: add ctrl parameter
		args        args
		wantErr     bool
		assertErr   func(t *testing.T, err error)
	}{
		{
			name: "Normal flow - all success",
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event: &entity.ExptScheduleEvent{
					ExptID:      1,
					ExptRunID:   2,
					SpaceID:     3,
					ExptRunMode: 1,
					Session:     &entity.Session{UserID: testUserID},
				},
				completeItems: []*entity.ExptEvalItem{
					{ItemID: 1, State: entity.ItemRunState_Success},
					{ItemID: 2, State: entity.ItemRunState_Fail},
				},
			},
			prepareMock: func(f *fields, ctrl *gomock.Controller, args args) { // Modification: add ctrl parameter
				f.ResultSvc.EXPECT().RecordItemRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
				mockMode.EXPECT().PublishResult(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				f.ResultSvc.EXPECT().UpsertExptTurnResultFilter(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				f.Publisher.EXPECT().PublishExptTurnResultFilterEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			},
			wantErr: false,
			assertErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fields{
				ResultSvc: svcmocks.NewMockExptResultService(ctrl),
				Publisher: eventmocks.NewMockExptEventPublisher(ctrl),
			}

			if tt.prepareMock != nil {
				tt.prepareMock(f, ctrl, tt.args) // Modification: pass ctrl
			}

			svc := &ExptSchedulerImpl{
				ResultSvc: f.ResultSvc,
				Publisher: f.Publisher,
			}

			err := svc.recordEvalItemRunLogs(tt.args.ctx, tt.args.event, tt.args.completeItems, mockMode, nil)
			if tt.assertErr != nil {
				tt.assertErr(t, err)
			}
		})
	}
}

func TestExptSchedulerImpl_SubmitItemEval(t *testing.T) {
	testUserID := "test_user_id_123"

	type fields struct {
		exptItemResultRepo *mock_repo.MockIExptItemResultRepo
		exptTurnResultRepo *mock_repo.MockIExptTurnResultRepo
		exptStatsRepo      *mock_repo.MockIExptStatsRepo
		configer           *configmocks.MockIConfiger
		publisher          *eventmocks.MockExptEventPublisher
		metric             *metricsmocks.MockExptMetric
		resultSvc          *svcmocks.MockExptResultService
	}

	type args struct {
		ctx       context.Context
		event     *entity.ExptScheduleEvent
		toSubmits []*entity.ExptEvalItem
		expt      *entity.Experiment
	}

	tests := []struct {
		name        string
		prepareMock func(f *fields, ctrl *gomock.Controller, args args) // Modification: add ctrl parameter
		args        args
		wantErr     bool
		assertErr   func(t *testing.T, err error)
	}{
		{
			name: "Normal flow - all success",
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event: &entity.ExptScheduleEvent{
					ExptID:      1,
					ExptRunID:   2,
					SpaceID:     3,
					ExptRunMode: 1,
					Session:     &entity.Session{UserID: testUserID},
				},
				toSubmits: []*entity.ExptEvalItem{
					{ItemID: 1, State: entity.ItemRunState_Success},
					{ItemID: 2, State: entity.ItemRunState_Fail},
					{ItemID: 3, State: entity.ItemRunState_Queueing},
					{ItemID: 4, State: entity.ItemRunState_Processing},
				},
				expt: &entity.Experiment{
					ID:       1,
					SpaceID:  1,
					ExptType: entity.ExptType_Offline,
				},
			},
			prepareMock: func(f *fields, ctrl *gomock.Controller, args args) { // Modification: add ctrl parameter
				f.exptItemResultRepo.EXPECT().UpdateItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				f.exptItemResultRepo.EXPECT().UpdateItemsResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				f.exptItemResultRepo.EXPECT().BatchGet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResult{}, nil).AnyTimes()
				f.exptTurnResultRepo.EXPECT().UpdateTurnResultsWithItemIDs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				f.exptTurnResultRepo.EXPECT().BatchGet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResult{}, nil).AnyTimes()
				f.publisher.EXPECT().BatchPublishExptRecordEvalEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				f.configer.EXPECT().GetExptExecConf(gomock.Any(), int64(3)).Return(&entity.ExptExecConf{
					ExptItemEvalConf: &entity.ExptItemEvalConf{
						ConcurNum:      1,
						IntervalSecond: 1,
					},
				}).AnyTimes()
				f.configer.EXPECT().GetConsumerConf(gomock.Any()).Return(&entity.ExptConsumerConf{}).AnyTimes()
				f.exptStatsRepo.EXPECT().ArithOperateCount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				f.metric.EXPECT().EmitItemExecEval(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				f.resultSvc.EXPECT().UpsertExptTurnResultFilter(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			},
			wantErr: false,
			assertErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			f := &fields{
				exptItemResultRepo: mock_repo.NewMockIExptItemResultRepo(ctrl),
				exptTurnResultRepo: mock_repo.NewMockIExptTurnResultRepo(ctrl),
				exptStatsRepo:      mock_repo.NewMockIExptStatsRepo(ctrl),
				configer:           configmocks.NewMockIConfiger(ctrl),
				publisher:          eventmocks.NewMockExptEventPublisher(ctrl),
				metric:             metricsmocks.NewMockExptMetric(ctrl),
				resultSvc:          svcmocks.NewMockExptResultService(ctrl),
			}

			if tt.prepareMock != nil {
				tt.prepareMock(f, ctrl, tt.args) // Modification: pass ctrl
			}

			svc := &ExptSchedulerImpl{
				ExptItemResultRepo: f.exptItemResultRepo,
				ExptTurnResultRepo: f.exptTurnResultRepo,
				ExptStatsRepo:      f.exptStatsRepo,
				Configer:           f.configer,
				Publisher:          f.publisher,
				Metric:             f.metric,
				ResultSvc:          f.resultSvc,
			}

			err := svc.handleToSubmits(tt.args.ctx, tt.args.event, tt.args.toSubmits)
			if tt.assertErr != nil {
				tt.assertErr(t, err)
			}
		})
	}
}

func TestNewExptSchedulerSvc(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	manager := svcmocks.NewMockIExptManager(ctrl)
	exptRepo := mock_repo.NewMockIExperimentRepo(ctrl)
	exptItemResultRepo := mock_repo.NewMockIExptItemResultRepo(ctrl)
	exptTurnResultRepo := mock_repo.NewMockIExptTurnResultRepo(ctrl)
	evaluatorRecordRepo := mock_repo.NewMockIEvaluatorRecordRepo(ctrl)
	exptStatsRepo := mock_repo.NewMockIExptStatsRepo(ctrl)
	exptRunLogRepo := mock_repo.NewMockIExptRunLogRepo(ctrl)
	idem := idemmocks.NewMockIdempotentService(ctrl)
	configer := configmocks.NewMockIConfiger(ctrl)
	quotaRepo := mock_repo.NewMockQuotaRepo(ctrl)
	mutex := lockmocks.NewMockILocker(ctrl)
	publisher := eventmocks.NewMockExptEventPublisher(ctrl)
	auditClient := auditmocks.NewMockIAuditService(ctrl)
	metric := metricsmocks.NewMockExptMetric(ctrl)
	resultSvc := svcmocks.NewMockExptResultService(ctrl)
	idGen := idgenmocks.NewMockIIDGenerator(ctrl)
	evalSetItemSvc := svcmocks.NewMockEvaluationSetItemService(ctrl)
	schedulerModeFactory := svcmocks.NewMockSchedulerModeFactory(ctrl)
	evalTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	exptItemRefRepo := mock_repo.NewMockIExptItemRefRepo(ctrl)

	svc := NewExptSchedulerSvc(
		manager,
		exptRepo,
		exptItemResultRepo,
		exptTurnResultRepo,
		evaluatorRecordRepo,
		exptStatsRepo,
		exptRunLogRepo,
		idem,
		configer,
		quotaRepo,
		mutex,
		publisher,
		auditClient,
		metric,
		resultSvc,
		idGen,
		evalSetItemSvc,
		schedulerModeFactory,
		evalTargetSvc,
		nil, // itemCompletePublisher: 开源侧 nil, scheduler 循环内以非空守卫跳过发送
		exptItemRefRepo,
		metricsmocks.NewMockSandboxAgentMetrics(ctrl),
		component.NewNoopCentralReservationGuard(),
	)
	assert.NotNil(t, svc)
	assert.Implements(t, (*ExptSchedulerEvent)(nil), svc)
	impl, ok := svc.(*ExptSchedulerImpl)
	assert.True(t, ok)
	assert.Equal(t, manager, impl.Manager)
	assert.Equal(t, exptRepo, impl.ExptRepo)
	assert.Equal(t, exptItemResultRepo, impl.ExptItemResultRepo)
	assert.Equal(t, exptTurnResultRepo, impl.ExptTurnResultRepo)
	assert.Equal(t, evaluatorRecordRepo, impl.EvaluatorRecordRepo)
	assert.Equal(t, exptStatsRepo, impl.ExptStatsRepo)
	assert.Equal(t, exptRunLogRepo, impl.ExptRunLogRepo)
	assert.Equal(t, idem, impl.Idem)
	assert.Equal(t, configer, impl.Configer)
	assert.Equal(t, quotaRepo, impl.QuotaRepo)
	assert.Equal(t, mutex, impl.Mutex)
	assert.Equal(t, publisher, impl.Publisher)
	assert.Equal(t, auditClient, impl.AuditClient)
	assert.Equal(t, metric, impl.Metric)
	assert.Equal(t, resultSvc, impl.ResultSvc)
	assert.Equal(t, idGen, impl.IDGen)
	assert.Equal(t, evalSetItemSvc, impl.evaluationSetItemService)
	assert.Equal(t, schedulerModeFactory, impl.schedulerModeFactory)
	assert.Equal(t, exptItemRefRepo, impl.exptItemRefRepo)
}

func TestExptSchedulerImpl_HandleEventLock(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mutex := lockmocks.NewMockILocker(ctrl)
	svc := &ExptSchedulerImpl{
		Mutex: mutex,
	}

	type lockArgs struct {
		event   *entity.ExptScheduleEvent
		locked  bool
		lockErr error
	}

	tests := []struct {
		name    string
		args    lockArgs
		next    func(ctx context.Context, event *entity.ExptScheduleEvent) error
		wantErr bool
		wantNil bool // whether nil is expected (i.e. when lock is not obtained)
	}{
		{
			name: "Normal lock and call next",
			args: lockArgs{
				event:   &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2},
				locked:  true,
				lockErr: nil,
			},
			next: func(ctx context.Context, event *entity.ExptScheduleEvent) error {
				return nil
			},
			wantErr: false,
			wantNil: false,
		},
		{
			name: "Lock failure returns error",
			args: lockArgs{
				event:   &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2},
				locked:  false,
				lockErr: errors.New("lock error"),
			},
			next: func(ctx context.Context, event *entity.ExptScheduleEvent) error {
				return nil
			},
			wantErr: true,
			wantNil: false,
		},
		{
			name: "Return nil directly if lock is not obtained",
			args: lockArgs{
				event:   &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2},
				locked:  false,
				lockErr: nil,
			},
			next: func(ctx context.Context, event *entity.ExptScheduleEvent) error {
				return errors.New("should not be called")
			},
			wantErr: false,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unlockCalled := false
			mutex.EXPECT().LockBackoffWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(tt.args.locked, context.Background(), func() { unlockCalled = true }, tt.args.lockErr)
			if tt.args.locked && tt.args.lockErr == nil {
				mutex.EXPECT().Unlock(gomock.Any()).Return(true, nil)
			}
			handler := svc.HandleEventLock(tt.next)
			err := handler(context.Background(), tt.args.event)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			if tt.wantNil {
				assert.Nil(t, err)
			}
			if tt.args.locked && !tt.wantErr {
				assert.True(t, unlockCalled, "unlock should be called when locked")
			}
		})
	}
}

func TestExptSchedulerImpl_HandleEventCheck(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	manager := svcmocks.NewMockIExptManager(ctrl)
	configer := configmocks.NewMockIConfiger(ctrl)
	svc := &ExptSchedulerImpl{
		Manager:  manager,
		Configer: configer,
	}

	type checkArgs struct {
		event      *entity.ExptScheduleEvent
		runLog     *entity.ExptRunLog
		runLogErr  error
		zombieSecs int64
		createdAt  int64
	}

	tests := []struct {
		name        string
		args        checkArgs
		next        func(ctx context.Context, event *entity.ExptScheduleEvent) error
		preparemock func()
		wantErr     bool
	}{
		{
			name: "Normal flow, not finished, no timeout, call next",
			args: checkArgs{
				event:      &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, CreatedAt: time.Now().Unix()},
				runLog:     &entity.ExptRunLog{Status: int64(entity.ExptStatus_Processing)},
				runLogErr:  nil,
				zombieSecs: 10000,
				createdAt:  time.Now().Unix(),
			},
			next: func(ctx context.Context, event *entity.ExptScheduleEvent) error { return nil },
			preparemock: func() {
				configer.EXPECT().GetExptExecConf(gomock.Any(), gomock.Any()).Return(&entity.ExptExecConf{ZombieIntervalSecond: int(10000)}).Times(1)
				manager.EXPECT().GetRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.ExptRunLog{Status: int64(entity.ExptStatus_Processing)}, nil).Times(1)
			},
			wantErr: false,
		},
		{
			name: "runLog returns error",
			args: checkArgs{
				event:      &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3},
				runLog:     nil,
				runLogErr:  errors.New("db error"),
				zombieSecs: 10000,
				createdAt:  time.Now().Unix(),
			},
			next: func(ctx context.Context, event *entity.ExptScheduleEvent) error { return nil },
			preparemock: func() {
				manager.EXPECT().GetRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("db error")).Times(1)
			},
			wantErr: true,
		},
		{
			name: "Experiment completed, return nil directly",
			args: checkArgs{
				event:      &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3},
				runLog:     &entity.ExptRunLog{Status: int64(entity.ExptStatus_Success)},
				runLogErr:  nil,
				zombieSecs: 10000,
				createdAt:  time.Now().Unix(),
			},
			next: func(ctx context.Context, event *entity.ExptScheduleEvent) error {
				return errors.New("should not be called")
			},
			preparemock: func() {
				manager.EXPECT().GetRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.ExptRunLog{Status: int64(entity.ExptStatus_Success)}, nil).Times(1)
			},
			wantErr: false,
		},
		{
			name: "Experiment terminating, return nil directly",
			args: checkArgs{
				event:      &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3},
				runLog:     &entity.ExptRunLog{Status: int64(entity.ExptStatus_Terminating)},
				runLogErr:  nil,
				zombieSecs: 10000,
				createdAt:  time.Now().Unix(),
			},
			next: func(ctx context.Context, event *entity.ExptScheduleEvent) error {
				return errors.New("should not be called")
			},
			preparemock: func() {
				manager.EXPECT().GetRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.ExptRunLog{Status: int64(entity.ExptStatus_Terminating)}, nil).Times(1)
			},
			wantErr: false,
		},
		{
			name: "Experiment draining, return nil directly",
			args: checkArgs{
				event:      &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3},
				runLog:     &entity.ExptRunLog{Status: int64(entity.ExptStatus_Draining)},
				runLogErr:  nil,
				zombieSecs: 10000,
				createdAt:  time.Now().Unix(),
			},
			next: func(ctx context.Context, event *entity.ExptScheduleEvent) error {
				return errors.New("should not be called")
			},
			preparemock: func() {
				manager.EXPECT().GetRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.ExptRunLog{Status: int64(entity.ExptStatus_Draining)}, nil).Times(1)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.preparemock()
			handler := svc.HandleEventCheck(tt.next)
			err := handler(context.Background(), tt.args.event)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptSchedulerImpl_handleZombies(t *testing.T) {
	testUserID := "test_user_id_123"

	type fields struct {
		configer            *configmocks.MockIConfiger
		exptItemResultRepo  *mock_repo.MockIExptItemResultRepo
		exptTurnResultRepo  *mock_repo.MockIExptTurnResultRepo
		evaluatorRecordRepo *mock_repo.MockIEvaluatorRecordRepo
	}

	type args struct {
		ctx   context.Context
		event *entity.ExptScheduleEvent
		items []*entity.ExptEvalItem
	}

	now := time.Now()
	zombieTime := now.Add(-10 * time.Minute)
	aliveTime := now.Add(-1 * time.Minute)

	tests := []struct {
		name        string
		prepareMock func(f *fields, ctrl *gomock.Controller, args args)
		args        args
		wantAlives  []*entity.ExptEvalItem
		wantZombies []*entity.ExptEvalItem
		wantErr     bool
		assertErr   func(t *testing.T, err error)
	}{
		{
			name: "Normal case - no zombie tasks",
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event: &entity.ExptScheduleEvent{
					ExptID:    1,
					ExptRunID: 2,
					SpaceID:   3,
					Session:   &entity.Session{UserID: testUserID},
				},
				items: []*entity.ExptEvalItem{
					{
						ExptID:    1,
						ItemID:    1,
						State:     entity.ItemRunState_Processing,
						UpdatedAt: &aliveTime,
					},
					{
						ExptID:    1,
						ItemID:    2,
						State:     entity.ItemRunState_Processing,
						UpdatedAt: &aliveTime,
					},
				},
			},
			prepareMock: func(f *fields, ctrl *gomock.Controller, args args) {
				f.configer.EXPECT().GetConsumerConf(gomock.Any()).Return(&entity.ExptConsumerConf{
					SpaceExptExecConf: map[int64]*entity.ExptExecConf{
						3: {
							ExptItemEvalConf: &entity.ExptItemEvalConf{
								ZombieSecond: 300,
							},
						},
					},
				}).Times(1)
			},
			wantAlives: []*entity.ExptEvalItem{
				{
					ExptID:    1,
					ItemID:    1,
					State:     entity.ItemRunState_Processing,
					UpdatedAt: &aliveTime,
				},
				{
					ExptID:    1,
					ItemID:    2,
					State:     entity.ItemRunState_Processing,
					UpdatedAt: &aliveTime,
				},
			},
			wantZombies: []*entity.ExptEvalItem{},
			wantErr:     false,
			assertErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "Normal case - zombie tasks need to be handled",
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event: &entity.ExptScheduleEvent{
					ExptID:    1,
					ExptRunID: 2,
					SpaceID:   3,
					Session:   &entity.Session{UserID: testUserID},
				},
				items: []*entity.ExptEvalItem{
					{
						ExptID:    1,
						ItemID:    1,
						State:     entity.ItemRunState_Processing,
						UpdatedAt: &zombieTime,
					},
					{
						ExptID:    1,
						ItemID:    2,
						State:     entity.ItemRunState_Processing,
						UpdatedAt: &aliveTime,
					},
					{
						ExptID:    1,
						ItemID:    3,
						State:     entity.ItemRunState_Processing,
						UpdatedAt: &zombieTime,
					},
				},
			},
			prepareMock: func(f *fields, ctrl *gomock.Controller, args args) {
				f.configer.EXPECT().GetConsumerConf(gomock.Any()).Return(&entity.ExptConsumerConf{
					SpaceExptExecConf: map[int64]*entity.ExptExecConf{
						3: {
							ExptItemEvalConf: &entity.ExptItemEvalConf{
								ZombieSecond: 300,
							},
						},
					},
				}).Times(1)
				f.evaluatorRecordRepo.EXPECT().BatchGetEvaluatorRecord(
					gomock.Any(), gomock.Any(), false, false,
				).Return(nil, nil).AnyTimes()
				f.exptTurnResultRepo.EXPECT().MGetItemTurnRunLogs(
					gomock.Any(), int64(1), int64(2), []int64{1, 3}, int64(3),
				).Return(nil, nil).Times(1)
				f.exptItemResultRepo.EXPECT().UpdateItemRunLog(
					gomock.Any(),
					int64(1),
					int64(2),
					[]int64{1, 3},
					gomock.Any(),
					int64(3),
				).DoAndReturn(func(_ context.Context, _, _ int64, _ []int64, ufields map[string]any, _ int64) error {
					assert.Equal(t, int32(entity.ItemRunState_Fail), ufields["status"])
					assert.Equal(t, int32(entity.ExptItemResultStateLogged), ufields["result_state"])
					assert.NotNil(t, ufields["err_msg"])
					return nil
				}).Times(1)
				f.exptItemResultRepo.EXPECT().UpdateItemsResult(
					gomock.Any(),
					int64(3),
					int64(1),
					[]int64{1, 3},
					gomock.Any(),
				).DoAndReturn(func(_ context.Context, _, _ int64, _ []int64, ufields map[string]any) error {
					// 与 sweep 路径同理: 主表只补 err_msg, 不预写 status=Fail,
					// 让 RecordItemRunLogs 走正常 statsCntOp diff.
					_, hasStatus := ufields["status"]
					assert.False(t, hasStatus, "UpdateItemsResult must not pre-write status on zombie path")
					assert.NotNil(t, ufields["err_msg"])
					return nil
				}).Times(1)
				f.exptTurnResultRepo.EXPECT().CreateOrUpdateItemsTurnRunLogStatus(
					gomock.Any(),
					int64(3),
					int64(1),
					int64(2),
					[]int64{1, 3},
					entity.TurnRunState_Fail,
				).Return(nil).Times(1)
				// zombie 场景不再清 run_log 的 target_result_id / evaluator_result_ids，
				// 保留 record id 让 /results/batch_get 能返回 eval_target_record.id
			},
			wantAlives: []*entity.ExptEvalItem{
				{
					ExptID:    1,
					ItemID:    2,
					State:     entity.ItemRunState_Processing,
					UpdatedAt: &aliveTime,
				},
			},
			wantZombies: []*entity.ExptEvalItem{
				{
					ExptID:    1,
					ItemID:    1,
					State:     entity.ItemRunState_Fail,
					UpdatedAt: &zombieTime,
				},
				{
					ExptID:    1,
					ItemID:    3,
					State:     entity.ItemRunState_Fail,
					UpdatedAt: &zombieTime,
				},
			},
			wantErr: false,
			assertErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "Error case - UpdateItemRunLog failed",
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event: &entity.ExptScheduleEvent{
					ExptID:    1,
					ExptRunID: 2,
					SpaceID:   3,
					Session:   &entity.Session{UserID: testUserID},
				},
				items: []*entity.ExptEvalItem{
					{
						ExptID:    1,
						ItemID:    1,
						State:     entity.ItemRunState_Processing,
						UpdatedAt: &zombieTime,
					},
				},
			},
			prepareMock: func(f *fields, ctrl *gomock.Controller, args args) {
				f.configer.EXPECT().GetConsumerConf(gomock.Any()).Return(&entity.ExptConsumerConf{
					SpaceExptExecConf: map[int64]*entity.ExptExecConf{
						3: {
							ExptItemEvalConf: &entity.ExptItemEvalConf{
								ZombieSecond: 300,
							},
						},
					},
				}).Times(1)
				f.evaluatorRecordRepo.EXPECT().BatchGetEvaluatorRecord(
					gomock.Any(), gomock.Any(), false, false,
				).Return(nil, nil).AnyTimes()
				f.exptTurnResultRepo.EXPECT().MGetItemTurnRunLogs(
					gomock.Any(), int64(1), int64(2), []int64{1}, int64(3),
				).Return(nil, nil).Times(1)
				f.exptItemResultRepo.EXPECT().UpdateItemRunLog(
					gomock.Any(),
					int64(1),
					int64(2),
					[]int64{1},
					gomock.Any(),
					int64(3),
				).Return(errors.New("update item run log failed")).Times(1)
			},
			wantAlives:  nil,
			wantZombies: nil,
			wantErr:     true,
			assertErr: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "update item run log failed")
			},
		},
		{
			name: "Error case - UpdateTurnRunLog failed",
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event: &entity.ExptScheduleEvent{
					ExptID:    1,
					ExptRunID: 2,
					SpaceID:   3,
					Session:   &entity.Session{UserID: testUserID},
				},
				items: []*entity.ExptEvalItem{
					{
						ExptID:    1,
						ItemID:    1,
						State:     entity.ItemRunState_Processing,
						UpdatedAt: &zombieTime,
					},
				},
			},
			prepareMock: func(f *fields, ctrl *gomock.Controller, args args) {
				f.configer.EXPECT().GetConsumerConf(gomock.Any()).Return(&entity.ExptConsumerConf{
					SpaceExptExecConf: map[int64]*entity.ExptExecConf{
						3: {
							ExptItemEvalConf: &entity.ExptItemEvalConf{
								ZombieSecond: 300,
							},
						},
					},
				}).Times(1)
				f.evaluatorRecordRepo.EXPECT().BatchGetEvaluatorRecord(
					gomock.Any(), gomock.Any(), false, false,
				).Return(nil, nil).AnyTimes()
				f.exptTurnResultRepo.EXPECT().MGetItemTurnRunLogs(
					gomock.Any(), int64(1), int64(2), []int64{1}, int64(3),
				).Return(nil, nil).Times(1)
				f.exptItemResultRepo.EXPECT().UpdateItemRunLog(
					gomock.Any(),
					int64(1),
					int64(2),
					[]int64{1},
					gomock.Any(),
					int64(3),
				).Return(nil).Times(1)
				f.exptItemResultRepo.EXPECT().UpdateItemsResult(
					gomock.Any(),
					int64(3),
					int64(1),
					[]int64{1},
					gomock.Any(),
				).Return(nil).Times(1)
				f.exptTurnResultRepo.EXPECT().CreateOrUpdateItemsTurnRunLogStatus(
					gomock.Any(),
					int64(3),
					int64(1),
					int64(2),
					[]int64{1},
					entity.TurnRunState_Fail,
				).Return(errors.New("update turn run log failed")).Times(1)
			},
			wantAlives:  nil,
			wantZombies: nil,
			wantErr:     true,
			assertErr: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "update turn run log failed")
			},
		},
		{
			name: "Edge case - all tasks are zombies",
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event: &entity.ExptScheduleEvent{
					ExptID:    1,
					ExptRunID: 2,
					SpaceID:   3,
					Session:   &entity.Session{UserID: testUserID},
				},
				items: []*entity.ExptEvalItem{
					{
						ExptID:    1,
						ItemID:    1,
						State:     entity.ItemRunState_Processing,
						UpdatedAt: &zombieTime,
					},
					{
						ExptID:    1,
						ItemID:    2,
						State:     entity.ItemRunState_Processing,
						UpdatedAt: &zombieTime,
					},
				},
			},
			prepareMock: func(f *fields, ctrl *gomock.Controller, args args) {
				f.configer.EXPECT().GetConsumerConf(gomock.Any()).Return(&entity.ExptConsumerConf{
					SpaceExptExecConf: map[int64]*entity.ExptExecConf{
						3: {
							ExptItemEvalConf: &entity.ExptItemEvalConf{
								ZombieSecond: 300,
							},
						},
					},
				}).Times(1)
				f.evaluatorRecordRepo.EXPECT().BatchGetEvaluatorRecord(
					gomock.Any(), gomock.Any(), false, false,
				).Return(nil, nil).AnyTimes()
				f.exptTurnResultRepo.EXPECT().MGetItemTurnRunLogs(
					gomock.Any(), int64(1), int64(2), []int64{1, 2}, int64(3),
				).Return(nil, nil).Times(1)
				f.exptItemResultRepo.EXPECT().UpdateItemRunLog(
					gomock.Any(),
					int64(1),
					int64(2),
					[]int64{1, 2},
					gomock.Any(),
					int64(3),
				).Return(nil).Times(1)
				f.exptItemResultRepo.EXPECT().UpdateItemsResult(
					gomock.Any(),
					int64(3),
					int64(1),
					[]int64{1, 2},
					gomock.Any(),
				).Return(nil).Times(1)
				f.exptTurnResultRepo.EXPECT().CreateOrUpdateItemsTurnRunLogStatus(
					gomock.Any(),
					int64(3),
					int64(1),
					int64(2),
					[]int64{1, 2},
					entity.TurnRunState_Fail,
				).Return(nil).Times(1)
				// zombie 场景不再清 run_log 的 target_result_id / evaluator_result_ids
			},
			wantAlives: []*entity.ExptEvalItem{},
			wantZombies: []*entity.ExptEvalItem{
				{
					ExptID:    1,
					ItemID:    1,
					State:     entity.ItemRunState_Fail,
					UpdatedAt: &zombieTime,
				},
				{
					ExptID:    1,
					ItemID:    2,
					State:     entity.ItemRunState_Fail,
					UpdatedAt: &zombieTime,
				},
			},
			wantErr: false,
			assertErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "Edge case - task update time is nil",
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event: &entity.ExptScheduleEvent{
					ExptID:    1,
					ExptRunID: 2,
					SpaceID:   3,
					Session:   &entity.Session{UserID: testUserID},
				},
				items: []*entity.ExptEvalItem{
					{
						ExptID:    1,
						ItemID:    1,
						State:     entity.ItemRunState_Processing,
						UpdatedAt: nil,
					},
					{
						ExptID:    1,
						ItemID:    2,
						State:     entity.ItemRunState_Processing,
						UpdatedAt: &time.Time{},
					},
					{
						ExptID:    1,
						ItemID:    3,
						State:     entity.ItemRunState_Processing,
						UpdatedAt: &aliveTime,
					},
				},
			},
			prepareMock: func(f *fields, ctrl *gomock.Controller, args args) {
				f.configer.EXPECT().GetConsumerConf(gomock.Any()).Return(&entity.ExptConsumerConf{
					SpaceExptExecConf: map[int64]*entity.ExptExecConf{
						3: {
							ExptItemEvalConf: &entity.ExptItemEvalConf{
								ZombieSecond: 300,
							},
						},
					},
				}).Times(1)
			},
			wantAlives: []*entity.ExptEvalItem{
				{
					ExptID:    1,
					ItemID:    3,
					State:     entity.ItemRunState_Processing,
					UpdatedAt: &aliveTime,
				},
			},
			wantZombies: []*entity.ExptEvalItem{},
			wantErr:     false,
			assertErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "Edge case - tasks with non-Processing state",
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event: &entity.ExptScheduleEvent{
					ExptID:    1,
					ExptRunID: 2,
					SpaceID:   3,
					Session:   &entity.Session{UserID: testUserID},
				},
				items: []*entity.ExptEvalItem{
					{
						ExptID:    1,
						ItemID:    1,
						State:     entity.ItemRunState_Queueing,
						UpdatedAt: &zombieTime,
					},
					{
						ExptID:    1,
						ItemID:    2,
						State:     entity.ItemRunState_Success,
						UpdatedAt: &zombieTime,
					},
					{
						ExptID:    1,
						ItemID:    3,
						State:     entity.ItemRunState_Processing,
						UpdatedAt: &aliveTime,
					},
				},
			},
			prepareMock: func(f *fields, ctrl *gomock.Controller, args args) {
				f.configer.EXPECT().GetConsumerConf(gomock.Any()).Return(&entity.ExptConsumerConf{
					SpaceExptExecConf: map[int64]*entity.ExptExecConf{
						3: {
							ExptItemEvalConf: &entity.ExptItemEvalConf{
								ZombieSecond: 300,
							},
						},
					},
				}).Times(1)
			},
			wantAlives: []*entity.ExptEvalItem{
				{
					ExptID:    1,
					ItemID:    3,
					State:     entity.ItemRunState_Processing,
					UpdatedAt: &aliveTime,
				},
			},
			wantZombies: []*entity.ExptEvalItem{},
			wantErr:     false,
			assertErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			f := &fields{
				configer:            configmocks.NewMockIConfiger(ctrl),
				exptItemResultRepo:  mock_repo.NewMockIExptItemResultRepo(ctrl),
				exptTurnResultRepo:  mock_repo.NewMockIExptTurnResultRepo(ctrl),
				evaluatorRecordRepo: mock_repo.NewMockIEvaluatorRecordRepo(ctrl),
			}

			if tt.prepareMock != nil {
				tt.prepareMock(f, ctrl, tt.args)
			}

			svc := &ExptSchedulerImpl{
				Configer:            f.configer,
				ExptItemResultRepo:  f.exptItemResultRepo,
				ExptTurnResultRepo:  f.exptTurnResultRepo,
				EvaluatorRecordRepo: f.evaluatorRecordRepo,
			}

			alives, zombies, err := svc.handleZombies(tt.args.ctx, tt.args.event, tt.args.items, nil)

			if tt.assertErr != nil {
				tt.assertErr(t, err)
			}

			if !tt.wantErr {
				assert.Equal(t, len(tt.wantAlives), len(alives), "alives count should match")
				assert.Equal(t, len(tt.wantZombies), len(zombies), "zombies count should match")

				for i, expectedAlive := range tt.wantAlives {
					if i < len(alives) {
						assert.Equal(t, expectedAlive.ItemID, alives[i].ItemID, "alive item ID should match")
						assert.Equal(t, expectedAlive.State, alives[i].State, "alive item state should match")
					}
				}

				for i, expectedZombie := range tt.wantZombies {
					if i < len(zombies) {
						assert.Equal(t, expectedZombie.ItemID, zombies[i].ItemID, "zombie item ID should match")
						assert.Equal(t, expectedZombie.State, zombies[i].State, "zombie item state should be Fail")
					}
				}
			}
		})
	}
}

func TestExptSchedulerImpl_Schedule_ContextCancelled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockManager := svcmocks.NewMockIExptManager(ctrl)
	mockFactory := svcmocks.NewMockSchedulerModeFactory(ctrl)
	mockConfiger := configmocks.NewMockIConfiger(ctrl)
	mockResultSvc := svcmocks.NewMockExptResultService(ctrl)

	svc := &ExptSchedulerImpl{
		Manager:              mockManager,
		schedulerModeFactory: mockFactory,
		Configer:             mockConfiger,
		ResultSvc:            mockResultSvc,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	event := &entity.ExptScheduleEvent{ExptID: 1, SpaceID: 1, ExptRunMode: 1}
	exptDetail := &entity.Experiment{ID: 1}
	mockMode := entitymocks.NewMockExptSchedulerMode(ctrl)

	mockManager.EXPECT().GetDetail(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(exptDetail, nil)
	mockFactory.EXPECT().NewSchedulerMode(gomock.Any()).Return(mockMode, nil)
	mockMode.EXPECT().ExptStart(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	mockMode.EXPECT().ScheduleStart(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	mockMode.EXPECT().ScanEvalItems(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, nil, nil)
	mockConfiger.EXPECT().GetConsumerConf(gomock.Any()).Return(&entity.ExptConsumerConf{}).AnyTimes()
	mockMode.EXPECT().ExptEnd(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)

	err := svc.schedule(ctx, event)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded))
}

func TestExptSchedulerImpl_terminateZombieEvaluatorRecords(t *testing.T) {
	type args struct {
		ctx           context.Context
		event         *entity.ExptScheduleEvent
		zombieItemIDs []int64
	}

	tests := []struct {
		name        string
		args        args
		prepareMock func(ctrl *gomock.Controller) (*mock_repo.MockIExptTurnResultRepo, *mock_repo.MockIEvaluatorRecordRepo)
		wantErr     bool
		assertErr   func(t *testing.T, err error)
	}{
		{
			name: "empty zombieItemIDs returns nil",
			args: args{
				ctx:           context.Background(),
				event:         &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3},
				zombieItemIDs: []int64{},
			},
			prepareMock: func(ctrl *gomock.Controller) (*mock_repo.MockIExptTurnResultRepo, *mock_repo.MockIEvaluatorRecordRepo) {
				turnResultRepo := mock_repo.NewMockIExptTurnResultRepo(ctrl)
				evalRecordRepo := mock_repo.NewMockIEvaluatorRecordRepo(ctrl)
				// no calls expected
				return turnResultRepo, evalRecordRepo
			},
			wantErr: false,
		},
		{
			name: "MGetItemTurnRunLogs returns error",
			args: args{
				ctx:           context.Background(),
				event:         &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3},
				zombieItemIDs: []int64{100, 200},
			},
			prepareMock: func(ctrl *gomock.Controller) (*mock_repo.MockIExptTurnResultRepo, *mock_repo.MockIEvaluatorRecordRepo) {
				turnResultRepo := mock_repo.NewMockIExptTurnResultRepo(ctrl)
				evalRecordRepo := mock_repo.NewMockIEvaluatorRecordRepo(ctrl)
				turnResultRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), int64(1), int64(2), []int64{100, 200}, int64(3)).
					Return(nil, errors.New("db error"))
				return turnResultRepo, evalRecordRepo
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.EqualError(t, err, "db error")
			},
		},
		{
			name: "turn run logs have no evaluator result IDs returns nil",
			args: args{
				ctx:           context.Background(),
				event:         &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3},
				zombieItemIDs: []int64{100},
			},
			prepareMock: func(ctrl *gomock.Controller) (*mock_repo.MockIExptTurnResultRepo, *mock_repo.MockIEvaluatorRecordRepo) {
				turnResultRepo := mock_repo.NewMockIExptTurnResultRepo(ctrl)
				evalRecordRepo := mock_repo.NewMockIEvaluatorRecordRepo(ctrl)
				turnResultRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), int64(1), int64(2), []int64{100}, int64(3)).
					Return([]*entity.ExptTurnResultRunLog{
						nil,
						{EvaluatorResultIds: nil},
						{EvaluatorResultIds: &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{}}},
					}, nil)
				return turnResultRepo, evalRecordRepo
			},
			wantErr: false,
		},
		{
			name: "BatchGetEvaluatorRecord returns error",
			args: args{
				ctx:           context.Background(),
				event:         &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3},
				zombieItemIDs: []int64{100},
			},
			prepareMock: func(ctrl *gomock.Controller) (*mock_repo.MockIExptTurnResultRepo, *mock_repo.MockIEvaluatorRecordRepo) {
				turnResultRepo := mock_repo.NewMockIExptTurnResultRepo(ctrl)
				evalRecordRepo := mock_repo.NewMockIEvaluatorRecordRepo(ctrl)
				turnResultRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), int64(1), int64(2), []int64{100}, int64(3)).
					Return([]*entity.ExptTurnResultRunLog{
						{EvaluatorResultIds: &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{1: 1001}}},
					}, nil)
				evalRecordRepo.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), false, false).
					Return(nil, errors.New("batch get error"))
				return turnResultRepo, evalRecordRepo
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.EqualError(t, err, "batch get error")
			},
		},
		{
			name: "records with AsyncInvoking status get updated to Fail",
			args: args{
				ctx:           context.Background(),
				event:         &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3},
				zombieItemIDs: []int64{100},
			},
			prepareMock: func(ctrl *gomock.Controller) (*mock_repo.MockIExptTurnResultRepo, *mock_repo.MockIEvaluatorRecordRepo) {
				turnResultRepo := mock_repo.NewMockIExptTurnResultRepo(ctrl)
				evalRecordRepo := mock_repo.NewMockIEvaluatorRecordRepo(ctrl)
				turnResultRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), int64(1), int64(2), []int64{100}, int64(3)).
					Return([]*entity.ExptTurnResultRunLog{
						{EvaluatorResultIds: &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{1: 1001, 2: 1002}}},
					}, nil)
				evalRecordRepo.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), false, false).
					Return([]*entity.EvaluatorRecord{
						{ID: 1001, Status: entity.EvaluatorRunStatusAsyncInvoking},
						{ID: 1002, Status: entity.EvaluatorRunStatusAsyncInvoking},
					}, nil)
				evalRecordRepo.EXPECT().UpdateEvaluatorRecordResult(gomock.Any(), int64(1001), entity.EvaluatorRunStatusFail, gomock.Any()).Return(nil)
				evalRecordRepo.EXPECT().UpdateEvaluatorRecordResult(gomock.Any(), int64(1002), entity.EvaluatorRunStatusFail, gomock.Any()).Return(nil)
				return turnResultRepo, evalRecordRepo
			},
			wantErr: false,
		},
		{
			name: "records with non-AsyncInvoking status are skipped",
			args: args{
				ctx:           context.Background(),
				event:         &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3},
				zombieItemIDs: []int64{100},
			},
			prepareMock: func(ctrl *gomock.Controller) (*mock_repo.MockIExptTurnResultRepo, *mock_repo.MockIEvaluatorRecordRepo) {
				turnResultRepo := mock_repo.NewMockIExptTurnResultRepo(ctrl)
				evalRecordRepo := mock_repo.NewMockIEvaluatorRecordRepo(ctrl)
				turnResultRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), int64(1), int64(2), []int64{100}, int64(3)).
					Return([]*entity.ExptTurnResultRunLog{
						{EvaluatorResultIds: &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{1: 1001, 2: 1002, 3: 1003}}},
					}, nil)
				evalRecordRepo.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), false, false).
					Return([]*entity.EvaluatorRecord{
						{ID: 1001, Status: entity.EvaluatorRunStatusSuccess},
						{ID: 1002, Status: entity.EvaluatorRunStatusFail},
						nil,
					}, nil)
				// no UpdateEvaluatorRecordResult calls expected
				return turnResultRepo, evalRecordRepo
			},
			wantErr: false,
		},
		{
			name: "UpdateEvaluatorRecordResult error returns first error",
			args: args{
				ctx:           context.Background(),
				event:         &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3},
				zombieItemIDs: []int64{100},
			},
			prepareMock: func(ctrl *gomock.Controller) (*mock_repo.MockIExptTurnResultRepo, *mock_repo.MockIEvaluatorRecordRepo) {
				turnResultRepo := mock_repo.NewMockIExptTurnResultRepo(ctrl)
				evalRecordRepo := mock_repo.NewMockIEvaluatorRecordRepo(ctrl)
				turnResultRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), int64(1), int64(2), []int64{100}, int64(3)).
					Return([]*entity.ExptTurnResultRunLog{
						{EvaluatorResultIds: &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{1: 1001, 2: 1002}}},
					}, nil)
				evalRecordRepo.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), false, false).
					Return([]*entity.EvaluatorRecord{
						{ID: 1001, Status: entity.EvaluatorRunStatusAsyncInvoking},
						{ID: 1002, Status: entity.EvaluatorRunStatusAsyncInvoking},
					}, nil)
				evalRecordRepo.EXPECT().UpdateEvaluatorRecordResult(gomock.Any(), int64(1001), entity.EvaluatorRunStatusFail, gomock.Any()).
					Return(errors.New("update error 1"))
				evalRecordRepo.EXPECT().UpdateEvaluatorRecordResult(gomock.Any(), int64(1002), entity.EvaluatorRunStatusFail, gomock.Any()).
					Return(errors.New("update error 2"))
				return turnResultRepo, evalRecordRepo
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.EqualError(t, err, "update error 1")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			turnResultRepo, evalRecordRepo := tt.prepareMock(ctrl)

			svc := &ExptSchedulerImpl{
				ExptTurnResultRepo:  turnResultRepo,
				EvaluatorRecordRepo: evalRecordRepo,
			}

			err := svc.terminateZombieEvaluatorRecords(tt.args.ctx, tt.args.event, tt.args.zombieItemIDs)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.assertErr != nil {
					tt.assertErr(t, err)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIsSchedulerInfraError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "context.Canceled",
			err:  context.Canceled,
			want: true,
		},
		{
			name: "context.DeadlineExceeded",
			err:  context.DeadlineExceeded,
			want: true,
		},
		{
			name: "wrapped context.Canceled",
			err:  errors.Join(errors.New("outer"), context.Canceled),
			want: true,
		},
		{
			name: "send batch message fail",
			err:  errors.New("send batch message fail, producer_key: expt_scheduler_event_rmq"),
			want: true,
		},
		{
			name: "rpc context canceled string",
			err:  errors.New("rpc error: code = Canceled desc = context canceled"),
			want: true,
		},
		{
			name: "rpc context deadline exceeded string",
			err:  errors.New("rpc error: code = DeadlineExceeded desc = context deadline exceeded"),
			want: true,
		},
		{
			name: "business error",
			err:  errors.New("expt exec found timeout event"),
			want: false,
		},
		{
			name: "other error",
			err:  errors.New("some unknown error"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isSchedulerInfraError(tt.err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExptSchedulerImpl_HandleEventErr(t *testing.T) {
	tests := []struct {
		name        string
		event       *entity.ExptScheduleEvent
		nextErr     error
		prepareMock func(f *handleEventErrFields)
		wantErr     bool
		assertEvent func(t *testing.T, event *entity.ExptScheduleEvent)
	}{
		{
			name: "next returns nil - success path",
			event: &entity.ExptScheduleEvent{
				ExptID: 1, ExptRunID: 2, SpaceID: 3,
				Session: &entity.Session{UserID: "user1"},
			},
			nextErr:     nil,
			prepareMock: func(f *handleEventErrFields) {},
			wantErr:     false,
		},
		{
			name: "infra error - reschedule success",
			event: &entity.ExptScheduleEvent{
				ExptID: 1, ExptRunID: 2, SpaceID: 3,
				InfraErrorRetryTimes: 0,
				Session:              &entity.Session{UserID: "user1"},
			},
			nextErr: context.Canceled,
			prepareMock: func(f *handleEventErrFields) {
				f.publisher.EXPECT().PublishExptScheduleEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			wantErr: false,
			assertEvent: func(t *testing.T, event *entity.ExptScheduleEvent) {
				assert.Equal(t, 1, event.InfraErrorRetryTimes)
			},
		},
		{
			name: "infra error - reschedule publish fails, return error for MQ retry",
			event: &entity.ExptScheduleEvent{
				ExptID: 1, ExptRunID: 2, SpaceID: 3,
				InfraErrorRetryTimes: 2,
				Session:              &entity.Session{UserID: "user1"},
			},
			nextErr: errors.New("send batch message fail, producer_key: expt_scheduler_event_rmq"),
			prepareMock: func(f *handleEventErrFields) {
				f.publisher.EXPECT().PublishExptScheduleEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("mq down")).Times(1)
			},
			wantErr: true,
			assertEvent: func(t *testing.T, event *entity.ExptScheduleEvent) {
				assert.Equal(t, 3, event.InfraErrorRetryTimes)
			},
		},
		{
			name: "infra error - retry exhausted, terminate experiment",
			event: &entity.ExptScheduleEvent{
				ExptID: 1, ExptRunID: 2, SpaceID: 3,
				InfraErrorRetryTimes: 10,
				Session:              &entity.Session{UserID: "user1"},
			},
			nextErr: context.Canceled,
			prepareMock: func(f *handleEventErrFields) {
				f.manager.EXPECT().CompleteRun(gomock.Any(), int64(1), int64(2), int64(3), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
				f.manager.EXPECT().CompleteExpt(gomock.Any(), int64(1), gomock.Any(), int64(3), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			wantErr: false,
		},
		{
			name: "business error - terminate experiment",
			event: &entity.ExptScheduleEvent{
				ExptID: 1, ExptRunID: 2, SpaceID: 3,
				Session: &entity.Session{UserID: "user1"},
			},
			nextErr: errors.New("expt exec found timeout event"),
			prepareMock: func(f *handleEventErrFields) {
				f.manager.EXPECT().CompleteRun(gomock.Any(), int64(1), int64(2), int64(3), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
				f.manager.EXPECT().CompleteExpt(gomock.Any(), int64(1), gomock.Any(), int64(3), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			f := &handleEventErrFields{
				manager:   svcmocks.NewMockIExptManager(ctrl),
				publisher: eventmocks.NewMockExptEventPublisher(ctrl),
			}
			tt.prepareMock(f)

			svc := &ExptSchedulerImpl{
				Manager:   f.manager,
				Publisher: f.publisher,
			}

			next := func(ctx context.Context, event *entity.ExptScheduleEvent) error {
				return tt.nextErr
			}

			handler := svc.HandleEventErr(next)
			err := handler(context.Background(), tt.event)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			if tt.assertEvent != nil {
				tt.assertEvent(t, tt.event)
			}
		})
	}
}

type handleEventErrFields struct {
	manager   *svcmocks.MockIExptManager
	publisher *eventmocks.MockExptEventPublisher
}

// TestUserVisibleErrMsg 覆盖 HandleEventErr 里 err → 用户可见 msg 的三个分支：
// - nil 返回空
// - ErrImpl 返回 Msg 字段（用户友好中文描述）
// - 普通 error 回退到 Error() 字符串
func TestUserVisibleErrMsg(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		assert.Equal(t, "", userVisibleErrMsg(nil))
	})

	t.Run("ErrImpl 走 Msg 字段", func(t *testing.T) {
		err := errno.NewExptZombieTimeoutErr(60, 111, 222)
		got := userVisibleErrMsg(err)
		assert.Contains(t, got, "60s")
		assert.Contains(t, got, "expt_id=111")
		// 明确不应回退到 error 的 Error() 输出（那种输出前缀是 ErrMsg=...）
		assert.NotContains(t, got, "ErrMsg=")
	})

	t.Run("ErrImpl 无 Msg 时回退到 Error", func(t *testing.T) {
		err := errno.WrapMQRetryErr(errors.New("boom"))
		got := userVisibleErrMsg(err)
		// Msg 为空 → fallback 到 err.Error()（会带 Cause=...）
		assert.Contains(t, got, "boom")
	})

	t.Run("普通 error 回退到 Error", func(t *testing.T) {
		err := errors.New("plain error")
		assert.Equal(t, "plain error", userVisibleErrMsg(err))
	})
}

// TestExptSchedulerImpl_sweepTerminatedSandboxItems 覆盖 sandbox-status 巡检的分支:
// - expt==nil / target 非 SandboxAgent → 直接返回原 items, 不发 RPC
// - 无 Processing item → 直接返回
// - MGetItemTurnRunLogs 出错 → 冒泡
// - CheckSandboxTerminated 无命中 → 全部走 alives
// - 有命中 → 分区正确 + 写库 3 次 + 触发 TerminateAsyncRecordsAndDestroySandbox(zombieTimeout=false)
func TestExptSchedulerImpl_sweepTerminatedSandboxItems(t *testing.T) {
	sandboxExpt := func() *entity.Experiment {
		return &entity.Experiment{
			ID: 1,
			Target: &entity.EvalTarget{
				EvalTargetVersion: &entity.EvalTargetVersion{
					EvalTargetType: entity.EvalTargetTypeSandboxAgent,
				},
			},
		}
	}
	nonSandboxExpt := func() *entity.Experiment {
		return &entity.Experiment{
			ID: 1,
			Target: &entity.EvalTarget{
				EvalTargetVersion: &entity.EvalTargetVersion{
					EvalTargetType: entity.EvalTargetTypeCozeBot,
				},
			},
		}
	}
	baseEvent := &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3}

	t.Run("expt=nil 不走 RPC", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc := &ExptSchedulerImpl{evalTargetService: svcmocks.NewMockIEvalTargetService(ctrl)}
		items := []*entity.ExptEvalItem{{ItemID: 10, State: entity.ItemRunState_Processing}}
		alives, terminated, err := svc.sweepTerminatedSandboxItems(context.Background(), baseEvent, items, nil)
		assert.NoError(t, err)
		assert.Len(t, alives, 1)
		assert.Empty(t, terminated)
	})

	t.Run("非 SandboxAgent 不走 RPC", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc := &ExptSchedulerImpl{evalTargetService: svcmocks.NewMockIEvalTargetService(ctrl)}
		items := []*entity.ExptEvalItem{{ItemID: 10, State: entity.ItemRunState_Processing}}
		alives, terminated, err := svc.sweepTerminatedSandboxItems(context.Background(), baseEvent, items, nonSandboxExpt())
		assert.NoError(t, err)
		assert.Len(t, alives, 1)
		assert.Empty(t, terminated)
	})

	t.Run("无 Processing item 直接返回", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc := &ExptSchedulerImpl{evalTargetService: svcmocks.NewMockIEvalTargetService(ctrl)}
		items := []*entity.ExptEvalItem{{ItemID: 10, State: entity.ItemRunState_Success}}
		alives, terminated, err := svc.sweepTerminatedSandboxItems(context.Background(), baseEvent, items, sandboxExpt())
		assert.NoError(t, err)
		assert.Len(t, alives, 1)
		assert.Empty(t, terminated)
	})

	t.Run("MGetItemTurnRunLogs 出错冒泡", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockTurnRepo := mock_repo.NewMockIExptTurnResultRepo(ctrl)
		svc := &ExptSchedulerImpl{
			ExptTurnResultRepo: mockTurnRepo,
			evalTargetService:  svcmocks.NewMockIEvalTargetService(ctrl),
		}
		mockTurnRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), int64(1), int64(2), []int64{10}, int64(3)).
			Return(nil, errors.New("db err"))
		items := []*entity.ExptEvalItem{{ItemID: 10, State: entity.ItemRunState_Processing}}
		alives, terminated, err := svc.sweepTerminatedSandboxItems(context.Background(), baseEvent, items, sandboxExpt())
		assert.Error(t, err)
		assert.Nil(t, terminated)
		_ = alives
	})

	t.Run("CheckSandboxTerminated 无命中 → 全部 alives", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockTurnRepo := mock_repo.NewMockIExptTurnResultRepo(ctrl)
		mockTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
		svc := &ExptSchedulerImpl{ExptTurnResultRepo: mockTurnRepo, evalTargetService: mockTargetSvc}

		mockTurnRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), int64(1), int64(2), []int64{10}, int64(3)).
			Return([]*entity.ExptTurnResultRunLog{{ItemID: 10, TargetResultID: 500}}, nil)
		mockTargetSvc.EXPECT().CheckSandboxTerminated(gomock.Any(), int64(3), []int64{500}).
			Return(nil, nil)

		items := []*entity.ExptEvalItem{{ItemID: 10, State: entity.ItemRunState_Processing}}
		alives, terminated, err := svc.sweepTerminatedSandboxItems(context.Background(), baseEvent, items, sandboxExpt())
		assert.NoError(t, err)
		assert.Len(t, alives, 1)
		assert.Empty(t, terminated)
	})

	t.Run("命中: 分区 + 写库 + Destroy 触发", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockTurnRepo := mock_repo.NewMockIExptTurnResultRepo(ctrl)
		mockItemRepo := mock_repo.NewMockIExptItemResultRepo(ctrl)
		mockTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
		svc := &ExptSchedulerImpl{
			ExptTurnResultRepo: mockTurnRepo,
			ExptItemResultRepo: mockItemRepo,
			evalTargetService:  mockTargetSvc,
		}

		// item 10 关联 record 500 (命中), item 11 关联 record 501 (未命中)
		mockTurnRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), int64(1), int64(2), []int64{10, 11}, int64(3)).
			Return([]*entity.ExptTurnResultRunLog{
				{ItemID: 10, TargetResultID: 500},
				{ItemID: 11, TargetResultID: 501},
			}, nil)
		mockTargetSvc.EXPECT().CheckSandboxTerminated(gomock.Any(), int64(3), gomock.Any()).
			DoAndReturn(func(_ context.Context, _ int64, rids []int64) ([]int64, map[int64]string) {
				assert.ElementsMatch(t, []int64{500, 501}, rids)
				return []int64{500}, map[int64]string{500: "Failed"}
			})

		// terminate 调用
		mockTargetSvc.EXPECT().TerminateAsyncRecordsAndDestroySandbox(
			gomock.Any(), int64(3), []int64{500}, int32(errno.SandboxTerminatedBeforeReportCode),
			gomock.Any(), false, // zombieTimeout=false
		).Times(1)

		// 三次写库
		mockItemRepo.EXPECT().UpdateItemRunLog(gomock.Any(), int64(1), int64(2), []int64{10}, gomock.Any(), int64(3)).
			DoAndReturn(func(_ context.Context, _, _ int64, _ []int64, ufields map[string]any, _ int64) error {
				assert.Equal(t, int32(entity.ItemRunState_Fail), ufields["status"])
				assert.NotEmpty(t, ufields["err_msg"])
				return nil
			})
		mockItemRepo.EXPECT().UpdateItemsResult(gomock.Any(), int64(3), int64(1), []int64{10}, gomock.Any()).
			DoAndReturn(func(_ context.Context, _, _ int64, _ []int64, ufields map[string]any) error {
				// 主表只补 err_msg, 不预写 status=Fail; status 由后续 RecordItemRunLogs
				// 从 itemRunLog.Status 统一写入, 保证 statsCntOp 里 Processing 的减项不丢.
				_, hasStatus := ufields["status"]
				assert.False(t, hasStatus, "UpdateItemsResult must not pre-write status on sweep-terminated path")
				assert.NotEmpty(t, ufields["err_msg"])
				return nil
			})
		mockTurnRepo.EXPECT().CreateOrUpdateItemsTurnRunLogStatus(gomock.Any(), int64(3), int64(1), int64(2), []int64{10}, entity.TurnRunState_Fail).
			Return(nil)

		items := []*entity.ExptEvalItem{
			{ItemID: 10, State: entity.ItemRunState_Processing},
			{ItemID: 11, State: entity.ItemRunState_Processing},
		}
		alives, terminated, err := svc.sweepTerminatedSandboxItems(context.Background(), baseEvent, items, sandboxExpt())
		assert.NoError(t, err)
		assert.Len(t, alives, 1)
		assert.Equal(t, int64(11), alives[0].ItemID)
		assert.Len(t, terminated, 1)
		assert.Equal(t, int64(10), terminated[0].ItemID)
		assert.Equal(t, entity.ItemRunState_Fail, terminated[0].State)
	})

	// 打点回归: 命中 sweep 时每个 record 补一次 EmitInvokeFinished，
	// tag 与提交侧 emitInvokeStarted 对齐 (invoke_id = record.ID, target/dataset 来自 expt)。
	t.Run("命中: EmitInvokeFinished 打点 + tag 校验", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockTurnRepo := mock_repo.NewMockIExptTurnResultRepo(ctrl)
		mockItemRepo := mock_repo.NewMockIExptItemResultRepo(ctrl)
		mockTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
		mockSandboxMetric := metricsmocks.NewMockSandboxAgentMetrics(ctrl)
		svc := &ExptSchedulerImpl{
			ExptTurnResultRepo:  mockTurnRepo,
			ExptItemResultRepo:  mockItemRepo,
			evalTargetService:   mockTargetSvc,
			sandboxAgentMetrics: mockSandboxMetric,
		}

		mockTurnRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), int64(1), int64(2), []int64{10}, int64(3)).
			Return([]*entity.ExptTurnResultRunLog{{ItemID: 10, TargetResultID: 500}}, nil)
		mockTargetSvc.EXPECT().CheckSandboxTerminated(gomock.Any(), int64(3), gomock.Any()).
			Return([]int64{500}, map[int64]string{500: "Failed"})
		mockTargetSvc.EXPECT().TerminateAsyncRecordsAndDestroySandbox(
			gomock.Any(), int64(3), []int64{500}, int32(errno.SandboxTerminatedBeforeReportCode),
			gomock.Any(), false,
		).Times(1)
		mockItemRepo.EXPECT().UpdateItemRunLog(gomock.Any(), int64(1), int64(2), []int64{10}, gomock.Any(), int64(3)).Return(nil)
		mockItemRepo.EXPECT().UpdateItemsResult(gomock.Any(), int64(3), int64(1), []int64{10}, gomock.Any()).Return(nil)
		mockTurnRepo.EXPECT().CreateOrUpdateItemsTurnRunLogStatus(gomock.Any(), int64(3), int64(1), int64(2), []int64{10}, entity.TurnRunState_Fail).Return(nil)

		// 关键: EmitInvokeFinished 必被调用一次, tag 值需与 expt / event / record 对齐
		mockSandboxMetric.EXPECT().EmitInvokeFinished(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(tags metrics.SandboxAgentInvokeTags, err error, errCode int32, _ time.Time) {
				assert.Equal(t, int64(1), tags.ExperimentID)
				assert.Equal(t, int64(10), tags.ItemID)
				assert.Equal(t, "500", tags.InvokeID)
				assert.Equal(t, int64(77), tags.DatasetID)
				assert.Equal(t, int64(88), tags.DatasetVersion)
				assert.Equal(t, int64(99), tags.TargetID)
				assert.Equal(t, int32(errno.SandboxTerminatedBeforeReportCode), errCode)
				assert.Error(t, err) // classifier 分支需要 err != nil
				// sweep 路径新增 tag: 从 event / expt.Target 反查填充。
				assert.Equal(t, int64(3), tags.SpaceID)
				assert.Equal(t, int64(2), tags.ExperimentRunID)
				assert.Equal(t, "sweep-agent", tags.AgentName)
				assert.Equal(t, "app-sweep", tags.ApplicationID)
			}).Times(1)

		expt := &entity.Experiment{
			ID:       1,
			TargetID: 99,
			Target: &entity.EvalTarget{
				SourceTargetID: "app-sweep",
				EvalTargetVersion: &entity.EvalTargetVersion{
					EvalTargetType: entity.EvalTargetTypeSandboxAgent,
					SandboxAgent:   &entity.SandboxAgent{Name: "sweep-agent"},
				},
			},
			EvalSet: &entity.EvaluationSet{
				ID:                   77,
				EvaluationSetVersion: &entity.EvaluationSetVersion{ID: 88},
			},
		}
		items := []*entity.ExptEvalItem{{ItemID: 10, State: entity.ItemRunState_Processing}}
		_, terminated, err := svc.sweepTerminatedSandboxItems(context.Background(), baseEvent, items, expt)
		assert.NoError(t, err)
		assert.Len(t, terminated, 1)
	})

	// sandboxAgentMetrics 未注入时 sweep 仍然工作 (打点静默 no-op)
	t.Run("命中: 未注入 metric 打点静默跳过", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockTurnRepo := mock_repo.NewMockIExptTurnResultRepo(ctrl)
		mockItemRepo := mock_repo.NewMockIExptItemResultRepo(ctrl)
		mockTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
		svc := &ExptSchedulerImpl{
			ExptTurnResultRepo: mockTurnRepo,
			ExptItemResultRepo: mockItemRepo,
			evalTargetService:  mockTargetSvc,
			// sandboxAgentMetrics 显式留空
		}
		mockTurnRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), int64(1), int64(2), []int64{10}, int64(3)).
			Return([]*entity.ExptTurnResultRunLog{{ItemID: 10, TargetResultID: 500}}, nil)
		mockTargetSvc.EXPECT().CheckSandboxTerminated(gomock.Any(), int64(3), gomock.Any()).
			Return([]int64{500}, map[int64]string{500: "Failed"})
		mockTargetSvc.EXPECT().TerminateAsyncRecordsAndDestroySandbox(
			gomock.Any(), int64(3), gomock.Any(), gomock.Any(), gomock.Any(), false,
		).Times(1)
		mockItemRepo.EXPECT().UpdateItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		mockItemRepo.EXPECT().UpdateItemsResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		mockTurnRepo.EXPECT().CreateOrUpdateItemsTurnRunLogStatus(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

		items := []*entity.ExptEvalItem{{ItemID: 10, State: entity.ItemRunState_Processing}}
		_, terminated, err := svc.sweepTerminatedSandboxItems(context.Background(), baseEvent, items, sandboxExpt())
		assert.NoError(t, err)
		assert.Len(t, terminated, 1)
	})
}

// TestExptSchedulerImpl_terminateZombieEvalTargetRecords_Metric 覆盖 zombie 兜底路径的打点分支:
// - SandboxAgent 类型 expt → EmitInvokeFinished 被调, tag 与 sweep 路径一致但 errCode 使用 zombie code
// - 非 SandboxAgent expt (含 nil) → 不打点, 保护 sandbox_agent 看板
// - sandboxAgentMetrics 未注入 → 打点静默 no-op
func TestExptSchedulerImpl_terminateZombieEvalTargetRecords_Metric(t *testing.T) {
	baseEvent := &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3}
	sandboxExpt := &entity.Experiment{
		ID:       1,
		TargetID: 99,
		Target: &entity.EvalTarget{
			SourceTargetID: "app-zombie",
			EvalTargetVersion: &entity.EvalTargetVersion{
				EvalTargetType: entity.EvalTargetTypeSandboxAgent,
				SandboxAgent:   &entity.SandboxAgent{Name: "zombie-agent"},
			},
		},
		EvalSet: &entity.EvaluationSet{
			ID:                   77,
			EvaluationSetVersion: &entity.EvaluationSetVersion{ID: 88},
		},
	}
	nonSandboxExpt := &entity.Experiment{
		Target: &entity.EvalTarget{
			EvalTargetVersion: &entity.EvalTargetVersion{EvalTargetType: entity.EvalTargetTypeCozeBot},
		},
	}

	t.Run("SandboxAgent zombie → 打点 + tag/errCode 校验", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockTurnRepo := mock_repo.NewMockIExptTurnResultRepo(ctrl)
		mockTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
		mockSandboxMetric := metricsmocks.NewMockSandboxAgentMetrics(ctrl)
		svc := &ExptSchedulerImpl{
			ExptTurnResultRepo:  mockTurnRepo,
			evalTargetService:   mockTargetSvc,
			sandboxAgentMetrics: mockSandboxMetric,
		}

		mockTurnRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), int64(1), int64(2), []int64{10}, int64(3)).
			Return([]*entity.ExptTurnResultRunLog{{ItemID: 10, TargetResultID: 500}}, nil)
		mockTargetSvc.EXPECT().TerminateAsyncRecordsAndDestroySandbox(
			gomock.Any(), int64(3), []int64{500}, int32(errno.AsyncEvalTargetZombieTimeoutCode),
			gomock.Any(), true, // zombieTimeout=true
		).Times(1)
		mockSandboxMetric.EXPECT().EmitInvokeFinished(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(tags metrics.SandboxAgentInvokeTags, err error, errCode int32, _ time.Time) {
				assert.Equal(t, int64(1), tags.ExperimentID)
				assert.Equal(t, int64(10), tags.ItemID)
				assert.Equal(t, "500", tags.InvokeID)
				assert.Equal(t, int64(77), tags.DatasetID)
				assert.Equal(t, int64(88), tags.DatasetVersion)
				assert.Equal(t, int64(99), tags.TargetID)
				assert.Equal(t, int32(errno.AsyncEvalTargetZombieTimeoutCode), errCode)
				assert.Error(t, err)
				// zombie 兜底路径同样携带四个新 tag。
				assert.Equal(t, int64(3), tags.SpaceID)
				assert.Equal(t, int64(2), tags.ExperimentRunID)
				assert.Equal(t, "zombie-agent", tags.AgentName)
				assert.Equal(t, "app-zombie", tags.ApplicationID)
			}).Times(1)

		err := svc.terminateZombieEvalTargetRecords(context.Background(), baseEvent, sandboxExpt, []int64{10})
		assert.NoError(t, err)
	})

	t.Run("非 SandboxAgent zombie → 不打点 (terminate 仍调用, 内部会 no-op)", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockTurnRepo := mock_repo.NewMockIExptTurnResultRepo(ctrl)
		mockTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
		mockSandboxMetric := metricsmocks.NewMockSandboxAgentMetrics(ctrl)
		svc := &ExptSchedulerImpl{
			ExptTurnResultRepo:  mockTurnRepo,
			evalTargetService:   mockTargetSvc,
			sandboxAgentMetrics: mockSandboxMetric,
		}
		mockTurnRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), int64(1), int64(2), []int64{10}, int64(3)).
			Return([]*entity.ExptTurnResultRunLog{{ItemID: 10, TargetResultID: 500}}, nil)
		mockTargetSvc.EXPECT().TerminateAsyncRecordsAndDestroySandbox(
			gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
		).Times(1)
		// EmitInvokeFinished 断言 Times(0) —— 非 sandbox 类型不该打点

		err := svc.terminateZombieEvalTargetRecords(context.Background(), baseEvent, nonSandboxExpt, []int64{10})
		assert.NoError(t, err)
	})

	t.Run("nil expt zombie → 不打点 (向后兼容旧调用)", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockTurnRepo := mock_repo.NewMockIExptTurnResultRepo(ctrl)
		mockTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
		mockSandboxMetric := metricsmocks.NewMockSandboxAgentMetrics(ctrl)
		svc := &ExptSchedulerImpl{
			ExptTurnResultRepo:  mockTurnRepo,
			evalTargetService:   mockTargetSvc,
			sandboxAgentMetrics: mockSandboxMetric,
		}
		mockTurnRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), int64(1), int64(2), []int64{10}, int64(3)).
			Return([]*entity.ExptTurnResultRunLog{{ItemID: 10, TargetResultID: 500}}, nil)
		mockTargetSvc.EXPECT().TerminateAsyncRecordsAndDestroySandbox(
			gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
		).Times(1)

		err := svc.terminateZombieEvalTargetRecords(context.Background(), baseEvent, nil, []int64{10})
		assert.NoError(t, err)
	})

	t.Run("SandboxAgent zombie 但 metric 未注入 → terminate 仍调用, 打点静默", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockTurnRepo := mock_repo.NewMockIExptTurnResultRepo(ctrl)
		mockTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
		svc := &ExptSchedulerImpl{
			ExptTurnResultRepo: mockTurnRepo,
			evalTargetService:  mockTargetSvc,
			// sandboxAgentMetrics 显式留空
		}
		mockTurnRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), int64(1), int64(2), []int64{10}, int64(3)).
			Return([]*entity.ExptTurnResultRunLog{{ItemID: 10, TargetResultID: 500}}, nil)
		mockTargetSvc.EXPECT().TerminateAsyncRecordsAndDestroySandbox(
			gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
		).Times(1)

		err := svc.terminateZombieEvalTargetRecords(context.Background(), baseEvent, sandboxExpt, []int64{10})
		assert.NoError(t, err)
	})
}

// TestIsSandboxAgentExpt 覆盖 helper 的所有分支。
func TestIsSandboxAgentExpt(t *testing.T) {
	assert.False(t, isSandboxAgentExpt(nil))
	assert.False(t, isSandboxAgentExpt(&entity.Experiment{}))
	assert.False(t, isSandboxAgentExpt(&entity.Experiment{Target: &entity.EvalTarget{}}))
	assert.False(t, isSandboxAgentExpt(&entity.Experiment{Target: &entity.EvalTarget{
		EvalTargetVersion: &entity.EvalTargetVersion{EvalTargetType: entity.EvalTargetTypeCozeBot},
	}}))
	assert.True(t, isSandboxAgentExpt(&entity.Experiment{Target: &entity.EvalTarget{
		EvalTargetVersion: &entity.EvalTargetVersion{EvalTargetType: entity.EvalTargetTypeSandboxAgent},
	}}))
}

// Test_resolveItemCompleteMeta 覆盖 item-complete 组装前的批量补集/版本:
// 单集(走主集) / 多集(走 expt_item_ref) / ref 缺失跳过 / 主集缺失 / BatchGet 失败 / 无 publisher 不调用。
func Test_resolveItemCompleteMeta(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	baseExpt := func(multiset bool) *entity.Experiment {
		e := &entity.Experiment{
			EvalSet:        &entity.EvaluationSet{ID: 70, EvaluationSetVersion: &entity.EvaluationSetVersion{ID: 80}},
			EvalSetSpaceID: 0,
		}
		if multiset {
			e.EvalSetSourceType = entity.ExptEvalSetSourceType_MultiSetConfig
		}
		return e
	}
	event := &entity.ExptScheduleEvent{SpaceID: 9, ExptID: 100, ExptRunID: 200}
	items := []*entity.ExptEvalItem{{ItemID: 11}, {ItemID: 12}}

	t.Run("single set - primary", func(t *testing.T) {
		refRepo := mock_repo.NewMockIExptItemRefRepo(ctrl)
		setItemSvc := svcmocks.NewMockEvaluationSetItemService(ctrl)
		setItemSvc.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).
			Return([]*entity.EvaluationSetItem{{ItemID: 11, ItemKey: "k11", EvaluationSetID: 70}, {ItemID: 12, ItemKey: "k12", EvaluationSetID: 70}}, nil)
		svc := &ExptSchedulerImpl{exptItemRefRepo: refRepo, evaluationSetItemService: setItemSvc}
		meta, ver := svc.resolveItemCompleteMeta(context.Background(), event, items, baseExpt(false))
		assert.Equal(t, "k11", meta[11].ItemKey)
		assert.Equal(t, int64(80), ver[11]) // 单集: 主集版本
		assert.Equal(t, int64(80), ver[12])
	})

	t.Run("multiset - per-item ref version", func(t *testing.T) {
		refRepo := mock_repo.NewMockIExptItemRefRepo(ctrl)
		refRepo.EXPECT().MGetByExptIDAndItemIDs(gomock.Any(), int64(9), int64(100), []int64{11, 12}).
			Return([]*entity.ExptItemRef{
				{ItemID: 11, EvalSetID: 71, EvalSetVersionID: 81},
				{ItemID: 12, EvalSetID: 72, EvalSetVersionID: 82},
			}, nil)
		setItemSvc := svcmocks.NewMockEvaluationSetItemService(ctrl)
		setItemSvc.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, p *entity.BatchGetEvaluationSetItemsParam) ([]*entity.EvaluationSetItem, error) {
				return []*entity.EvaluationSetItem{{ItemID: p.ItemIDs[0], EvaluationSetID: p.EvaluationSetID}}, nil
			}).Times(2) // 两个 item 属不同集, 分两组各查一次
		svc := &ExptSchedulerImpl{exptItemRefRepo: refRepo, evaluationSetItemService: setItemSvc}
		_, ver := svc.resolveItemCompleteMeta(context.Background(), event, items, baseExpt(true))
		assert.Equal(t, int64(81), ver[11]) // 多集: 各 item 归属集版本
		assert.Equal(t, int64(82), ver[12])
	})

	t.Run("multiset - ref missing skip item", func(t *testing.T) {
		refRepo := mock_repo.NewMockIExptItemRefRepo(ctrl)
		refRepo.EXPECT().MGetByExptIDAndItemIDs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]*entity.ExptItemRef{{ItemID: 11, EvalSetID: 71, EvalSetVersionID: 81}}, nil) // 缺 item 12
		setItemSvc := svcmocks.NewMockEvaluationSetItemService(ctrl)
		setItemSvc.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).
			Return([]*entity.EvaluationSetItem{{ItemID: 11}}, nil)
		svc := &ExptSchedulerImpl{exptItemRefRepo: refRepo, evaluationSetItemService: setItemSvc}
		_, ver := svc.resolveItemCompleteMeta(context.Background(), event, items, baseExpt(true))
		assert.Equal(t, int64(81), ver[11])
		_, ok := ver[12]
		assert.False(t, ok) // 12 无 ref 被跳过
	})

	t.Run("multiset - MGet fail returns empty", func(t *testing.T) {
		refRepo := mock_repo.NewMockIExptItemRefRepo(ctrl)
		refRepo.EXPECT().MGetByExptIDAndItemIDs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, assert.AnError)
		setItemSvc := svcmocks.NewMockEvaluationSetItemService(ctrl)
		svc := &ExptSchedulerImpl{exptItemRefRepo: refRepo, evaluationSetItemService: setItemSvc}
		meta, ver := svc.resolveItemCompleteMeta(context.Background(), event, items, baseExpt(true))
		assert.Empty(t, meta)
		assert.Empty(t, ver)
	})

	t.Run("single set - primary set missing returns empty", func(t *testing.T) {
		refRepo := mock_repo.NewMockIExptItemRefRepo(ctrl)
		setItemSvc := svcmocks.NewMockEvaluationSetItemService(ctrl)
		expt := baseExpt(false)
		expt.EvalSet = nil // 主集缺失
		svc := &ExptSchedulerImpl{exptItemRefRepo: refRepo, evaluationSetItemService: setItemSvc}
		meta, _ := svc.resolveItemCompleteMeta(context.Background(), event, items, expt)
		assert.Empty(t, meta)
	})

	t.Run("BatchGet fail - group skipped, ver still set", func(t *testing.T) {
		refRepo := mock_repo.NewMockIExptItemRefRepo(ctrl)
		setItemSvc := svcmocks.NewMockEvaluationSetItemService(ctrl)
		setItemSvc.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
		svc := &ExptSchedulerImpl{exptItemRefRepo: refRepo, evaluationSetItemService: setItemSvc}
		meta, ver := svc.resolveItemCompleteMeta(context.Background(), event, items, baseExpt(false))
		assert.Empty(t, meta)               // BatchGet 失败, meta 空
		assert.Equal(t, int64(80), ver[11]) // ver 在分组时已填, 不受影响
	})
}

// Test_findEvalSetForItem 覆盖归属集查找: nil / 单集主集 / 多集命中 EvalSetDetails / 多集未命中不回退。
func Test_findEvalSetForItem(t *testing.T) {
	assert.Nil(t, findEvalSetForItem(nil, 1))

	single := &entity.Experiment{EvalSet: &entity.EvaluationSet{ID: 70}}
	assert.Equal(t, int64(70), findEvalSetForItem(single, 70).ID) // 单集命中
	assert.Equal(t, int64(70), findEvalSetForItem(single, 0).ID)  // datasetID=0 用主集
	assert.Nil(t, findEvalSetForItem(single, 999))                // 单集但 id 不匹配

	multi := &entity.Experiment{
		EvalSetSourceType: entity.ExptEvalSetSourceType_MultiSetConfig,
		EvalSetDetails: []*entity.ExptEvalSetDetail{
			{EvalSetID: 71, EvalSet: &entity.EvaluationSet{ID: 71}},
		},
	}
	assert.Equal(t, int64(71), findEvalSetForItem(multi, 71).ID) // 多集命中
	assert.Nil(t, findEvalSetForItem(multi, 72))                 // 多集未命中, 不回退主集
}

// Test_sendItemComplete_nilMeta 覆盖 evalSetItem==nil 时 sendItemComplete 直接跳过、不 publish、返回 nil(不阻断调度)。
func Test_sendItemComplete_nilMeta(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	stub := &stubItemCompletePublisher{}
	svc := &ExptSchedulerImpl{itemCompletePublisher: stub}
	event := &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3}
	item := &entity.ExptEvalItem{ItemID: 10, State: entity.ItemRunState_Success}
	err := svc.sendItemComplete(context.Background(), event, &entity.Experiment{}, item, nil /*evalSetItem*/, 100)
	assert.NoError(t, err)       // meta nil 属组装侧问题, 跳过不阻断, 返回 nil
	assert.Empty(t, stub.events) // 未 publish
}

// Test_sendItemComplete_publish 覆盖 publish 成功(返回 nil) / 失败(返回 error, 由调用方中断本次调度靠下次调度补发) 两条路径。
func Test_sendItemComplete_publish(t *testing.T) {
	event := &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3}
	item := &entity.ExptEvalItem{ItemID: 10, State: entity.ItemRunState_Success}
	evalSetItem := &entity.EvaluationSetItem{ItemID: 10, ItemKey: "k", EvaluationSetID: 70}

	t.Run("publish ok", func(t *testing.T) {
		stub := &stubItemCompletePublisher{}
		svc := &ExptSchedulerImpl{itemCompletePublisher: stub}
		err := svc.sendItemComplete(context.Background(), event, &entity.Experiment{}, item, evalSetItem, 80)
		assert.NoError(t, err)        // 发送成功返回 nil
		assert.Len(t, stub.events, 1) // 组装并发送一次
	})

	t.Run("publish fail returns error", func(t *testing.T) {
		stub := &stubItemCompletePublisher{err: assert.AnError}
		svc := &ExptSchedulerImpl{itemCompletePublisher: stub}
		// 发失败返回 error, 由调用方中断本次调度, 下次调度重扫 completeItems 补发
		err := svc.sendItemComplete(context.Background(), event, &entity.Experiment{}, item, evalSetItem, 80)
		assert.Error(t, err)
		assert.Len(t, stub.events, 1) // 仍尝试发送一次
	})
}

// Test_recordEvalItemRunLogs_publishFailInterrupts 锁定本次 fix 的核心不变量:
// item-complete(success) 发送失败时, recordEvalItemRunLogs 循环必须在落库之前 return err 中断本次调度,
// 使 item 停留在 complete(未推进到 Resulted), 下次调度重扫 completeItems 补发。
// 因此断言发失败时 RecordItemRunLogs(落库/推进状态) 调用 0 次; 对照发成功时正常落库 1 次。
// 防回归: 未来若把 return err 改成 continue、或调换"先发送后落库"顺序, 本用例会失败。
func Test_recordEvalItemRunLogs_publishFailInterrupts(t *testing.T) {
	// 单集实验, resolveItemCompleteMeta 走主集路径, 需 evaluationSetItemService 返回该 item 的 meta(非 nil), 才会真正触发发送。
	newSvc := func(ctrl *gomock.Controller, pubErr error) (*ExptSchedulerImpl, *svcmocks.MockExptResultService, *entitymocks.MockExptSchedulerMode) {
		setItemSvc := svcmocks.NewMockEvaluationSetItemService(ctrl)
		setItemSvc.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).
			Return([]*entity.EvaluationSetItem{{ItemID: 10, ItemKey: "k10", EvaluationSetID: 70}}, nil)
		resultSvc := svcmocks.NewMockExptResultService(ctrl)
		publisher := eventmocks.NewMockExptEventPublisher(ctrl)
		mode := entitymocks.NewMockExptSchedulerMode(ctrl)
		return &ExptSchedulerImpl{
			itemCompletePublisher:    &stubItemCompletePublisher{err: pubErr},
			evaluationSetItemService: setItemSvc,
			exptItemRefRepo:          mock_repo.NewMockIExptItemRefRepo(ctrl),
			ResultSvc:                resultSvc,
			Publisher:                publisher,
		}, resultSvc, mode
	}

	expt := &entity.Experiment{EvalSet: &entity.EvaluationSet{ID: 70, EvaluationSetVersion: &entity.EvaluationSetVersion{ID: 80}}}
	event := &entity.ExptScheduleEvent{SpaceID: 9, ExptID: 100, ExptRunID: 200}
	items := []*entity.ExptEvalItem{{ItemID: 10, State: entity.ItemRunState_Success}}

	t.Run("publish fail -> 落库前中断, RecordItemRunLogs 0 次", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, resultSvc, mode := newSvc(ctrl, assert.AnError)
		resultSvc.EXPECT().RecordItemRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
		mode.EXPECT().PublishResult(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

		err := svc.recordEvalItemRunLogs(context.Background(), event, items, mode, expt)
		assert.Error(t, err) // 发失败 error 上抛, 调用方据此中断本次调度
	})

	t.Run("publish ok -> 正常落库, RecordItemRunLogs 1 次", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, resultSvc, mode := newSvc(ctrl, nil)
		resultSvc.EXPECT().RecordItemRunLogs(gomock.Any(), int64(100), int64(200), int64(10), int64(9), gomock.Any()).
			Return(nil, nil).Times(1)
		mode.EXPECT().PublishResult(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
		// 循环结束后的批量收尾
		resultSvc.EXPECT().UpsertExptTurnResultFilter(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
		svc.Publisher.(*eventmocks.MockExptEventPublisher).EXPECT().
			PublishExptTurnResultFilterEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)

		err := svc.recordEvalItemRunLogs(context.Background(), event, items, mode, expt)
		assert.NoError(t, err)
	})
}

// TestExptSchedulerImpl_terminateZombieEvalTargetRecords_CrossSpace 覆盖 zombie 兜底销毁沙箱时
// 传入 TerminateAsyncRecordsAndDestroySandbox 的 spaceID 解析：
//   - Expt.TargetSpaceID > 0 → 用来源空间 (跨空间共享评测对象), 避免 DAO SpaceID.Eq 过滤把 record 过滤空
//   - Expt.TargetSpaceID = 0 → 回退到 event.SpaceID (消费方 = 来源方)
//   - expt = nil → 回退到 event.SpaceID (向后兼容)
func TestExptSchedulerImpl_terminateZombieEvalTargetRecords_CrossSpace(t *testing.T) {
	baseEvent := &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3}

	newSvc := func(ctrl *gomock.Controller) (*ExptSchedulerImpl, *mock_repo.MockIExptTurnResultRepo, *svcmocks.MockIEvalTargetService) {
		mockTurnRepo := mock_repo.NewMockIExptTurnResultRepo(ctrl)
		mockTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
		return &ExptSchedulerImpl{
			ExptTurnResultRepo: mockTurnRepo,
			evalTargetService:  mockTargetSvc,
		}, mockTurnRepo, mockTargetSvc
	}

	t.Run("TargetSpaceID>0 → 用来源空间 (99) 而不是 event.SpaceID (3)", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, mockTurnRepo, mockTargetSvc := newSvc(ctrl)

		expt := &entity.Experiment{
			ID:            1,
			TargetSpaceID: 99, // 跨空间共享: EvalTargetRecord.SpaceID 落库为 99
			Target: &entity.EvalTarget{
				EvalTargetVersion: &entity.EvalTargetVersion{EvalTargetType: entity.EvalTargetTypeSandboxAgent},
			},
		}
		mockTurnRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), int64(1), int64(2), []int64{10}, int64(3)).
			Return([]*entity.ExptTurnResultRunLog{{ItemID: 10, TargetResultID: 500}}, nil)
		mockTargetSvc.EXPECT().TerminateAsyncRecordsAndDestroySandbox(
			gomock.Any(), int64(99), []int64{500}, int32(errno.AsyncEvalTargetZombieTimeoutCode),
			gomock.Any(), true,
		).Times(1)

		err := svc.terminateZombieEvalTargetRecords(context.Background(), baseEvent, expt, []int64{10})
		assert.NoError(t, err)
	})

	t.Run("TargetSpaceID=0 → 回退到 event.SpaceID (3)", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, mockTurnRepo, mockTargetSvc := newSvc(ctrl)

		expt := &entity.Experiment{
			ID:            1,
			TargetSpaceID: 0, // 同空间/老数据
			Target: &entity.EvalTarget{
				EvalTargetVersion: &entity.EvalTargetVersion{EvalTargetType: entity.EvalTargetTypeSandboxAgent},
			},
		}
		mockTurnRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), int64(1), int64(2), []int64{10}, int64(3)).
			Return([]*entity.ExptTurnResultRunLog{{ItemID: 10, TargetResultID: 500}}, nil)
		mockTargetSvc.EXPECT().TerminateAsyncRecordsAndDestroySandbox(
			gomock.Any(), int64(3), []int64{500}, int32(errno.AsyncEvalTargetZombieTimeoutCode),
			gomock.Any(), true,
		).Times(1)

		err := svc.terminateZombieEvalTargetRecords(context.Background(), baseEvent, expt, []int64{10})
		assert.NoError(t, err)
	})

	t.Run("expt=nil → 回退到 event.SpaceID (3)", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, mockTurnRepo, mockTargetSvc := newSvc(ctrl)

		mockTurnRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), int64(1), int64(2), []int64{10}, int64(3)).
			Return([]*entity.ExptTurnResultRunLog{{ItemID: 10, TargetResultID: 500}}, nil)
		mockTargetSvc.EXPECT().TerminateAsyncRecordsAndDestroySandbox(
			gomock.Any(), int64(3), []int64{500}, int32(errno.AsyncEvalTargetZombieTimeoutCode),
			gomock.Any(), true,
		).Times(1)

		err := svc.terminateZombieEvalTargetRecords(context.Background(), baseEvent, nil, []int64{10})
		assert.NoError(t, err)
	})
}

// TestExptSchedulerImpl_sweepTerminatedSandboxItems_CrossSpace 覆盖 sweep 分支中
// CheckSandboxTerminated 与 TerminateAsyncRecordsAndDestroySandbox 传入的 spaceID 解析:
//   - Expt.TargetSpaceID > 0 → 都用来源空间
//   - Expt.TargetSpaceID = 0 → 都用 event.SpaceID
func TestExptSchedulerImpl_sweepTerminatedSandboxItems_CrossSpace(t *testing.T) {
	baseEvent := &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3}
	sandboxTarget := &entity.EvalTarget{
		EvalTargetVersion: &entity.EvalTargetVersion{EvalTargetType: entity.EvalTargetTypeSandboxAgent},
	}

	newSvc := func(ctrl *gomock.Controller) (*ExptSchedulerImpl, *mock_repo.MockIExptTurnResultRepo, *mock_repo.MockIExptItemResultRepo, *svcmocks.MockIEvalTargetService) {
		mockTurnRepo := mock_repo.NewMockIExptTurnResultRepo(ctrl)
		mockItemRepo := mock_repo.NewMockIExptItemResultRepo(ctrl)
		mockTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
		return &ExptSchedulerImpl{
			ExptTurnResultRepo: mockTurnRepo,
			ExptItemResultRepo: mockItemRepo,
			evalTargetService:  mockTargetSvc,
		}, mockTurnRepo, mockItemRepo, mockTargetSvc
	}

	t.Run("TargetSpaceID>0 → Check/Terminate 都用来源空间 (99)", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, mockTurnRepo, mockItemRepo, mockTargetSvc := newSvc(ctrl)

		expt := &entity.Experiment{ID: 1, TargetSpaceID: 99, Target: sandboxTarget}
		mockTurnRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), int64(1), int64(2), []int64{10}, int64(3)).
			Return([]*entity.ExptTurnResultRunLog{{ItemID: 10, TargetResultID: 500}}, nil)
		// 关键: CheckSandboxTerminated 用 targetSpaceID=99
		mockTargetSvc.EXPECT().CheckSandboxTerminated(gomock.Any(), int64(99), []int64{500}).
			Return([]int64{500}, map[int64]string{500: "Failed"})
		// 关键: Terminate 也用 targetSpaceID=99, 与 Check 侧保持一致
		mockTargetSvc.EXPECT().TerminateAsyncRecordsAndDestroySandbox(
			gomock.Any(), int64(99), []int64{500}, int32(errno.SandboxTerminatedBeforeReportCode),
			gomock.Any(), false,
		).Times(1)
		// 消费方侧写库仍然用 event.SpaceID=3
		mockItemRepo.EXPECT().UpdateItemRunLog(gomock.Any(), int64(1), int64(2), []int64{10}, gomock.Any(), int64(3)).Return(nil)
		// ★ 与 handleZombies 同一条不变量：主表只写 err_msg，status 归 RecordItemRunLogs。
		// 本处曾照抄 handleZombies 的写库形状（连 status 一起抄），把同一个净零 bug 复制了一份。
		mockItemRepo.EXPECT().UpdateItemsResult(gomock.Any(), int64(3), int64(1), []int64{10}, gomock.Any()).
			DoAndReturn(func(_ context.Context, _, _ int64, _ []int64, ufields map[string]any) error {
				_, hasStatus := ufields["status"]
				assert.False(t, hasStatus, "sandbox sweep 路径不得抢先写主表 status")
				assert.NotNil(t, ufields["err_msg"])
				return nil
			})
		mockTurnRepo.EXPECT().CreateOrUpdateItemsTurnRunLogStatus(gomock.Any(), int64(3), int64(1), int64(2), []int64{10}, entity.TurnRunState_Fail).Return(nil)

		items := []*entity.ExptEvalItem{{ItemID: 10, State: entity.ItemRunState_Processing}}
		_, terminated, err := svc.sweepTerminatedSandboxItems(context.Background(), baseEvent, items, expt)
		assert.NoError(t, err)
		assert.Len(t, terminated, 1)
	})

	t.Run("TargetSpaceID=0 → Check/Terminate 都回退到 event.SpaceID (3)", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, mockTurnRepo, mockItemRepo, mockTargetSvc := newSvc(ctrl)

		expt := &entity.Experiment{ID: 1, TargetSpaceID: 0, Target: sandboxTarget}
		mockTurnRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), int64(1), int64(2), []int64{10}, int64(3)).
			Return([]*entity.ExptTurnResultRunLog{{ItemID: 10, TargetResultID: 500}}, nil)
		mockTargetSvc.EXPECT().CheckSandboxTerminated(gomock.Any(), int64(3), []int64{500}).
			Return([]int64{500}, map[int64]string{500: "Failed"})
		mockTargetSvc.EXPECT().TerminateAsyncRecordsAndDestroySandbox(
			gomock.Any(), int64(3), []int64{500}, int32(errno.SandboxTerminatedBeforeReportCode),
			gomock.Any(), false,
		).Times(1)
		mockItemRepo.EXPECT().UpdateItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		mockItemRepo.EXPECT().UpdateItemsResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		mockTurnRepo.EXPECT().CreateOrUpdateItemsTurnRunLogStatus(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

		items := []*entity.ExptEvalItem{{ItemID: 10, State: entity.ItemRunState_Processing}}
		_, terminated, err := svc.sweepTerminatedSandboxItems(context.Background(), baseEvent, items, expt)
		assert.NoError(t, err)
		assert.Len(t, terminated, 1)
	})
}

// TestSandboxAgentTargetTagsFromExpt 覆盖沙箱 agent target 反查 helper 的所有短路分支:
// nil expt / nil Target / nil EvalTargetVersion / nil SandboxAgent 均返回空串,
// 完整链路返回 Name + SourceTargetID (即 AgentKit application_id)。
func TestSandboxAgentTargetTagsFromExpt(t *testing.T) {
	// nil expt
	name, appID := sandboxAgentTargetTagsFromExpt(nil)
	assert.Equal(t, "", name)
	assert.Equal(t, "", appID)

	// nil Target
	name, appID = sandboxAgentTargetTagsFromExpt(&entity.Experiment{})
	assert.Equal(t, "", name)
	assert.Equal(t, "", appID)

	// SourceTargetID 有值但 EvalTargetVersion 为 nil: application_id 仍应返回, agent_name 留空
	name, appID = sandboxAgentTargetTagsFromExpt(&entity.Experiment{Target: &entity.EvalTarget{
		SourceTargetID: "app-only",
	}})
	assert.Equal(t, "", name)
	assert.Equal(t, "app-only", appID)

	// EvalTargetVersion 非 nil 但 SandboxAgent 为 nil (非沙箱 target)
	name, appID = sandboxAgentTargetTagsFromExpt(&entity.Experiment{Target: &entity.EvalTarget{
		SourceTargetID:    "app-x",
		EvalTargetVersion: &entity.EvalTargetVersion{},
	}})
	assert.Equal(t, "", name)
	assert.Equal(t, "app-x", appID)

	// 完整链路
	name, appID = sandboxAgentTargetTagsFromExpt(&entity.Experiment{Target: &entity.EvalTarget{
		SourceTargetID: "app-full",
		EvalTargetVersion: &entity.EvalTargetVersion{
			SandboxAgent: &entity.SandboxAgent{Name: "full-agent"},
		},
	}})
	assert.Equal(t, "full-agent", name)
	assert.Equal(t, "app-full", appID)
}

// TestExptSchedulerImpl_emitSandboxSweptInvokeFinished_ShortCircuits 直接覆盖 emit helper
// 的三个短路场景, 独立于 sweepTerminatedSandboxItems 主流程:
//   - sandboxAgentMetrics 未注入 → 直接 return
//   - terminatedRecordIDs 为空 → 直接 return
//   - recordIDToItemIDs 缺 mapping → 走 []int64{0} 保底遍历一条
func TestExptSchedulerImpl_emitSandboxSweptInvokeFinished_ShortCircuits(t *testing.T) {
	baseEvent := &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3}
	expt := &entity.Experiment{
		TargetID: 99,
		Target: &entity.EvalTarget{
			SourceTargetID: "app-1",
			EvalTargetVersion: &entity.EvalTargetVersion{
				SandboxAgent: &entity.SandboxAgent{Name: "agent-1"},
			},
		},
	}

	t.Run("sandboxAgentMetrics 未注入 → no-op", func(t *testing.T) {
		svc := &ExptSchedulerImpl{}
		assert.NotPanics(t, func() {
			svc.emitSandboxSweptInvokeFinished(context.Background(), baseEvent, expt, []int64{500}, nil)
		})
	})

	t.Run("terminatedRecordIDs 为空 → no-op", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockMetric := metricsmocks.NewMockSandboxAgentMetrics(ctrl)
		svc := &ExptSchedulerImpl{sandboxAgentMetrics: mockMetric}
		// 未 EXPECT EmitInvokeFinished, 被调则失败
		svc.emitSandboxSweptInvokeFinished(context.Background(), baseEvent, expt, nil, nil)
	})

	t.Run("recordIDToItemIDs 缺 mapping → 用 itemID=0 保底", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockMetric := metricsmocks.NewMockSandboxAgentMetrics(ctrl)
		svc := &ExptSchedulerImpl{sandboxAgentMetrics: mockMetric}
		mockMetric.EXPECT().EmitInvokeFinished(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(tags metrics.SandboxAgentInvokeTags, _ error, _ int32, _ time.Time) {
				assert.Equal(t, int64(0), tags.ItemID)
				assert.Equal(t, "500", tags.InvokeID)
				assert.Equal(t, int64(3), tags.SpaceID)
				assert.Equal(t, int64(2), tags.ExperimentRunID)
				assert.Equal(t, "agent-1", tags.AgentName)
				assert.Equal(t, "app-1", tags.ApplicationID)
			}).Times(1)
		svc.emitSandboxSweptInvokeFinished(context.Background(), baseEvent, expt, []int64{500}, map[int64][]int64{})
	})

	t.Run("expt 为 nil → datasetID/targetID 归零, 不 panic", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockMetric := metricsmocks.NewMockSandboxAgentMetrics(ctrl)
		svc := &ExptSchedulerImpl{sandboxAgentMetrics: mockMetric}
		mockMetric.EXPECT().EmitInvokeFinished(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(tags metrics.SandboxAgentInvokeTags, _ error, _ int32, _ time.Time) {
				assert.Equal(t, int64(0), tags.DatasetID)
				assert.Equal(t, int64(0), tags.TargetID)
				assert.Equal(t, "", tags.AgentName)
				assert.Equal(t, "", tags.ApplicationID)
			}).Times(1)
		svc.emitSandboxSweptInvokeFinished(context.Background(), baseEvent, nil, []int64{600}, map[int64][]int64{600: {7}})
	})
}

// TestExptSchedulerImpl_emitSandboxZombieInvokeFinished_ShortCircuits 直接覆盖 zombie
// emit helper 的短路场景, 与 swept 版本对称。errCode 使用 AsyncEvalTargetZombieTimeoutCode。
func TestExptSchedulerImpl_emitSandboxZombieInvokeFinished_ShortCircuits(t *testing.T) {
	baseEvent := &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3}
	expt := &entity.Experiment{
		TargetID: 99,
		Target: &entity.EvalTarget{
			SourceTargetID: "app-z",
			EvalTargetVersion: &entity.EvalTargetVersion{
				SandboxAgent: &entity.SandboxAgent{Name: "agent-z"},
			},
		},
	}

	t.Run("sandboxAgentMetrics 未注入 → no-op", func(t *testing.T) {
		svc := &ExptSchedulerImpl{}
		assert.NotPanics(t, func() {
			svc.emitSandboxZombieInvokeFinished(context.Background(), baseEvent, expt, []int64{500}, nil)
		})
	})

	t.Run("recordIDs 为空 → no-op", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockMetric := metricsmocks.NewMockSandboxAgentMetrics(ctrl)
		svc := &ExptSchedulerImpl{sandboxAgentMetrics: mockMetric}
		svc.emitSandboxZombieInvokeFinished(context.Background(), baseEvent, expt, nil, nil)
	})

	t.Run("recordIDToItemIDs 缺 mapping → 用 itemID=0 保底", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockMetric := metricsmocks.NewMockSandboxAgentMetrics(ctrl)
		svc := &ExptSchedulerImpl{sandboxAgentMetrics: mockMetric}
		mockMetric.EXPECT().EmitInvokeFinished(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(tags metrics.SandboxAgentInvokeTags, _ error, _ int32, _ time.Time) {
				assert.Equal(t, int64(0), tags.ItemID)
				assert.Equal(t, "500", tags.InvokeID)
			}).Times(1)
		svc.emitSandboxZombieInvokeFinished(context.Background(), baseEvent, expt, []int64{500}, map[int64][]int64{})
	})

	t.Run("多 item 展开: recordIDToItemIDs 一 record 对多 item, 逐条打点", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockMetric := metricsmocks.NewMockSandboxAgentMetrics(ctrl)
		svc := &ExptSchedulerImpl{sandboxAgentMetrics: mockMetric}
		itemIDs := []int64{}
		mockMetric.EXPECT().EmitInvokeFinished(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(tags metrics.SandboxAgentInvokeTags, _ error, errCode int32, _ time.Time) {
				assert.Equal(t, int32(errno.AsyncEvalTargetZombieTimeoutCode), errCode)
				assert.Equal(t, "agent-z", tags.AgentName)
				assert.Equal(t, "app-z", tags.ApplicationID)
				itemIDs = append(itemIDs, tags.ItemID)
			}).Times(2)
		svc.emitSandboxZombieInvokeFinished(context.Background(), baseEvent, expt, []int64{600},
			map[int64][]int64{600: {11, 22}})
		assert.ElementsMatch(t, []int64{11, 22}, itemIDs)
	})
}

// TestExptSchedulerImpl_emitSandboxSweptInvokeFinished 覆盖 sweep 补打点 tag 组装:
//   - sandboxAgentMetrics=nil / 空 record → no-op
//   - 命中路径断言新增 5 tag (SpaceID / ExperimentRunID / AgentName / ApplicationID / TargetID) 落到 EmitInvokeFinished
//   - recordIDToItemIDs 为空 → itemID=0 保底展开
//   - 一个 record → 多 item 展开成 N 次
//   - expt=nil → agent/application tag 空但 SpaceID 仍来自 event
func TestExptSchedulerImpl_emitSandboxSweptInvokeFinished(t *testing.T) {
	event := &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3}
	sandboxExpt := &entity.Experiment{
		TargetID: 99,
		Target: &entity.EvalTarget{
			SourceTargetID: "app-1",
			EvalTargetVersion: &entity.EvalTargetVersion{
				SandboxAgent: &entity.SandboxAgent{Name: "my-agent"},
			},
		},
		EvalSet: &entity.EvaluationSet{
			ID:                   77,
			EvaluationSetVersion: &entity.EvaluationSetVersion{ID: 88},
		},
	}

	t.Run("sandboxAgentMetrics=nil → no-op", func(t *testing.T) {
		svc := &ExptSchedulerImpl{}
		svc.emitSandboxSweptInvokeFinished(context.Background(), event, sandboxExpt, []int64{500}, map[int64][]int64{500: {10}})
	})

	t.Run("空 record → no-op", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockMetric := metricsmocks.NewMockSandboxAgentMetrics(ctrl)
		svc := &ExptSchedulerImpl{sandboxAgentMetrics: mockMetric}
		svc.emitSandboxSweptInvokeFinished(context.Background(), event, sandboxExpt, nil, nil)
	})

	t.Run("单 record 单 item → 5 新 tag 完整落到 EmitInvokeFinished", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockMetric := metricsmocks.NewMockSandboxAgentMetrics(ctrl)
		svc := &ExptSchedulerImpl{sandboxAgentMetrics: mockMetric}

		mockMetric.EXPECT().EmitInvokeFinished(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Do(func(tags metrics.SandboxAgentInvokeTags, _ error, errCode int32, _ time.Time) {
				assert.Equal(t, int64(3), tags.SpaceID)
				assert.Equal(t, int64(1), tags.ExperimentID)
				assert.Equal(t, int64(2), tags.ExperimentRunID)
				assert.Equal(t, "my-agent", tags.AgentName)
				assert.Equal(t, "app-1", tags.ApplicationID)
				assert.Equal(t, int64(10), tags.ItemID)
				assert.Equal(t, "500", tags.InvokeID)
				assert.Equal(t, int64(77), tags.DatasetID)
				assert.Equal(t, int64(88), tags.DatasetVersion)
				assert.Equal(t, int64(99), tags.TargetID)
				assert.Equal(t, int32(errno.SandboxTerminatedBeforeReportCode), errCode)
			}).Times(1)

		svc.emitSandboxSweptInvokeFinished(context.Background(), event, sandboxExpt, []int64{500}, map[int64][]int64{500: {10}})
	})

	t.Run("recordID→itemIDs 缺失 → itemID=0 保底展开一次", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockMetric := metricsmocks.NewMockSandboxAgentMetrics(ctrl)
		svc := &ExptSchedulerImpl{sandboxAgentMetrics: mockMetric}

		mockMetric.EXPECT().EmitInvokeFinished(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Do(func(tags metrics.SandboxAgentInvokeTags, _ error, _ int32, _ time.Time) {
				assert.Equal(t, int64(0), tags.ItemID)
				assert.Equal(t, "500", tags.InvokeID)
			}).Times(1)

		svc.emitSandboxSweptInvokeFinished(context.Background(), event, sandboxExpt, []int64{500}, nil)
	})

	t.Run("一个 record 关联多个 item → 展开成 N 次", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockMetric := metricsmocks.NewMockSandboxAgentMetrics(ctrl)
		svc := &ExptSchedulerImpl{sandboxAgentMetrics: mockMetric}

		got := map[int64]bool{}
		mockMetric.EXPECT().EmitInvokeFinished(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Do(func(tags metrics.SandboxAgentInvokeTags, _ error, _ int32, _ time.Time) {
				got[tags.ItemID] = true
			}).Times(2)

		svc.emitSandboxSweptInvokeFinished(context.Background(), event, sandboxExpt, []int64{500}, map[int64][]int64{500: {10, 11}})
		assert.True(t, got[10] && got[11])
	})

	t.Run("expt=nil → agent/application tag 为空, SpaceID 仍来自 event", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockMetric := metricsmocks.NewMockSandboxAgentMetrics(ctrl)
		svc := &ExptSchedulerImpl{sandboxAgentMetrics: mockMetric}

		mockMetric.EXPECT().EmitInvokeFinished(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Do(func(tags metrics.SandboxAgentInvokeTags, _ error, _ int32, _ time.Time) {
				assert.Equal(t, "", tags.AgentName)
				assert.Equal(t, "", tags.ApplicationID)
				assert.Equal(t, int64(0), tags.TargetID)
				assert.Equal(t, int64(3), tags.SpaceID)
			}).Times(1)

		svc.emitSandboxSweptInvokeFinished(context.Background(), event, nil, []int64{500}, map[int64][]int64{500: {10}})
	})
}

// TestExptSchedulerImpl_emitSandboxZombieInvokeFinished 与 sweep 版本对称:
// zombie errCode + 同一套 5 新 tag 组装; 覆盖 metrics=nil / 空 record / 命中三条主路径。
func TestExptSchedulerImpl_emitSandboxZombieInvokeFinished(t *testing.T) {
	event := &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3}
	sandboxExpt := &entity.Experiment{
		TargetID: 99,
		Target: &entity.EvalTarget{
			SourceTargetID: "app-z",
			EvalTargetVersion: &entity.EvalTargetVersion{
				SandboxAgent: &entity.SandboxAgent{Name: "zombie-agent"},
			},
		},
		EvalSet: &entity.EvaluationSet{
			ID:                   77,
			EvaluationSetVersion: &entity.EvaluationSetVersion{ID: 88},
		},
	}

	t.Run("sandboxAgentMetrics=nil → no-op", func(t *testing.T) {
		svc := &ExptSchedulerImpl{}
		svc.emitSandboxZombieInvokeFinished(context.Background(), event, sandboxExpt, []int64{500}, map[int64][]int64{500: {10}})
	})

	t.Run("空 record → no-op", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockMetric := metricsmocks.NewMockSandboxAgentMetrics(ctrl)
		svc := &ExptSchedulerImpl{sandboxAgentMetrics: mockMetric}
		svc.emitSandboxZombieInvokeFinished(context.Background(), event, sandboxExpt, nil, nil)
	})

	t.Run("命中: 5 新 tag + zombie errCode", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockMetric := metricsmocks.NewMockSandboxAgentMetrics(ctrl)
		svc := &ExptSchedulerImpl{sandboxAgentMetrics: mockMetric}

		mockMetric.EXPECT().EmitInvokeFinished(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Do(func(tags metrics.SandboxAgentInvokeTags, _ error, errCode int32, _ time.Time) {
				assert.Equal(t, int64(3), tags.SpaceID)
				assert.Equal(t, int64(2), tags.ExperimentRunID)
				assert.Equal(t, "zombie-agent", tags.AgentName)
				assert.Equal(t, "app-z", tags.ApplicationID)
				assert.Equal(t, int32(errno.AsyncEvalTargetZombieTimeoutCode), errCode)
			}).Times(1)

		svc.emitSandboxZombieInvokeFinished(context.Background(), event, sandboxExpt, []int64{500}, map[int64][]int64{500: {10}})
	})
}
