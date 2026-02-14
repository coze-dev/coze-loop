// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm/clause"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	model "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	mocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/mocks"
)

func TestExptItemResultRepoImpl_BatchGet(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDAO := mocks.NewMockIExptItemResultDAO(ctrl)
	repo := &ExptItemResultRepoImpl{exptItemResultDAO: mockDAO}

	tests := []struct {
		name      string
		spaceID   int64
		exptID    int64
		itemIDs   []int64
		mockSetup func()
		wantLen   int
		wantErr   bool
	}{
		{
			name:    "success",
			spaceID: 1, exptID: 2, itemIDs: []int64{3, 4},
			mockSetup: func() {
				mockDAO.EXPECT().BatchGet(gomock.Any(), int64(1), int64(2), []int64{3, 4}).Return([]*model.ExptItemResult{{}, {}}, nil)
			},
			wantLen: 2, wantErr: false,
		},
		{
			name:    "fail_dao_error",
			spaceID: 2, exptID: 3, itemIDs: []int64{5},
			mockSetup: func() {
				mockDAO.EXPECT().BatchGet(gomock.Any(), int64(2), int64(3), []int64{5}).Return(nil, errors.New("dao error"))
			},
			wantLen: 0, wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			got, err := repo.BatchGet(context.Background(), tt.spaceID, tt.exptID, tt.itemIDs)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.Len(t, got, tt.wantLen)
			}
		})
	}
}

