// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func TestNewEvaluationSetVersionServiceImpl(t *testing.T) {
	// 重置单例，确保测试的独立性
	evaluationSetVersionServiceOnce = sync.Once{}
	evaluationSetVersionServiceImpl = nil

	tests := []struct {
		name string
		test func(t *testing.T)
	}{
		{
			name: "首次初始化服务",
			test: func(t *testing.T) {
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				mockDatasetRPCAdapter := mocks.NewMockIDatasetRPCAdapter(ctrl)
				service := NewEvaluationSetVersionServiceImpl(mockDatasetRPCAdapter)

				// 验证返回的服务实例不为空
				assert.NotNil(t, service)

				// 验证返回的是单例实例
				assert.Equal(t, evaluationSetVersionServiceImpl, service)

				// 验证服务实例的类型
				_, ok := service.(*EvaluationSetVersionServiceImpl)
				assert.True(t, ok)

				// 验证依赖注入是否正确
				impl := service.(*EvaluationSetVersionServiceImpl)
				assert.Equal(t, mockDatasetRPCAdapter, impl.datasetRPCAdapter)
			},
		},
		{
			name: "传入空adapter时仍能正常初始化",
			test: func(t *testing.T) {
				// 重置单例
				evaluationSetVersionServiceOnce = sync.Once{}
				evaluationSetVersionServiceImpl = nil

				// 使用 nil adapter 初始化
				service := NewEvaluationSetVersionServiceImpl(nil)

				// 验证返回的服务实例不为空
				assert.NotNil(t, service)

				// 验证服务实例的类型
				impl, ok := service.(*EvaluationSetVersionServiceImpl)
				assert.True(t, ok)

				// 验证 adapter 为空
				assert.Nil(t, impl.datasetRPCAdapter)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.test)
	}
}

// 假设存在一个模拟的 DatasetRPCAdapter
func TestCreateEvaluationSetVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 创建模拟的 DatasetRPCAdapter
	mockAdapter := mocks.NewMockIDatasetRPCAdapter(ctrl)

	// 创建 EvaluationSetVersionServiceImpl 实例
	service := &EvaluationSetVersionServiceImpl{
		datasetRPCAdapter: mockAdapter,
	}

	// 定义测试用例
	testCases := []struct {
		name        string
		param       *entity.CreateEvaluationSetVersionParam
		expectedID  int64
		expectedErr error
		mockSetup   func()
	}{
		{
			name: "成功创建评估集版本",
			param: &entity.CreateEvaluationSetVersionParam{
				SpaceID:         1,
				EvaluationSetID: 1,
				Version:         "1.0",
				Description:     func(s string) *string { return &s }("This is a test version"),
			},
			expectedID:  123,
			expectedErr: nil,
			mockSetup: func() {
				mockAdapter.EXPECT().CreateDatasetVersion(gomock.Any(), int64(1), int64(1), "1.0", gomock.Any()).Return(int64(123), nil)
			},
		},
		{
			name:        "参数为空",
			param:       nil,
			expectedID:  0,
			expectedErr: errorx.NewByCode(errno.CommonInternalErrorCode),
			mockSetup:   func() {},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.mockSetup()

			id, err := service.CreateEvaluationSetVersion(context.Background(), tc.param)

			if id != tc.expectedID {
				t.Errorf("期望 ID 为 %d, 但得到 %d", tc.expectedID, id)
			}

			if (err == nil && tc.expectedErr != nil) || (err != nil && tc.expectedErr == nil) {
				t.Errorf("期望错误为 %v, 但得到 %v", tc.expectedErr, err)
			}
		})
	}
}

func TestGetEvaluationSetVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAdapter := mocks.NewMockIDatasetRPCAdapter(ctrl)
	service := &EvaluationSetVersionServiceImpl{
		datasetRPCAdapter: mockAdapter,
	}

	spaceID := int64(1)
	versionID := int64(1)
	deletedAt := false

	expectedVersion := &entity.EvaluationSetVersion{}
	expectedSet := &entity.EvaluationSet{}
	ctx := context.Background()
	// 模拟成功情况
	mockAdapter.EXPECT().GetDatasetVersion(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(expectedVersion, expectedSet, nil).AnyTimes()
	version, set, err := service.GetEvaluationSetVersion(ctx, spaceID, versionID, &deletedAt)
	if err != nil {
		t.Errorf("GetEvaluationSetVersion failed with error: %v", err)
	}
	if version != expectedVersion || set != expectedSet {
		t.Errorf("Expected version %v and set %v, but got %v and %v", expectedVersion, expectedSet, version, set)
	}
}

func TestListEvaluationSetVersions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAdapter := mocks.NewMockIDatasetRPCAdapter(ctrl)
	service := &EvaluationSetVersionServiceImpl{
		datasetRPCAdapter: mockAdapter,
	}

	param := &entity.ListEvaluationSetVersionsParam{
		SpaceID:         1,
		EvaluationSetID: 1,
	}

	expectedSets := []*entity.EvaluationSetVersion{}
	var total int64 = 0
	nextCursor := ""

	// 模拟成功情况
	mockAdapter.EXPECT().ListDatasetVersions(gomock.Any(), param.SpaceID, param.EvaluationSetID, param.PageToken, param.PageNumber, param.PageSize, param.VersionLike, gomock.Any()).Return(expectedSets, &total, &nextCursor, nil)
	sets, totalResult, nextCursorResult, err := service.ListEvaluationSetVersions(context.Background(), param)
	if err != nil {
		t.Errorf("ListEvaluationSetVersions failed with error: %v", err)
	}
	if len(sets) != len(expectedSets) || *totalResult != total || *nextCursorResult != nextCursor {
		t.Errorf("Expected sets %v, total %d, nextCursor %s, but got %v, %d, %s", expectedSets, total, nextCursor, sets, *totalResult, *nextCursorResult)
	}

	// 模拟参数为空情况
	_, _, _, err = service.ListEvaluationSetVersions(context.Background(), nil)
	if err == nil {
		t.Errorf("Expected error when param is nil, but got nil")
	}
}

func TestBatchGetEvaluationSetVersions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAdapter := mocks.NewMockIDatasetRPCAdapter(ctrl)
	service := &EvaluationSetVersionServiceImpl{
		datasetRPCAdapter: mockAdapter,
	}

	spaceID := int64(1)
	versionIDs := []int64{1, 2, 3}
	deletedAt := false

	expectedSets := []*rpc.BatchGetVersionedDatasetsResult{
		{
			Version:       &entity.EvaluationSetVersion{},
			EvaluationSet: &entity.EvaluationSet{},
		},
	}

	// 模拟成功情况
	mockAdapter.EXPECT().BatchGetVersionedDatasets(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(expectedSets, nil)
	sets, err := service.BatchGetEvaluationSetVersions(context.Background(), &spaceID, versionIDs, &deletedAt)
	if err != nil {
		t.Errorf("BatchGetEvaluationSetVersions failed with error: %v", err)
	}
	if len(sets) != len(expectedSets) {
		t.Errorf("Expected %d sets, but got %d", len(expectedSets), len(sets))
	}
}
