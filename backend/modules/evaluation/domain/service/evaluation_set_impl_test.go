// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

// 假设存在一个模拟的 DatasetRPCAdapter
func TestCreateEvaluationSet(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 创建模拟的 DatasetRPCAdapter
	mockAdapter := mocks.NewMockIDatasetRPCAdapter(ctrl)

	// 创建 EvaluationSetServiceImpl 实例
	service := &EvaluationSetServiceImpl{
		datasetRPCAdapter: mockAdapter,
	}

	// 定义测试用例
	testCases := []struct {
		name        string
		param       *entity.CreateEvaluationSetParam
		expectedID  int64
		expectedErr error
		mockSetup   func()
	}{
		{
			name: "成功创建评估集",
			param: &entity.CreateEvaluationSetParam{
				SpaceID:             1,
				Name:                "Test Set",
				Description:         func(s string) *string { return &s }("This is a test set"),
				EvaluationSetSchema: &entity.EvaluationSetSchema{},
			},
			expectedID:  123,
			expectedErr: nil,
			mockSetup: func() {
				mockAdapter.EXPECT().CreateDataset(gomock.Any(), &rpc.CreateDatasetParam{
					SpaceID:            1,
					Name:               "Test Set",
					Desc:               func(s string) *string { return &s }("This is a test set"),
					EvaluationSetItems: &entity.EvaluationSetSchema{},
				}).Return(int64(123), nil)
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

			id, err := service.CreateEvaluationSet(context.Background(), tc.param)

			if id != tc.expectedID {
				t.Errorf("期望 ID 为 %d, 但得到 %d", tc.expectedID, id)
			}

			if (err == nil && tc.expectedErr != nil) || (err != nil && tc.expectedErr == nil) {
				t.Errorf("期望错误为 %v, 但得到 %v", tc.expectedErr, err)
			}
		})
	}
}

func TestBatchGetEvaluationSets(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRPCAdapter := mocks.NewMockIDatasetRPCAdapter(ctrl)
	serviceImpl := NewEvaluationSetServiceImpl(mockRPCAdapter)

	spaceID := int64(1)
	evaluationSetIDs := []int64{1, 2, 3}
	deletedAt := false

	expectedSets := []*entity.EvaluationSet{
		{}, {}, {},
	}
	// 模拟成功情况
	mockRPCAdapter.EXPECT().BatchGetDatasets(context.Background(), &spaceID, evaluationSetIDs, &deletedAt).Return(expectedSets, nil)
	sets, err := serviceImpl.BatchGetEvaluationSets(context.Background(), &spaceID, evaluationSetIDs, &deletedAt)
	if err != nil {
		t.Errorf("BatchGetEvaluationSets failed with error: %v", err)
	}
	if len(sets) != len(expectedSets) {
		t.Errorf("Expected %d sets, but got %d", len(expectedSets), len(sets))
	}
}

