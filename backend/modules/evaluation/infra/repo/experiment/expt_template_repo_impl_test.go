// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	mysqlMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/mocks"
)

func newTemplateRepo(ctrl *gomock.Controller) (*exptTemplateRepoImpl, *mysqlMocks.MockIExptTemplateDAO, *mysqlMocks.MockIExptTemplateEvaluatorRefDAO, *mocks.MockIIDGenerator) {
	mockTemplateDAO := mysqlMocks.NewMockIExptTemplateDAO(ctrl)
	mockRefDAO := mysqlMocks.NewMockIExptTemplateEvaluatorRefDAO(ctrl)
	mockIDGen := mocks.NewMockIIDGenerator(ctrl)
	return &exptTemplateRepoImpl{
		idgen:                   mockIDGen,
		templateDAO:             mockTemplateDAO,
		templateEvaluatorRefDAO: mockRefDAO,
	}, mockTemplateDAO, mockRefDAO, mockIDGen
}

func TestExptTemplateRepoImpl_Create(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo, mockTemplateDAO, mockRefDAO, mockIDGen := newTemplateRepo(ctrl)

	template := &entity.ExptTemplate{
		Meta: &entity.ExptTemplateMeta{
			ID:          0,
			WorkspaceID: 100,
			Name:        "Test Template",
			Desc:        "Test Description",
			ExptType:    entity.ExptType_Offline,
		},
		TripleConfig: &entity.ExptTemplateTuple{
			EvalSetID:        1,
			EvalSetVersionID: 1,
			TargetID:         1,
			TargetVersionID:  1,
			TargetType:       entity.EvalTargetTypeLoopPrompt,
		},
		BaseInfo: &entity.BaseInfo{
			CreatedBy: &entity.UserInfo{
				UserID: gptr.Of("user123"),
			},
		},
	}
	refs := []*entity.ExptTemplateEvaluatorRef{
		{
			ID:                 0,
			SpaceID:            100,
			ExptTemplateID:     0,
			EvaluatorID:        1,
			EvaluatorVersionID: 1,
		},
		{
			ID:                 0,
			SpaceID:            100,
			ExptTemplateID:     0,
			EvaluatorID:        2,
			EvaluatorVersionID: 2,
		},
	}

	tests := []struct {
		name      string
		mockSetup func()
		wantErr   bool
	}{
		{
			name: "success",
			mockSetup: func() {
				mockTemplateDAO.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
				mockIDGen.EXPECT().GenMultiIDs(gomock.Any(), 2).Return([]int64{1, 2}, nil)
				mockRefDAO.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "success_no_refs",
			mockSetup: func() {
				mockTemplateDAO.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "fail_templateDAO",
			mockSetup: func() {
				mockTemplateDAO.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errors.New("dao error"))
			},
			wantErr: true,
		},
		{
			name: "fail_idgen",
			mockSetup: func() {
				mockTemplateDAO.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
				mockIDGen.EXPECT().GenMultiIDs(gomock.Any(), 2).Return(nil, errors.New("idgen error"))
			},
			wantErr: true,
		},
		{
			name: "fail_refDAO",
			mockSetup: func() {
				mockTemplateDAO.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
				mockIDGen.EXPECT().GenMultiIDs(gomock.Any(), 2).Return([]int64{1, 2}, nil)
				mockRefDAO.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errors.New("ref error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			testRefs := refs
			if tt.name == "success_no_refs" {
				testRefs = nil
			}
			err := repo.Create(context.Background(), template, testRefs)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptTemplateRepoImpl_GetByID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo, mockTemplateDAO, mockRefDAO, _ := newTemplateRepo(ctrl)

	tests := []struct {
		name      string
		mockSetup func()
		wantErr   bool
		found     bool
		spaceID   int64
	}{
		{
			name: "success",
			mockSetup: func() {
				mockTemplateDAO.EXPECT().GetByID(gomock.Any(), int64(1)).Return(&model.ExptTemplate{
					ID:      1,
					SpaceID: 100,
					Name:    "Test Template",
				}, nil)
				mockRefDAO.EXPECT().GetByTemplateIDs(gomock.Any(), []int64{1}).Return([]*model.ExptTemplateEvaluatorRef{
					{ID: 1, ExptTemplateID: 1, EvaluatorID: 1, EvaluatorVersionID: 1},
				}, nil)
			},
			wantErr: false,
			found:   true,
			spaceID: 100,
		},
		{
			name: "not_found",
			mockSetup: func() {
				mockTemplateDAO.EXPECT().GetByID(gomock.Any(), int64(2)).Return(nil, nil)
			},
			wantErr: false,
			found:   false,
			spaceID: 100,
		},
		{
			name: "wrong_spaceID",
			mockSetup: func() {
				mockTemplateDAO.EXPECT().GetByID(gomock.Any(), int64(3)).Return(&model.ExptTemplate{
					ID:      3,
					SpaceID: 200,
					Name:    "Test Template",
				}, nil)
			},
			wantErr: true,
			found:   false,
			spaceID: 100,
		},
		{
			name: "fail_templateDAO",
			mockSetup: func() {
				mockTemplateDAO.EXPECT().GetByID(gomock.Any(), int64(4)).Return(nil, errors.New("dao error"))
			},
			wantErr: true,
			found:   false,
			spaceID: 100,
		},
		{
			name: "fail_refDAO",
			mockSetup: func() {
				mockTemplateDAO.EXPECT().GetByID(gomock.Any(), int64(5)).Return(&model.ExptTemplate{
					ID:      5,
					SpaceID: 100,
					Name:    "Test Template",
				}, nil)
				mockRefDAO.EXPECT().GetByTemplateIDs(gomock.Any(), []int64{5}).Return(nil, errors.New("ref error"))
			},
			wantErr: true,
			found:   false,
			spaceID: 100,
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
			case "wrong_spaceID":
				id = 3
			case "fail_templateDAO":
				id = 4
			case "fail_refDAO":
				id = 5
			default:
				id = 1
			}
			var spaceIDPtr *int64
			if tt.spaceID != 0 {
				spaceIDPtr = &tt.spaceID
			}
			got, err := repo.GetByID(context.Background(), id, spaceIDPtr)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				if tt.found {
					assert.NotNil(t, got)
				} else {
					assert.Nil(t, got)
				}
			}
		})
	}
}

func TestExptTemplateRepoImpl_GetByName(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo, mockTemplateDAO, mockRefDAO, _ := newTemplateRepo(ctrl)

	tests := []struct {
		name      string
		mockSetup func()
		wantErr   bool
		found     bool
		spaceID   int64
	}{
		{
			name: "success",
			mockSetup: func() {
				mockTemplateDAO.EXPECT().GetByName(gomock.Any(), "Test Template", int64(100)).Return(&model.ExptTemplate{
					ID:      1,
					SpaceID: 100,
					Name:    "Test Template",
				}, nil)
				mockRefDAO.EXPECT().GetByTemplateIDs(gomock.Any(), []int64{1}).Return([]*model.ExptTemplateEvaluatorRef{
					{ID: 1, ExptTemplateID: 1, EvaluatorID: 1, EvaluatorVersionID: 1},
				}, nil)
			},
			wantErr: false,
			found:   true,
			spaceID: 100,
		},
		{
			name: "not_found",
			mockSetup: func() {
				mockTemplateDAO.EXPECT().GetByName(gomock.Any(), "Not Found", int64(100)).Return(nil, nil)
			},
			wantErr: false,
			found:   false,
			spaceID: 100,
		},
		{
			name: "fail_templateDAO",
			mockSetup: func() {
				mockTemplateDAO.EXPECT().GetByName(gomock.Any(), "Error Template", int64(100)).Return(nil, errors.New("dao error"))
			},
			wantErr: true,
			found:   false,
			spaceID: 100,
		},
		{
			name: "fail_refDAO",
			mockSetup: func() {
				mockTemplateDAO.EXPECT().GetByName(gomock.Any(), "Ref Error", int64(100)).Return(&model.ExptTemplate{
					ID:      2,
					SpaceID: 100,
					Name:    "Ref Error",
				}, nil)
				mockRefDAO.EXPECT().GetByTemplateIDs(gomock.Any(), []int64{2}).Return(nil, errors.New("ref error"))
			},
			wantErr: true,
			found:   false,
			spaceID: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			var name string
			switch tt.name {
			case "success":
				name = "Test Template"
			case "not_found":
				name = "Not Found"
			case "fail_templateDAO":
				name = "Error Template"
			case "fail_refDAO":
				name = "Ref Error"
			default:
				name = "Test Template"
			}
			got, found, err := repo.GetByName(context.Background(), name, tt.spaceID)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
				assert.False(t, found)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.found, found)
				if tt.found {
					assert.NotNil(t, got)
				} else {
					assert.Nil(t, got)
				}
			}
		})
	}
}

