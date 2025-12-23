// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
	"context"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	dbmocks "github.com/coze-dev/coze-loop/backend/infra/db/mocks"
	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/gorm_gen/model"
	evaluatormocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/mocks"
)

func TestEvaluatorRecordRepoImpl_CreateEvaluatorRecord(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockEvaluatorRecordDAO := evaluatormocks.NewMockEvaluatorRecordDAO(ctrl)
	mockDBProvider := dbmocks.NewMockProvider(ctrl)

	tests := []struct {
		name          string
		record        *entity.EvaluatorRecord
		mockSetup     func()
		expectedError error
	}{
		{
			name: "成功创建评估记录",
			record: &entity.EvaluatorRecord{
				ID:                 1,
				SpaceID:            1,
				EvaluatorVersionID: 1,
				ExperimentID:       1,
				ExperimentRunID:    1,
				ItemID:             1,
				TurnID:             1,
				TraceID:            "test_trace_id",
				LogID:              "test_log_id",
				Status:             entity.EvaluatorRunStatusSuccess,
				BaseInfo: &entity.BaseInfo{
					UpdatedBy: &entity.UserInfo{
						UserID: gptr.Of("test_user"),
					},
				},
			},
			mockSetup: func() {
				mockEvaluatorRecordDAO.EXPECT().
					CreateEvaluatorRecord(gomock.Any(), gomock.Any()).
					Return(nil)
			},
			expectedError: nil,
		},
		{
			name: "创建评估记录失败",
			record: &entity.EvaluatorRecord{
				ID:                 1,
				SpaceID:            1,
				EvaluatorVersionID: 1,
				ExperimentID:       1,
				ExperimentRunID:    1,
				ItemID:             1,
				TurnID:             1,
				TraceID:            "test_trace_id",
				LogID:              "test_log_id",
				Status:             entity.EvaluatorRunStatusSuccess,
				BaseInfo: &entity.BaseInfo{
					UpdatedBy: &entity.UserInfo{
						UserID: gptr.Of("test_user"),
					},
				},
			},
			mockSetup: func() {
				mockEvaluatorRecordDAO.EXPECT().
					CreateEvaluatorRecord(gomock.Any(), gomock.Any()).
					Return(assert.AnError)
			},
			expectedError: assert.AnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			repo := &EvaluatorRecordRepoImpl{
				evaluatorRecordDao: mockEvaluatorRecordDAO,
				dbProvider:         mockDBProvider,
				idgen:              mockIDGen,
			}

			err := repo.CreateEvaluatorRecord(context.Background(), tt.record)
			assert.Equal(t, tt.expectedError, err)
		})
	}
}

func TestEvaluatorRecordRepoImpl_CorrectEvaluatorRecord(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockEvaluatorRecordDAO := evaluatormocks.NewMockEvaluatorRecordDAO(ctrl)
	mockDBProvider := dbmocks.NewMockProvider(ctrl)

	tests := []struct {
		name          string
		record        *entity.EvaluatorRecord
		mockSetup     func()
		expectedError error
	}{
		{
			name: "成功修正评估记录",
			record: &entity.EvaluatorRecord{
				ID:                 1,
				SpaceID:            1,
				EvaluatorVersionID: 1,
				ExperimentID:       1,
				ExperimentRunID:    1,
				ItemID:             1,
				TurnID:             1,
				TraceID:            "test_trace_id",
				LogID:              "test_log_id",
				Status:             entity.EvaluatorRunStatusSuccess,
				BaseInfo: &entity.BaseInfo{
					UpdatedBy: &entity.UserInfo{
						UserID: gptr.Of("test_user"),
					},
				},
			},
			mockSetup: func() {
				mockEvaluatorRecordDAO.EXPECT().
					UpdateEvaluatorRecord(gomock.Any(), gomock.Any()).
					Return(nil)
			},
			expectedError: nil,
		},
		{
			name: "修正评估记录失败",
			record: &entity.EvaluatorRecord{
				ID:                 1,
				SpaceID:            1,
				EvaluatorVersionID: 1,
				ExperimentID:       1,
				ExperimentRunID:    1,
				ItemID:             1,
				TurnID:             1,
				TraceID:            "test_trace_id",
				LogID:              "test_log_id",
				Status:             entity.EvaluatorRunStatusSuccess,
				BaseInfo: &entity.BaseInfo{
					UpdatedBy: &entity.UserInfo{
						UserID: gptr.Of("test_user"),
					},
				},
			},
			mockSetup: func() {
				mockEvaluatorRecordDAO.EXPECT().
					UpdateEvaluatorRecord(gomock.Any(), gomock.Any()).
					Return(assert.AnError)
			},
			expectedError: assert.AnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			repo := &EvaluatorRecordRepoImpl{
				evaluatorRecordDao: mockEvaluatorRecordDAO,
				dbProvider:         mockDBProvider,
				idgen:              mockIDGen,
			}

			err := repo.CorrectEvaluatorRecord(context.Background(), tt.record)
			assert.Equal(t, tt.expectedError, err)
		})
	}
}