func TestEvaluationSetServiceImpl_UpdateEvaluationSet(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDatasetRPCAdapter := mocks.NewMockIDatasetRPCAdapter(ctrl)

	// Instantiate the service with the mock adapter
	service := &EvaluationSetServiceImpl{
		datasetRPCAdapter: mockDatasetRPCAdapter,
	}

	ctx := context.Background() // Standard context for tests

	// Common test data
	testSpaceID := int64(1001)
	testEvaluationSetID := int64(2002)
	testName := "Updated Dataset Name"
	testDescription := "Updated Dataset Description"

	// Define test cases
	tests := []struct {
		name        string
		param       *entity.UpdateEvaluationSetParam
		mockSetup   func(adapter *mocks.MockIDatasetRPCAdapter) // Function to set up mock expectations
		wantErr     bool
		expectedErr error // Used if we want to match a specific error instance
		wantErrCode int32 // Used if we expect an errorx error with a specific code
	}{
		{
			name:  "参数为nil时返回内部错误",
			param: nil,
			mockSetup: func(adapter *mocks.MockIDatasetRPCAdapter) {
				// UpdateDataset should not be called in this case
			},
			wantErr:     true,
			wantErrCode: errno.CommonInternalErrorCode,
		},
		{
			name: "成功更新评估集",
			param: &entity.UpdateEvaluationSetParam{
				SpaceID:         testSpaceID,
				EvaluationSetID: testEvaluationSetID,
				Name:            gptr.Of(testName),
				Description:     gptr.Of(testDescription),
			},
			mockSetup: func(adapter *mocks.MockIDatasetRPCAdapter) {
				adapter.EXPECT().UpdateDataset(
					ctx,
					testSpaceID,
					testEvaluationSetID,
					gptr.Of(testName),
					gptr.Of(testDescription),
				).Return(nil).Times(1) // Expect one call, returning no error
			},
			wantErr: false,
		},
		{
			name: "成功更新评估集 - 名称和描述为nil",
			param: &entity.UpdateEvaluationSetParam{
				SpaceID:         testSpaceID,
				EvaluationSetID: testEvaluationSetID,
				Name:            nil, // Name is nil
				Description:     nil, // Description is nil
			},
			mockSetup: func(adapter *mocks.MockIDatasetRPCAdapter) {
				adapter.EXPECT().UpdateDataset(
					ctx,
					testSpaceID,
					testEvaluationSetID,
					nil, // Expecting nil name
					nil, // Expecting nil description
				).Return(nil).Times(1)
			},
			wantErr: false,
		},
		{
			name: "成功更新评估集 - 仅名称",
			param: &entity.UpdateEvaluationSetParam{
				SpaceID:         testSpaceID,
				EvaluationSetID: testEvaluationSetID,
				Name:            gptr.Of(testName),
				Description:     nil,
			},
			mockSetup: func(adapter *mocks.MockIDatasetRPCAdapter) {
				adapter.EXPECT().UpdateDataset(
					ctx,
					testSpaceID,
					testEvaluationSetID,
					gptr.Of(testName),
					nil,
				).Return(nil).Times(1)
			},
			wantErr: false,
		},
		{
			name: "成功更新评估集 - 仅描述",
			param: &entity.UpdateEvaluationSetParam{
				SpaceID:         testSpaceID,
				EvaluationSetID: testEvaluationSetID,
				Name:            nil,
				Description:     gptr.Of(testDescription),
			},
			mockSetup: func(adapter *mocks.MockIDatasetRPCAdapter) {
				adapter.EXPECT().UpdateDataset(
					ctx,
					testSpaceID,
					testEvaluationSetID,
					nil,
					gptr.Of(testDescription),
				).Return(nil).Times(1)
			},
			wantErr: false,
		},
		{
			name: "RPC适配器UpdateDataset返回错误",
			param: &entity.UpdateEvaluationSetParam{
				SpaceID:         testSpaceID,
				EvaluationSetID: testEvaluationSetID,
				Name:            gptr.Of(testName),
				Description:     gptr.Of(testDescription),
			},
			mockSetup: func(adapter *mocks.MockIDatasetRPCAdapter) {
				adapter.EXPECT().UpdateDataset(
					ctx,
					testSpaceID,
					testEvaluationSetID,
					gptr.Of(testName),
					gptr.Of(testDescription),
				).Return(errors.New("RPC call failed")).Times(1) // Expect one call, returning an error
			},
			wantErr:     true,
			expectedErr: errors.New("RPC call failed"),
		},
		{
			name: "RPC适配器UpdateDataset返回errorx错误",
			param: &entity.UpdateEvaluationSetParam{
				SpaceID:         testSpaceID,
				EvaluationSetID: testEvaluationSetID,
				Name:            gptr.Of(testName),
				Description:     gptr.Of(testDescription),
			},
			mockSetup: func(adapter *mocks.MockIDatasetRPCAdapter) {
				adapter.EXPECT().UpdateDataset(
					ctx,
					testSpaceID,
					testEvaluationSetID,
					gptr.Of(testName),
					gptr.Of(testDescription),
				).Return(errorx.NewByCode(errno.CommonRPCErrorCode)).Times(1)
			},
			wantErr:     true,
			wantErrCode: errno.CommonRPCErrorCode,
		},
	}

	// Iterate over test cases
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock expectations for this specific test case
			if tt.mockSetup != nil {
				tt.mockSetup(mockDatasetRPCAdapter)
			}

			// Call the method under test
			err := service.UpdateEvaluationSet(ctx, tt.param)

			// Assertions
			if tt.wantErr {
				assert.Error(t, err, "Expected an error")
				if tt.expectedErr != nil {
					// assert.EqualError(t, err, tt.expectedErr.Error(), "Error message mismatch")
					// For more robust error comparison, especially with custom error types or wrapped errors,
					// checking specific fields or using errors.Is/As might be better.
					// Here, we directly compare the error if it's a simple one.
					assert.Equal(t, tt.expectedErr, err, "Error instance mismatch")
				}
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok, "Error should be a status error from errorx")
					if ok {
						assert.Equal(t, tt.wantErrCode, statusErr.Code(), "Error code mismatch")
					}
				}
			} else {
				assert.NoError(t, err, "Expected no error")
			}
		})
	}
}