func TestExptTemplateRepoImpl_MGetByID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo, mockTemplateDAO, mockRefDAO, _ := newTemplateRepo(ctrl)

	tests := []struct {
		name      string
		mockSetup func()
		wantErr   bool
		wantLen   int
		ids       []int64
		spaceID   int64
	}{
		{
			name: "success",
			mockSetup: func() {
				mockTemplateDAO.EXPECT().MGetByID(gomock.Any(), []int64{1, 2}, gomock.Any()).Return([]*model.ExptTemplate{
					{ID: 1, SpaceID: 100, Name: "Template 1"},
					{ID: 2, SpaceID: 100, Name: "Template 2"},
				}, nil)
				mockRefDAO.EXPECT().GetByTemplateIDs(gomock.Any(), []int64{1, 2}).Return([]*model.ExptTemplateEvaluatorRef{
					{ID: 1, ExptTemplateID: 1, EvaluatorID: 1, EvaluatorVersionID: 1},
					{ID: 2, ExptTemplateID: 2, EvaluatorID: 2, EvaluatorVersionID: 2},
				}, nil)
			},
			wantErr: false,
			wantLen: 2,
			ids:     []int64{1, 2},
			spaceID: 100,
		},
		{
			name: "empty_ids",
			mockSetup: func() {
				// 空 IDs 应该直接返回，不调用任何 DAO
			},
			wantErr: false,
			wantLen: 0,
			ids:     []int64{},
			spaceID: 100,
		},
		{
			name: "filter_by_spaceID",
			mockSetup: func() {
				mockTemplateDAO.EXPECT().MGetByID(gomock.Any(), []int64{3, 4}, gomock.Any()).Return([]*model.ExptTemplate{
					{ID: 3, SpaceID: 100, Name: "Template 3"},
					{ID: 4, SpaceID: 200, Name: "Template 4"}, // 不同的 spaceID，应该被过滤
				}, nil)
				mockRefDAO.EXPECT().GetByTemplateIDs(gomock.Any(), []int64{3}).Return([]*model.ExptTemplateEvaluatorRef{
					{ID: 3, ExptTemplateID: 3, EvaluatorID: 1, EvaluatorVersionID: 1},
				}, nil)
			},
			wantErr: false,
			wantLen: 1,
			ids:     []int64{3, 4},
			spaceID: 100,
		},
		{
			name: "fail_templateDAO",
			mockSetup: func() {
				mockTemplateDAO.EXPECT().MGetByID(gomock.Any(), []int64{5, 6}, gomock.Any()).Return(nil, errors.New("dao error"))
			},
			wantErr: true,
			wantLen: 0,
			ids:     []int64{5, 6},
			spaceID: 100,
		},
		{
			name: "fail_refDAO",
			mockSetup: func() {
				mockTemplateDAO.EXPECT().MGetByID(gomock.Any(), []int64{7, 8}, gomock.Any()).Return([]*model.ExptTemplate{
					{ID: 7, SpaceID: 100, Name: "Template 7"},
					{ID: 8, SpaceID: 100, Name: "Template 8"},
				}, nil)
				mockRefDAO.EXPECT().GetByTemplateIDs(gomock.Any(), []int64{7, 8}).Return(nil, errors.New("ref error"))
			},
			wantErr: true,
			wantLen: 0,
			ids:     []int64{7, 8},
			spaceID: 100,
		},
		{
			name: "no_matching_spaceID",
			mockSetup: func() {
				mockTemplateDAO.EXPECT().MGetByID(gomock.Any(), []int64{9, 10}, gomock.Any()).Return([]*model.ExptTemplate{
					{ID: 9, SpaceID: 200, Name: "Template 9"},
					{ID: 10, SpaceID: 300, Name: "Template 10"},
				}, nil)
				// 没有匹配的 spaceID，不会调用 refDAO
			},
			wantErr: false,
			wantLen: 0,
			ids:     []int64{9, 10},
			spaceID: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			got, err := repo.MGetByID(context.Background(), tt.ids, tt.spaceID)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				if tt.wantLen == 0 {
					assert.Nil(t, got)
				} else {
					assert.Len(t, got, tt.wantLen)
				}
			}
		})
	}
}

