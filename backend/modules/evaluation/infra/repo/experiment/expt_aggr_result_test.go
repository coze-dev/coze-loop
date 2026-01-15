// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	idgenMocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	daoMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/mocks"
)

func TestExptAggrResultRepoImpl_GetExptAggrResult(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDAO := daoMocks.NewMockExptAggrResultDAO(ctrl)
	mockIDGen := idgenMocks.NewMockIIDGenerator(ctrl)
	repo := &ExptAggrResultRepoImpl{
		exptAggrResultDAO: mockDAO,
		idgenerator:       mockIDGen,
	}

	tests := []struct {
		name         string
		experimentID int64
		fieldType    int32
		fieldKey     string
		mockSetup    func()
		want         *entity.ExptAggrResult
		wantErr      bool
	}{
		{
			name:         "success",
			experimentID: 1,
			fieldType:    2,
			fieldKey:     "key",
			mockSetup: func() {
				mockDAO.EXPECT().GetExptAggrResult(gomock.Any(), int64(1), int32(2), "key").Return(&model.ExptAggrResult{}, nil)
			},
			want:    &entity.ExptAggrResult{}, // 假设convert.ExptAggrResultPOToDO返回空结构体
			wantErr: false,
		},
		{
			name:         "fail_dao_error",
			experimentID: 2,
			fieldType:    3,
			fieldKey:     "fail",
			mockSetup: func() {
				mockDAO.EXPECT().GetExptAggrResult(gomock.Any(), int64(2), int32(3), "fail").Return(nil, errors.New("dao error"))
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			got, err := repo.GetExptAggrResult(context.Background(), tt.experimentID, tt.fieldType, tt.fieldKey)
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

func TestExptAggrResultRepoImpl_GetExptAggrResultByExperimentID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDAO := daoMocks.NewMockExptAggrResultDAO(ctrl)
	mockIDGen := idgenMocks.NewMockIIDGenerator(ctrl)
	repo := &ExptAggrResultRepoImpl{
		exptAggrResultDAO: mockDAO,
		idgenerator:       mockIDGen,
	}

	tests := []struct {
		name         string
		experimentID int64
		mockSetup    func()
		wantLen      int
		wantErr      bool
	}{
		{
			name:         "success",
			experimentID: 1,
			mockSetup: func() {
				mockDAO.EXPECT().GetExptAggrResultByExperimentID(gomock.Any(), int64(1)).Return([]*model.ExptAggrResult{{}, {}}, nil)
			},
			wantLen: 2,
			wantErr: false,
		},
		{
			name:         "fail_dao_error",
			experimentID: 2,
			mockSetup: func() {
				mockDAO.EXPECT().GetExptAggrResultByExperimentID(gomock.Any(), int64(2)).Return(nil, errors.New("dao error"))
			},
			wantLen: 0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			got, err := repo.GetExptAggrResultByExperimentID(context.Background(), tt.experimentID)
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

func TestExptAggrResultRepoImpl_BatchGetExptAggrResultByExperimentIDs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDAO := daoMocks.NewMockExptAggrResultDAO(ctrl)
	mockIDGen := idgenMocks.NewMockIIDGenerator(ctrl)
	repo := &ExptAggrResultRepoImpl{
		exptAggrResultDAO: mockDAO,
		idgenerator:       mockIDGen,
	}

	tests := []struct {
		name          string
		experimentIDs []int64
		mockSetup     func()
		wantLen       int
		wantErr       bool
	}{
		{
			name:          "success",
			experimentIDs: []int64{1, 2},
			mockSetup: func() {
				mockDAO.EXPECT().BatchGetExptAggrResultByExperimentIDs(gomock.Any(), []int64{1, 2}).Return([]*model.ExptAggrResult{{}, {}}, nil)
			},
			wantLen: 2,
			wantErr: false,
		},
		{
			name:          "fail_dao_error",
			experimentIDs: []int64{3, 4},
			mockSetup: func() {
				mockDAO.EXPECT().BatchGetExptAggrResultByExperimentIDs(gomock.Any(), []int64{3, 4}).Return(nil, errors.New("dao error"))
			},
			wantLen: 0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			got, err := repo.BatchGetExptAggrResultByExperimentIDs(context.Background(), tt.experimentIDs)
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

func TestExptAggrResultRepoImpl_CreateExptAggrResult(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDAO := daoMocks.NewMockExptAggrResultDAO(ctrl)
	mockIDGen := idgenMocks.NewMockIIDGenerator(ctrl)
	repo := &ExptAggrResultRepoImpl{
		exptAggrResultDAO: mockDAO,
		idgenerator:       mockIDGen,
	}

	tests := []struct {
		name      string
		input     *entity.ExptAggrResult
		mockSetup func()
		wantErr   bool
	}{
		{
			name:  "success",
			input: &entity.ExptAggrResult{},
			mockSetup: func() {
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(123), nil)
				mockDAO.EXPECT().CreateExptAggrResult(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: false,
		},
		{
			name:  "fail_idgen",
			input: &entity.ExptAggrResult{},
			mockSetup: func() {
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(0), errors.New("idgen error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			err := repo.CreateExptAggrResult(context.Background(), tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, int64(123), tt.input.ID)
			}
		})
	}
}

func TestExptAggrResultRepoImpl_BatchCreateExptAggrResult(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDAO := daoMocks.NewMockExptAggrResultDAO(ctrl)
	mockIDGen := idgenMocks.NewMockIIDGenerator(ctrl)
	repo := &ExptAggrResultRepoImpl{
		exptAggrResultDAO: mockDAO,
		idgenerator:       mockIDGen,
	}

	tests := []struct {
		name      string
		input     []*entity.ExptAggrResult
		mockSetup func()
		wantErr   bool
	}{
		{
			name:  "success",
			input: []*entity.ExptAggrResult{{}, {}},
			mockSetup: func() {
				mockIDGen.EXPECT().GenMultiIDs(gomock.Any(), 2).Return([]int64{1, 2}, nil)
				mockDAO.EXPECT().BatchCreateExptAggrResult(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: false,
		},
		{
			name:  "fail_idgen",
			input: []*entity.ExptAggrResult{{}, {}},
			mockSetup: func() {
				mockIDGen.EXPECT().GenMultiIDs(gomock.Any(), 2).Return(nil, errors.New("idgen error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			err := repo.BatchCreateExptAggrResult(context.Background(), tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, int64(1), tt.input[0].ID)
				assert.Equal(t, int64(2), tt.input[1].ID)
			}
		})
	}
}

func TestExptAggrResultRepoImpl_UpdateExptAggrResultByVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDAO := daoMocks.NewMockExptAggrResultDAO(ctrl)
	mockIDGen := idgenMocks.NewMockIIDGenerator(ctrl)
	repo := &ExptAggrResultRepoImpl{
		exptAggrResultDAO: mockDAO,
		idgenerator:       mockIDGen,
	}

	tests := []struct {
		name      string
		input     *entity.ExptAggrResult
		version   int64
		mockSetup func()
		wantErr   bool
	}{
		{
			name:    "success",
			input:   &entity.ExptAggrResult{},
			version: 10,
			mockSetup: func() {
				mockDAO.EXPECT().UpdateExptAggrResultByVersion(gomock.Any(), gomock.Any(), int64(10)).Return(nil)
			},
			wantErr: false,
		},
		{
			name:    "fail_dao_error",
			input:   &entity.ExptAggrResult{},
			version: 20,
			mockSetup: func() {
				mockDAO.EXPECT().UpdateExptAggrResultByVersion(gomock.Any(), gomock.Any(), int64(20)).Return(errors.New("dao error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			err := repo.UpdateExptAggrResultByVersion(context.Background(), tt.input, tt.version)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptAggrResultRepoImpl_UpdateAndGetLatestVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDAO := daoMocks.NewMockExptAggrResultDAO(ctrl)
	mockIDGen := idgenMocks.NewMockIIDGenerator(ctrl)
	repo := &ExptAggrResultRepoImpl{
		exptAggrResultDAO: mockDAO,
		idgenerator:       mockIDGen,
	}

	tests := []struct {
		name         string
		experimentID int64
		fieldType    int32
		fieldKey     string
		mockSetup    func()
		wantVersion  int64
		wantErr      bool
	}{
		{
			name:         "success",
			experimentID: 1,
			fieldType:    2,
			fieldKey:     "key",
			mockSetup: func() {
				mockDAO.EXPECT().UpdateAndGetLatestVersion(gomock.Any(), int64(1), int32(2), "key").Return(int64(100), nil)
			},
			wantVersion: 100,
			wantErr:     false,
		},
		{
			name:         "fail_dao_error",
			experimentID: 2,
			fieldType:    3,
			fieldKey:     "fail",
			mockSetup: func() {
				mockDAO.EXPECT().UpdateAndGetLatestVersion(gomock.Any(), int64(2), int32(3), "fail").Return(int64(0), errors.New("dao error"))
			},
			wantVersion: 0,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			got, err := repo.UpdateAndGetLatestVersion(context.Background(), tt.experimentID, tt.fieldType, tt.fieldKey)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Equal(t, int64(0), got)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantVersion, got)
			}
		})
	}
}
