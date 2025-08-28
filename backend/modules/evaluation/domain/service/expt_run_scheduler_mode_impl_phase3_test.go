// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"go.uber.org/mock/gomock"

	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	idemmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem/mocks"
	configmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	mock_repo "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

// Phase 3: Target skip functionality and scheduler updates tests
func TestExptAppendExec_ScheduleStart_WithTargetSkip(t *testing.T) {
	testCases := []struct {
		name      string
		event     *entity.ExptScheduleEvent
		expt      *entity.Experiment
		mockSetup func(mockRepo *mock_repo.MockIExperimentRepo)
		wantErr   bool
	}{
		{
			name: "experiment_with_target_normal_processing",
			event: &entity.ExptScheduleEvent{
				ExptID:  1,
				SpaceID: 100,
			},
			expt: &entity.Experiment{
				ID:           1,
				SpaceID:      100,
				Status:       entity.ExptStatus_Processing,
				TargetID:     10,
				TargetType:   entity.EvalTargetTypeLoopPrompt,
				MaxAliveTime: 3600000,                                    // 1 hour
				StartAt:      gptr.Of(time.Now().Add(-30 * time.Minute)), // Started 30 minutes ago
			},
			mockSetup: func(mockRepo *mock_repo.MockIExperimentRepo) {
				// No update expected as experiment is within time limit
			},
			wantErr: false,
		},
		{
			name: "experiment_without_target_skip_processing",
			event: &entity.ExptScheduleEvent{
				ExptID:  1,
				SpaceID: 100,
			},
			expt: &entity.Experiment{
				ID:           1,
				SpaceID:      100,
				Status:       entity.ExptStatus_Processing,
				TargetID:     0, // No target - should be skipped
				TargetType:   0,
				MaxAliveTime: 3600000,
				StartAt:      gptr.Of(time.Now().Add(-30 * time.Minute)),
			},
			mockSetup: func(mockRepo *mock_repo.MockIExperimentRepo) {
				// No update expected as experiment without target continues normally
			},
			wantErr: false,
		},
		{
			name: "experiment_max_alive_time_exceeded",
			event: &entity.ExptScheduleEvent{
				ExptID:  1,
				SpaceID: 100,
			},
			expt: &entity.Experiment{
				ID:           1,
				SpaceID:      100,
				Status:       entity.ExptStatus_Processing,
				TargetID:     10,
				TargetType:   entity.EvalTargetTypeLoopPrompt,
				MaxAliveTime: 3600000,                                 // 1 hour
				StartAt:      gptr.Of(time.Now().Add(-2 * time.Hour)), // Started 2 hours ago
			},
			mockSetup: func(mockRepo *mock_repo.MockIExperimentRepo) {
				mockRepo.EXPECT().
					Update(gomock.Any(), &entity.Experiment{
						ID:      1,
						SpaceID: 100,
						Status:  entity.ExptStatus_Draining,
					}).
					Return(nil)
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRepo := mock_repo.NewMockIExperimentRepo(ctrl)
			tc.mockSetup(mockRepo)

			exec := &ExptAppendExec{
				exptRepo: mockRepo,
			}

			err := exec.ScheduleStart(context.Background(), tc.event, tc.expt)
			if (err != nil) != tc.wantErr {
				t.Errorf("ScheduleStart() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestExptAppendExec_ExptEnd_WithTargetSkip(t *testing.T) {
	type mockFields struct {
		manager   *svcmocks.MockIExptManager
		idem      *idemmocks.MockIdempotentService
		configer  *configmocks.MockIConfiger
		itemRepo  *mock_repo.MockIExptItemResultRepo
		publisher *eventmocks.MockExptEventPublisher
	}

	testCases := []struct {
		name       string
		event      *entity.ExptScheduleEvent
		expt       *entity.Experiment
		toSubmit   int
		incomplete int
		mockSetup  func(f *mockFields)
		wantTick   bool
		wantErr    bool
	}{
		{
			name: "experiment_without_target_draining_complete",
			event: &entity.ExptScheduleEvent{
				ExptID:      1,
				ExptRunID:   2,
				SpaceID:     100,
				ExptRunMode: entity.EvaluationModeAppend,
				Session:     &entity.Session{UserID: "user1"},
			},
			expt: &entity.Experiment{
				ID:         1,
				SpaceID:    100,
				Status:     entity.ExptStatus_Draining,
				TargetID:   0, // No target
				TargetType: 0,
			},
			toSubmit:   0,
			incomplete: 0,
			mockSetup: func(f *mockFields) {
				f.idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, nil)
				f.manager.EXPECT().CompleteRun(gomock.Any(), int64(1), int64(2), entity.EvaluationModeAppend, int64(100), gomock.Any(), gomock.Any()).Return(nil)
				f.manager.EXPECT().CompleteExpt(gomock.Any(), int64(1), int64(100), gomock.Any(), gomock.Any()).Return(nil)
				f.configer.EXPECT().GetExptExecConf(gomock.Any(), int64(100)).Return(&entity.ExptExecConf{ZombieIntervalSecond: 60})
				f.idem.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			wantTick: false,
			wantErr:  false,
		},
		{
			name: "experiment_without_target_still_processing",
			event: &entity.ExptScheduleEvent{
				ExptID:      1,
				ExptRunID:   2,
				SpaceID:     100,
				ExptRunMode: entity.EvaluationModeAppend,
				Session:     &entity.Session{UserID: "user1"},
			},
			expt: &entity.Experiment{
				ID:         1,
				SpaceID:    100,
				Status:     entity.ExptStatus_Processing,
				TargetID:   0, // No target
				TargetType: 0,
			},
			toSubmit:   0,
			incomplete: 1,
			mockSetup: func(f *mockFields) {
				// No mock setup needed as function should return early
			},
			wantTick: true,
			wantErr:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			f := &mockFields{
				manager:   svcmocks.NewMockIExptManager(ctrl),
				idem:      idemmocks.NewMockIdempotentService(ctrl),
				configer:  configmocks.NewMockIConfiger(ctrl),
				itemRepo:  mock_repo.NewMockIExptItemResultRepo(ctrl),
				publisher: eventmocks.NewMockExptEventPublisher(ctrl),
			}

			if tc.mockSetup != nil {
				tc.mockSetup(f)
			}

			exec := &ExptAppendExec{
				manager:            f.manager,
				idem:               f.idem,
				configer:           f.configer,
				exptItemResultRepo: f.itemRepo,
				publisher:          f.publisher,
			}

			nextTick, err := exec.ExptEnd(context.Background(), tc.event, tc.expt, tc.toSubmit, tc.incomplete)

			if (err != nil) != tc.wantErr {
				t.Errorf("ExptEnd() error = %v, wantErr %v", err, tc.wantErr)
			}

			if nextTick != tc.wantTick {
				t.Errorf("ExptEnd() nextTick = %v, want %v", nextTick, tc.wantTick)
			}
		})
	}
}

func TestDefaultSchedulerModeFactory_NewSchedulerMode_Integration(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create all required mocks
	mockManager := svcmocks.NewMockIExptManager(ctrl)
	mockItemRepo := mock_repo.NewMockIExptItemResultRepo(ctrl)
	mockStatsRepo := mock_repo.NewMockIExptStatsRepo(ctrl)
	mockTurnRepo := mock_repo.NewMockIExptTurnResultRepo(ctrl)
	mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockEvalSetItemService := svcmocks.NewMockEvaluationSetItemService(ctrl)
	mockExptRepo := mock_repo.NewMockIExperimentRepo(ctrl)
	mockIdem := idemmocks.NewMockIdempotentService(ctrl)
	mockConfiger := configmocks.NewMockIConfiger(ctrl)
	mockPublisher := eventmocks.NewMockExptEventPublisher(ctrl)
	mockEvaluatorRecordService := svcmocks.NewMockEvaluatorRecordService(ctrl)
	mockResultSvc := svcmocks.NewMockExptResultService(ctrl)

	factory := NewSchedulerModeFactory(
		mockManager,
		mockItemRepo,
		mockStatsRepo,
		mockTurnRepo,
		mockIDGen,
		mockEvalSetItemService,
		mockExptRepo,
		mockIdem,
		mockConfiger,
		mockPublisher,
		mockEvaluatorRecordService,
		mockResultSvc,
	)

	tests := []struct {
		name         string
		mode         entity.ExptRunMode
		expectedType string
		wantErr      bool
	}{
		{
			name:         "submit_mode_success",
			mode:         entity.EvaluationModeSubmit,
			expectedType: "*service.ExptSubmitExec",
			wantErr:      false,
		},
		{
			name:         "fail_retry_mode_success",
			mode:         entity.EvaluationModeFailRetry,
			expectedType: "*service.ExptFailRetryExec",
			wantErr:      false,
		},
		{
			name:         "append_mode_success",
			mode:         entity.EvaluationModeAppend,
			expectedType: "*service.ExptAppendExec",
			wantErr:      false,
		},
		{
			name:         "unknown_mode_error",
			mode:         entity.ExptRunMode(999),
			expectedType: "",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheduler, err := factory.NewSchedulerMode(tt.mode)

			if (err != nil) != tt.wantErr {
				t.Errorf("NewSchedulerMode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if scheduler == nil {
					t.Errorf("NewSchedulerMode() returned nil scheduler")
					return
				}

				// Verify the returned scheduler mode matches the input
				if scheduler.Mode() != tt.mode {
					t.Errorf("NewSchedulerMode() returned scheduler with mode %v, want %v", scheduler.Mode(), tt.mode)
				}

				// Verify the type is correct
				actualType := fmt.Sprintf("%T", scheduler)
				if actualType != tt.expectedType {
					t.Errorf("NewSchedulerMode() returned type %v, want %v", actualType, tt.expectedType)
				}
			}
		})
	}
}