func TestExptTemplateRepoImpl_Update(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo, mockTemplateDAO, _, _ := newTemplateRepo(ctrl)

	template := &entity.ExptTemplate{
		Meta: &entity.ExptTemplateMeta{
			ID:          1,
			WorkspaceID: 100,
			Name:        "Updated Template",
			Desc:        "Updated Description",
			ExptType:    entity.ExptType_Offline,
		},
		BaseInfo: &entity.BaseInfo{
			UpdatedBy: &entity.UserInfo{
				UserID: gptr.Of("user123"),
			},
		},
	}

	tests := []struct {
		name      string
		mockSetup func()
		wantErr   bool
	}{
		{
			name: "success",
			mockSetup: func() {
				mockTemplateDAO.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "fail",
			mockSetup: func() {
				mockTemplateDAO.EXPECT().Update(gomock.Any(), gomock.Any()).Return(errors.New("dao error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			err := repo.Update(context.Background(), template)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptTemplateRepoImpl_UpdateFields(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo, mockTemplateDAO, _, _ := newTemplateRepo(ctrl)

	tests := []struct {
		name      string
		mockSetup func()
		wantErr   bool
		fields    map[string]any
	}{
		{
			name: "success",
			mockSetup: func() {
				mockTemplateDAO.EXPECT().UpdateFields(gomock.Any(), int64(1), gomock.Any()).Return(nil)
			},
			wantErr: false,
			fields:  map[string]any{"name": "Updated Name"},
		},
		{
			name: "fail",
			mockSetup: func() {
				mockTemplateDAO.EXPECT().UpdateFields(gomock.Any(), int64(2), gomock.Any()).Return(errors.New("dao error"))
			},
			wantErr: true,
			fields:  map[string]any{"name": "Updated Name"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			var id int64
			if tt.name == "success" {
				id = 1
			} else {
				id = 2
			}
			err := repo.UpdateFields(context.Background(), id, tt.fields)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptTemplateRepoImpl_UpdateWithRefs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo, mockTemplateDAO, mockRefDAO, mockIDGen := newTemplateRepo(ctrl)

	template := &entity.ExptTemplate{
		Meta: &entity.ExptTemplateMeta{
			ID:          1,
			WorkspaceID: 100,
			Name:        "Updated Template",
			Desc:        "Updated Description",
			ExptType:    entity.ExptType_Offline,
		},
		BaseInfo: &entity.BaseInfo{
			UpdatedBy: &entity.UserInfo{
				UserID: gptr.Of("user123"),
			},
		},
	}

	tests := []struct {
		name      string
		mockSetup func()
		wantErr   bool
		refs      []*entity.ExptTemplateEvaluatorRef
	}{
		{
			name: "success_no_refs",
			mockSetup: func() {
				mockTemplateDAO.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
				mockRefDAO.EXPECT().GetByTemplateIDsIncludeDeleted(gomock.Any(), []int64{1}).Return([]*model.ExptTemplateEvaluatorRef{}, nil)
			},
			wantErr: false,
			refs:    []*entity.ExptTemplateEvaluatorRef{},
		},
		{
			name: "success_create_new_refs",
			mockSetup: func() {
				mockTemplateDAO.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
				mockRefDAO.EXPECT().GetByTemplateIDsIncludeDeleted(gomock.Any(), []int64{1}).Return([]*model.ExptTemplateEvaluatorRef{}, nil)
				mockIDGen.EXPECT().GenMultiIDs(gomock.Any(), 1).Return([]int64{10}, nil)
				mockRefDAO.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: false,
			refs: []*entity.ExptTemplateEvaluatorRef{
				{
					ID:                 0,
					SpaceID:            100,
					ExptTemplateID:     1,
					EvaluatorID:        1,
					EvaluatorVersionID: 1,
				},
			},
		},
		{
			name: "success_restore_deleted_refs",
			mockSetup: func() {
				mockTemplateDAO.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
				// 返回一个已删除的引用
				mockRefDAO.EXPECT().GetByTemplateIDsIncludeDeleted(gomock.Any(), []int64{1}).Return([]*model.ExptTemplateEvaluatorRef{
					{
						ID:                 20,
						ExptTemplateID:     1,
						EvaluatorID:        1,
						EvaluatorVersionID: 1,
						DeletedAt:          gorm.DeletedAt{Valid: true, Time: time.Now()}, // 已删除
					},
				}, nil)
				mockRefDAO.EXPECT().RestoreByIDs(gomock.Any(), []int64{20}).Return(nil)
			},
			wantErr: false,
			refs: []*entity.ExptTemplateEvaluatorRef{
				{
					ID:                 0,
					SpaceID:            100,
					ExptTemplateID:     1,
					EvaluatorID:        1,
					EvaluatorVersionID: 1,
				},
			},
		},
		{
			name: "success_soft_delete_refs",
			mockSetup: func() {
				mockTemplateDAO.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
				// 返回一个未删除的引用，但不在新的 refs 列表中
				mockRefDAO.EXPECT().GetByTemplateIDsIncludeDeleted(gomock.Any(), []int64{1}).Return([]*model.ExptTemplateEvaluatorRef{
					{
						ID:                 30,
						ExptTemplateID:     1,
						EvaluatorID:        1,
						EvaluatorVersionID: 1,
						DeletedAt:          gorm.DeletedAt{Valid: false}, // 未删除
					},
				}, nil)
				mockRefDAO.EXPECT().SoftDeleteByIDs(gomock.Any(), []int64{30}).Return(nil)
			},
			wantErr: false,
			refs:    []*entity.ExptTemplateEvaluatorRef{}, // 空的 refs，应该软删除现有的
		},
		{
			name: "fail_templateDAO",
			mockSetup: func() {
				mockTemplateDAO.EXPECT().Update(gomock.Any(), gomock.Any()).Return(errors.New("dao error"))
			},
			wantErr: true,
			refs:    []*entity.ExptTemplateEvaluatorRef{},
		},
		{
			name: "fail_get_existing_refs",
			mockSetup: func() {
				mockTemplateDAO.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
				mockRefDAO.EXPECT().GetByTemplateIDsIncludeDeleted(gomock.Any(), []int64{1}).Return(nil, errors.New("ref error"))
			},
			wantErr: true,
			refs:    []*entity.ExptTemplateEvaluatorRef{},
		},
		{
			name: "fail_restore",
			mockSetup: func() {
				mockTemplateDAO.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
				mockRefDAO.EXPECT().GetByTemplateIDsIncludeDeleted(gomock.Any(), []int64{1}).Return([]*model.ExptTemplateEvaluatorRef{
					{
						ID:                 40,
						ExptTemplateID:     1,
						EvaluatorID:        1,
						EvaluatorVersionID: 1,
						DeletedAt:          gorm.DeletedAt{Valid: true, Time: time.Now()},
					},
				}, nil)
				mockRefDAO.EXPECT().RestoreByIDs(gomock.Any(), []int64{40}).Return(errors.New("restore error"))
			},
			wantErr: true,
			refs: []*entity.ExptTemplateEvaluatorRef{
				{
					ID:                 0,
					SpaceID:            100,
					ExptTemplateID:     1,
					EvaluatorID:        1,
					EvaluatorVersionID: 1,
				},
			},
		},
		{
			name: "fail_soft_delete",
			mockSetup: func() {
				mockTemplateDAO.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
				mockRefDAO.EXPECT().GetByTemplateIDsIncludeDeleted(gomock.Any(), []int64{1}).Return([]*model.ExptTemplateEvaluatorRef{
					{
						ID:                 50,
						ExptTemplateID:     1,
						EvaluatorID:        1,
						EvaluatorVersionID: 1,
						DeletedAt:          gorm.DeletedAt{Valid: false},
					},
				}, nil)
				mockRefDAO.EXPECT().SoftDeleteByIDs(gomock.Any(), []int64{50}).Return(errors.New("soft delete error"))
			},
			wantErr: true,
			refs:    []*entity.ExptTemplateEvaluatorRef{},
		},
		{
			name: "fail_create_refs",
			mockSetup: func() {
				mockTemplateDAO.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
				mockRefDAO.EXPECT().GetByTemplateIDsIncludeDeleted(gomock.Any(), []int64{1}).Return([]*model.ExptTemplateEvaluatorRef{}, nil)
				mockIDGen.EXPECT().GenMultiIDs(gomock.Any(), 1).Return([]int64{60}, nil)
				mockRefDAO.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errors.New("create error"))
			},
			wantErr: true,
			refs: []*entity.ExptTemplateEvaluatorRef{
				{
					ID:                 0,
					SpaceID:            100,
					ExptTemplateID:     1,
					EvaluatorID:        1,
					EvaluatorVersionID: 1,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			err := repo.UpdateWithRefs(context.Background(), template, tt.refs)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptTemplateRepoImpl_Delete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo, mockTemplateDAO, _, _ := newTemplateRepo(ctrl)

	tests := []struct {
		name      string
		mockSetup func()
		wantErr   bool
		id        int64
		spaceID   int64
	}{
		{
			name: "success",
			mockSetup: func() {
				mockTemplateDAO.EXPECT().GetByID(gomock.Any(), int64(1)).Return(&model.ExptTemplate{
					ID:      1,
					SpaceID: 100,
					Name:    "Test Template",
				}, nil)
				mockTemplateDAO.EXPECT().Delete(gomock.Any(), int64(1)).Return(nil)
			},
			wantErr: false,
			id:      1,
			spaceID: 100,
		},
		{
			name: "not_found",
			mockSetup: func() {
				mockTemplateDAO.EXPECT().GetByID(gomock.Any(), int64(2)).Return(nil, nil)
			},
			wantErr: true,
			id:      2,
			spaceID: 100,
		},
		{
			name: "wrong_spaceID",
			mockSetup: func() {
				mockTemplateDAO.EXPECT().GetByID(gomock.Any(), int64(3)).Return(&model.ExptTemplate{
					ID:      3,
					SpaceID: 200,
					Name:    "Test Template",
				}, nil)
			},
			wantErr: true,
			id:      3,
			spaceID: 100,
		},
		{
			name: "fail_getByID",
			mockSetup: func() {
				mockTemplateDAO.EXPECT().GetByID(gomock.Any(), int64(4)).Return(nil, errors.New("dao error"))
			},
			wantErr: true,
			id:      4,
			spaceID: 100,
		},
		{
			name: "fail_delete",
			mockSetup: func() {
				mockTemplateDAO.EXPECT().GetByID(gomock.Any(), int64(5)).Return(&model.ExptTemplate{
					ID:      5,
					SpaceID: 100,
					Name:    "Test Template",
				}, nil)
				mockTemplateDAO.EXPECT().Delete(gomock.Any(), int64(5)).Return(errors.New("delete error"))
			},
			wantErr: true,
			id:      5,
			spaceID: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			err := repo.Delete(context.Background(), tt.id, tt.spaceID)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptTemplateRepoImpl_List(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo, mockTemplateDAO, mockRefDAO, _ := newTemplateRepo(ctrl)

	tests := []struct {
		name      string
		mockSetup func()
		wantErr   bool
		wantLen   int
		wantCount int64
		page      int32
		size      int32
		spaceID   int64
	}{
		{
			name: "success",
			mockSetup: func() {
				mockTemplateDAO.EXPECT().List(gomock.Any(), int32(1), int32(10), gomock.Any(), gomock.Any(), int64(100)).Return([]*model.ExptTemplate{
					{ID: 1, SpaceID: 100, Name: "Template 1"},
					{ID: 2, SpaceID: 100, Name: "Template 2"},
				}, int64(2), nil)
				mockRefDAO.EXPECT().GetByTemplateIDs(gomock.Any(), []int64{1, 2}).Return([]*model.ExptTemplateEvaluatorRef{
					{ID: 1, ExptTemplateID: 1, EvaluatorID: 1, EvaluatorVersionID: 1},
					{ID: 2, ExptTemplateID: 2, EvaluatorID: 2, EvaluatorVersionID: 2},
				}, nil)
			},
			wantErr:   false,
			wantLen:   2,
			wantCount: 2,
			page:      1,
			size:      10,
			spaceID:   100,
		},
		{
			name: "empty_list",
			mockSetup: func() {
				mockTemplateDAO.EXPECT().List(gomock.Any(), int32(1), int32(10), gomock.Any(), gomock.Any(), int64(100)).Return([]*model.ExptTemplate{}, int64(0), nil)
			},
			wantErr:   false,
			wantLen:   0,
			wantCount: 0,
			page:      1,
			size:      10,
			spaceID:   100,
		},
		{
			name: "fail_templateDAO",
			mockSetup: func() {
				mockTemplateDAO.EXPECT().List(gomock.Any(), int32(1), int32(10), gomock.Any(), gomock.Any(), int64(100)).Return(nil, int64(0), errors.New("dao error"))
			},
			wantErr:   true,
			wantLen:   0,
			wantCount: 0,
			page:      1,
			size:      10,
			spaceID:   100,
		},
		{
			name: "fail_refDAO",
			mockSetup: func() {
				mockTemplateDAO.EXPECT().List(gomock.Any(), int32(1), int32(10), gomock.Any(), gomock.Any(), int64(100)).Return([]*model.ExptTemplate{
					{ID: 3, SpaceID: 100, Name: "Template 3"},
				}, int64(1), nil)
				mockRefDAO.EXPECT().GetByTemplateIDs(gomock.Any(), []int64{3}).Return(nil, errors.New("ref error"))
			},
			wantErr:   true,
			wantLen:   0,
			wantCount: 0,
			page:      1,
			size:      10,
			spaceID:   100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			got, count, err := repo.List(context.Background(), tt.page, tt.size, nil, nil, tt.spaceID)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantCount, count)
				if tt.wantLen == 0 {
					assert.Nil(t, got)
				} else {
					assert.Len(t, got, tt.wantLen)
				}
			}
		})
	}
}