func TestEvaluationSetServiceImpl_DeleteEvaluationSet(t *testing.T) {
	// 创建 gomock 控制器，用于管理 mock 对象的生命周期和期望
	ctrl := gomock.NewController(t)
	// defer ctrl.Finish() 会在测试函数结束时检查所有 mock 的期望是否都已满足
	defer ctrl.Finish()
	evaluationSetServiceOnce = sync.Once{}
	// 创建 IDatasetRPCAdapter 接口的 mock 实现
	mockDatasetRPCAdapter := mocks.NewMockIDatasetRPCAdapter(ctrl)

	// 创建被测服务实例，并注入 mock adapter
	// 重要前提: 此处假设 service 包中存在 NewEvaluationSetServiceImpl 构造函数，如下所示：
	// func NewEvaluationSetServiceImpl(adapter rpc.IDatasetRPCAdapter) *EvaluationSetServiceImpl {
	//	 return &EvaluationSetServiceImpl{datasetRPCAdapter: adapter}
	// }
	// 如果该构造函数不存在，则需要添加到 service 包中，否则此测试无法正确进行依赖注入。
	serviceImpl := NewEvaluationSetServiceImpl(mockDatasetRPCAdapter)

	// 定义测试用例的结构体
	tests := []struct {
		name            string // 测试用例名称
		spaceID         int64  // 输入参数 spaceID
		evaluationSetID int64  // 输入参数 evaluationSetID
		setupMock       func() // 用于设置当前测试用例的 mock 期望的函数
		wantErr         bool   // 期望是否发生错误
		expectedErr     error  // 期望的错误对象（如果 wantErr 为 true）
	}{
		{
			name:            "成功删除评估集",
			spaceID:         int64(12345),
			evaluationSetID: int64(67890),
			setupMock: func() {
				// 设置 mockDatasetRPCAdapter.DeleteDataset 方法的期望
				// 当以任意 context.Context、指定的 spaceID 和 evaluationSetID 调用时
				// 期望返回 nil (表示成功)
				mockDatasetRPCAdapter.EXPECT().DeleteDataset(
					gomock.Any(), // context.Context 参数，使用 gomock.Any() 匹配任意上下文
					gomock.Any(), // spaceID 参数
					gomock.Any(), // evaluationSetID 参数
				).Return(nil) // 模拟成功删除，返回 nil 错误
			},
			wantErr:     false, // 期望不发生错误
			expectedErr: nil,   // 期望错误为 nil
		},
		{
			name:            "删除评估集失败 - RPC调用返回错误",
			spaceID:         111,
			evaluationSetID: 222,
			setupMock: func() {
				// 设置 mockDatasetRPCAdapter.DeleteDataset 方法的期望
				// 当以任意 context.Context、指定的 spaceID 和 evaluationSetID 调用时
				// 期望返回一个指定的错误
				mockDatasetRPCAdapter.EXPECT().DeleteDataset(
					gomock.Any(),
					int64(111),
					int64(222),
				).Return(errors.New("RPC communication error")) // 模拟 RPC 调用失败
			},
			wantErr:     true,                                  // 期望发生错误
			expectedErr: errors.New("RPC communication error"), // 期望具体的错误信息
		},
		{
			name:            "删除评估集失败 - 另一个RPC错误场景",
			spaceID:         333,
			evaluationSetID: 444,
			setupMock: func() {
				mockDatasetRPCAdapter.EXPECT().DeleteDataset(
					gomock.Any(),
					int64(333),
					int64(444),
				).Return(errors.New("dataset not found via RPC")) // 模拟另一种 RPC 错误
			},
			wantErr:     true,
			expectedErr: errors.New("dataset not found via RPC"),
		},
	}

	// 遍历所有测试用例
	for _, tt := range tests {
		// 使用 t.Run 为每个测试用例创建一个子测试，使其独立运行
		t.Run(tt.name, func(t *testing.T) {
			// 调用为当前测试用例设置 mock 期望的函数
			tt.setupMock()

			// 执行被测方法
			err := serviceImpl.DeleteEvaluationSet(context.Background(), tt.spaceID, tt.evaluationSetID)

			// 断言错误情况
			if tt.wantErr {
				// 如果期望发生错误，断言确实发生了错误
				assert.Error(t, err, "期望应返回错误")
				if tt.expectedErr != nil {
					// 如果期望了特定的错误信息，断言错误信息一致
					assert.EqualError(t, err, tt.expectedErr.Error(), "错误信息不匹配")
				}
			} else {
				// 如果不期望发生错误，断言没有发生错误
				assert.NoError(t, err, "期望不应返回错误")
			}
		})
	}
}

