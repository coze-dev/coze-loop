// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/external/audit"
	auditMocks "github.com/coze-dev/coze-loop/backend/infra/external/audit/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	benefitMocks "github.com/coze-dev/coze-loop/backend/infra/external/benefit/mocks"
	idgenMocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	lockMocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	idemMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem/mocks"
	metricsMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	componentMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventsMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	repoMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestExptMangerImpl_CheckRun(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mgr := newTestExptManager(ctrl)
	ctx := context.Background()
	session := &entity.Session{UserID: "1"}

	// 设置审计服务的mock
	mgr.audit.(*auditMocks.MockIAuditService).
		EXPECT().
		Audit(gomock.Any(), gomock.Any()).
		Return(audit.AuditRecord{AuditStatus: audit.AuditStatus_Approved}, nil).
		AnyTimes()

	// 设置评估集服务的mock
	mgr.evaluationSetService.(*svcMocks.MockIEvaluationSetService).
		EXPECT().
		BatchGetEvaluationSets(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]*entity.EvaluationSet{{
			EvaluationSetVersion: &entity.EvaluationSetVersion{
				ItemCount: 10,
			},
		}}, nil).
		AnyTimes()

	tests := []struct {
		name    string
		expt    *entity.Experiment
		wantErr bool
	}{
		{
			name: "success",
			expt: &entity.Experiment{
				ID:               1,
				ExptType:         entity.ExptType_Offline,
				EvalSetID:        1,
				EvalSetVersionID: 1,
				TargetID:         1,
				TargetVersionID:  1,
				Target:           &entity.EvalTarget{},
				EvalSet: &entity.EvaluationSet{
					EvaluationSetVersion: &entity.EvaluationSetVersion{
						ItemCount: 10,
					},
				},
				EvaluatorVersionRef: []*entity.ExptEvaluatorVersionRef{{EvaluatorVersionID: 1}},
				Evaluators:          []*entity.Evaluator{{ID: 1}},
			},
			wantErr: false,
		},
		{
			name: "fail_no_target",
			expt: &entity.Experiment{
				ID:               1,
				ExptType:         entity.ExptType_Offline,
				EvalSetID:        1,
				EvalSetVersionID: 1,
				Target:           nil, // 缺少目标，应该导致失败
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mgr.CheckRun(ctx, tt.expt, 1, session)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckRun() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExptMangerImpl_Run(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mgr := newTestExptManager(ctrl)
	ctx := context.Background()
	session := &entity.Session{UserID: "1"}

	// 设置配置服务的mock
	mgr.configer.(*componentMocks.MockIConfiger).
		EXPECT().
		GetExptExecConf(gomock.Any(), gomock.Any()).
		Return(&entity.ExptExecConf{}).
		AnyTimes()

	tests := []struct {
		name    string
		setup   func()
		exptID  int64
		runID   int64
		spaceID int64
		runMode entity.ExptRunMode
		wantErr bool
	}{
		{
			name: "success",
			setup: func() {
				// 配额检查通过
				mgr.quotaRepo.(*repoMocks.MockQuotaRepo).
					EXPECT().
					CreateOrUpdate(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil)

				// 事件发布成功
				mgr.publisher.(*eventsMocks.MockExptEventPublisher).
					EXPECT().
					PublishExptScheduleEvent(
						gomock.Any(),
						gomock.Any(),
						gomock.Any(),
					).
					Return(nil)
			},
			exptID:  1,
			runID:   1,
			spaceID: 1,
			runMode: entity.EvaluationModeSubmit,
			wantErr: false,
		},
		{
			name: "fail_quota_exceeded",
			setup: func() {
				// 配额检查失败
				mgr.quotaRepo.(*repoMocks.MockQuotaRepo).
					EXPECT().
					CreateOrUpdate(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(errors.New("quota exceeded"))
			},
			exptID:  1,
			runID:   1,
			spaceID: 1,
			runMode: entity.EvaluationModeSubmit,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			err := mgr.Run(ctx, tt.exptID, tt.runID, tt.spaceID, session, tt.runMode, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("Run() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExptMangerImpl_RetryUnSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mgr := newTestExptManager(ctrl)
	ctx := context.Background()
	session := &entity.Session{UserID: "1"}

	// 设置配置服务的mock
	mgr.configer.(*componentMocks.MockIConfiger).
		EXPECT().
		GetExptExecConf(gomock.Any(), gomock.Any()).
		Return(&entity.ExptExecConf{}).
		AnyTimes()

	tests := []struct {
		name    string
		setup   func()
		exptID  int64
		runID   int64
		spaceID int64
		wantErr bool
	}{
		{
			name: "success",
			setup: func() {
				// 配额检查通过
				mgr.quotaRepo.(*repoMocks.MockQuotaRepo).
					EXPECT().
					CreateOrUpdate(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil)

				// 事件发布成功
				mgr.publisher.(*eventsMocks.MockExptEventPublisher).
					EXPECT().
					PublishExptScheduleEvent(
						gomock.Any(),
						gomock.AssignableToTypeOf(&entity.ExptScheduleEvent{}),
						gomock.Any(),
					).
					DoAndReturn(func(_ context.Context, event *entity.ExptScheduleEvent, _ *time.Duration) error {
						if event.SpaceID != 1 ||
							event.ExptID != 1 ||
							event.ExptRunID != 1 ||
							event.ExptRunMode != entity.EvaluationModeFailRetry ||
							event.Session != session {
							t.Errorf("unexpected event: got %+v", event)
						}
						return nil
					})
			},
			exptID:  1,
			runID:   1,
			spaceID: 1,
			wantErr: false,
		},
		{
			name: "fail_quota_exceeded",
			setup: func() {
				// 配额检查失败
				mgr.quotaRepo.(*repoMocks.MockQuotaRepo).
					EXPECT().
					CreateOrUpdate(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(errors.New("quota exceeded"))
			},
			exptID:  1,
			runID:   1,
			spaceID: 1,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			err := mgr.RetryUnSuccess(ctx, tt.exptID, tt.runID, tt.spaceID, session, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("RetryUnSuccess() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExptMangerImpl_Invoke(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mgr := newTestExptManager(ctrl)
	ctx := context.Background()
	session := &entity.Session{UserID: "1"}

	tests := []struct {
		name    string
		req     *entity.InvokeExptReq
		setup   func()
		wantErr bool
	}{
		{
			name: "success",
			req: &entity.InvokeExptReq{
				SpaceID: 1,
				ExptID:  1,
				RunID:   1,
				Session: session,
				Items: []*entity.EvaluationSetItem{
					{
						ItemID: 100,
						Turns: []*entity.Turn{
							{ID: 1000},
							{ID: 1001},
						},
					},
					{
						ItemID: 101,
						Turns: []*entity.Turn{
							{ID: 1002},
						},
					},
				},
			},
			setup: func() {
				// 模拟获取已存在的item IDs
				mgr.itemResultRepo.(*repoMocks.MockIExptItemResultRepo).
					EXPECT().
					GetItemIDListByExptID(gomock.Any(), int64(1), int64(1)).
					Return([]int64{}, nil)

				// 模拟获取最大item索引
				mgr.itemResultRepo.(*repoMocks.MockIExptItemResultRepo).
					EXPECT().
					GetMaxItemIdxByExptID(gomock.Any(), int64(1), int64(1)).
					Return(int32(0), nil)

				// 模拟生成IDs (2个item + 3个turn = 5个ID)
				mgr.idgenerator.(*idgenMocks.MockIIDGenerator).
					EXPECT().
					GenMultiIDs(gomock.Any(), 5).
					Return([]int64{1, 2, 3, 4, 5}, nil)

				// 模拟创建turn结果
				mgr.turnResultRepo.(*repoMocks.MockIExptTurnResultRepo).
					EXPECT().
					BatchCreateNX(gomock.Any(), gomock.Any()).
					Return(nil)

				// 模拟创建item结果
				mgr.itemResultRepo.(*repoMocks.MockIExptItemResultRepo).
					EXPECT().
					BatchCreateNX(gomock.Any(), gomock.Any()).
					Return(nil)

				// 模拟生成运行日志的IDs
				mgr.idgenerator.(*idgenMocks.MockIIDGenerator).
					EXPECT().
					GenMultiIDs(gomock.Any(), 2).
					Return([]int64{6, 7}, nil)

				// 模拟创建运行日志
				mgr.itemResultRepo.(*repoMocks.MockIExptItemResultRepo).
					EXPECT().
					BatchCreateNXRunLogs(gomock.Any(), gomock.Any()).
					Return(nil)

				// 模拟更新统计信息
				mgr.statsRepo.(*repoMocks.MockIExptStatsRepo).EXPECT().ArithOperateCount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				// 模拟发布事件
				mgr.publisher.(*eventsMocks.MockExptEventPublisher).
					EXPECT().
					PublishExptScheduleEvent(
						gomock.Any(),
						gomock.AssignableToTypeOf(&entity.ExptScheduleEvent{}),
						gomock.Any(),
					).
					DoAndReturn(func(_ context.Context, event *entity.ExptScheduleEvent, _ *time.Duration) error {
						if event.SpaceID != 1 ||
							event.ExptID != 1 ||
							event.ExptRunID != 1 ||
							event.ExptRunMode != entity.EvaluationModeAppend ||
							event.Session != session {
							t.Errorf("unexpected event: got %+v", event)
						}
						return nil
					})
			},
			wantErr: false,
		},
		{
			name: "fail_item_already_exists",
			req: &entity.InvokeExptReq{
				SpaceID: 1,
				ExptID:  1,
				RunID:   1,
				Session: session,
				Items: []*entity.EvaluationSetItem{
					{
						ItemID: 100,
						Turns: []*entity.Turn{
							{ID: 1000},
						},
					},
				},
			},
			setup: func() {
				// 模拟获取已存在的item IDs - 返回请求中的item ID，表示已存在
				mgr.itemResultRepo.(*repoMocks.MockIExptItemResultRepo).
					EXPECT().
					GetItemIDListByExptID(gomock.Any(), int64(1), int64(1)).
					Return([]int64{100}, nil)
			},
			wantErr: false, // 注意：当items都已存在时，Invoke会返回nil而不是错误
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			err := mgr.Invoke(ctx, tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("Invoke() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExptMangerImpl_Finish(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mgr := newTestExptManager(ctrl)
	ctx := context.Background()
	session := &entity.Session{UserID: "1"}

	tests := []struct {
		name    string
		expt    *entity.Experiment
		runID   int64
		setup   func()
		wantErr bool
	}{
		{
			name: "success",
			expt: &entity.Experiment{
				ID:      1,
				SpaceID: 100,
			},
			runID: 1,
			setup: func() {
				// 模拟幂等性检查 - 返回不存在
				mgr.idem.(*idemMocks.MockIdempotentService).
					EXPECT().
					Exist(gomock.Any(), "FinishExpt:1").
					Return(false, nil)

				// 模拟更新实验状态
				mgr.exptRepo.(*repoMocks.MockIExperimentRepo).
					EXPECT().
					Update(gomock.Any(), &entity.Experiment{
						ID:      1,
						SpaceID: 100,
						Status:  entity.ExptStatus_Draining,
					}).
					Return(nil)

				// 模拟发布事件
				mgr.publisher.(*eventsMocks.MockExptEventPublisher).
					EXPECT().
					PublishExptScheduleEvent(
						gomock.Any(),
						gomock.AssignableToTypeOf(&entity.ExptScheduleEvent{}),
						gomock.Any(),
					).
					DoAndReturn(func(_ context.Context, event *entity.ExptScheduleEvent, _ *time.Duration) error {
						if event.SpaceID != 100 ||
							event.ExptID != 1 ||
							event.ExptRunID != 1 ||
							event.ExptRunMode != entity.EvaluationModeAppend ||
							event.Session != session {
							t.Errorf("unexpected event: got %+v", event)
						}
						return nil
					})

				// 模拟设置幂等标记
				mgr.idem.(*idemMocks.MockIdempotentService).
					EXPECT().
					Set(gomock.Any(), "FinishExpt:1", time.Second*60).
					Return(nil)
			},
			wantErr: false,
		},
		{
			name: "fail_already_finished",
			expt: &entity.Experiment{
				ID:      1,
				SpaceID: 100,
			},
			runID: 1,
			setup: func() {
				// 模拟幂等性检查 - 返回已存在
				mgr.idem.(*idemMocks.MockIdempotentService).
					EXPECT().
					Exist(gomock.Any(), "FinishExpt:1").
					Return(true, nil)
			},
			wantErr: false, // 注意：当实验已经完成时，Finish会返回nil而不是错误
		},
		{
			name: "fail_update_error",
			expt: &entity.Experiment{
				ID:      1,
				SpaceID: 100,
			},
			runID: 1,
			setup: func() {
				// 模拟幂等性检查 - 返回不存在
				mgr.idem.(*idemMocks.MockIdempotentService).
					EXPECT().
					Exist(gomock.Any(), "FinishExpt:1").
					Return(false, nil)

				// 模拟更新实验状态失败
				mgr.exptRepo.(*repoMocks.MockIExperimentRepo).
					EXPECT().
					Update(gomock.Any(), gomock.Any()).
					Return(errors.New("update failed"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			err := mgr.Finish(ctx, tt.expt, tt.runID, session)
			if (err != nil) != tt.wantErr {
				t.Errorf("Finish() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExptMangerImpl_PendRun(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mgr := newTestExptManager(ctrl)
	ctx := context.Background()
	session := &entity.Session{UserID: "1"}

	tests := []struct {
		name    string
		exptID  int64
		runID   int64
		spaceID int64
		setup   func()
		wantErr bool
	}{
		{
			name:    "success",
			exptID:  1,
			runID:   1,
			spaceID: 100,
			setup: func() {
				// 模拟获取运行日志
				mgr.runLogRepo.(*repoMocks.MockIExptRunLogRepo).
					EXPECT().
					Get(gomock.Any(), int64(1), int64(1)).
					Return(&entity.ExptRunLog{
						ID:        1,
						ExptID:    1,
						ExptRunID: 1,
						SpaceID:   100,
					}, nil)

				// 模拟获取turn结果列表
				mgr.turnResultRepo.(*repoMocks.MockIExptTurnResultRepo).
					EXPECT().
					ListTurnResult(
						gomock.Any(),
						int64(100),
						int64(1),
						nil,
						gomock.Any(),
						false,
					).
					Return([]*entity.ExptTurnResult{
						{Status: int32(entity.TurnRunState_Success)},
						{Status: int32(entity.TurnRunState_Fail)},
						{Status: int32(entity.TurnRunState_Queueing)},
					}, int64(3), nil)

				// 模拟保存运行日志
				mgr.runLogRepo.(*repoMocks.MockIExptRunLogRepo).
					EXPECT().
					Save(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, runLog *entity.ExptRunLog) error {
						if runLog.Status != int64(entity.ExptStatus_Pending) {
							t.Errorf("unexpected status: got %v, want %v", runLog.Status, entity.ExptStatus_Pending)
						}
						if runLog.SuccessCnt != 1 || runLog.FailCnt != 1 || runLog.PendingCnt != 1 {
							t.Errorf("unexpected counts: success=%v, fail=%v, pending=%v", runLog.SuccessCnt, runLog.FailCnt, runLog.PendingCnt)
						}
						return nil
					})
			},
			wantErr: false,
		},
		{
			name:    "fail_get_run_log",
			exptID:  1,
			runID:   1,
			spaceID: 100,
			setup: func() {
				// 模拟获取运行日志失败
				mgr.runLogRepo.(*repoMocks.MockIExptRunLogRepo).
					EXPECT().
					Get(gomock.Any(), int64(1), int64(1)).
					Return(nil, errors.New("failed to get run log"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			err := mgr.PendRun(ctx, tt.exptID, tt.runID, tt.spaceID, session)
			if (err != nil) != tt.wantErr {
				t.Errorf("PendRun() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExptMangerImpl_PendExpt(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mgr := newTestExptManager(ctrl)
	ctx := context.Background()
	session := &entity.Session{UserID: "1"}

	tests := []struct {
		name    string
		exptID  int64
		spaceID int64
		setup   func()
		wantErr bool
	}{
		{
			name:    "success",
			exptID:  1,
			spaceID: 100,
			setup: func() {
				// 模拟计算统计信息
				mgr.exptResultService.(*svcMocks.MockExptResultService).
					EXPECT().
					CalculateStats(gomock.Any(), int64(1), int64(100), session).
					Return(&entity.ExptCalculateStats{
						SuccessItemCnt:    10,
						PendingItemCnt:    2,
						FailItemCnt:       1,
						ProcessingItemCnt: 3,
						TerminatedItemCnt: 1,
					}, nil)

				// 模拟更新统计信息
				mgr.statsRepo.(*repoMocks.MockIExptStatsRepo).
					EXPECT().
					UpdateByExptID(
						gomock.Any(),
						int64(1),
						int64(100),
						&entity.ExptStats{
							SuccessItemCnt:    10,
							PendingItemCnt:    2,
							FailItemCnt:       1,
							ProcessingItemCnt: 3,
							TerminatedItemCnt: 1,
						},
					).
					Return(nil)
			},
			wantErr: false,
		},
		{
			name:    "fail_calculate_stats",
			exptID:  1,
			spaceID: 100,
			setup: func() {
				// 模拟计算统计信息失败
				mgr.exptResultService.(*svcMocks.MockExptResultService).
					EXPECT().
					CalculateStats(gomock.Any(), int64(1), int64(100), session).
					Return(nil, errors.New("failed to calculate stats"))
			},
			wantErr: true,
		},
		{
			name:    "fail_update_stats",
			exptID:  1,
			spaceID: 100,
			setup: func() {
				// 模拟计算统计信息成功
				mgr.exptResultService.(*svcMocks.MockExptResultService).
					EXPECT().
					CalculateStats(gomock.Any(), int64(1), int64(100), session).
					Return(&entity.ExptCalculateStats{
						SuccessItemCnt:    10,
						PendingItemCnt:    2,
						FailItemCnt:       1,
						ProcessingItemCnt: 3,
						TerminatedItemCnt: 1,
					}, nil)

				// 模拟更新统计信息失败
				mgr.statsRepo.(*repoMocks.MockIExptStatsRepo).
					EXPECT().
					UpdateByExptID(
						gomock.Any(),
						int64(1),
						int64(100),
						gomock.Any(),
					).
					Return(errors.New("failed to update stats"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			err := mgr.PendExpt(ctx, tt.exptID, tt.spaceID, session)
			if (err != nil) != tt.wantErr {
				t.Errorf("PendExpt() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExptMangerImpl_CompleteRun(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mgr := newTestExptManager(ctrl)
	ctx := context.Background()
	session := &entity.Session{UserID: "1"}

	tests := []struct {
		name    string
		exptID  int64
		runID   int64
		mode    entity.ExptRunMode
		spaceID int64
		opts    []entity.CompleteExptOptionFn
		setup   func()
		wantErr bool
	}{
		{
			name:    "success",
			exptID:  1,
			runID:   1,
			mode:    entity.EvaluationModeSubmit,
			spaceID: 100,
			opts: []entity.CompleteExptOptionFn{
				entity.WithCID("test-cid"),
				entity.WithStatus(entity.ExptStatus_Success),
				entity.WithStatusMessage("success"),
			},
			setup: func() {
				// 模拟幂等性检查
				mgr.idem.(*idemMocks.MockIdempotentService).
					EXPECT().
					Exist(gomock.Any(), "CompleteRun:test-cid").
					Return(false, nil)

				// 模拟获取运行日志
				mgr.runLogRepo.(*repoMocks.MockIExptRunLogRepo).
					EXPECT().
					Get(gomock.Any(), int64(1), int64(1)).
					Return(&entity.ExptRunLog{
						ID:        1,
						ExptID:    1,
						ExptRunID: 1,
						SpaceID:   100,
					}, nil)

				// 模拟获取turn结果列表
				mgr.turnResultRepo.(*repoMocks.MockIExptTurnResultRepo).
					EXPECT().
					ListTurnResult(
						gomock.Any(),
						int64(100),
						int64(1),
						nil,
						gomock.Any(),
						false,
					).
					Return([]*entity.ExptTurnResult{
						{Status: int32(entity.TurnRunState_Success)},
						{Status: int32(entity.TurnRunState_Success)},
						{Status: int32(entity.TurnRunState_Success)},
					}, int64(3), nil)

				// 模拟解锁实验
				mgr.mutex.(*lockMocks.MockILocker).
					EXPECT().
					Unlock(mgr.makeExptMutexLockKey(int64(1))).
					Return(true, nil)

				// 模拟保存运行日志
				mgr.runLogRepo.(*repoMocks.MockIExptRunLogRepo).
					EXPECT().
					Save(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, runLog *entity.ExptRunLog) error {
						if runLog.Status != int64(entity.ExptStatus_Success) {
							t.Errorf("unexpected status: got %v, want %v", runLog.Status, entity.ExptStatus_Success)
						}
						if string(runLog.StatusMessage) != "success" {
							t.Errorf("unexpected status message: got %v, want %v", string(runLog.StatusMessage), "success")
						}
						if runLog.SuccessCnt != 3 {
							t.Errorf("unexpected success count: got %v, want %v", runLog.SuccessCnt, 3)
						}
						return nil
					})

				// 模拟设置幂等标记
				mgr.idem.(*idemMocks.MockIdempotentService).
					EXPECT().
					Set(gomock.Any(), "CompleteRun:test-cid", time.Second*60*3).
					Return(nil)
			},
			wantErr: false,
		},
		{
			name:    "fail_get_run_log",
			exptID:  1,
			runID:   1,
			mode:    entity.EvaluationModeSubmit,
			spaceID: 100,
			opts: []entity.CompleteExptOptionFn{
				entity.WithCID("test-cid"),
			},
			setup: func() {
				// 模拟幂等性检查
				mgr.idem.(*idemMocks.MockIdempotentService).
					EXPECT().
					Exist(gomock.Any(), "CompleteRun:test-cid").
					Return(false, nil)

				// 模拟获取运行日志失败
				mgr.runLogRepo.(*repoMocks.MockIExptRunLogRepo).
					EXPECT().
					Get(gomock.Any(), int64(1), int64(1)).
					Return(nil, errors.New("failed to get run log"))
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

func TestExptMangerImpl_CompleteExpt(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mgr := newTestExptManager(ctrl)
	ctx := context.Background()
	session := &entity.Session{UserID: "1"}

	tests := []struct {
		name    string
		exptID  int64
		spaceID int64
		opts    []entity.CompleteExptOptionFn
		setup   func()
		wantErr bool
	}{
		{
			name:    "success",
			exptID:  1,
			spaceID: 100,
			opts: []entity.CompleteExptOptionFn{
				entity.WithCID("test-cid"),
				entity.WithStatus(entity.ExptStatus_Success),
				entity.WithStatusMessage("success"),
			},
			setup: func() {
				// 模拟幂等性检查
				mgr.idem.(*idemMocks.MockIdempotentService).
					EXPECT().
					Exist(gomock.Any(), "CompleteExpt:test-cid").
					Return(false, nil)

				// 模拟获取实验信息
				mgr.exptRepo.(*repoMocks.MockIExperimentRepo).
					EXPECT().
					GetByID(gomock.Any(), int64(1), int64(100)).
					Return(&entity.Experiment{
						ID:      1,
						SpaceID: 100,
						StartAt: gptr.Of(time.Now()),
					}, nil)

				// 模拟发布聚合计算事件
				mgr.publisher.(*eventsMocks.MockExptEventPublisher).
					EXPECT().
					PublishExptAggrCalculateEvent(
						gomock.Any(),
						[]*entity.AggrCalculateEvent{
							{
								ExperimentID:  1,
								SpaceID:       100,
								CalculateMode: entity.CreateAllFields,
							},
						},
						gomock.Any(),
					).
					Return(nil)

				// 模拟计算统计信息
				mgr.exptResultService.(*svcMocks.MockExptResultService).
					EXPECT().
					CalculateStats(gomock.Any(), int64(1), int64(100), session).
					Return(&entity.ExptCalculateStats{
						SuccessItemCnt:    10,
						PendingItemCnt:    0,
						FailItemCnt:       0,
						ProcessingItemCnt: 0,
						TerminatedItemCnt: 0,
					}, nil)

				// 模拟更新统计信息
				mgr.statsRepo.(*repoMocks.MockIExptStatsRepo).
					EXPECT().
					UpdateByExptID(
						gomock.Any(),
						int64(1),
						int64(100),
						&entity.ExptStats{
							SuccessItemCnt:    10,
							PendingItemCnt:    0,
							FailItemCnt:       0,
							ProcessingItemCnt: 0,
							TerminatedItemCnt: 0,
						},
					).
					Return(nil)

				// 模拟更新实验状态
				mgr.exptRepo.(*repoMocks.MockIExperimentRepo).
					EXPECT().
					Update(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, expt *entity.Experiment) error {
						if expt.Status != entity.ExptStatus_Success {
							t.Errorf("unexpected status: got %v, want %v", expt.Status, entity.ExptStatus_Success)
						}
						if expt.StatusMessage != "success" {
							t.Errorf("unexpected status message: got %v, want %v", expt.StatusMessage, "success")
						}
						return nil
					})

				// 模拟释放实验运行配额
				mgr.configer.(*componentMocks.MockIConfiger).
					EXPECT().
					GetExptExecConf(gomock.Any(), int64(100)).
					Return(&entity.ExptExecConf{}).
					AnyTimes()

				mgr.quotaRepo.(*repoMocks.MockQuotaRepo).
					EXPECT().
					CreateOrUpdate(
						gomock.Any(),
						int64(100),
						gomock.Any(),
						gomock.Any(),
					).
					Return(nil)

				// 模拟设置幂等标记
				mgr.idem.(*idemMocks.MockIdempotentService).
					EXPECT().
					Set(gomock.Any(), "CompleteExpt:test-cid", time.Second*180).
					Return(nil)

				// 模拟发送指标
				mgr.mtr.(*metricsMocks.MockExptMetric).
					EXPECT().
					EmitExptExecResult(int64(100), gomock.Any(), int64(11), gomock.Any()).
					Return()
			},
			wantErr: false,
		},
		{
			name:    "fail_calculate_stats",
			exptID:  1,
			spaceID: 100,
			opts: []entity.CompleteExptOptionFn{
				entity.WithCID("test-cid"),
			},
			setup: func() {
				// 模拟幂等性检查
				mgr.idem.(*idemMocks.MockIdempotentService).
					EXPECT().
					Exist(gomock.Any(), "CompleteExpt:test-cid").
					Return(false, nil)

				// 模拟获取实验信息
				mgr.exptRepo.(*repoMocks.MockIExperimentRepo).
					EXPECT().
					GetByID(gomock.Any(), int64(1), int64(100)).
					Return(&entity.Experiment{
						ID:      1,
						SpaceID: 100,
					}, nil)

				// 模拟发布聚合计算事件
				mgr.publisher.(*eventsMocks.MockExptEventPublisher).
					EXPECT().
					PublishExptAggrCalculateEvent(
						gomock.Any(),
						[]*entity.AggrCalculateEvent{
							{
								ExperimentID:  1,
								SpaceID:       100,
								CalculateMode: entity.CreateAllFields,
							},
						},
						gomock.Any(),
					).
					Return(nil)

				// 模拟计算统计信息失败
				mgr.exptResultService.(*svcMocks.MockExptResultService).
					EXPECT().
					CalculateStats(gomock.Any(), int64(1), int64(100), session).
					Return(nil, errors.New("failed to calculate stats"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			err := mgr.CompleteExpt(ctx, tt.exptID, tt.spaceID, session, tt.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("CompleteExpt() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExptMangerImpl_LogRun(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mgr := newTestExptManager(ctrl)
	ctx := context.Background()
	session := &entity.Session{UserID: "1"}

	tests := []struct {
		name    string
		exptID  int64
		runID   int64
		mode    entity.ExptRunMode
		spaceID int64
		setup   func()
		wantErr bool
	}{
		{
			name:    "success",
			exptID:  1,
			runID:   10,
			mode:    entity.EvaluationModeSubmit,
			spaceID: 100,
			setup: func() {
				// 配置服务返回配置
				mgr.configer.(*componentMocks.MockIConfiger).
					EXPECT().
					GetExptExecConf(gomock.Any(), int64(100)).
					Return(&entity.ExptExecConf{ZombieIntervalSecond: 60}).
					AnyTimes()

				// locker加锁成功
				mgr.mutex.(*lockMocks.MockILocker).
					EXPECT().
					LockBackoff(gomock.Any(), mgr.makeExptMutexLockKey(int64(1)), time.Second*60, time.Second).
					Return(true, nil)

				// runLogRepo创建成功
				mgr.runLogRepo.(*repoMocks.MockIExptRunLogRepo).
					EXPECT().
					Create(gomock.Any(), gomock.Any()).
					Return(nil)

				// 指标埋点
				mgr.mtr.(*metricsMocks.MockExptMetric).
					EXPECT().
					EmitExptExecRun(int64(100), int64(entity.EvaluationModeSubmit)).
					Return()
			},
			wantErr: false,
		},
		{
			name:    "fail_lock",
			exptID:  1,
			runID:   10,
			mode:    entity.EvaluationModeSubmit,
			spaceID: 100,
			setup: func() {
				mgr.configer.(*componentMocks.MockIConfiger).
					EXPECT().
					GetExptExecConf(gomock.Any(), int64(100)).
					Return(&entity.ExptExecConf{ZombieIntervalSecond: 60}).
					AnyTimes()

				// locker加锁失败
				mgr.mutex.(*lockMocks.MockILocker).
					EXPECT().
					LockBackoff(gomock.Any(), mgr.makeExptMutexLockKey(int64(1)), time.Second*60, time.Second).
					Return(false, nil)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			err := mgr.LogRun(ctx, tt.exptID, tt.runID, tt.mode, tt.spaceID, session)
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
	session := &entity.Session{UserID: "1"}

	tests := []struct {
		name       string
		exptID     int64
		runID      int64
		spaceID    int64
		setup      func()
		want       *entity.ExptRunLog
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:    "success",
			exptID:  1,
			runID:   10,
			spaceID: 100,
			setup: func() {
				// 模拟获取运行日志成功
				mgr.runLogRepo.(*repoMocks.MockIExptRunLogRepo).
					EXPECT().
					Get(gomock.Any(), int64(1), int64(10)).
					Return(&entity.ExptRunLog{
						ID:        1,
						SpaceID:   100,
						ExptID:    1,
						ExptRunID: 10,
						Mode:      int32(entity.EvaluationModeSubmit),
						Status:    int64(entity.ExptStatus_Pending),
						CreatedBy: "1",
					}, nil)
			},
			want: &entity.ExptRunLog{
				ID:        1,
				SpaceID:   100,
				ExptID:    1,
				ExptRunID: 10,
				Mode:      int32(entity.EvaluationModeSubmit),
				Status:    int64(entity.ExptStatus_Pending),
				CreatedBy: "1",
			},
			wantErr: false,
		},
		{
			name:    "fail_not_found",
			exptID:  1,
			runID:   10,
			spaceID: 100,
			setup: func() {
				// 模拟获取运行日志失败
				mgr.runLogRepo.(*repoMocks.MockIExptRunLogRepo).
					EXPECT().
					Get(gomock.Any(), int64(1), int64(10)).
					Return(nil, errors.New("run log not found"))
			},
			want:       nil,
			wantErr:    true,
			wantErrMsg: "run log not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			got, err := mgr.GetRunLog(ctx, tt.exptID, tt.runID, tt.spaceID, session)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetRunLog() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				if err.Error() != tt.wantErrMsg {
					t.Errorf("GetRunLog() error message = %v, wantErrMsg %v", err.Error(), tt.wantErrMsg)
				}
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetRunLog() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExptMangerImpl_CheckConnector(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mgr := newTestExptManager(ctrl)
	ctx := context.Background()
	session := &entity.Session{UserID: "1"}

	evalTargetVersion := &entity.EvalTargetVersion{
		OutputSchema: []*entity.ArgsSchema{{Key: gptr.Of("field1")}},
	}
	evalTarget := &entity.EvalTarget{
		EvalTargetType:    1,
		EvalTargetVersion: evalTargetVersion,
	}
	evalSetSchema := &entity.EvaluationSetSchema{
		FieldSchemas: []*entity.FieldSchema{{Name: "field1"}},
	}
	evalSetVersion := &entity.EvaluationSetVersion{
		EvaluationSetSchema: evalSetSchema,
		ItemCount:           1,
	}
	evalSet := &entity.EvaluationSet{
		EvaluationSetVersion: evalSetVersion,
	}
	evaluatorConf := &entity.EvaluatorConf{
		EvaluatorVersionID: 1,
		IngressConf: &entity.EvaluatorIngressConf{
			EvalSetAdapter: &entity.FieldAdapter{
				FieldConfs: []*entity.FieldConf{{FromField: "field1"}},
			},
			TargetAdapter: &entity.FieldAdapter{
				FieldConfs: []*entity.FieldConf{{FromField: "field1"}},
			},
		},
	}

	evaluatorsConf := &entity.EvaluatorsConf{
		EvaluatorConf: []*entity.EvaluatorConf{evaluatorConf},
	}

	targetConf := &entity.TargetConf{
		TargetVersionID: 1,
		IngressConf: &entity.TargetIngressConf{
			EvalSetAdapter: &entity.FieldAdapter{
				FieldConfs: []*entity.FieldConf{{FromField: "field1"}},
			},
		},
	}

	evalConf := &entity.EvaluationConfiguration{
		ConnectorConf: entity.Connector{
			TargetConf:     targetConf,
			EvaluatorsConf: evaluatorsConf,
		},
	}

	tests := []struct {
		name    string
		expt    *entity.Experiment
		wantErr bool
	}{
		{
			name:    "正常流程",
			expt:    &entity.Experiment{EvalConf: evalConf, Target: evalTarget, EvalSet: evalSet},
			wantErr: false,
		},
		{
			name:    "EvalConf为nil",
			expt:    &entity.Experiment{EvalConf: nil},
			wantErr: false,
		},
		{
			name: "EvaluatorsConf为nil",
			expt: func() *entity.Experiment {
				badConf := *evalConf
				badConf.ConnectorConf.EvaluatorsConf = nil
				return &entity.Experiment{EvalConf: &badConf, Target: evalTarget, EvalSet: evalSet}
			}(),
			wantErr: true,
		},
		{
			name: "EvaluatorConf为空数组",
			expt: func() *entity.Experiment {
				badConf := *evalConf
				badConf.ConnectorConf.EvaluatorsConf = &entity.EvaluatorsConf{EvaluatorConf: []*entity.EvaluatorConf{}}
				return &entity.Experiment{EvalConf: &badConf, Target: evalTarget, EvalSet: evalSet}
			}(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mgr.CheckConnector(ctx, tt.expt, session)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckConnector() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExptMangerImpl_CheckBenefit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mgr := newTestExptManager(ctrl)
	ctx := context.Background()
	session := &entity.Session{UserID: "1"}

	expt := &entity.Experiment{
		ID:         1,
		SpaceID:    2,
		CreditCost: entity.CreditCostDefault,
	}
	exptFree := &entity.Experiment{
		ID:         1,
		SpaceID:    2,
		CreditCost: entity.CreditCostFree,
	}

	tests := []struct {
		name    string
		prepare func()
		expt    *entity.Experiment
		wantErr bool
	}{
		{
			name: "正常流程",
			prepare: func() {
				mgr.benefitService.(*benefitMocks.MockIBenefitService).
					EXPECT().
					CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).
					Return(&benefit.CheckAndDeductEvalBenefitResult{IsFreeEvaluate: gptr.Of(false)}, nil)
			},
			expt:    expt,
			wantErr: false,
		},
		{
			name:    "CreditCostFree直接返回",
			prepare: func() {},
			expt:    exptFree,
			wantErr: false,
		},
		{
			name: "benefitService返回错误",
			prepare: func() {
				mgr.benefitService.(*benefitMocks.MockIBenefitService).
					EXPECT().
					CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("mock benefit error"))
			},
			expt:    expt,
			wantErr: true,
		},
		{
			name: "DenyReason返回错误",
			prepare: func() {
				mgr.benefitService.(*benefitMocks.MockIBenefitService).
					EXPECT().
					CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).
					Return(&benefit.CheckAndDeductEvalBenefitResult{DenyReason: ptr.Of(benefit.DenyReasonInsufficient)}, nil)
			},
			expt:    expt,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.prepare()
			err := mgr.CheckBenefit(ctx, tt.expt, session)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckBenefit() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExptMangerImpl_terminateItemTurns(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mgr := newTestExptManager(ctrl)
	ctx := context.Background()
	session := &entity.Session{UserID: "1"}

	exptID := int64(1)
	spaceID := int64(2)
	itemTurnIDs := []*entity.ItemTurnID{{ItemID: 10, TurnID: 100}, {ItemID: 11, TurnID: 101}}

	tests := []struct {
		name    string
		prepare func()
		wantErr bool
	}{
		{
			name: "正常流程",
			prepare: func() {
				mgr.itemResultRepo.(*repoMocks.MockIExptItemResultRepo).
					EXPECT().
					UpdateItemsResult(ctx, spaceID, exptID, []int64{10, 11}, map[string]any{"status": int32(entity.ItemRunState_Terminal)}).
					Return(nil)
				mgr.turnResultRepo.(*repoMocks.MockIExptTurnResultRepo).
					EXPECT().
					UpdateTurnResults(ctx, exptID, itemTurnIDs, spaceID, map[string]any{"status": int32(entity.TurnRunState_Terminal)}).
					Return(nil)
			},
			wantErr: false,
		},
		{
			name: "itemResultRepo返回错误",
			prepare: func() {
				mgr.itemResultRepo.(*repoMocks.MockIExptItemResultRepo).
					EXPECT().
					UpdateItemsResult(ctx, spaceID, exptID, []int64{10, 11}, map[string]any{"status": int32(entity.ItemRunState_Terminal)}).
					Return(errors.New("mock itemResultRepo error"))
			},
			wantErr: true,
		},
		{
			name: "turnResultRepo返回错误",
			prepare: func() {
				mgr.itemResultRepo.(*repoMocks.MockIExptItemResultRepo).
					EXPECT().
					UpdateItemsResult(ctx, spaceID, exptID, []int64{10, 11}, map[string]any{"status": int32(entity.ItemRunState_Terminal)}).
					Return(nil)
				mgr.turnResultRepo.(*repoMocks.MockIExptTurnResultRepo).
					EXPECT().
					UpdateTurnResults(ctx, exptID, itemTurnIDs, spaceID, map[string]any{"status": int32(entity.TurnRunState_Terminal)}).
					Return(errors.New("mock turnResultRepo error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.prepare()
			err := mgr.terminateItemTurns(ctx, exptID, itemTurnIDs, spaceID, session)
			if (err != nil) != tt.wantErr {
				t.Errorf("terminateItemTurns() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
