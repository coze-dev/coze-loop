// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/mocks"
)

func TestExptStatsRepo_Create(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockDAO := mocks.NewMockIExptStatsDAO(ctrl)
	repo := &exptStatsRepo{exptStatsDAO: mockDAO}
	input := &entity.ExptStats{}

	tests := []struct {
		name      string
		mockSetup func()
		wantErr   bool
	}{
		{
			name: "success",
			mockSetup: func() {
				mockDAO.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "fail",
			mockSetup: func() {
				mockDAO.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errors.New("dao error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			err := repo.Create(context.Background(), input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptStatsRepo_Get(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockDAO := mocks.NewMockIExptStatsDAO(ctrl)
	repo := &exptStatsRepo{exptStatsDAO: mockDAO}

	tests := []struct {
		name      string
		exptID    int64
		spaceID   int64
		mockSetup func()
		wantErr   bool
	}{
		{
			name:    "success",
			exptID:  1,
			spaceID: 2,
			mockSetup: func() {
				mockDAO.EXPECT().Get(gomock.Any(), int64(1), int64(2)).Return(&model.ExptStats{}, nil)
			},
			wantErr: false,
		},
		{
			name:    "fail",
			exptID:  3,
			spaceID: 4,
			mockSetup: func() {
				mockDAO.EXPECT().Get(gomock.Any(), int64(3), int64(4)).Return(nil, errors.New("dao error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			got, err := repo.Get(context.Background(), tt.exptID, tt.spaceID)
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

func TestExptStatsRepo_MGet(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockDAO := mocks.NewMockIExptStatsDAO(ctrl)
	repo := &exptStatsRepo{exptStatsDAO: mockDAO}

	tests := []struct {
		name      string
		exptIDs   []int64
		spaceID   int64
		mockSetup func()
		wantErr   bool
		wantLen   int
	}{
		{
			name:    "success",
			exptIDs: []int64{1, 2},
			spaceID: 3,
			mockSetup: func() {
				mockDAO.EXPECT().MGet(gomock.Any(), []int64{1, 2}, int64(3)).Return([]*model.ExptStats{{}, {}}, nil)
			},
			wantErr: false,
			wantLen: 2,
		},
		{
			name:    "fail",
			exptIDs: []int64{4, 5},
			spaceID: 6,
			mockSetup: func() {
				mockDAO.EXPECT().MGet(gomock.Any(), []int64{4, 5}, int64(6)).Return(nil, errors.New("dao error"))
			},
			wantErr: true,
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			got, err := repo.MGet(context.Background(), tt.exptIDs, tt.spaceID)
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

func TestExptStatsRepo_UpdateByExptID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockDAO := mocks.NewMockIExptStatsDAO(ctrl)
	repo := &exptStatsRepo{exptStatsDAO: mockDAO}
	input := &entity.ExptStats{}

	tests := []struct {
		name      string
		exptID    int64
		spaceID   int64
		mockSetup func()
		wantErr   bool
	}{
		{
			name:    "success",
			exptID:  1,
			spaceID: 2,
			mockSetup: func() {
				mockDAO.EXPECT().UpdateByExptID(gomock.Any(), int64(1), int64(2), gomock.Any()).Return(nil)
			},
			wantErr: false,
		},
		{
			name:    "fail",
			exptID:  3,
			spaceID: 4,
			mockSetup: func() {
				mockDAO.EXPECT().UpdateByExptID(gomock.Any(), int64(3), int64(4), gomock.Any()).Return(errors.New("dao error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			err := repo.UpdateByExptID(context.Background(), tt.exptID, tt.spaceID, input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptStatsRepo_ArithOperateCount(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockDAO := mocks.NewMockIExptStatsDAO(ctrl)
	repo := &exptStatsRepo{exptStatsDAO: mockDAO}
	input := &entity.StatsCntArithOp{}

	tests := []struct {
		name      string
		exptID    int64
		spaceID   int64
		mockSetup func()
		wantErr   bool
	}{
		{
			name:    "success",
			exptID:  1,
			spaceID: 2,
			mockSetup: func() {
				mockDAO.EXPECT().ArithOperateCount(gomock.Any(), int64(1), int64(2), input).Return(nil)
			},
			wantErr: false,
		},
		{
			name:    "fail",
			exptID:  3,
			spaceID: 4,
			mockSetup: func() {
				mockDAO.EXPECT().ArithOperateCount(gomock.Any(), int64(3), int64(4), input).Return(errors.New("dao error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			err := repo.ArithOperateCount(context.Background(), tt.exptID, tt.spaceID, input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptStatsRepo_Save(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockDAO := mocks.NewMockIExptStatsDAO(ctrl)
	repo := &exptStatsRepo{exptStatsDAO: mockDAO}
	input := &entity.ExptStats{}

	tests := []struct {
		name      string
		mockSetup func()
		wantErr   bool
	}{
		{
			name: "success",
			mockSetup: func() {
				mockDAO.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "fail",
			mockSetup: func() {
				mockDAO.EXPECT().Save(gomock.Any(), gomock.Any()).Return(errors.New("dao error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			err := repo.Save(context.Background(), input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
