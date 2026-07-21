// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	mysqlMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/mocks"
)

func newRepo(ctrl *gomock.Controller) (*exptRepoImpl, *mysqlMocks.MockIExptDAO, *mysqlMocks.MockIExptEvaluatorRefDAO, *mocks.MockIIDGenerator) {
	mockExptDAO := mysqlMocks.NewMockIExptDAO(ctrl)
	mockRefDAO := mysqlMocks.NewMockIExptEvaluatorRefDAO(ctrl)
	mockIDGen := mocks.NewMockIIDGenerator(ctrl)
	return &exptRepoImpl{
		idgen:               mockIDGen,
		exptDAO:             mockExptDAO,
		exptEvaluatorRefDAO: mockRefDAO,
	}, mockExptDAO, mockRefDAO, mockIDGen
}

func TestExptRepoImpl_Create(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo, mockExptDAO, mockRefDAO, mockIDGen := newRepo(ctrl)
	expt := &entity.Experiment{}
	rels := []*entity.ExptEvaluatorRef{{}, {}}

	tests := []struct {
		name      string
		mockSetup func()
		wantErr   bool
	}{
		{
			name: "success",
			mockSetup: func() {
				mockExptDAO.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
				mockIDGen.EXPECT().GenMultiIDs(gomock.Any(), 2).Return([]int64{1, 2}, nil)
				mockRefDAO.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "fail_exptDAO",
			mockSetup: func() {
				mockExptDAO.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errors.New("dao error"))
			},
			wantErr: true,
		},
		{
			name: "fail_idgen",
			mockSetup: func() {
				mockExptDAO.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
				mockIDGen.EXPECT().GenMultiIDs(gomock.Any(), 2).Return(nil, errors.New("idgen error"))
			},
			wantErr: true,
		},
		{
			name: "fail_refDAO",
			mockSetup: func() {
				mockExptDAO.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
				mockIDGen.EXPECT().GenMultiIDs(gomock.Any(), 2).Return([]int64{1, 2}, nil)
				mockRefDAO.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errors.New("ref error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			err := repo.Create(context.Background(), expt, rels)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptRepoImpl_Update(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo, mockExptDAO, _, _ := newRepo(ctrl)
	expt := &entity.Experiment{}

	tests := []struct {
		name      string
		mockSetup func()
		wantErr   bool
	}{
		{
			name: "success",
			mockSetup: func() {
				mockExptDAO.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "fail",
			mockSetup: func() {
				mockExptDAO.EXPECT().Update(gomock.Any(), gomock.Any()).Return(errors.New("dao error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			err := repo.Update(context.Background(), expt)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptRepoImpl_Delete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo, mockExptDAO, _, _ := newRepo(ctrl)

	tests := []struct {
		name      string
		mockSetup func()
		wantErr   bool
	}{
		{
			name: "success",
			mockSetup: func() {
				mockExptDAO.EXPECT().Delete(gomock.Any(), int64(1)).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "fail",
			mockSetup: func() {
				mockExptDAO.EXPECT().Delete(gomock.Any(), int64(2)).Return(errors.New("dao error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			var id int64
			if tt.wantErr {
				id = 2
			} else {
				id = 1
			}
			err := repo.Delete(context.Background(), id, 0)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptRepoImpl_MDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo, mockExptDAO, _, _ := newRepo(ctrl)

	tests := []struct {
		name      string
		mockSetup func()
		wantErr   bool
	}{
		{
			name: "success",
			mockSetup: func() {
				mockExptDAO.EXPECT().MDelete(gomock.Any(), []int64{1, 2}).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "fail",
			mockSetup: func() {
				mockExptDAO.EXPECT().MDelete(gomock.Any(), []int64{3, 4}).Return(errors.New("dao error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			var ids []int64
			if tt.wantErr {
				ids = []int64{3, 4}
			} else {
				ids = []int64{1, 2}
			}
			err := repo.MDelete(context.Background(), ids, 0)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptRepoImpl_List(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo, mockExptDAO, mockRefDAO, _ := newRepo(ctrl)

	tests := []struct {
		name      string
		mockSetup func()
		wantErr   bool
		wantLen   int
	}{
		{
			name: "success",
			mockSetup: func() {
				mockExptDAO.EXPECT().List(gomock.Any(), int32(1), int32(10), gomock.Any(), gomock.Any(), int64(1)).Return([]*model.Experiment{{ID: 1}}, int64(1), nil)
				mockRefDAO.EXPECT().MGetByExptID(gomock.Any(), []int64{1}, int64(1)).Return([]*model.ExptEvaluatorRef{{ExptID: 1}}, nil)
			},
			wantErr: false,
			wantLen: 1,
		},
		{
			name: "fail_list",
			mockSetup: func() {
				mockExptDAO.EXPECT().List(gomock.Any(), int32(1), int32(10), gomock.Any(), gomock.Any(), int64(1)).Return(nil, int64(0), errors.New("dao error"))
			},
			wantErr: true,
			wantLen: 0,
		},
		{
			name: "fail_ref",
			mockSetup: func() {
				mockExptDAO.EXPECT().List(gomock.Any(), int32(1), int32(10), gomock.Any(), gomock.Any(), int64(1)).Return([]*model.Experiment{{ID: 1}}, int64(1), nil)
				mockRefDAO.EXPECT().MGetByExptID(gomock.Any(), []int64{1}, int64(1)).Return(nil, errors.New("ref error"))
			},
			wantErr: true,
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			got, _, err := repo.List(context.Background(), 1, 10, nil, nil, 1)
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

func TestExptRepoImpl_GetByID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo, mockExptDAO, mockRefDAO, _ := newRepo(ctrl)

	tests := []struct {
		name      string
		mockSetup func()
		wantErr   bool
		found     bool
	}{
		{
			name: "success",
			mockSetup: func() {
				mockExptDAO.EXPECT().MGetByID(gomock.Any(), []int64{1}).Return([]*model.Experiment{{ID: 1, SpaceID: 1}}, nil)
				mockRefDAO.EXPECT().MGetByExptID(gomock.Any(), []int64{1}, int64(1)).Return([]*model.ExptEvaluatorRef{{ExptID: 1}}, nil)
			},
			wantErr: false,
			found:   true,
		},
		{
			name: "fail_mget",
			mockSetup: func() {
				mockExptDAO.EXPECT().MGetByID(gomock.Any(), []int64{3}).Return(nil, errors.New("dao error"))
			},
			wantErr: true,
			found:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			var id int64
			switch tt.name {
			case "success":
				id = 1
			case "not_found":
				id = 2
			default:
				id = 3
			}
			got, err := repo.GetByID(context.Background(), id, 1)
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

func TestExptRepoImpl_MGetByID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo, mockExptDAO, mockRefDAO, _ := newRepo(ctrl)

	tests := []struct {
		name      string
		mockSetup func()
		wantErr   bool
		wantLen   int
	}{
		{
			name: "success",
			mockSetup: func() {
				mockExptDAO.EXPECT().MGetByID(gomock.Any(), []int64{1, 2}).Return([]*model.Experiment{{ID: 1, SpaceID: 1}, {ID: 2, SpaceID: 1}}, nil)
				mockRefDAO.EXPECT().MGetByExptID(gomock.Any(), []int64{1, 2}, int64(1)).Return([]*model.ExptEvaluatorRef{{ExptID: 1}, {ExptID: 2}}, nil)
			},
			wantErr: false,
			wantLen: 2,
		},
		{
			name: "filter_cross_space",
			mockSetup: func() {
				// 主键命中但 space 不匹配的行应被兜底过滤，防跨空间越权读取。
				mockExptDAO.EXPECT().MGetByID(gomock.Any(), []int64{7, 8}).Return([]*model.Experiment{{ID: 7, SpaceID: 1}, {ID: 8, SpaceID: 2}}, nil)
				mockRefDAO.EXPECT().MGetByExptID(gomock.Any(), []int64{7}, int64(1)).Return([]*model.ExptEvaluatorRef{{ExptID: 7}}, nil)
			},
			wantErr: false,
			wantLen: 1,
		},
		{
			name: "fail_mget",
			mockSetup: func() {
				mockExptDAO.EXPECT().MGetByID(gomock.Any(), []int64{3, 4}).Return(nil, errors.New("dao error"))
			},
			wantErr: true,
			wantLen: 0,
		},
		{
			name: "fail_ref",
			mockSetup: func() {
				mockExptDAO.EXPECT().MGetByID(gomock.Any(), []int64{5, 6}).Return([]*model.Experiment{{ID: 5, SpaceID: 1}, {ID: 6, SpaceID: 1}}, nil)
				mockRefDAO.EXPECT().MGetByExptID(gomock.Any(), []int64{5, 6}, int64(1)).Return(nil, errors.New("ref error"))
			},
			wantErr: true,
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			var ids []int64
			switch tt.name {
			case "success":
				ids = []int64{1, 2}
			case "filter_cross_space":
				ids = []int64{7, 8}
			case "fail_mget":
				ids = []int64{3, 4}
			default:
				ids = []int64{5, 6}
			}
			got, err := repo.MGetByID(context.Background(), ids, 1)
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

func TestExptRepoImpl_MGetBasicByID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo, mockExptDAO, _, _ := newRepo(ctrl)

	tests := []struct {
		name      string
		mockSetup func()
		wantErr   bool
		wantLen   int
	}{
		{
			name: "success",
			mockSetup: func() {
				mockExptDAO.EXPECT().MGetByID(gomock.Any(), []int64{1, 2}).Return([]*model.Experiment{{ID: 1}, {ID: 2}}, nil)
			},
			wantErr: false,
			wantLen: 2,
		},
		{
			name: "fail_mget",
			mockSetup: func() {
				mockExptDAO.EXPECT().MGetByID(gomock.Any(), []int64{3, 4}).Return(nil, errors.New("dao error"))
			},
			wantErr: true,
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			var ids []int64
			if tt.name == "success" {
				ids = []int64{1, 2}
			} else {
				ids = []int64{3, 4}
			}
			got, err := repo.MGetBasicByID(context.Background(), ids)
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

func TestExptRepoImpl_GetByName(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo, mockExptDAO, _, _ := newRepo(ctrl)

	tests := []struct {
		name      string
		mockSetup func()
		wantErr   bool
		found     bool
	}{
		{
			name: "success",
			mockSetup: func() {
				mockExptDAO.EXPECT().GetByName(gomock.Any(), "foo", int64(1)).Return(&model.Experiment{ID: 1}, nil)
			},
			wantErr: false,
			found:   true,
		},
		{
			name: "fail",
			mockSetup: func() {
				mockExptDAO.EXPECT().GetByName(gomock.Any(), "baz", int64(1)).Return(nil, errors.New("dao error"))
			},
			wantErr: true,
			found:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			var name string
			switch tt.name {
			case "success":
				name = "foo"
			case "not_found":
				name = "bar"
			default:
				name = "baz"
			}
			got, found, err := repo.GetByName(context.Background(), name, 1)
			if tt.wantErr {
				assert.Error(t, err)
				assert.False(t, found)
				assert.Nil(t, got)
			} else if !tt.found {
				assert.NoError(t, err)
				assert.False(t, found)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.True(t, found)
				assert.NotNil(t, got)
			}
		})
	}
}

func TestExptRepoImpl_GetEvaluatorRefByExptIDs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo, _, mockRefDAO, _ := newRepo(ctrl)

	tests := []struct {
		name      string
		mockSetup func()
		wantErr   bool
		wantLen   int
	}{
		{
			name: "success",
			mockSetup: func() {
				mockRefDAO.EXPECT().MGetByExptID(gomock.Any(), []int64{1, 2}, int64(1)).Return([]*model.ExptEvaluatorRef{{ExptID: 1}, {ExptID: 2}}, nil)
			},
			wantErr: false,
			wantLen: 2,
		},
		{
			name: "fail",
			mockSetup: func() {
				mockRefDAO.EXPECT().MGetByExptID(gomock.Any(), []int64{3, 4}, int64(1)).Return(nil, errors.New("dao error"))
			},
			wantErr: true,
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			var ids []int64
			if tt.wantErr {
				ids = []int64{3, 4}
			} else {
				ids = []int64{1, 2}
			}
			got, err := repo.GetEvaluatorRefByExptIDs(context.Background(), ids, 1)
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