func TestEvaluationSetServiceImpl_GetEvaluationSet(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	evaluationSetServiceOnce = sync.Once{}
	mockDatasetRPCAdapter := mocks.NewMockIDatasetRPCAdapter(ctrl)
	service := NewEvaluationSetServiceImpl(mockDatasetRPCAdapter)

	tests := []struct {
		name        string
		spaceID     *int64
		evalSetID   int64
		deletedAt   *bool
		mockSetup   func()
		wantEvalSet *entity.EvaluationSet
		wantErr     bool
		wantErrCode int32
	}{
		{
			name:      "成功获取评估集",
			spaceID:   gptr.Of[int64](123),
			evalSetID: 456,
			deletedAt: gptr.Of(false),
			mockSetup: func() {
				mockDatasetRPCAdapter.EXPECT().
					GetDataset(gomock.Any(), gptr.Of[int64](123), int64(456), gptr.Of(false)).
					Return(&entity.EvaluationSet{
						ID:          456,
						SpaceID:     123,
						Name:        "Test Set",
						Description: "Test Description",
					}, nil)
			},
			wantEvalSet: &entity.EvaluationSet{
				ID:          456,
				SpaceID:     123,
				Name:        "Test Set",
				Description: "Test Description",
			},
			wantErr: false,
		},
		{
			name:      "RPC调用失败",
			spaceID:   gptr.Of[int64](123),
			evalSetID: 456,
			mockSetup: func() {
				mockDatasetRPCAdapter.EXPECT().
					GetDataset(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			wantErr:     true,
			wantErrCode: errno.CommonInternalErrorCode,
		},
		{
			name:      "评估集不存在",
			spaceID:   gptr.Of[int64](123),
			evalSetID: 789,
			mockSetup: func() {
				mockDatasetRPCAdapter.EXPECT().
					GetDataset(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, nil)
			},
			wantEvalSet: nil,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			gotEvalSet, err := service.GetEvaluationSet(context.Background(), tt.spaceID, tt.evalSetID, tt.deletedAt)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.wantErrCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantEvalSet, gotEvalSet)
			}
		})
	}
}