func TestExptItemResultRepoImpl_UpdateItemsResult(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDAO := mocks.NewMockIExptItemResultDAO(ctrl)
	repo := &ExptItemResultRepoImpl{exptItemResultDAO: mockDAO}

	tests := []struct {
		name      string
		spaceID   int64
		exptID    int64
		itemIDs   []int64
		ufields   map[string]any
		mockSetup func()
		wantErr   bool
	}{
		{
			name:    "success",
			spaceID: 1, exptID: 2, itemIDs: []int64{3}, ufields: map[string]any{"status": 1},
			mockSetup: func() {
				mockDAO.EXPECT().UpdateItemsResult(gomock.Any(), int64(1), int64(2), []int64{3}, map[string]any{"status": 1}).Return(nil)
			},
			wantErr: false,
		},
		{
			name:    "fail_dao_error",
			spaceID: 2, exptID: 3, itemIDs: []int64{4}, ufields: map[string]any{"status": 2},
			mockSetup: func() {
				mockDAO.EXPECT().UpdateItemsResult(gomock.Any(), int64(2), int64(3), []int64{4}, map[string]any{"status": 2}).Return(errors.New("dao error"))
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			err := repo.UpdateItemsResult(context.Background(), tt.spaceID, tt.exptID, tt.itemIDs, tt.ufields)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptItemResultRepoImpl_GetItemTurnResults(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDAO := mocks.NewMockIExptItemResultDAO(ctrl)
	repo := &ExptItemResultRepoImpl{exptItemResultDAO: mockDAO}

	tests := []struct {
		name      string
		spaceID   int64
		exptID    int64
		itemID    int64
		mockSetup func()
		wantLen   int
		wantErr   bool
	}{
		{
			name:    "success",
			spaceID: 1, exptID: 2, itemID: 3,
			mockSetup: func() {
				mockDAO.EXPECT().GetItemTurnResults(gomock.Any(), int64(1), int64(2), int64(3)).Return([]*model.ExptTurnResult{{}, {}}, nil)
			},
			wantLen: 2, wantErr: false,
		},
		{
			name:    "fail_dao_error",
			spaceID: 2, exptID: 3, itemID: 4,
			mockSetup: func() {
				mockDAO.EXPECT().GetItemTurnResults(gomock.Any(), int64(2), int64(3), int64(4)).Return(nil, errors.New("dao error"))
			},
			wantLen: 0, wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			got, err := repo.GetItemTurnResults(context.Background(), tt.spaceID, tt.exptID, tt.itemID)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.Len(t, got, tt.wantLen)
			}
		})
	}
}

func TestExptItemResultRepoImpl_SaveItemResults(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDAO := mocks.NewMockIExptItemResultDAO(ctrl)
	repo := &ExptItemResultRepoImpl{exptItemResultDAO: mockDAO}
	input := []*entity.ExptItemResult{{}, {}}

	tests := []struct {
		name      string
		mockSetup func()
		wantErr   bool
	}{
		{
			name: "success",
			mockSetup: func() {
				mockDAO.EXPECT().SaveItemResults(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "fail_dao_error",
			mockSetup: func() {
				mockDAO.EXPECT().SaveItemResults(gomock.Any(), gomock.Any()).Return(errors.New("dao error"))
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			err := repo.SaveItemResults(context.Background(), input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptItemResultRepoImpl_GetItemRunLog(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDAO := mocks.NewMockIExptItemResultDAO(ctrl)
	repo := &ExptItemResultRepoImpl{exptItemResultDAO: mockDAO}

	tests := []struct {
		name      string
		exptID    int64
		exptRunID int64
		itemID    int64
		spaceID   int64
		mockSetup func()
		wantNil   bool
		wantErr   bool
	}{
		{
			name:   "success",
			exptID: 1, exptRunID: 2, itemID: 3, spaceID: 4,
			mockSetup: func() {
				mockDAO.EXPECT().GetItemRunLog(gomock.Any(), int64(1), int64(2), int64(3), int64(4)).Return(&model.ExptItemResultRunLog{}, nil)
			},
			wantNil: false, wantErr: false,
		},
		{
			name:   "fail_dao_error",
			exptID: 2, exptRunID: 3, itemID: 4, spaceID: 5,
			mockSetup: func() {
				mockDAO.EXPECT().GetItemRunLog(gomock.Any(), int64(2), int64(3), int64(4), int64(5)).Return(nil, errors.New("dao error"))
			},
			wantNil: true, wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			got, err := repo.GetItemRunLog(context.Background(), tt.exptID, tt.exptRunID, tt.itemID, tt.spaceID)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, got)
			}
		})
	}
}

func TestExptItemResultRepoImpl_MGetItemRunLog(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDAO := mocks.NewMockIExptItemResultDAO(ctrl)
	repo := &ExptItemResultRepoImpl{exptItemResultDAO: mockDAO}

	tests := []struct {
		name      string
		exptID    int64
		exptRunID int64
		itemIDs   []int64
		spaceID   int64
		mockSetup func()
		wantLen   int
		wantErr   bool
	}{
		{
			name:   "success",
			exptID: 1, exptRunID: 2, itemIDs: []int64{3, 4}, spaceID: 5,
			mockSetup: func() {
				mockDAO.EXPECT().MGetItemRunLog(gomock.Any(), int64(1), int64(2), []int64{3, 4}, int64(5)).Return([]*model.ExptItemResultRunLog{{}, {}}, nil)
			},
			wantLen: 2, wantErr: false,
		},
		{
			name:   "fail_dao_error",
			exptID: 2, exptRunID: 3, itemIDs: []int64{5}, spaceID: 6,
			mockSetup: func() {
				mockDAO.EXPECT().MGetItemRunLog(gomock.Any(), int64(2), int64(3), []int64{5}, int64(6)).Return(nil, errors.New("dao error"))
			},
			wantLen: 0, wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			got, err := repo.MGetItemRunLog(context.Background(), tt.exptID, tt.exptRunID, tt.itemIDs, tt.spaceID)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.Len(t, got, tt.wantLen)
			}
		})
	}
}

func TestExptItemResultRepoImpl_UpdateItemRunLog(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDAO := mocks.NewMockIExptItemResultDAO(ctrl)
	repo := &ExptItemResultRepoImpl{exptItemResultDAO: mockDAO}

	tests := []struct {
		name      string
		exptID    int64
		exptRunID int64
		itemID    []int64
		ufields   map[string]any
		spaceID   int64
		mockSetup func()
		wantErr   bool
	}{
		{
			name:   "success",
			exptID: 1, exptRunID: 2, itemID: []int64{3}, ufields: map[string]any{"status": 1}, spaceID: 4,
			mockSetup: func() {
				mockDAO.EXPECT().UpdateItemRunLog(gomock.Any(), int64(1), int64(2), []int64{3}, map[string]any{"status": 1}, int64(4)).Return(nil)
			},
			wantErr: false,
		},
		{
			name:   "fail_dao_error",
			exptID: 2, exptRunID: 3, itemID: []int64{4}, ufields: map[string]any{"status": 2}, spaceID: 5,
			mockSetup: func() {
				mockDAO.EXPECT().UpdateItemRunLog(gomock.Any(), int64(2), int64(3), []int64{4}, map[string]any{"status": 2}, int64(5)).Return(errors.New("dao error"))
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			err := repo.UpdateItemRunLog(context.Background(), tt.exptID, tt.exptRunID, tt.itemID, tt.ufields, tt.spaceID)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptItemResultRepoImpl_ScanItemResults(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDAO := mocks.NewMockIExptItemResultDAO(ctrl)
	repo := &ExptItemResultRepoImpl{exptItemResultDAO: mockDAO}

	tests := []struct {
		name      string
		exptID    int64
		cursor    int64
		limit     int64
		status    []int32
		spaceID   int64
		mockSetup func()
		wantLen   int
		wantErr   bool
	}{
		{
			name:   "success",
			exptID: 1, cursor: 2, limit: 3, status: []int32{4}, spaceID: 5,
			mockSetup: func() {
				mockDAO.EXPECT().ScanItemResults(gomock.Any(), int64(1), int64(2), int64(3), []int32{4}, int64(5)).Return([]*model.ExptItemResult{{}, {}}, int64(10), nil)
			},
			wantLen: 2, wantErr: false,
		},
		{
			name:   "fail_dao_error",
			exptID: 2, cursor: 3, limit: 4, status: []int32{5}, spaceID: 6,
			mockSetup: func() {
				mockDAO.EXPECT().ScanItemResults(gomock.Any(), int64(2), int64(3), int64(4), []int32{5}, int64(6)).Return(nil, int64(0), errors.New("dao error"))
			},
			wantLen: 0, wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			got, _, err := repo.ScanItemResults(context.Background(), tt.exptID, tt.cursor, tt.limit, tt.status, tt.spaceID)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.Len(t, got, tt.wantLen)
			}
		})
	}
}

func TestExptItemResultRepoImpl_GetItemIDListByExptID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDAO := mocks.NewMockIExptItemResultDAO(ctrl)
	repo := &ExptItemResultRepoImpl{exptItemResultDAO: mockDAO}

	tests := []struct {
		name      string
		exptID    int64
		spaceID   int64
		mockSetup func()
		wantLen   int
		wantErr   bool
	}{
		{
			name:   "success",
			exptID: 1, spaceID: 2,
			mockSetup: func() {
				mockDAO.EXPECT().GetItemIDListByExptID(gomock.Any(), int64(1), int64(2)).Return([]int64{3, 4}, nil)
			},
			wantLen: 2, wantErr: false,
		},
		{
			name:   "fail_dao_error",
			exptID: 2, spaceID: 3,
			mockSetup: func() {
				mockDAO.EXPECT().GetItemIDListByExptID(gomock.Any(), int64(2), int64(3)).Return(nil, errors.New("dao error"))
			},
			wantLen: 0, wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			got, err := repo.GetItemIDListByExptID(context.Background(), tt.exptID, tt.spaceID)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.Len(t, got, tt.wantLen)
			}
		})
	}
}

func TestExptItemResultRepoImpl_ScanItemRunLogs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDAO := mocks.NewMockIExptItemResultDAO(ctrl)
	repo := &ExptItemResultRepoImpl{exptItemResultDAO: mockDAO}
	filter := &entity.ExptItemRunLogFilter{}

	tests := []struct {
		name      string
		exptID    int64
		exptRunID int64
		cursor    int64
		limit     int64
		spaceID   int64
		mockSetup func()
		wantLen   int
		wantErr   bool
	}{
		{
			name:   "success",
			exptID: 1, exptRunID: 2, cursor: 3, limit: 4, spaceID: 5,
			mockSetup: func() {
				mockDAO.EXPECT().ScanItemRunLogs(gomock.Any(), int64(1), int64(2), filter, int64(3), int64(4), int64(5)).Return([]*model.ExptItemResultRunLog{{}, {}}, int64(10), nil)
			},
			wantLen: 2, wantErr: false,
		},
		{
			name:   "fail_dao_error",
			exptID: 2, exptRunID: 3, cursor: 4, limit: 5, spaceID: 6,
			mockSetup: func() {
				mockDAO.EXPECT().ScanItemRunLogs(gomock.Any(), int64(2), int64(3), filter, int64(4), int64(5), int64(6)).Return(nil, int64(0), errors.New("dao error"))
			},
			wantLen: 0, wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			got, _, err := repo.ScanItemRunLogs(context.Background(), tt.exptID, tt.exptRunID, filter, tt.cursor, tt.limit, tt.spaceID)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.Len(t, got, tt.wantLen)
			}
		})
	}
}

