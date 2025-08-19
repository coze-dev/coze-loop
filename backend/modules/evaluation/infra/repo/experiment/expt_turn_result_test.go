// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	mysqlMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func TestExptTurnResultRepoImpl_UpdateTurnResultsWithItemIDs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExptTurnResultDAO := mysqlMocks.NewMockExptTurnResultDAO(ctrl)
	mockExptTurnEvaluatorResultRefDAO := mysqlMocks.NewMockIExptTurnEvaluatorResultRefDAO(ctrl)

	repo := &ExptTurnResultRepoImpl{
		exptTurnResultDAO:             mockExptTurnResultDAO,
		exptTurnEvaluatorResultRefDAO: mockExptTurnEvaluatorResultRefDAO,
	}

	tests := []struct {
		name      string
		exptID    int64
		itemIDs   []int64
		spaceID   int64
		ufields   map[string]any
		mockSetup func()
		wantErr   bool
	}{
		{
			name:    "成功更新轮次结果",
			exptID:  1,
			itemIDs: []int64{1, 2},
			spaceID: 1,
			ufields: map[string]any{
				"status": 1,
			},
			mockSetup: func() {
				mockExptTurnResultDAO.EXPECT().
					UpdateTurnResultsWithItemIDs(gomock.Any(), int64(1), []int64{1, 2}, int64(1), map[string]any{
						"status": 1,
					}).
					Return(nil)
			},
			wantErr: false,
		},
		{
			name:    "更新失败",
			exptID:  1,
			itemIDs: []int64{1, 2},
			spaceID: 1,
			ufields: map[string]any{
				"status": 1,
			},
			mockSetup: func() {
				mockExptTurnResultDAO.EXPECT().
					UpdateTurnResultsWithItemIDs(gomock.Any(), int64(1), []int64{1, 2}, int64(1), map[string]any{
						"status": 1,
					}).
					Return(errorx.New("更新失败"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			err := repo.UpdateTurnResultsWithItemIDs(context.Background(), tt.exptID, tt.itemIDs, tt.spaceID, tt.ufields)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptTurnResultRepoImpl_UpdateTurnResults(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExptTurnResultDAO := mysqlMocks.NewMockExptTurnResultDAO(ctrl)
	mockExptTurnEvaluatorResultRefDAO := mysqlMocks.NewMockIExptTurnEvaluatorResultRefDAO(ctrl)

	repo := &ExptTurnResultRepoImpl{
		exptTurnResultDAO:             mockExptTurnResultDAO,
		exptTurnEvaluatorResultRefDAO: mockExptTurnEvaluatorResultRefDAO,
	}

	tests := []struct {
		name        string
		exptID      int64
		itemTurnIDs []*entity.ItemTurnID
		spaceID     int64
		ufields     map[string]any
		mockSetup   func()
		wantErr     bool
	}{
		{
			name:   "成功更新轮次结果",
			exptID: 1,
			itemTurnIDs: []*entity.ItemTurnID{
				{ItemID: 1, TurnID: 1},
				{ItemID: 2, TurnID: 2},
			},
			spaceID: 1,
			ufields: map[string]any{
				"status": 1,
			},
			mockSetup: func() {
				mockExptTurnResultDAO.EXPECT().
					UpdateTurnResults(gomock.Any(), int64(1), []*entity.ItemTurnID{
						{ItemID: 1, TurnID: 1},
						{ItemID: 2, TurnID: 2},
					}, int64(1), map[string]any{
						"status": 1,
					}).
					Return(nil)
			},
			wantErr: false,
		},
		{
			name:   "更新失败",
			exptID: 1,
			itemTurnIDs: []*entity.ItemTurnID{
				{ItemID: 1, TurnID: 1},
				{ItemID: 2, TurnID: 2},
			},
			spaceID: 1,
			ufields: map[string]any{
				"status": 1,
			},
			mockSetup: func() {
				mockExptTurnResultDAO.EXPECT().
					UpdateTurnResults(gomock.Any(), int64(1), []*entity.ItemTurnID{
						{ItemID: 1, TurnID: 1},
						{ItemID: 2, TurnID: 2},
					}, int64(1), map[string]any{
						"status": 1,
					}).
					Return(errorx.New("更新失败"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			err := repo.UpdateTurnResults(context.Background(), tt.exptID, tt.itemTurnIDs, tt.spaceID, tt.ufields)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptTurnResultRepoImpl_ScanTurnResults(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExptTurnResultDAO := mysqlMocks.NewMockExptTurnResultDAO(ctrl)
	mockExptTurnEvaluatorResultRefDAO := mysqlMocks.NewMockIExptTurnEvaluatorResultRefDAO(ctrl)

	repo := &ExptTurnResultRepoImpl{
		exptTurnResultDAO:             mockExptTurnResultDAO,
		exptTurnEvaluatorResultRefDAO: mockExptTurnEvaluatorResultRefDAO,
	}

	now := time.Now()
	tests := []struct {
		name      string
		exptID    int64
		status    []int32
		cursor    int64
		limit     int64
		spaceID   int64
		mockSetup func()
		want      []*entity.ExptTurnResult
		wantNext  int64
		wantErr   bool
	}{
		{
			name:    "成功扫描轮次结果",
			exptID:  1,
			status:  []int32{1, 2},
			cursor:  0,
			limit:   10,
			spaceID: 1,
			mockSetup: func() {
				results := []*model.ExptTurnResult{
					{
						ID:        1,
						SpaceID:   1,
						ExptID:    1,
						ItemID:    1,
						TurnID:    1,
						Status:    1,
						CreatedAt: now,
						UpdatedAt: now,
					},
				}
				mockExptTurnResultDAO.EXPECT().
					ScanTurnResults(gomock.Any(), int64(1), []int32{1, 2}, int64(0), int64(10), int64(1)).
					Return(results, int64(1), nil)
			},
			want: []*entity.ExptTurnResult{
				{
					ID:      1,
					SpaceID: 1,
					ExptID:  1,
					ItemID:  1,
					TurnID:  1,
					Status:  1,
				},
			},
			wantNext: 1,
			wantErr:  false,
		},
		{
			name:    "扫描失败",
			exptID:  1,
			status:  []int32{1, 2},
			cursor:  0,
			limit:   10,
			spaceID: 1,
			mockSetup: func() {
				mockExptTurnResultDAO.EXPECT().
					ScanTurnResults(gomock.Any(), int64(1), []int32{1, 2}, int64(0), int64(10), int64(1)).
					Return(nil, int64(0), errorx.New("扫描失败"))
			},
			want:     nil,
			wantNext: 0,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			got, next, err := repo.ScanTurnResults(context.Background(), tt.exptID, tt.status, tt.cursor, tt.limit, tt.spaceID)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
				assert.Equal(t, tt.wantNext, next)
			}
		})
	}
}

func TestExptTurnResultRepoImpl_ScanTurnRunLogs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExptTurnResultDAO := mysqlMocks.NewMockExptTurnResultDAO(ctrl)
	mockExptTurnEvaluatorResultRefDAO := mysqlMocks.NewMockIExptTurnEvaluatorResultRefDAO(ctrl)

	repo := &ExptTurnResultRepoImpl{
		exptTurnResultDAO:             mockExptTurnResultDAO,
		exptTurnEvaluatorResultRefDAO: mockExptTurnEvaluatorResultRefDAO,
	}

	now := time.Now()
	tests := []struct {
		name      string
		exptID    int64
		cursor    int64
		limit     int64
		spaceID   int64
		mockSetup func()
		want      []*entity.ExptTurnResultRunLog
		wantNext  int64
		wantErr   bool
	}{
		{
			name:    "成功扫描运行日志",
			exptID:  1,
			cursor:  0,
			limit:   10,
			spaceID: 1,
			mockSetup: func() {
				results := []*model.ExptTurnResultRunLog{
					{
						ID:        1,
						SpaceID:   1,
						ExptID:    1,
						ExptRunID: 1,
						ItemID:    1,
						TurnID:    1,
						CreatedAt: now,
						UpdatedAt: now,
					},
				}
				mockExptTurnResultDAO.EXPECT().
					ScanTurnRunLogs(gomock.Any(), int64(1), int64(0), int64(10), int64(1)).
					Return(results, int64(1), nil)
			},
			want: []*entity.ExptTurnResultRunLog{
				{
					ID:        1,
					SpaceID:   1,
					ExptID:    1,
					ExptRunID: 1,
					ItemID:    1,
					TurnID:    1,
				},
			},
			wantNext: 1,
			wantErr:  false,
		},
		{
			name:    "扫描失败",
			exptID:  1,
			cursor:  0,
			limit:   10,
			spaceID: 1,
			mockSetup: func() {
				mockExptTurnResultDAO.EXPECT().
					ScanTurnRunLogs(gomock.Any(), int64(1), int64(0), int64(10), int64(1)).
					Return(nil, int64(0), errorx.New("扫描失败"))
			},
			want:     nil,
			wantNext: 0,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			got, _, err := repo.ScanTurnRunLogs(context.Background(), tt.exptID, tt.cursor, tt.limit, tt.spaceID)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want[0].ID, got[0].ID)
				assert.Equal(t, tt.want[0].ExptID, got[0].ExptID)
				assert.Equal(t, tt.want[0].ExptRunID, got[0].ExptRunID)
				assert.Equal(t, tt.want[0].ItemID, got[0].ItemID)
				assert.Equal(t, tt.want[0].TurnID, got[0].TurnID)
			}
		})
	}
}

func TestExptTurnResultRepoImpl_BatchCreateNX(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExptTurnResultDAO := mysqlMocks.NewMockExptTurnResultDAO(ctrl)
	mockExptTurnEvaluatorResultRefDAO := mysqlMocks.NewMockIExptTurnEvaluatorResultRefDAO(ctrl)

	repo := &ExptTurnResultRepoImpl{
		exptTurnResultDAO:             mockExptTurnResultDAO,
		exptTurnEvaluatorResultRefDAO: mockExptTurnEvaluatorResultRefDAO,
	}

	tests := []struct {
		name        string
		turnResults []*entity.ExptTurnResult
		mockSetup   func()
		wantErr     bool
	}{
		{
			name: "成功批量创建轮次结果",
			turnResults: []*entity.ExptTurnResult{
				{
					ID:      1,
					SpaceID: 1,
					ExptID:  1,
					ItemID:  1,
					TurnID:  1,
					Status:  1,
				},
			},
			mockSetup: func() {
				mockExptTurnResultDAO.EXPECT().
					BatchCreateNX(gomock.Any(), gomock.Any()).
					Return(nil)
			},
			wantErr: false,
		},
		{
			name: "创建失败",
			turnResults: []*entity.ExptTurnResult{
				{
					ID:      1,
					SpaceID: 1,
					ExptID:  1,
					ItemID:  1,
					TurnID:  1,
					Status:  1,
				},
			},
			mockSetup: func() {
				mockExptTurnResultDAO.EXPECT().
					BatchCreateNX(gomock.Any(), gomock.Any()).
					Return(errorx.New("创建失败"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			err := repo.BatchCreateNX(context.Background(), tt.turnResults)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptTurnResultRepoImpl_CreateTurnEvaluatorRefs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExptTurnResultDAO := mysqlMocks.NewMockExptTurnResultDAO(ctrl)
	mockExptTurnEvaluatorResultRefDAO := mysqlMocks.NewMockIExptTurnEvaluatorResultRefDAO(ctrl)

	repo := &ExptTurnResultRepoImpl{
		exptTurnResultDAO:             mockExptTurnResultDAO,
		exptTurnEvaluatorResultRefDAO: mockExptTurnEvaluatorResultRefDAO,
	}

	tests := []struct {
		name      string
		refs      []*entity.ExptTurnEvaluatorResultRef
		mockSetup func()
		wantErr   bool
	}{
		{
			name: "成功创建评估器结果引用",
			refs: []*entity.ExptTurnEvaluatorResultRef{
				{
					ID:                 1,
					SpaceID:            1,
					ExptID:             1,
					ExptTurnResultID:   1,
					EvaluatorVersionID: 1,
					EvaluatorResultID:  1,
				},
			},
			mockSetup: func() {
				mockExptTurnResultDAO.EXPECT().
					CreateTurnEvaluatorRefs(gomock.Any(), gomock.Any()).
					Return(nil)
			},
			wantErr: false,
		},
		{
			name: "创建失败",
			refs: []*entity.ExptTurnEvaluatorResultRef{
				{
					ID:                 1,
					SpaceID:            1,
					ExptID:             1,
					ExptTurnResultID:   1,
					EvaluatorVersionID: 1,
					EvaluatorResultID:  1,
				},
			},
			mockSetup: func() {
				mockExptTurnResultDAO.EXPECT().
					CreateTurnEvaluatorRefs(gomock.Any(), gomock.Any()).
					Return(errorx.New("创建失败"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			err := repo.CreateTurnEvaluatorRefs(context.Background(), tt.refs)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptTurnResultRepoImpl_BatchGet(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExptTurnResultDAO := mysqlMocks.NewMockExptTurnResultDAO(ctrl)
	mockExptTurnEvaluatorResultRefDAO := mysqlMocks.NewMockIExptTurnEvaluatorResultRefDAO(ctrl)

	repo := &ExptTurnResultRepoImpl{
		exptTurnResultDAO:             mockExptTurnResultDAO,
		exptTurnEvaluatorResultRefDAO: mockExptTurnEvaluatorResultRefDAO,
	}

	now := time.Now()
	tests := []struct {
		name      string
		spaceID   int64
		exptID    int64
		itemIDs   []int64
		mockSetup func()
		want      []*entity.ExptTurnResult
		wantErr   bool
	}{
		{
			name:    "成功批量获取轮次结果",
			spaceID: 1,
			exptID:  1,
			itemIDs: []int64{1, 2},
			mockSetup: func() {
				results := []*model.ExptTurnResult{
					{
						ID:        1,
						SpaceID:   1,
						ExptID:    1,
						ItemID:    1,
						TurnID:    1,
						Status:    1,
						CreatedAt: now,
						UpdatedAt: now,
					},
					{
						ID:        2,
						SpaceID:   1,
						ExptID:    1,
						ItemID:    2,
						TurnID:    1,
						Status:    1,
						CreatedAt: now,
						UpdatedAt: now,
					},
				}
				mockExptTurnResultDAO.EXPECT().
					BatchGet(gomock.Any(), int64(1), int64(1), []int64{1, 2}).
					Return(results, nil)
			},
			want: []*entity.ExptTurnResult{
				{
					ID:      1,
					SpaceID: 1,
					ExptID:  1,
					ItemID:  1,
					TurnID:  1,
					Status:  1,
				},
				{
					ID:      2,
					SpaceID: 1,
					ExptID:  1,
					ItemID:  2,
					TurnID:  1,
					Status:  1,
				},
			},
			wantErr: false,
		},
		{
			name:    "获取失败",
			spaceID: 1,
			exptID:  1,
			itemIDs: []int64{1, 2},
			mockSetup: func() {
				mockExptTurnResultDAO.EXPECT().
					BatchGet(gomock.Any(), int64(1), int64(1), []int64{1, 2}).
					Return(nil, errorx.New("获取失败"))
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			got, err := repo.BatchGet(context.Background(), tt.spaceID, tt.exptID, tt.itemIDs)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestExptTurnResultRepoImpl_SaveTurnResults(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExptTurnResultDAO := mysqlMocks.NewMockExptTurnResultDAO(ctrl)
	mockExptTurnEvaluatorResultRefDAO := mysqlMocks.NewMockIExptTurnEvaluatorResultRefDAO(ctrl)

	repo := &ExptTurnResultRepoImpl{
		exptTurnResultDAO:             mockExptTurnResultDAO,
		exptTurnEvaluatorResultRefDAO: mockExptTurnEvaluatorResultRefDAO,
	}

	tests := []struct {
		name        string
		turnResults []*entity.ExptTurnResult
		mockSetup   func()
		wantErr     bool
	}{
		{
			name: "成功保存轮次结果",
			turnResults: []*entity.ExptTurnResult{
				{
					ID:      1,
					SpaceID: 1,
					ExptID:  1,
					ItemID:  1,
					TurnID:  1,
					Status:  1,
				},
			},
			mockSetup: func() {
				mockExptTurnResultDAO.EXPECT().
					SaveTurnResults(gomock.Any(), gomock.Any()).
					Return(nil)
			},
			wantErr: false,
		},
		{
			name: "保存失败",
			turnResults: []*entity.ExptTurnResult{
				{
					ID:      1,
					SpaceID: 1,
					ExptID:  1,
					ItemID:  1,
					TurnID:  1,
					Status:  1,
				},
			},
			mockSetup: func() {
				mockExptTurnResultDAO.EXPECT().
					SaveTurnResults(gomock.Any(), gomock.Any()).
					Return(errorx.New("保存失败"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			err := repo.SaveTurnResults(context.Background(), tt.turnResults)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptTurnResultRepoImpl_SaveTurnRunLogs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExptTurnResultDAO := mysqlMocks.NewMockExptTurnResultDAO(ctrl)
	mockExptTurnEvaluatorResultRefDAO := mysqlMocks.NewMockIExptTurnEvaluatorResultRefDAO(ctrl)

	repo := &ExptTurnResultRepoImpl{
		exptTurnResultDAO:             mockExptTurnResultDAO,
		exptTurnEvaluatorResultRefDAO: mockExptTurnEvaluatorResultRefDAO,
	}

	tests := []struct {
		name      string
		runLogs   []*entity.ExptTurnResultRunLog
		mockSetup func()
		wantErr   bool
	}{
		{
			name: "成功保存运行日志",
			runLogs: []*entity.ExptTurnResultRunLog{
				{
					ID:        1,
					SpaceID:   1,
					ExptID:    1,
					ExptRunID: 1,
					ItemID:    1,
					TurnID:    1,
				},
			},
			mockSetup: func() {
				mockExptTurnResultDAO.EXPECT().
					SaveTurnRunLogs(gomock.Any(), gomock.Any()).
					Return(nil)
			},
			wantErr: false,
		},
		{
			name: "保存失败",
			runLogs: []*entity.ExptTurnResultRunLog{
				{
					ID:        1,
					SpaceID:   1,
					ExptID:    1,
					ExptRunID: 1,
					ItemID:    1,
					TurnID:    1,
				},
			},
			mockSetup: func() {
				mockExptTurnResultDAO.EXPECT().
					SaveTurnRunLogs(gomock.Any(), gomock.Any()).
					Return(errorx.New("保存失败"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			err := repo.SaveTurnRunLogs(context.Background(), tt.runLogs)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptTurnResultRepoImpl_GetItemTurnResults(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExptTurnResultDAO := mysqlMocks.NewMockExptTurnResultDAO(ctrl)
	mockExptTurnEvaluatorResultRefDAO := mysqlMocks.NewMockIExptTurnEvaluatorResultRefDAO(ctrl)

	repo := &ExptTurnResultRepoImpl{
		exptTurnResultDAO:             mockExptTurnResultDAO,
		exptTurnEvaluatorResultRefDAO: mockExptTurnEvaluatorResultRefDAO,
	}

	now := time.Now()
	tests := []struct {
		name      string
		exptID    int64
		itemID    int64
		spaceID   int64
		mockSetup func()
		want      []*entity.ExptTurnResult
		wantErr   bool
	}{
		{
			name:    "成功获取轮次结果",
			exptID:  1,
			itemID:  1,
			spaceID: 1,
			mockSetup: func() {
				results := []*model.ExptTurnResult{
					{
						ID:        1,
						SpaceID:   1,
						ExptID:    1,
						ItemID:    1,
						TurnID:    1,
						Status:    1,
						CreatedAt: now,
						UpdatedAt: now,
					},
				}
				mockExptTurnResultDAO.EXPECT().
					GetItemTurnResults(gomock.Any(), int64(1), int64(1), int64(1)).
					Return(results, nil)
			},
			want: []*entity.ExptTurnResult{
				{
					ID:      1,
					SpaceID: 1,
					ExptID:  1,
					ItemID:  1,
					TurnID:  1,
					Status:  1,
				},
			},
			wantErr: false,
		},
		{
			name:    "获取失败",
			exptID:  1,
			itemID:  1,
			spaceID: 1,
			mockSetup: func() {
				mockExptTurnResultDAO.EXPECT().
					GetItemTurnResults(gomock.Any(), int64(1), int64(1), int64(1)).
					Return(nil, errorx.New("获取失败"))
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			got, err := repo.GetItemTurnResults(context.Background(), tt.exptID, tt.itemID, tt.spaceID)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestExptTurnResultRepoImpl_GetItemTurnRunLogs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExptTurnResultDAO := mysqlMocks.NewMockExptTurnResultDAO(ctrl)
	mockExptTurnEvaluatorResultRefDAO := mysqlMocks.NewMockIExptTurnEvaluatorResultRefDAO(ctrl)

	repo := &ExptTurnResultRepoImpl{
		exptTurnResultDAO:             mockExptTurnResultDAO,
		exptTurnEvaluatorResultRefDAO: mockExptTurnEvaluatorResultRefDAO,
	}

	now := time.Now()
	tests := []struct {
		name      string
		exptID    int64
		exptRunID int64
		itemID    int64
		spaceID   int64
		mockSetup func()
		want      []*entity.ExptTurnResultRunLog
		wantErr   bool
	}{
		{
			name:      "成功获取运行日志",
			exptID:    1,
			exptRunID: 1,
			itemID:    1,
			spaceID:   1,
			mockSetup: func() {
				results := []*model.ExptTurnResultRunLog{
					{
						ID:        1,
						SpaceID:   1,
						ExptID:    1,
						ExptRunID: 1,
						ItemID:    1,
						TurnID:    1,
						CreatedAt: now,
						UpdatedAt: now,
					},
				}
				mockExptTurnResultDAO.EXPECT().
					GetItemTurnRunLogs(gomock.Any(), int64(1), int64(1), int64(1), int64(1)).
					Return(results, nil)
			},
			want: []*entity.ExptTurnResultRunLog{
				{
					ID:        1,
					SpaceID:   1,
					ExptID:    1,
					ExptRunID: 1,
					ItemID:    1,
					TurnID:    1,
				},
			},
			wantErr: false,
		},
		{
			name:      "获取失败",
			exptID:    1,
			exptRunID: 1,
			itemID:    1,
			spaceID:   1,
			mockSetup: func() {
				mockExptTurnResultDAO.EXPECT().
					GetItemTurnRunLogs(gomock.Any(), int64(1), int64(1), int64(1), int64(1)).
					Return(nil, errorx.New("获取失败"))
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			got, err := repo.GetItemTurnRunLogs(context.Background(), tt.exptID, tt.exptRunID, tt.itemID, tt.spaceID)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want[0].ID, got[0].ID)
				assert.Equal(t, tt.want[0].SpaceID, got[0].SpaceID)
				assert.Equal(t, tt.want[0].ExptID, got[0].ExptID)
				assert.Equal(t, tt.want[0].ExptRunID, got[0].ExptRunID)
				assert.Equal(t, tt.want[0].ItemID, got[0].ItemID)
				assert.Equal(t, tt.want[0].TurnID, got[0].TurnID)
			}
		})
	}
}

func TestExptTurnResultRepoImpl_MGetItemTurnRunLogs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExptTurnResultDAO := mysqlMocks.NewMockExptTurnResultDAO(ctrl)
	mockExptTurnEvaluatorResultRefDAO := mysqlMocks.NewMockIExptTurnEvaluatorResultRefDAO(ctrl)

	repo := &ExptTurnResultRepoImpl{
		exptTurnResultDAO:             mockExptTurnResultDAO,
		exptTurnEvaluatorResultRefDAO: mockExptTurnEvaluatorResultRefDAO,
	}

	now := time.Now()
	tests := []struct {
		name      string
		exptID    int64
		exptRunID int64
		itemIDs   []int64
		spaceID   int64
		mockSetup func()
		want      []*entity.ExptTurnResultRunLog
		wantErr   bool
	}{
		{
			name:      "成功批量获取运行日志",
			exptID:    1,
			exptRunID: 1,
			itemIDs:   []int64{1, 2},
			spaceID:   1,
			mockSetup: func() {
				results := []*model.ExptTurnResultRunLog{
					{
						ID:        1,
						SpaceID:   1,
						ExptID:    1,
						ExptRunID: 1,
						ItemID:    1,
						TurnID:    1,
						CreatedAt: now,
						UpdatedAt: now,
					},
					{
						ID:        2,
						SpaceID:   1,
						ExptID:    1,
						ExptRunID: 1,
						ItemID:    2,
						TurnID:    1,
						CreatedAt: now,
						UpdatedAt: now,
					},
				}
				mockExptTurnResultDAO.EXPECT().
					MGetItemTurnRunLogs(gomock.Any(), int64(1), int64(1), []int64{1, 2}, int64(1)).
					Return(results, nil)
			},
			want: []*entity.ExptTurnResultRunLog{
				{
					ID:        1,
					SpaceID:   1,
					ExptID:    1,
					ExptRunID: 1,
					ItemID:    1,
					TurnID:    1,
				},
				{
					ID:        2,
					SpaceID:   1,
					ExptID:    1,
					ExptRunID: 1,
					ItemID:    2,
					TurnID:    1,
				},
			},
			wantErr: false,
		},
		{
			name:      "获取失败",
			exptID:    1,
			exptRunID: 1,
			itemIDs:   []int64{1, 2},
			spaceID:   1,
			mockSetup: func() {
				mockExptTurnResultDAO.EXPECT().
					MGetItemTurnRunLogs(gomock.Any(), int64(1), int64(1), []int64{1, 2}, int64(1)).
					Return(nil, errorx.New("获取失败"))
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			got, err := repo.MGetItemTurnRunLogs(context.Background(), tt.exptID, tt.exptRunID, tt.itemIDs, tt.spaceID)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want[0].ID, got[0].ID)
				assert.Equal(t, tt.want[0].SpaceID, got[0].SpaceID)
				assert.Equal(t, tt.want[0].ExptID, got[0].ExptID)
				assert.Equal(t, tt.want[0].ExptRunID, got[0].ExptRunID)
				assert.Equal(t, tt.want[0].ItemID, got[0].ItemID)
				assert.Equal(t, tt.want[0].TurnID, got[0].TurnID)
				assert.Equal(t, tt.want[0].Status, got[0].Status)
			}
		})
	}
}

func TestExptTurnResultRepoImpl_BatchCreateNXRunLog(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExptTurnResultDAO := mysqlMocks.NewMockExptTurnResultDAO(ctrl)
	mockExptTurnEvaluatorResultRefDAO := mysqlMocks.NewMockIExptTurnEvaluatorResultRefDAO(ctrl)

	repo := &ExptTurnResultRepoImpl{
		exptTurnResultDAO:             mockExptTurnResultDAO,
		exptTurnEvaluatorResultRefDAO: mockExptTurnEvaluatorResultRefDAO,
	}

	tests := []struct {
		name      string
		runLogs   []*entity.ExptTurnResultRunLog
		mockSetup func()
		wantErr   bool
	}{
		{
			name: "成功批量创建运行日志",
			runLogs: []*entity.ExptTurnResultRunLog{
				{
					ID:        1,
					SpaceID:   1,
					ExptID:    1,
					ExptRunID: 1,
					ItemID:    1,
					TurnID:    1,
				},
			},
			mockSetup: func() {
				mockExptTurnResultDAO.EXPECT().
					BatchCreateNXRunLog(gomock.Any(), gomock.Any()).
					Return(nil)
			},
			wantErr: false,
		},
		{
			name: "创建失败",
			runLogs: []*entity.ExptTurnResultRunLog{
				{
					ID:        1,
					SpaceID:   1,
					ExptID:    1,
					ExptRunID: 1,
					ItemID:    1,
					TurnID:    1,
				},
			},
			mockSetup: func() {
				mockExptTurnResultDAO.EXPECT().
					BatchCreateNXRunLog(gomock.Any(), gomock.Any()).
					Return(errorx.New("创建失败"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			err := repo.BatchCreateNXRunLog(context.Background(), tt.runLogs)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptTurnResultRepoImpl_ListTurnResult(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExptTurnResultDAO := mysqlMocks.NewMockExptTurnResultDAO(ctrl)
	mockExptTurnEvaluatorResultRefDAO := mysqlMocks.NewMockIExptTurnEvaluatorResultRefDAO(ctrl)

	repo := &ExptTurnResultRepoImpl{
		exptTurnResultDAO:             mockExptTurnResultDAO,
		exptTurnEvaluatorResultRefDAO: mockExptTurnEvaluatorResultRefDAO,
	}

	tests := []struct {
		name           string
		spaceID        int64
		exptID         int64
		filter         *entity.ExptTurnResultFilter
		page           entity.Page
		desc           bool
		mockSetup      func()
		expectedResult []*entity.ExptTurnResult
		expectedTotal  int64
		expectedErr    error
	}{
		{
			name:    "成功获取轮次结果",
			spaceID: 1,
			exptID:  1,
			filter: &entity.ExptTurnResultFilter{
				TrunRunStateFilters: []*entity.TurnRunStateFilter{
					{
						Status:   []entity.TurnRunState{entity.TurnRunState_Success},
						Operator: "=",
					},
				},
				ScoreFilters: []*entity.ScoreFilter{
					{
						Score:              0.8,
						Operator:           ">=",
						EvaluatorVersionID: 1,
					},
				},
			},
			page: entity.NewPage(1, 10),
			desc: true,
			mockSetup: func() {
				mockExptTurnResultDAO.EXPECT().ListTurnResult(
					gomock.Any(),
					int64(1),
					int64(1),
					&entity.ExptTurnResultFilter{
						TrunRunStateFilters: []*entity.TurnRunStateFilter{
							{
								Status:   []entity.TurnRunState{entity.TurnRunState_Success},
								Operator: "=",
							},
						},
						ScoreFilters: []*entity.ScoreFilter{
							{
								Score:              0.8,
								Operator:           ">=",
								EvaluatorVersionID: 1,
							},
						},
					},
					entity.NewPage(1, 10),
					true,
				).Return([]*model.ExptTurnResult{
					{
						ID:        1,
						SpaceID:   1,
						ExptID:    1,
						ItemID:    1,
						TurnID:    1,
						Status:    int32(entity.TurnRunState_Success),
						TraceID:   1,
						LogID:     "log1",
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
				}, int64(1), nil)
			},
			expectedResult: []*entity.ExptTurnResult{
				{
					ID:      1,
					SpaceID: 1,
					ExptID:  1,
					ItemID:  1,
					TurnID:  1,
					Status:  int32(entity.TurnRunState_Success),
					TraceID: 1,
					LogID:   "log1",
				},
			},
			expectedTotal: 1,
			expectedErr:   nil,
		},
		{
			name:    "获取轮次结果失败",
			spaceID: 1,
			exptID:  1,
			filter:  nil,
			page:    entity.NewPage(1, 10),
			desc:    false,
			mockSetup: func() {
				mockExptTurnResultDAO.EXPECT().ListTurnResult(
					gomock.Any(),
					int64(1),
					int64(1),
					(*entity.ExptTurnResultFilter)(nil),
					entity.NewPage(1, 10),
					false,
				).Return(nil, int64(0), errors.New("db error"))
			},
			expectedResult: nil,
			expectedTotal:  0,
			expectedErr:    errors.New("db error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			result, total, err := repo.ListTurnResult(context.Background(), tt.spaceID, tt.exptID, tt.filter, tt.page, tt.desc)
			if tt.expectedErr != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedTotal, total)
				assert.Equal(t, len(tt.expectedResult), len(result))
				for i, r := range result {
					assert.Equal(t, tt.expectedResult[i].ID, r.ID)
					assert.Equal(t, tt.expectedResult[i].SpaceID, r.SpaceID)
					assert.Equal(t, tt.expectedResult[i].ExptID, r.ExptID)
					assert.Equal(t, tt.expectedResult[i].ItemID, r.ItemID)
					assert.Equal(t, tt.expectedResult[i].TurnID, r.TurnID)
					assert.Equal(t, tt.expectedResult[i].Status, r.Status)
					assert.Equal(t, tt.expectedResult[i].TraceID, r.TraceID)
					assert.Equal(t, tt.expectedResult[i].LogID, r.LogID)
				}
			}
		})
	}
}

func TestExptTurnResultRepoImpl_BatchGetTurnEvaluatorResultRef(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExptTurnResultDAO := mysqlMocks.NewMockExptTurnResultDAO(ctrl)
	mockExptTurnEvaluatorResultRefDAO := mysqlMocks.NewMockIExptTurnEvaluatorResultRefDAO(ctrl)

	repo := &ExptTurnResultRepoImpl{
		exptTurnResultDAO:             mockExptTurnResultDAO,
		exptTurnEvaluatorResultRefDAO: mockExptTurnEvaluatorResultRefDAO,
	}

	tests := []struct {
		name           string
		spaceID        int64
		turnResultIDs  []int64
		mockSetup      func()
		expectedResult []*entity.ExptTurnEvaluatorResultRef
		expectedErr    error
	}{
		{
			name:          "成功批量获取评估器结果引用",
			spaceID:       1,
			turnResultIDs: []int64{1, 2},
			mockSetup: func() {
				mockExptTurnEvaluatorResultRefDAO.EXPECT().
					BatchGet(gomock.Any(), int64(1), []int64{1, 2}).
					Return([]*model.ExptTurnEvaluatorResultRef{
						{
							ID:                 1,
							SpaceID:            1,
							ExptTurnResultID:   1,
							EvaluatorVersionID: 1,
							EvaluatorResultID:  1,
							ExptID:             1,
						},
						{
							ID:                 2,
							SpaceID:            1,
							ExptTurnResultID:   2,
							EvaluatorVersionID: 2,
							EvaluatorResultID:  2,
							ExptID:             1,
						},
					}, nil)
			},
			expectedResult: []*entity.ExptTurnEvaluatorResultRef{
				{
					ID:                 1,
					SpaceID:            1,
					ExptTurnResultID:   1,
					EvaluatorVersionID: 1,
					EvaluatorResultID:  1,
					ExptID:             1,
				},
				{
					ID:                 2,
					SpaceID:            1,
					ExptTurnResultID:   2,
					EvaluatorVersionID: 2,
					EvaluatorResultID:  2,
					ExptID:             1,
				},
			},
			expectedErr: nil,
		},
		{
			name:          "获取评估器结果引用失败",
			spaceID:       1,
			turnResultIDs: []int64{1, 2},
			mockSetup: func() {
				mockExptTurnEvaluatorResultRefDAO.EXPECT().
					BatchGet(gomock.Any(), int64(1), []int64{1, 2}).
					Return(nil, errors.New("db error"))
			},
			expectedResult: nil,
			expectedErr:    errors.New("db error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			result, err := repo.BatchGetTurnEvaluatorResultRef(context.Background(), tt.spaceID, tt.turnResultIDs)
			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedErr.Error(), err.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, len(tt.expectedResult), len(result))
				for i, r := range result {
					assert.Equal(t, tt.expectedResult[i].ID, r.ID)
					assert.Equal(t, tt.expectedResult[i].SpaceID, r.SpaceID)
					assert.Equal(t, tt.expectedResult[i].ExptTurnResultID, r.ExptTurnResultID)
					assert.Equal(t, tt.expectedResult[i].EvaluatorVersionID, r.EvaluatorVersionID)
					assert.Equal(t, tt.expectedResult[i].EvaluatorResultID, r.EvaluatorResultID)
					assert.Equal(t, tt.expectedResult[i].ExptID, r.ExptID)
				}
			}
		})
	}
}

func TestExptTurnResultRepoImpl_GetTurnEvaluatorResultRefByExptID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExptTurnResultDAO := mysqlMocks.NewMockExptTurnResultDAO(ctrl)
	mockExptTurnEvaluatorResultRefDAO := mysqlMocks.NewMockIExptTurnEvaluatorResultRefDAO(ctrl)

	repo := &ExptTurnResultRepoImpl{
		exptTurnResultDAO:             mockExptTurnResultDAO,
		exptTurnEvaluatorResultRefDAO: mockExptTurnEvaluatorResultRefDAO,
	}

	tests := []struct {
		name           string
		spaceID        int64
		exptID         int64
		mockSetup      func()
		expectedResult []*entity.ExptTurnEvaluatorResultRef
		expectedErr    error
	}{
		{
			name:    "成功获取评估器结果引用",
			spaceID: 1,
			exptID:  1,
			mockSetup: func() {
				mockExptTurnEvaluatorResultRefDAO.EXPECT().
					GetByExptID(gomock.Any(), int64(1), int64(1)).
					Return([]*model.ExptTurnEvaluatorResultRef{
						{
							ID:                 1,
							SpaceID:            1,
							ExptTurnResultID:   1,
							EvaluatorVersionID: 1,
							EvaluatorResultID:  1,
							ExptID:             1,
						},
					}, nil)
			},
			expectedResult: []*entity.ExptTurnEvaluatorResultRef{
				{
					ID:                 1,
					SpaceID:            1,
					ExptTurnResultID:   1,
					EvaluatorVersionID: 1,
					EvaluatorResultID:  1,
					ExptID:             1,
				},
			},
			expectedErr: nil,
		},
		{
			name:    "获取评估器结果引用失败",
			spaceID: 1,
			exptID:  1,
			mockSetup: func() {
				mockExptTurnEvaluatorResultRefDAO.EXPECT().
					GetByExptID(gomock.Any(), int64(1), int64(1)).
					Return(nil, errors.New("db error"))
			},
			expectedResult: nil,
			expectedErr:    errors.New("db error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			result, err := repo.GetTurnEvaluatorResultRefByExptID(context.Background(), tt.spaceID, tt.exptID)
			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedErr.Error(), err.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, len(tt.expectedResult), len(result))
				for i, r := range result {
					assert.Equal(t, tt.expectedResult[i].ID, r.ID)
					assert.Equal(t, tt.expectedResult[i].SpaceID, r.SpaceID)
					assert.Equal(t, tt.expectedResult[i].ExptTurnResultID, r.ExptTurnResultID)
					assert.Equal(t, tt.expectedResult[i].EvaluatorVersionID, r.EvaluatorVersionID)
					assert.Equal(t, tt.expectedResult[i].EvaluatorResultID, r.EvaluatorResultID)
					assert.Equal(t, tt.expectedResult[i].ExptID, r.ExptID)
				}
			}
		})
	}
}

func TestExptTurnResultRepoImpl_GetTurnEvaluatorResultRefByEvaluatorVersionID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExptTurnResultDAO := mysqlMocks.NewMockExptTurnResultDAO(ctrl)
	mockExptTurnEvaluatorResultRefDAO := mysqlMocks.NewMockIExptTurnEvaluatorResultRefDAO(ctrl)

	repo := &ExptTurnResultRepoImpl{
		exptTurnResultDAO:             mockExptTurnResultDAO,
		exptTurnEvaluatorResultRefDAO: mockExptTurnEvaluatorResultRefDAO,
	}

	now := time.Now()
	tests := []struct {
		name               string
		spaceID            int64
		exptID             int64
		evaluatorVersionID int64
		mockSetup          func()
		want               []*entity.ExptTurnEvaluatorResultRef
		wantErr            bool
	}{
		{
			name:               "成功获取评估器结果引用",
			spaceID:            1,
			exptID:             1,
			evaluatorVersionID: 1,
			mockSetup: func() {
				results := []*model.ExptTurnEvaluatorResultRef{
					{
						ID:                 1,
						SpaceID:            1,
						ExptID:             1,
						ExptTurnResultID:   1,
						EvaluatorVersionID: 1,
						EvaluatorResultID:  1,
						CreatedAt:          now,
						UpdatedAt:          now,
					},
				}
				mockExptTurnEvaluatorResultRefDAO.EXPECT().
					GetByExptEvaluatorVersionID(gomock.Any(), int64(1), int64(1), int64(1)).
					Return(results, nil)
			},
			want: []*entity.ExptTurnEvaluatorResultRef{
				{
					ID:                 1,
					SpaceID:            1,
					ExptID:             1,
					ExptTurnResultID:   1,
					EvaluatorVersionID: 1,
					EvaluatorResultID:  1,
				},
			},
			wantErr: false,
		},
		{
			name:               "获取失败",
			spaceID:            1,
			exptID:             1,
			evaluatorVersionID: 1,
			mockSetup: func() {
				mockExptTurnEvaluatorResultRefDAO.EXPECT().
					GetByExptEvaluatorVersionID(gomock.Any(), int64(1), int64(1), int64(1)).
					Return(nil, errorx.New("获取失败"))
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			got, err := repo.GetTurnEvaluatorResultRefByEvaluatorVersionID(context.Background(), tt.spaceID, tt.exptID, tt.evaluatorVersionID)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want[0].ID, got[0].ID)
				assert.Equal(t, tt.want[0].SpaceID, got[0].SpaceID)
				assert.Equal(t, tt.want[0].ExptID, got[0].ExptID)
				assert.Equal(t, tt.want[0].ExptTurnResultID, got[0].ExptTurnResultID)
				assert.Equal(t, tt.want[0].EvaluatorVersionID, got[0].EvaluatorVersionID)
				assert.Equal(t, tt.want[0].EvaluatorResultID, got[0].EvaluatorResultID)
			}
		})
	}
}

func TestExptTurnResultRepoImpl_CreateOrUpdateItemsTurnRunLogStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExptTurnResultDAO := mysqlMocks.NewMockExptTurnResultDAO(ctrl)
	mockExptTurnEvaluatorResultRefDAO := mysqlMocks.NewMockIExptTurnEvaluatorResultRefDAO(ctrl)
	mockIDGen := mocks.NewMockIIDGenerator(ctrl)

	repo := &ExptTurnResultRepoImpl{
		idgen:                         mockIDGen,
		exptTurnResultDAO:             mockExptTurnResultDAO,
		exptTurnEvaluatorResultRefDAO: mockExptTurnEvaluatorResultRefDAO,
	}

	tests := []struct {
		name      string
		spaceID   int64
		exptID    int64
		exptRunID int64
		itemIDs   []int64
		status    entity.TurnRunState
		mockSetup func()
		wantErr   bool
	}{
		{
			name:      "Normal: create new run logs",
			spaceID:   1,
			exptID:    100,
			exptRunID: 200,
			itemIDs:   []int64{1, 2},
			status:    entity.TurnRunState_Success,
			mockSetup: func() {
				turnResults := []*model.ExptTurnResult{
					{
						ID:      1,
						SpaceID: 1,
						ExptID:  100,
						ItemID:  1,
						TurnID:  10,
						Status:  int32(entity.TurnRunState_Processing),
					},
					{
						ID:      2,
						SpaceID: 1,
						ExptID:  100,
						ItemID:  2,
						TurnID:  20,
						Status:  int32(entity.TurnRunState_Processing),
					},
				}
				mockExptTurnResultDAO.EXPECT().
					BatchGet(gomock.Any(), int64(1), int64(100), []int64{1, 2}).
					Return(turnResults, nil)

				mockIDGen.EXPECT().
					GenMultiIDs(gomock.Any(), 2).
					Return([]int64{1001, 1002}, nil)

				mockExptTurnResultDAO.EXPECT().
					BatchCreateNXRunLog(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, runlogs []*model.ExptTurnResultRunLog, opts ...interface{}) error {
						assert.Len(t, runlogs, 2)
						assert.Equal(t, int64(1001), runlogs[0].ID)
						assert.Equal(t, int64(1), runlogs[0].SpaceID)
						assert.Equal(t, int64(100), runlogs[0].ExptID)
						assert.Equal(t, int64(200), runlogs[0].ExptRunID)
						assert.Equal(t, int64(1), runlogs[0].ItemID)
						assert.Equal(t, int64(10), runlogs[0].TurnID)
						assert.Equal(t, int32(entity.TurnRunState_Success), runlogs[0].Status)
						assert.NotNil(t, runlogs[0].ErrMsg)
						return nil
					})

				mockExptTurnResultDAO.EXPECT().
					UpdateTurnRunLogWithItemIDs(gomock.Any(), int64(1), int64(100), int64(200), []int64{1, 2}, map[string]any{"status": int32(entity.TurnRunState_Success)}).
					Return(nil)
			},
			wantErr: false,
		},
		{
			name:      "Normal: update existing run logs",
			spaceID:   1,
			exptID:    100,
			exptRunID: 200,
			itemIDs:   []int64{3},
			status:    entity.TurnRunState_Fail,
			mockSetup: func() {
				turnResults := []*model.ExptTurnResult{
					{
						ID:      3,
						SpaceID: 1,
						ExptID:  100,
						ItemID:  3,
						TurnID:  30,
						Status:  int32(entity.TurnRunState_Processing),
					},
				}
				mockExptTurnResultDAO.EXPECT().
					BatchGet(gomock.Any(), int64(1), int64(100), []int64{3}).
					Return(turnResults, nil)

				mockIDGen.EXPECT().
					GenMultiIDs(gomock.Any(), 1).
					Return([]int64{1003}, nil)

				mockExptTurnResultDAO.EXPECT().
					BatchCreateNXRunLog(gomock.Any(), gomock.Any()).
					Return(nil)

				mockExptTurnResultDAO.EXPECT().
					UpdateTurnRunLogWithItemIDs(gomock.Any(), int64(1), int64(100), int64(200), []int64{3}, map[string]any{"status": int32(entity.TurnRunState_Fail)}).
					Return(nil)
			},
			wantErr: false,
		},
		{
			name:      "Error: BatchGet failed",
			spaceID:   1,
			exptID:    100,
			exptRunID: 200,
			itemIDs:   []int64{1, 2},
			status:    entity.TurnRunState_Success,
			mockSetup: func() {
				mockExptTurnResultDAO.EXPECT().
					BatchGet(gomock.Any(), int64(1), int64(100), []int64{1, 2}).
					Return(nil, errorx.New("BatchGet failed"))
			},
			wantErr: true,
		},
		{
			name:      "Error: ID generation failed",
			spaceID:   1,
			exptID:    100,
			exptRunID: 200,
			itemIDs:   []int64{1},
			status:    entity.TurnRunState_Success,
			mockSetup: func() {
				turnResults := []*model.ExptTurnResult{
					{
						ID:      1,
						SpaceID: 1,
						ExptID:  100,
						ItemID:  1,
						TurnID:  10,
						Status:  int32(entity.TurnRunState_Processing),
					},
				}
				mockExptTurnResultDAO.EXPECT().
					BatchGet(gomock.Any(), int64(1), int64(100), []int64{1}).
					Return(turnResults, nil)

				mockIDGen.EXPECT().
					GenMultiIDs(gomock.Any(), 1).
					Return(nil, errorx.New("ID generation failed"))
			},
			wantErr: true,
		},
		{
			name:      "Error: BatchCreateNXRunLog failed",
			spaceID:   1,
			exptID:    100,
			exptRunID: 200,
			itemIDs:   []int64{1},
			status:    entity.TurnRunState_Success,
			mockSetup: func() {
				turnResults := []*model.ExptTurnResult{
					{
						ID:      1,
						SpaceID: 1,
						ExptID:  100,
						ItemID:  1,
						TurnID:  10,
						Status:  int32(entity.TurnRunState_Processing),
					},
				}
				mockExptTurnResultDAO.EXPECT().
					BatchGet(gomock.Any(), int64(1), int64(100), []int64{1}).
					Return(turnResults, nil)

				mockIDGen.EXPECT().
					GenMultiIDs(gomock.Any(), 1).
					Return([]int64{1001}, nil)

				mockExptTurnResultDAO.EXPECT().
					BatchCreateNXRunLog(gomock.Any(), gomock.Any()).
					Return(errorx.New("BatchCreateNXRunLog failed"))
			},
			wantErr: true,
		},
		{
			name:      "Error: UpdateTurnRunLogWithItemIDs failed",
			spaceID:   1,
			exptID:    100,
			exptRunID: 200,
			itemIDs:   []int64{1},
			status:    entity.TurnRunState_Success,
			mockSetup: func() {
				turnResults := []*model.ExptTurnResult{
					{
						ID:      1,
						SpaceID: 1,
						ExptID:  100,
						ItemID:  1,
						TurnID:  10,
						Status:  int32(entity.TurnRunState_Processing),
					},
				}
				mockExptTurnResultDAO.EXPECT().
					BatchGet(gomock.Any(), int64(1), int64(100), []int64{1}).
					Return(turnResults, nil)

				mockIDGen.EXPECT().
					GenMultiIDs(gomock.Any(), 1).
					Return([]int64{1001}, nil)

				mockExptTurnResultDAO.EXPECT().
					BatchCreateNXRunLog(gomock.Any(), gomock.Any()).
					Return(nil)

				mockExptTurnResultDAO.EXPECT().
					UpdateTurnRunLogWithItemIDs(gomock.Any(), int64(1), int64(100), int64(200), []int64{1}, map[string]any{"status": int32(entity.TurnRunState_Success)}).
					Return(errorx.New("UpdateTurnRunLogWithItemIDs failed"))
			},
			wantErr: true,
		},
		{
			name:      "Edge: empty itemIDs list",
			spaceID:   1,
			exptID:    100,
			exptRunID: 200,
			itemIDs:   []int64{},
			status:    entity.TurnRunState_Success,
			mockSetup: func() {
				mockExptTurnResultDAO.EXPECT().
					BatchGet(gomock.Any(), int64(1), int64(100), []int64{}).
					Return([]*model.ExptTurnResult{}, nil)

				mockIDGen.EXPECT().
					GenMultiIDs(gomock.Any(), 0).
					Return([]int64{}, nil)

				mockExptTurnResultDAO.EXPECT().
					BatchCreateNXRunLog(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, runlogs []*model.ExptTurnResultRunLog, opts ...interface{}) error {
						assert.Len(t, runlogs, 0)
						return nil
					})

				mockExptTurnResultDAO.EXPECT().
					UpdateTurnRunLogWithItemIDs(gomock.Any(), int64(1), int64(100), int64(200), []int64{}, map[string]any{"status": int32(entity.TurnRunState_Success)}).
					Return(nil)
			},
			wantErr: false,
		},
		{
			name:      "Edge: Terminal state",
			spaceID:   1,
			exptID:    100,
			exptRunID: 200,
			itemIDs:   []int64{1},
			status:    entity.TurnRunState_Terminal,
			mockSetup: func() {
				turnResults := []*model.ExptTurnResult{
					{
						ID:      1,
						SpaceID: 1,
						ExptID:  100,
						ItemID:  1,
						TurnID:  10,
						Status:  int32(entity.TurnRunState_Processing),
					},
				}
				mockExptTurnResultDAO.EXPECT().
					BatchGet(gomock.Any(), int64(1), int64(100), []int64{1}).
					Return(turnResults, nil)

				mockIDGen.EXPECT().
					GenMultiIDs(gomock.Any(), 1).
					Return([]int64{1001}, nil)

				mockExptTurnResultDAO.EXPECT().
					BatchCreateNXRunLog(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, runlogs []*model.ExptTurnResultRunLog, opts ...interface{}) error {
						assert.Equal(t, int32(entity.TurnRunState_Terminal), runlogs[0].Status)
						return nil
					})

				mockExptTurnResultDAO.EXPECT().
					UpdateTurnRunLogWithItemIDs(gomock.Any(), int64(1), int64(100), int64(200), []int64{1}, map[string]any{"status": int32(entity.TurnRunState_Terminal)}).
					Return(nil)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			err := repo.CreateOrUpdateItemsTurnRunLogStatus(context.Background(), tt.spaceID, tt.exptID, tt.exptRunID, tt.itemIDs, tt.status)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