func TestEvaluationSetServiceImpl_ListEvaluationSets(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	evaluationSetServiceOnce = sync.Once{}
	mockDatasetRPCAdapter := mocks.NewMockIDatasetRPCAdapter(ctrl)
	service := NewEvaluationSetServiceImpl(mockDatasetRPCAdapter)

	tests := []struct {
		name          string
		param         *entity.ListEvaluationSetsParam
		mockSetup     func()
		wantSets      []*entity.EvaluationSet
		wantTotal     *int64
		wantNextToken *string
		wantErr       bool
		wantErrCode   int32
	}{
		{
			name: "成功获取列表",
			param: &entity.ListEvaluationSetsParam{
				SpaceID:    int64(123),
				PageNumber: gptr.Of[int32](1),
				PageSize:   gptr.Of[int32](10),
				Name:       gptr.Of("test"),
			},
			mockSetup: func() {
				mockDatasetRPCAdapter.EXPECT().
					ListDatasets(gomock.Any(), &rpc.ListDatasetsParam{
						SpaceID:    int64(123),
						PageNumber: gptr.Of[int32](1),
						PageSize:   gptr.Of[int32](10),
						Name:       gptr.Of("test"),
					}).
					Return([]*entity.EvaluationSet{
						{
							ID:      1,
							SpaceID: 123,
							Name:    "Test Set 1",
						},
					}, gptr.Of[int64](1), gptr.Of("next_token"), nil)
			},
			wantSets: []*entity.EvaluationSet{
				{
					ID:      1,
					SpaceID: 123,
					Name:    "Test Set 1",
				},
			},
			wantTotal:     gptr.Of[int64](1),
			wantNextToken: gptr.Of("next_token"),
			wantErr:       false,
		},
		{
			name:  "参数为空",
			param: nil,
			mockSetup: func() {
				// 参数为空不应该调用RPC
			},
			wantErr:     true,
			wantErrCode: errno.CommonInternalErrorCode,
		},
		{
			name: "RPC调用失败",
			param: &entity.ListEvaluationSetsParam{
				SpaceID:    int64(123),
				PageNumber: gptr.Of[int32](1),
				PageSize:   gptr.Of[int32](10),
			},
			mockSetup: func() {
				mockDatasetRPCAdapter.EXPECT().
					ListDatasets(gomock.Any(), gomock.Any()).
					Return(nil, nil, nil, errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			wantErr:     true,
			wantErrCode: errno.CommonInternalErrorCode,
		},
		{
			name: "空结果",
			param: &entity.ListEvaluationSetsParam{
				SpaceID:    int64(123),
				PageNumber: gptr.Of[int32](1),
				PageSize:   gptr.Of[int32](10),
			},
			mockSetup: func() {
				mockDatasetRPCAdapter.EXPECT().
					ListDatasets(gomock.Any(), gomock.Any()).
					Return([]*entity.EvaluationSet{}, gptr.Of[int64](0), nil, nil)
			},
			wantSets:  []*entity.EvaluationSet{},
			wantTotal: gptr.Of[int64](0),
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			gotSets, gotTotal, gotNextToken, err := service.ListEvaluationSets(context.Background(), tt.param)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.wantErrCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantSets, gotSets)
				assert.Equal(t, tt.wantTotal, gotTotal)
				assert.Equal(t, tt.wantNextToken, gotNextToken)
			}
		})
	}
}

// ---------------- 追加：CreateEvaluationSetWithImport 与 ParseImportSourceFile 的单测 ----------------

