// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"testing"

	"go.uber.org/mock/gomock"

	dbmock "github.com/coze-dev/coze-loop/backend/infra/db/mocks"
	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/component/conf"
	confmocks "github.com/coze-dev/coze-loop/backend/modules/data/domain/component/conf/mocks"
	mqmock "github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/component/mq/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/entity"
	mock_repo "github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/pagination"
)

func TestDatasetServiceImpl_RunSnapshotItemJob(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repo.NewMockIDatasetAPI(ctrl)
	mockProvider := dbmock.NewMockProvider(ctrl)
	mockIConfig := confmocks.NewMockIConfig(ctrl)
	mockILocker := lockmocks.NewMockILocker(ctrl)
	mockIDatasetJobPublisher := mqmock.NewMockIDatasetJobPublisher(ctrl)
	service := &DatasetServiceImpl{
		repo:     mockRepo,
		txDB:     mockProvider,
		retryCfg: mockIConfig.GetSnapshotRetry,
		locker:   mockILocker,
		producer: mockIDatasetJobPublisher,
	}

	// 定义测试用例
	tests := []struct {
		name        string
		msg         *entity.JobRunMessage
		mockRepo    func()
		expectedErr bool
	}{
		{
			name: "正常场景",
			msg: &entity.JobRunMessage{
				Type: entity.DatasetSnapshotJob,
				Extra: map[string]string{
					"version_id": "1",
				},
			},
			mockRepo: func() {
				mockVersion := &entity.DatasetVersion{
					ID: 1,
				}
				mockRepo.EXPECT().GetVersion(context.Background(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mockVersion, nil)
				mockIConfig.EXPECT().GetSnapshotRetry().Return(&conf.SnapshotRetry{}).MaxTimes(2)
				mockILocker.EXPECT().LockBackoffWithRenew(context.Background(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
				mockRepo.EXPECT().ListItems(context.Background(), gomock.Any()).Return([]*entity.Item{}, &pagination.PageResult{}, nil)
				mockRepo.EXPECT().BatchUpsertItemSnapshots(gomock.Any(), gomock.Any()).Return(int64(0), nil)
				mockRepo.EXPECT().CountItemSnapshots(context.Background(), gomock.Any(), gomock.Any()).Return(int64(0), nil)
				mockRepo.EXPECT().PatchVersion(context.Background(), gomock.Any(), gomock.Any()).Return(nil)
			},
			expectedErr: false,
		},
		{
			name: "获取版本失败",
			msg: &entity.JobRunMessage{
				// 填充其他必要字段
				Type: entity.DatasetSnapshotJob,
				Extra: map[string]string{
					"version_id": "1",
				},
			},
			mockRepo: func() {
				mockIConfig.EXPECT().GetSnapshotRetry().Return(&conf.SnapshotRetry{})
				mockRepo.EXPECT().GetVersion(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("获取版本失败"))
			},
			expectedErr: true,
		},
		{
			name: "执行失败",
			msg: &entity.JobRunMessage{
				// 填充其他必要字段
				Type: entity.DatasetSnapshotJob,
				Extra: map[string]string{
					"version_id": "1",
				},
			},
			mockRepo: func() {
				mockVersion := &entity.DatasetVersion{
					ID: 1,
				}
				mockRepo.EXPECT().GetVersion(context.Background(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mockVersion, nil)
				mockIConfig.EXPECT().GetSnapshotRetry().Return(&conf.SnapshotRetry{
					MaxRetryTimes: 10,
				}).AnyTimes()
				mockILocker.EXPECT().LockBackoffWithRenew(context.Background(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, fmt.Errorf("执行lock失败"))
				mockIDatasetJobPublisher.EXPECT().Send(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			expectedErr: false,
		},
		// 可以根据需要添加更多测试用例
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockRepo()
			err := service.RunSnapshotItemJob(context.Background(), tt.msg)
			if (err != nil) != tt.expectedErr {
				t.Errorf("RunSnapshotItemJob() error = %v, expectedErr %v", err, tt.expectedErr)
			}
		})
	}
}

func TestDatasetServiceImpl_commitToInProgress(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repo.NewMockIDatasetAPI(ctrl)
	service := &DatasetServiceImpl{
		repo: mockRepo,
	}

	tests := []struct {
		name        string
		processCtx  *snapshotContext
		mockRepo    func()
		expectedErr bool
	}{
		{
			name: "正常场景",
			processCtx: &snapshotContext{
				version: &entity.DatasetVersion{
					ID:             1,
					SnapshotStatus: entity.SnapshotStatusInProgress,
					UpdateVersion:  0,
				},
			},
			mockRepo: func() {
				mockRepo.EXPECT().PatchVersion(
					context.Background(),
					buildUpdateVersionPatch(&snapshotContext{version: &entity.DatasetVersion{ID: 1}}, entity.SnapshotStatusInProgress),
					buildUpdateVersionWhere(&snapshotContext{version: &entity.DatasetVersion{ID: 1}}),
				).Return(nil)
			},
			expectedErr: false,
		},
		{
			name: "PatchVersion 失败",
			processCtx: &snapshotContext{
				version: &entity.DatasetVersion{
					ID:             1,
					SnapshotStatus: entity.SnapshotStatusInProgress,
					UpdateVersion:  0,
				},
			},
			mockRepo: func() {
				mockRepo.EXPECT().PatchVersion(
					context.Background(),
					buildUpdateVersionPatch(&snapshotContext{version: &entity.DatasetVersion{ID: 1}}, entity.SnapshotStatusInProgress),
					buildUpdateVersionWhere(&snapshotContext{version: &entity.DatasetVersion{ID: 1}}),
				).Return(fmt.Errorf("PatchVersion 失败"))
			},
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockRepo()
			err := service.commitToInProgress(context.Background(), tt.processCtx)
			if (err != nil) != tt.expectedErr {
				t.Errorf("commitToInProgress() error = %v, expectedErr %v", err, tt.expectedErr)
			}
			if !tt.expectedErr {
				if tt.processCtx.version.SnapshotStatus != entity.SnapshotStatusInProgress {
					t.Errorf("期望状态为 %v, 实际为 %v", entity.SnapshotStatusInProgress, tt.processCtx.version.SnapshotStatus)
				}
				if tt.processCtx.version.UpdateVersion != 1 {
					t.Errorf("期望 UpdateVersion 为 1, 实际为 %v", tt.processCtx.version.UpdateVersion)
				}
			}
		})
	}
}

func TestDatasetServiceImpl_commitToFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repo.NewMockIDatasetAPI(ctrl)
	service := &DatasetServiceImpl{
		repo: mockRepo,
	}

	tests := []struct {
		name        string
		processCtx  *snapshotContext
		mockRepo    func()
		expectedErr bool
	}{
		{
			name: "正常场景",
			processCtx: &snapshotContext{
				version: &entity.DatasetVersion{
					ID:             1,
					SnapshotStatus: entity.SnapshotStatusInProgress,
					UpdateVersion:  0,
				},
			},
			mockRepo: func() {
				mockRepo.EXPECT().PatchVersion(
					context.Background(),
					buildUpdateVersionPatch(&snapshotContext{version: &entity.DatasetVersion{ID: 1}}, entity.SnapshotStatusFailed),
					buildUpdateVersionWhere(&snapshotContext{version: &entity.DatasetVersion{ID: 1}}),
				).Return(nil)
			},
			expectedErr: false,
		},
		{
			name: "PatchVersion 失败",
			processCtx: &snapshotContext{
				version: &entity.DatasetVersion{
					ID:             1,
					SnapshotStatus: entity.SnapshotStatusInProgress,
					UpdateVersion:  0,
				},
			},
			mockRepo: func() {
				mockRepo.EXPECT().PatchVersion(
					context.Background(),
					buildUpdateVersionPatch(&snapshotContext{version: &entity.DatasetVersion{ID: 1}}, entity.SnapshotStatusFailed),
					buildUpdateVersionWhere(&snapshotContext{version: &entity.DatasetVersion{ID: 1}}),
				).Return(fmt.Errorf("PatchVersion 失败"))
			},
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockRepo()
			err := service.commitToFailed(context.Background(), tt.processCtx)
			if (err != nil) != tt.expectedErr {
				t.Errorf("commitToFailed() error = %v, expectedErr %v", err, tt.expectedErr)
			}
			if !tt.expectedErr {
				if tt.processCtx.isFinished != true {
					t.Errorf("期望 isFinished 为 true, 实际为 %v", tt.processCtx.isFinished)
				}
				if tt.processCtx.version.SnapshotStatus != entity.SnapshotStatusFailed {
					t.Errorf("期望状态为 %v, 实际为 %v", entity.SnapshotStatusFailed, tt.processCtx.version.SnapshotStatus)
				}
				if tt.processCtx.version.UpdateVersion != 1 {
					t.Errorf("期望 UpdateVersion 为 1, 实际为 %v", tt.processCtx.version.UpdateVersion)
				}
			}
		})
	}
}