func TestEvaluatorRecordRepoImpl_GetEvaluatorRecord(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockEvaluatorRecordDAO := evaluatormocks.NewMockEvaluatorRecordDAO(ctrl)
	mockDBProvider := dbmocks.NewMockProvider(ctrl)

	tests := []struct {
		name           string
		recordID       int64
		includeDeleted bool
		mockSetup      func()
		expectedResult *entity.EvaluatorRecord
		expectedError  error
	}{
		{
			name:           "成功获取评估记录",
			recordID:       1,
			includeDeleted: false,
			mockSetup: func() {
				mockEvaluatorRecordDAO.EXPECT().
					GetEvaluatorRecord(gomock.Any(), int64(1), false).
					Return(&model.EvaluatorRecord{
						ID:                 1,
						SpaceID:            1,
						EvaluatorVersionID: 1,
						ExperimentID:       gptr.Of(int64(1)),
						ExperimentRunID:    1,
						ItemID:             1,
						TurnID:             1,
						TraceID:            "test_trace_id",
						LogID:              gptr.Of("test_log_id"),
						Status:             int32(entity.EvaluatorRunStatusSuccess),
					}, nil)
			},
			expectedResult: &entity.EvaluatorRecord{
				ID:                 1,
				SpaceID:            1,
				EvaluatorVersionID: 1,
				ExperimentID:       1,
				ExperimentRunID:    1,
				ItemID:             1,
				TurnID:             1,
				TraceID:            "test_trace_id",
				LogID:              "test_log_id",
				Status:             entity.EvaluatorRunStatusSuccess,
			},
			expectedError: nil,
		},
		{
			name:           "评估记录不存在",
			recordID:       1,
			includeDeleted: false,
			mockSetup: func() {
				mockEvaluatorRecordDAO.EXPECT().
					GetEvaluatorRecord(gomock.Any(), int64(1), false).
					Return(nil, nil)
			},
			expectedResult: nil,
			expectedError:  nil,
		},
		{
			name:           "获取评估记录失败",
			recordID:       1,
			includeDeleted: false,
			mockSetup: func() {
				mockEvaluatorRecordDAO.EXPECT().
					GetEvaluatorRecord(gomock.Any(), int64(1), false).
					Return(nil, assert.AnError)
			},
			expectedResult: nil,
			expectedError:  assert.AnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			repo := &EvaluatorRecordRepoImpl{
				evaluatorRecordDao: mockEvaluatorRecordDAO,
				dbProvider:         mockDBProvider,
				idgen:              mockIDGen,
			}

			result, err := repo.GetEvaluatorRecord(context.Background(), tt.recordID, tt.includeDeleted)
			assert.Equal(t, tt.expectedError, err)
			if err == nil {
				if tt.expectedResult == nil {
					assert.Nil(t, result)
				} else {
					assert.Equal(t, tt.expectedResult.ID, result.ID)
					assert.Equal(t, tt.expectedResult.SpaceID, result.SpaceID)
					assert.Equal(t, tt.expectedResult.EvaluatorVersionID, result.EvaluatorVersionID)
					assert.Equal(t, tt.expectedResult.ExperimentID, result.ExperimentID)
					assert.Equal(t, tt.expectedResult.ExperimentRunID, result.ExperimentRunID)
					assert.Equal(t, tt.expectedResult.ItemID, result.ItemID)
					assert.Equal(t, tt.expectedResult.TurnID, result.TurnID)
					assert.Equal(t, tt.expectedResult.TraceID, result.TraceID)
					assert.Equal(t, tt.expectedResult.LogID, result.LogID)
					assert.Equal(t, tt.expectedResult.Status, result.Status)
				}
			}
		})
	}
}