func TestCreateEvaluationSetWithImport(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAdapter := mocks.NewMockIDatasetRPCAdapter(ctrl)
	service := &EvaluationSetServiceImpl{datasetRPCAdapter: mockAdapter}

	desc := "导入评估集"
	schema := &entity.EvaluationSetSchema{}
	sourceType := entity.SetSourceType_File
	source := &entity.DatasetIOEndpoint{
		File: &entity.DatasetIOFile{
			Provider: entity.StorageProvider_S3,
			Path:     "s3://bucket/data.csv",
			Format:   gptr.Of(entity.FileFormat_CSV),
		},
	}
	fieldMappings := []*entity.FieldMapping{{Source: "input", Target: "question"}}

	tests := []struct {
		name        string
		param       *entity.CreateEvaluationSetWithImportParam
		expectedID  int64
		expectedJob int64
		wantErr     bool
		wantErrCode int32
		mockSetup   func()
	}{
		{
			name: "成功创建评估集（包含导入源）",
			param: &entity.CreateEvaluationSetWithImportParam{
				SpaceID:             1,
				Name:                "Set With Import",
				Description:         &desc,
				EvaluationSetSchema: schema,
				SourceType:          gptr.Of(sourceType),
				Source:              source,
				FieldMappings:       fieldMappings,
			},
			expectedID:  101,
			expectedJob: 202,
			wantErr:     false,
			mockSetup: func() {
				mockAdapter.EXPECT().CreateDatasetWithImport(gomock.Any(), &rpc.CreateDatasetWithImportParam{
					SpaceID:            1,
					Name:               "Set With Import",
					Desc:               &desc,
					EvaluationSetItems: schema,
					SourceType:         gptr.Of(sourceType),
					Source:             source,
					FieldMappings:      fieldMappings,
				}).Return(int64(101), int64(202), nil)
			},
		},
		{
			name:        "参数为空",
			param:       nil,
			expectedID:  0,
			expectedJob: 0,
			wantErr:     true,
			wantErrCode: errno.CommonInternalErrorCode,
			mockSetup:   func() {},
		},
		{
			name: "RPC错误",
			param: &entity.CreateEvaluationSetWithImportParam{
				SpaceID:             9,
				Name:                "bad",
				Description:         &desc,
				EvaluationSetSchema: schema,
				SourceType:          gptr.Of(sourceType),
				Source:              source,
			},
			wantErr:     true,
			wantErrCode: errno.CommonRPCErrorCode,
			mockSetup: func() {
				mockAdapter.EXPECT().CreateDatasetWithImport(gomock.Any(), gomock.Any()).Return(int64(0), int64(0), errorx.NewByCode(errno.CommonRPCErrorCode))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mockSetup != nil {
				tt.mockSetup()
			}
			id, jobID, err := service.CreateEvaluationSetWithImport(context.Background(), tt.param)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.wantErrCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedID, id)
				assert.Equal(t, tt.expectedJob, jobID)
			}
		})
	}
}

func TestParseImportSourceFile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAdapter := mocks.NewMockIDatasetRPCAdapter(ctrl)
	service := &EvaluationSetServiceImpl{datasetRPCAdapter: mockAdapter}

	file := &entity.DatasetIOFile{
		Provider: entity.StorageProvider_S3,
		Path:     "s3://bucket/data.jsonl",
		Format:   gptr.Of(entity.FileFormat_JSONL),
	}
	param := &entity.ParseImportSourceFileParam{SpaceID: 1, File: file}

	expected := &entity.ParseImportSourceFileResult{Bytes: 1024, FieldSchemas: []*entity.FieldSchema{}, Conflicts: nil, FilesWithAmbiguousColumn: nil}

	t.Run("成功解析导入文件", func(t *testing.T) {
		mockAdapter.EXPECT().ParseImportSourceFile(gomock.Any(), param).Return(expected, nil)
		res, err := service.ParseImportSourceFile(context.Background(), param)
		assert.NoError(t, err)
		assert.Equal(t, expected, res)
	})
}