func TestExptItemResultRepoImpl_ScanItemRunLogs_WithRawFilter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDAO := mocks.NewMockIExptItemResultDAO(ctrl)
	repo := &ExptItemResultRepoImpl{exptItemResultDAO: mockDAO}
	rawFilter := &entity.ExptItemRunLogFilter{
		RawFilter: true,
		RawCond:   clause.Expr{SQL: "status IN (?) OR result_state = ?", Vars: []interface{}{[]int32{1}, int32(2)}},
	}

	rs := int32(2)
	mockDAO.EXPECT().ScanItemRunLogs(gomock.Any(), int64(1), int64(2), rawFilter, int64(0), int64(0), int64(3)).
		Return([]*model.ExptItemResultRunLog{
			{ItemID: 10, Status: 1},
			{ItemID: 20, Status: 2, ResultState: &rs},
		}, int64(0), nil)

	got, ncursor, err := repo.ScanItemRunLogs(context.Background(), 1, 2, rawFilter, 0, 0, 3)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), ncursor)
	assert.Len(t, got, 2)
	assert.Equal(t, int64(10), got[0].ItemID)
	assert.Equal(t, int64(20), got[1].ItemID)
	assert.Equal(t, int32(2), got[1].ResultState)
}

func TestExptItemResultRepoImpl_BatchCreateNX(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDAO := mocks.NewMockIExptItemResultDAO(ctrl)
	repo := &ExptItemResultRepoImpl{exptItemResultDAO: mockDAO}
	input := []*entity.ExptItemResult{{}, {}}

	tests := []struct {
		name      string
		mockSetup func()
		wantErr   bool
	}{
		{
			name: "success",
			mockSetup: func() {
				mockDAO.EXPECT().BatchCreateNX(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "fail_dao_error",
			mockSetup: func() {
				mockDAO.EXPECT().BatchCreateNX(gomock.Any(), gomock.Any()).Return(errors.New("dao error"))
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			err := repo.BatchCreateNX(context.Background(), input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptItemResultRepoImpl_BatchCreateNXRunLogs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDAO := mocks.NewMockIExptItemResultDAO(ctrl)
	repo := &ExptItemResultRepoImpl{exptItemResultDAO: mockDAO}
	input := []*entity.ExptItemResultRunLog{{}, {}}

	tests := []struct {
		name      string
		mockSetup func()
		wantErr   bool
	}{
		{
			name: "success",
			mockSetup: func() {
				mockDAO.EXPECT().BatchCreateNXRunLogs(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "fail_dao_error",
			mockSetup: func() {
				mockDAO.EXPECT().BatchCreateNXRunLogs(gomock.Any(), gomock.Any()).Return(errors.New("dao error"))
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			err := repo.BatchCreateNXRunLogs(context.Background(), input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptItemResultRepoImpl_GetMaxItemIdxByExptID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDAO := mocks.NewMockIExptItemResultDAO(ctrl)
	repo := &ExptItemResultRepoImpl{exptItemResultDAO: mockDAO}

	tests := []struct {
		name      string
		exptID    int64
		spaceID   int64
		mockSetup func()
		wantVal   int32
		wantErr   bool
	}{
		{
			name:   "success",
			exptID: 1, spaceID: 2,
			mockSetup: func() {
				mockDAO.EXPECT().GetMaxItemIdxByExptID(gomock.Any(), int64(1), int64(2)).Return(int32(10), nil)
			},
			wantVal: 10, wantErr: false,
		},
		{
			name:   "fail_dao_error",
			exptID: 2, spaceID: 3,
			mockSetup: func() {
				mockDAO.EXPECT().GetMaxItemIdxByExptID(gomock.Any(), int64(2), int64(3)).Return(int32(0), errors.New("dao error"))
			},
			wantVal: 0, wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			got, err := repo.GetMaxItemIdxByExptID(context.Background(), tt.exptID, tt.spaceID)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Equal(t, int32(0), got)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantVal, got)
			}
		})
	}
}