func TestEvaluatorRecordRepoImpl_BatchGetEvaluatorRecord(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockEvaluatorRecordDAO := evaluatormocks.NewMockEvaluatorRecordDAO(ctrl)
	mockDBProvider := dbmocks.NewMockProvider(ctrl)

	tests := []struct {
		name           string
		recordIDs      []int64
		includeDeleted bool
		mockSetup      func()
		expectedResult []*entity.EvaluatorRecord
		expectedError  error
	}{
		{
			name:           "成功批量获取评估记录",
			recordIDs:      []int64{1, 2},
			includeDeleted: false,
			mockSetup: func() {
				mockEvaluatorRecordDAO.EXPECT().
					BatchGetEvaluatorRecord(gomock.Any(), []int64{1, 2}, false).
					Return([]*model.EvaluatorRecord{
						{
							ID:                 1,
							SpaceID:            1,
							EvaluatorVersionID: 1,
							ExperimentID:       gptr.Of(int64(1)),
							ExperimentRunID:    1,
							ItemID:             1,
							TurnID:             1,
							TraceID:            "test_trace_id_1",
							LogID:              gptr.Of("test_log_id_1"),
							Status:             int32(entity.EvaluatorRunStatusSuccess),
						},
						{
							ID:                 2,
							SpaceID:            1,
							EvaluatorVersionID: 1,
							ExperimentID:       gptr.Of(int64(1)),
							ExperimentRunID:    1,
							ItemID:             1,
							TurnID:             1,
							TraceID:            "test_trace_id_2",
							LogID:              gptr.Of("test_log_id_2"),
							Status:             int32(entity.EvaluatorRunStatusSuccess),
						},
					}, nil)
			},
			expectedResult: []*entity.EvaluatorRecord{
				{
					ID:                 1,
					SpaceID:            1,
					EvaluatorVersionID: 1,
					ExperimentID:       1,
					ExperimentRunID:    1,
					ItemID:             1,
					TurnID:             1,
					TraceID:            "test_trace_id_1",
					LogID:              "test_log_id_1",
					Status:             entity.EvaluatorRunStatusSuccess,
				},
				{
					ID:                 2,
					SpaceID:            1,
					EvaluatorVersionID: 1,
					ExperimentID:       1,
					ExperimentRunID:    1,
					ItemID:             1,
					TurnID:             1,
					TraceID:            "test_trace_id_2",
					LogID:              "test_log_id_2",
					Status:             entity.EvaluatorRunStatusSuccess,
				},
			},
			expectedError: nil,
		},
		{
			name:           "部分记录不存在",
			recordIDs:      []int64{1, 2},
			includeDeleted: false,
			mockSetup: func() {
				mockEvaluatorRecordDAO.EXPECT().
					BatchGetEvaluatorRecord(gomock.Any(), []int64{1, 2}, false).
					Return([]*model.EvaluatorRecord{
						{
							ID:                 1,
							SpaceID:            1,
							EvaluatorVersionID: 1,
							ExperimentID:       gptr.Of(int64(1)),
							ExperimentRunID:    1,
							ItemID:             1,
							TurnID:             1,
							TraceID:            "test_trace_id_1",
							LogID:              gptr.Of("test_log_id_1"),
							Status:             int32(entity.EvaluatorRunStatusSuccess),
						},
					}, nil)
			},
			expectedResult: []*entity.EvaluatorRecord{
				{
					ID:                 1,
					SpaceID:            1,
					EvaluatorVersionID: 1,
					ExperimentID:       1,
					ExperimentRunID:    1,
					ItemID:             1,
					TurnID:             1,
					TraceID:            "test_trace_id_1",
					LogID:              "test_log_id_1",
					Status:             entity.EvaluatorRunStatusSuccess,
				},
			},
			expectedError: nil,
		},
		{
			name:           "所有记录都不存在",
			recordIDs:      []int64{1, 2},
			includeDeleted: false,
			mockSetup: func() {
				mockEvaluatorRecordDAO.EXPECT().
					BatchGetEvaluatorRecord(gomock.Any(), []int64{1, 2}, false).
					Return([]*model.EvaluatorRecord{}, nil)
			},
			expectedResult: []*entity.EvaluatorRecord{},
			expectedError:  nil,
		},
		{
			name:           "获取记录失败",
			recordIDs:      []int64{1, 2},
			includeDeleted: false,
			mockSetup: func() {
				mockEvaluatorRecordDAO.EXPECT().
					BatchGetEvaluatorRecord(gomock.Any(), []int64{1, 2}, false).
					Return(nil, assert.AnError)
			},
			expectedResult: nil,
			expectedError:  assert.AnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			repo := &EvaluatorRecordRepoImpl{
				evaluatorRecordDao: mockEvaluatorRecordDAO,
				dbProvider:         mockDBProvider,
				idgen:              mockIDGen,
			}

			result, err := repo.BatchGetEvaluatorRecord(context.Background(), tt.recordIDs, tt.includeDeleted)
			assert.Equal(t, tt.expectedError, err)
			if err == nil {
				assert.Equal(t, len(tt.expectedResult), len(result))
				for i, expected := range tt.expectedResult {
					assert.Equal(t, expected.ID, result[i].ID)
					assert.Equal(t, expected.SpaceID, result[i].SpaceID)
					assert.Equal(t, expected.EvaluatorVersionID, result[i].EvaluatorVersionID)
					assert.Equal(t, expected.ExperimentID, result[i].ExperimentID)
					assert.Equal(t, expected.ExperimentRunID, result[i].ExperimentRunID)
					assert.Equal(t, expected.ItemID, result[i].ItemID)
					assert.Equal(t, expected.TurnID, result[i].TurnID)
					assert.Equal(t, expected.TraceID, result[i].TraceID)
					assert.Equal(t, expected.LogID, result[i].LogID)
					assert.Equal(t, expected.Status, result[i].Status)
				}
			}
		})
	}
}

func TestEvaluatorRecordRepoImpl_BatchGetEvaluatorRecord_EmptyIDs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockEvaluatorRecordDAO := evaluatormocks.NewMockEvaluatorRecordDAO(ctrl)
	mockDBProvider := dbmocks.NewMockProvider(ctrl)

	repo := &EvaluatorRecordRepoImpl{
		evaluatorRecordDao: mockEvaluatorRecordDAO,
		dbProvider:         mockDBProvider,
		idgen:              mockIDGen,
	}

	result, err := repo.BatchGetEvaluatorRecord(context.Background(), []int64{}, false)
	assert.NoError(t, err)
	assert.Len(t, result, 0)
}

func TestEvaluatorRecordRepoImpl_BatchGetEvaluatorRecord_Pagination(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockEvaluatorRecordDAO := evaluatormocks.NewMockEvaluatorRecordDAO(ctrl)
	mockDBProvider := dbmocks.NewMockProvider(ctrl)

	// 构造 150 个 ID，覆盖跨批次场景（100 + 50）
	var recordIDs []int64
	for i := int64(1); i <= 150; i++ {
		recordIDs = append(recordIDs, i)
	}

	firstBatchIDs := recordIDs[:100]
	secondBatchIDs := recordIDs[100:]

	// 第一批返回 100 条记录
	var firstBatchPos []*model.EvaluatorRecord
	for _, id := range firstBatchIDs {
		firstBatchPos = append(firstBatchPos, &model.EvaluatorRecord{
			ID:                 id,
			SpaceID:            1,
			EvaluatorVersionID: 1,
			ExperimentID:       gptr.Of(int64(1)),
			ExperimentRunID:    1,
			ItemID:             1,
			TurnID:             1,
			TraceID:            "trace_first_batch",
			LogID:              gptr.Of("log_first_batch"),
			Status:             int32(entity.EvaluatorRunStatusSuccess),
			CreatedAt:          time.Unix(0, 0),
			UpdatedAt:          time.Unix(0, 0),
			CreatedBy:          "creator",
			UpdatedBy:          "updater",
		})
	}

	// 第二批返回 50 条记录
	var secondBatchPos []*model.EvaluatorRecord
	for _, id := range secondBatchIDs {
		secondBatchPos = append(secondBatchPos, &model.EvaluatorRecord{
			ID:                 id,
			SpaceID:            1,
			EvaluatorVersionID: 1,
			ExperimentID:       gptr.Of(int64(1)),
			ExperimentRunID:    1,
			ItemID:             1,
			TurnID:             1,
			TraceID:            "trace_second_batch",
			LogID:              gptr.Of("log_second_batch"),
			Status:             int32(entity.EvaluatorRunStatusSuccess),
			CreatedAt:          time.Unix(0, 0),
			UpdatedAt:          time.Unix(0, 0),
			CreatedBy:          "creator",
			UpdatedBy:          "updater",
		})
	}

	mockEvaluatorRecordDAO.EXPECT().
		BatchGetEvaluatorRecord(gomock.Any(), firstBatchIDs, false).
		Return(firstBatchPos, nil)
	mockEvaluatorRecordDAO.EXPECT().
		BatchGetEvaluatorRecord(gomock.Any(), secondBatchIDs, false).
		Return(secondBatchPos, nil)

	repo := &EvaluatorRecordRepoImpl{
		evaluatorRecordDao: mockEvaluatorRecordDAO,
		dbProvider:         mockDBProvider,
		idgen:              mockIDGen,
	}

	result, err := repo.BatchGetEvaluatorRecord(context.Background(), recordIDs, false)
	assert.NoError(t, err)
	assert.Len(t, result, 150)
	assert.Equal(t, int64(1), result[0].ID)
	assert.Equal(t, int64(150), result[len(result)-1].ID)
}

func TestEvaluatorRecordRepoImpl_BatchGetEvaluatorRecord_ConvertError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockEvaluatorRecordDAO := evaluatormocks.NewMockEvaluatorRecordDAO(ctrl)
	mockDBProvider := dbmocks.NewMockProvider(ctrl)

	// 构造一个包含非法 JSON 的记录，触发 convertor.ConvertEvaluatorRecordPO2DO 的错误分支
	invalidJSON := []byte("invalid-json")
	mockEvaluatorRecordDAO.EXPECT().
		BatchGetEvaluatorRecord(gomock.Any(), []int64{1}, false).
		Return([]*model.EvaluatorRecord{
			{
				ID:                 1,
				SpaceID:            1,
				EvaluatorVersionID: 1,
				ExperimentID:       gptr.Of(int64(1)),
				ExperimentRunID:    1,
				ItemID:             1,
				TurnID:             1,
				TraceID:            "trace_error",
				LogID:              gptr.Of("log_error"),
				Status:             int32(entity.EvaluatorRunStatusSuccess),
				OutputData:         &invalidJSON,
				CreatedAt:          time.Unix(0, 0),
				UpdatedAt:          time.Unix(0, 0),
				CreatedBy:          "creator",
				UpdatedBy:          "updater",
			},
		}, nil)

	repo := &EvaluatorRecordRepoImpl{
		evaluatorRecordDao: mockEvaluatorRecordDAO,
		dbProvider:         mockDBProvider,
		idgen:              mockIDGen,
	}

	result, err := repo.BatchGetEvaluatorRecord(context.Background(), []int64{1}, false)
	assert.Error(t, err)
	assert.Nil(t, result)
}
