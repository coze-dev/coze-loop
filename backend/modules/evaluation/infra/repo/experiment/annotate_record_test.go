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
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestExptAnnotateRepoImpl_GetTagRefByTagKeyID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTagRefDAO := daoMocks.NewMockIExptTurnResultTagRefDAO(ctrl)
	mockAnnotateRecordRefDAO := daoMocks.NewMockIExptTurnAnnotateRecordRefDAO(ctrl)
	mockAnnotateRecordDAO := daoMocks.NewMockIAnnotateRecordDAO(ctrl)
	mockIDGen := idgenMocks.NewMockIIDGenerator(ctrl)

	repo := &ExptAnnotateRepoImpl{
		exptTurnAnnotateRecordRefDAO: mockAnnotateRecordRefDAO,
		exptTurnResultTagRefDAO:      mockTagRefDAO,
		annotateRecordDAO:            mockAnnotateRecordDAO,
		idgenerator:                  mockIDGen,
	}

	tests := []struct {
		name      string
		exptID    int64
		spaceID   int64
		tagKeyID  int64
		mockSetup func()
		want      *entity.ExptTurnResultTagRef
		wantErr   bool
	}{{
		name:     "成功获取标签引用",
		exptID:   1,
		spaceID:  1,
		tagKeyID: 1,
		mockSetup: func() {
			mockTagRefDAO.EXPECT().GetByTagKeyID(gomock.Any(), int64(1), int64(1), int64(1)).
				Return(&model.ExptTurnResultTagRef{
					ID:          1,
					SpaceID:     1,
					ExptID:      1,
					TagKeyID:    1,
					TotalCnt:    10,
					CompleteCnt: 5,
				}, nil)
		},
		want: &entity.ExptTurnResultTagRef{
			ID:          1,
			SpaceID:     1,
			ExptID:      1,
			TagKeyID:    1,
			TotalCnt:    10,
			CompleteCnt: 5,
		},
		wantErr: false,
	}, {
		name:     "DAO错误",
		exptID:   2,
		spaceID:  2,
		tagKeyID: 2,
		mockSetup: func() {
			mockTagRefDAO.EXPECT().GetByTagKeyID(gomock.Any(), int64(2), int64(2), int64(2)).
				Return(nil, errors.New("数据库错误"))
		},
		want:    nil,
		wantErr: true,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			got, err := repo.GetTagRefByTagKeyID(context.Background(), tt.exptID, tt.spaceID, tt.tagKeyID)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestExptAnnotateRepoImpl_CreateExptTurnAnnotateRecordRefs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTagRefDAO := daoMocks.NewMockIExptTurnResultTagRefDAO(ctrl)
	mockAnnotateRecordRefDAO := daoMocks.NewMockIExptTurnAnnotateRecordRefDAO(ctrl)
	mockAnnotateRecordDAO := daoMocks.NewMockIAnnotateRecordDAO(ctrl)
	mockIDGen := idgenMocks.NewMockIIDGenerator(ctrl)

	repo := &ExptAnnotateRepoImpl{
		exptTurnAnnotateRecordRefDAO: mockAnnotateRecordRefDAO,
		exptTurnResultTagRefDAO:      mockTagRefDAO,
		annotateRecordDAO:            mockAnnotateRecordDAO,
		idgenerator:                  mockIDGen,
	}

	tests := []struct {
		name      string
		ref       *entity.ExptTurnAnnotateRecordRef
		mockSetup func()
		wantErr   bool
	}{{
		name: "成功创建标注记录引用",
		ref: &entity.ExptTurnAnnotateRecordRef{
			SpaceID:          1,
			ExptTurnResultID: 1,
			TagKeyID:         1,
			AnnotateRecordID: 1,
			ExptID:           1,
		},
		mockSetup: func() {
			mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(123), nil)
			mockAnnotateRecordRefDAO.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)
		},
		wantErr: false,
	}, {
		name: "ID生成器错误",
		ref: &entity.ExptTurnAnnotateRecordRef{
			SpaceID:          1,
			ExptTurnResultID: 1,
			TagKeyID:         1,
			AnnotateRecordID: 1,
			ExptID:           1,
		},
		mockSetup: func() {
			mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(0), errors.New("ID生成器错误"))
		},
		wantErr: true,
	}, {
		name: "DAO创建错误",
		ref: &entity.ExptTurnAnnotateRecordRef{
			SpaceID:          1,
			ExptTurnResultID: 1,
			TagKeyID:         1,
			AnnotateRecordID: 1,
			ExptID:           1,
		},
		mockSetup: func() {
			mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(123), nil)
			mockAnnotateRecordRefDAO.EXPECT().Save(gomock.Any(), gomock.Any()).Return(errors.New("创建失败"))
		},
		wantErr: true,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			err := repo.CreateExptTurnAnnotateRecordRefs(context.Background(), tt.ref)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, int64(123), tt.ref.ID)
			}
		})
	}
}

func TestExptAnnotateRepoImpl_CreateExptTurnResultTagRefs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTagRefDAO := daoMocks.NewMockIExptTurnResultTagRefDAO(ctrl)
	mockAnnotateRecordRefDAO := daoMocks.NewMockIExptTurnAnnotateRecordRefDAO(ctrl)
	mockAnnotateRecordDAO := daoMocks.NewMockIAnnotateRecordDAO(ctrl)
	mockIDGen := idgenMocks.NewMockIIDGenerator(ctrl)

	repo := &ExptAnnotateRepoImpl{
		exptTurnAnnotateRecordRefDAO: mockAnnotateRecordRefDAO,
		exptTurnResultTagRefDAO:      mockTagRefDAO,
		annotateRecordDAO:            mockAnnotateRecordDAO,
		idgenerator:                  mockIDGen,
	}

	tests := []struct {
		name      string
		refs      []*entity.ExptTurnResultTagRef
		mockSetup func()
		wantErr   bool
	}{{
		name: "成功创建多个标签引用",
		refs: []*entity.ExptTurnResultTagRef{
			{
				SpaceID:     1,
				ExptID:      1,
				TagKeyID:    1,
				TotalCnt:    10,
				CompleteCnt: 0,
			},
			{
				SpaceID:     1,
				ExptID:      1,
				TagKeyID:    2,
				TotalCnt:    20,
				CompleteCnt: 5,
			},
		},
		mockSetup: func() {
			mockIDGen.EXPECT().GenMultiIDs(gomock.Any(), 2).Return([]int64{100, 101}, nil)
			mockTagRefDAO.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
		},
		wantErr: false,
	}, {
		name: "ID生成失败",
		refs: []*entity.ExptTurnResultTagRef{
			{
				SpaceID:  1,
				ExptID:   1,
				TagKeyID: 1,
			},
		},
		mockSetup: func() {
			mockIDGen.EXPECT().GenMultiIDs(gomock.Any(), 1).Return(nil, errors.New("ID生成错误"))
		},
		wantErr: true,
	}, {
		name: "DAO创建失败",
		refs: []*entity.ExptTurnResultTagRef{
			{
				SpaceID:  1,
				ExptID:   1,
				TagKeyID: 1,
			},
		},
		mockSetup: func() {
			mockIDGen.EXPECT().GenMultiIDs(gomock.Any(), 1).Return([]int64{100}, nil)
			mockTagRefDAO.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errors.New("创建失败"))
		},
		wantErr: true,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			err := repo.CreateExptTurnResultTagRefs(context.Background(), tt.refs)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, int64(100), tt.refs[0].ID)
				if len(tt.refs) > 1 {
					assert.Equal(t, int64(101), tt.refs[1].ID)
				}
			}
		})
	}
}

func TestExptAnnotateRepoImpl_DeleteExptTurnResultTagRef(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTagRefDAO := daoMocks.NewMockIExptTurnResultTagRefDAO(ctrl)
	mockAnnotateRecordRefDAO := daoMocks.NewMockIExptTurnAnnotateRecordRefDAO(ctrl)
	mockAnnotateRecordDAO := daoMocks.NewMockIAnnotateRecordDAO(ctrl)
	mockIDGen := idgenMocks.NewMockIIDGenerator(ctrl)

	repo := &ExptAnnotateRepoImpl{
		exptTurnAnnotateRecordRefDAO: mockAnnotateRecordRefDAO,
		exptTurnResultTagRefDAO:      mockTagRefDAO,
		annotateRecordDAO:            mockAnnotateRecordDAO,
		idgenerator:                  mockIDGen,
	}

	tests := []struct {
		name      string
		exptID    int64
		spaceID   int64
		tagKeyID  int64
		mockSetup func()
		wantErr   bool
	}{{
		name:     "成功删除标签引用",
		exptID:   1,
		spaceID:  1,
		tagKeyID: 1,
		mockSetup: func() {
			mockTagRefDAO.EXPECT().Delete(gomock.Any(), int64(1), int64(1), int64(1), gomock.Any()).Return(nil)
		},
		wantErr: false,
	}, {
		name:     "删除失败-DAO错误",
		exptID:   2,
		spaceID:  2,
		tagKeyID: 2,
		mockSetup: func() {
			mockTagRefDAO.EXPECT().Delete(gomock.Any(), int64(2), int64(2), int64(2), gomock.Any()).
				Return(errors.New("删除失败"))
		},
		wantErr: true,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			err := repo.DeleteExptTurnResultTagRef(context.Background(), tt.exptID, tt.spaceID, tt.tagKeyID)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptAnnotateRepoImpl_DeleteTurnAnnotateRecordRef(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTagRefDAO := daoMocks.NewMockIExptTurnResultTagRefDAO(ctrl)
	mockAnnotateRecordRefDAO := daoMocks.NewMockIExptTurnAnnotateRecordRefDAO(ctrl)
	mockAnnotateRecordDAO := daoMocks.NewMockIAnnotateRecordDAO(ctrl)
	mockIDGen := idgenMocks.NewMockIIDGenerator(ctrl)

	repo := &ExptAnnotateRepoImpl{
		exptTurnAnnotateRecordRefDAO: mockAnnotateRecordRefDAO,
		exptTurnResultTagRefDAO:      mockTagRefDAO,
		annotateRecordDAO:            mockAnnotateRecordDAO,
		idgenerator:                  mockIDGen,
	}

	tests := []struct {
		name      string
		exptID    int64
		spaceID   int64
		tagKeyID  int64
		mockSetup func()
		wantErr   bool
	}{{
		name:     "成功删除标注记录引用",
		exptID:   1,
		spaceID:  1,
		tagKeyID: 1,
		mockSetup: func() {
			mockAnnotateRecordRefDAO.EXPECT().DeleteByTagKeyID(gomock.Any(), int64(1), int64(1), int64(1), gomock.Any()).Return(nil)
		},
		wantErr: false,
	}, {
		name:     "删除失败-DAO错误",
		exptID:   2,
		spaceID:  2,
		tagKeyID: 2,
		mockSetup: func() {
			mockAnnotateRecordRefDAO.EXPECT().DeleteByTagKeyID(gomock.Any(), int64(2), int64(2), int64(2), gomock.Any()).
				Return(errors.New("删除失败"))
		},
		wantErr: true,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			err := repo.DeleteTurnAnnotateRecordRef(context.Background(), tt.exptID, tt.spaceID, tt.tagKeyID)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptAnnotateRepoImpl_GetExptTurnAnnotateRecordRefs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTagRefDAO := daoMocks.NewMockIExptTurnResultTagRefDAO(ctrl)
	mockAnnotateRecordRefDAO := daoMocks.NewMockIExptTurnAnnotateRecordRefDAO(ctrl)
	mockAnnotateRecordDAO := daoMocks.NewMockIAnnotateRecordDAO(ctrl)
	mockIDGen := idgenMocks.NewMockIIDGenerator(ctrl)

	repo := &ExptAnnotateRepoImpl{
		exptTurnAnnotateRecordRefDAO: mockAnnotateRecordRefDAO,
		exptTurnResultTagRefDAO:      mockTagRefDAO,
		annotateRecordDAO:            mockAnnotateRecordDAO,
		idgenerator:                  mockIDGen,
	}

	tests := []struct {
		name      string
		exptID    int64
		spaceID   int64
		mockSetup func()
		wantLen   int
		wantErr   bool
	}{{
		name:    "成功获取标注记录引用列表",
		exptID:  1,
		spaceID: 1,
		mockSetup: func() {
			mockAnnotateRecordRefDAO.EXPECT().GetByExptID(gomock.Any(), int64(1), int64(1)).Return([]*model.ExptTurnAnnotateRecordRef{
				{
					ID:               1,
					SpaceID:          1,
					ExptTurnResultID: 100,
					TagKeyID:         10,
					AnnotateRecordID: 1000,
					ExptID:           1,
				},
				{
					ID:               2,
					SpaceID:          1,
					ExptTurnResultID: 101,
					TagKeyID:         10,
					AnnotateRecordID: 1001,
					ExptID:           1,
				},
			}, nil)
		},
		wantLen: 2,
		wantErr: false,
	}, {
		name:    "获取失败-DAO错误",
		exptID:  2,
		spaceID: 2,
		mockSetup: func() {
			mockAnnotateRecordRefDAO.EXPECT().GetByExptID(gomock.Any(), int64(2), int64(2)).
				Return(nil, errors.New("查询失败"))
		},
		wantLen: 0,
		wantErr: true,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			got, err := repo.GetExptTurnAnnotateRecordRefs(context.Background(), tt.exptID, tt.spaceID)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantLen, len(got))
			}
		})
	}
}

func TestExptAnnotateRepoImpl_BatchGetExptTurnAnnotateRecordRefs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTagRefDAO := daoMocks.NewMockIExptTurnResultTagRefDAO(ctrl)
	mockAnnotateRecordRefDAO := daoMocks.NewMockIExptTurnAnnotateRecordRefDAO(ctrl)
	mockAnnotateRecordDAO := daoMocks.NewMockIAnnotateRecordDAO(ctrl)
	mockIDGen := idgenMocks.NewMockIIDGenerator(ctrl)

	repo := &ExptAnnotateRepoImpl{
		exptTurnAnnotateRecordRefDAO: mockAnnotateRecordRefDAO,
		exptTurnResultTagRefDAO:      mockTagRefDAO,
		annotateRecordDAO:            mockAnnotateRecordDAO,
		idgenerator:                  mockIDGen,
	}

	tests := []struct {
		name      string
		exptIDs   []int64
		spaceID   int64
		mockSetup func()
		wantLen   int
		wantErr   bool
	}{{
		name:    "成功批量获取标注记录引用",
		exptIDs: []int64{1, 2},
		spaceID: 1,
		mockSetup: func() {
			mockAnnotateRecordRefDAO.EXPECT().BatchGetByExptIDs(gomock.Any(), int64(1), []int64{1, 2}).Return([]*model.ExptTurnAnnotateRecordRef{
				{
					ID:               1,
					SpaceID:          1,
					ExptTurnResultID: 100,
					TagKeyID:         10,
					AnnotateRecordID: 1000,
					ExptID:           1,
				},
				{
					ID:               2,
					SpaceID:          1,
					ExptTurnResultID: 101,
					TagKeyID:         10,
					AnnotateRecordID: 1001,
					ExptID:           2,
				},
			}, nil)
		},
		wantLen: 2,
		wantErr: false,
	}, {
		name:    "批量获取失败-DAO错误",
		exptIDs: []int64{3, 4},
		spaceID: 2,
		mockSetup: func() {
			mockAnnotateRecordRefDAO.EXPECT().BatchGetByExptIDs(gomock.Any(), int64(2), []int64{3, 4}).
				Return(nil, errors.New("批量查询失败"))
		},
		wantLen: 0,
		wantErr: true,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			got, err := repo.BatchGetExptTurnAnnotateRecordRefs(context.Background(), tt.exptIDs, tt.spaceID)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantLen, len(got))
			}
		})
	}
}

func TestExptAnnotateRepoImpl_GetExptTurnAnnotateRecordRefsByTurnResultIDs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTagRefDAO := daoMocks.NewMockIExptTurnResultTagRefDAO(ctrl)
	mockAnnotateRecordRefDAO := daoMocks.NewMockIExptTurnAnnotateRecordRefDAO(ctrl)
	mockAnnotateRecordDAO := daoMocks.NewMockIAnnotateRecordDAO(ctrl)
	mockIDGen := idgenMocks.NewMockIIDGenerator(ctrl)

	repo := &ExptAnnotateRepoImpl{
		exptTurnAnnotateRecordRefDAO: mockAnnotateRecordRefDAO,
		exptTurnResultTagRefDAO:      mockTagRefDAO,
		annotateRecordDAO:            mockAnnotateRecordDAO,
		idgenerator:                  mockIDGen,
	}

	tests := []struct {
		name          string
		spaceID       int64
		turnResultIDs []int64
		mockSetup     func()
		wantLen       int
		wantErr       bool
	}{{
		name:          "成功按TurnResultIDs获取引用",
		spaceID:       1,
		turnResultIDs: []int64{100, 101},
		mockSetup: func() {
			mockAnnotateRecordRefDAO.EXPECT().BatchGet(gomock.Any(), int64(1), []int64{100, 101}).Return([]*model.ExptTurnAnnotateRecordRef{
				{
					ID:               1,
					SpaceID:          1,
					ExptTurnResultID: 100,
					TagKeyID:         10,
					AnnotateRecordID: 1000,
					ExptID:           1,
				},
			}, nil)
		},
		wantLen: 1,
		wantErr: false,
	}, {
		name:          "获取失败-DAO错误",
		spaceID:       2,
		turnResultIDs: []int64{200, 201},
		mockSetup: func() {
			mockAnnotateRecordRefDAO.EXPECT().BatchGet(gomock.Any(), int64(2), []int64{200, 201}).
				Return(nil, errors.New("查询失败"))
		},
		wantLen: 0,
		wantErr: true,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			got, err := repo.GetExptTurnAnnotateRecordRefsByTurnResultIDs(context.Background(), tt.spaceID, tt.turnResultIDs)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantLen, len(got))
			}
		})
	}
}

func TestExptAnnotateRepoImpl_GetExptTurnAnnotateRecordRefsByTagKeyID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTagRefDAO := daoMocks.NewMockIExptTurnResultTagRefDAO(ctrl)
	mockAnnotateRecordRefDAO := daoMocks.NewMockIExptTurnAnnotateRecordRefDAO(ctrl)
	mockAnnotateRecordDAO := daoMocks.NewMockIAnnotateRecordDAO(ctrl)
	mockIDGen := idgenMocks.NewMockIIDGenerator(ctrl)

	repo := &ExptAnnotateRepoImpl{
		exptTurnAnnotateRecordRefDAO: mockAnnotateRecordRefDAO,
		exptTurnResultTagRefDAO:      mockTagRefDAO,
		annotateRecordDAO:            mockAnnotateRecordDAO,
		idgenerator:                  mockIDGen,
	}

	tests := []struct {
		name      string
		exptID    int64
		spaceID   int64
		tagKeyID  int64
		mockSetup func()
		wantLen   int
		wantErr   bool
	}{{
		name:     "成功按TagKeyID获取引用",
		exptID:   1,
		spaceID:  1,
		tagKeyID: 10,
		mockSetup: func() {
			mockAnnotateRecordRefDAO.EXPECT().GetByTagKeyID(gomock.Any(), int64(1), int64(1), int64(10)).Return([]*model.ExptTurnAnnotateRecordRef{
				{
					ID:               1,
					SpaceID:          1,
					ExptTurnResultID: 100,
					TagKeyID:         10,
					AnnotateRecordID: 1000,
					ExptID:           1,
				},
			}, nil)
		},
		wantLen: 1,
		wantErr: false,
	}, {
		name:     "获取失败-DAO错误",
		exptID:   2,
		spaceID:  2,
		tagKeyID: 20,
		mockSetup: func() {
			mockAnnotateRecordRefDAO.EXPECT().GetByTagKeyID(gomock.Any(), int64(2), int64(2), int64(20)).
				Return(nil, errors.New("查询失败"))
		},
		wantLen: 0,
		wantErr: true,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			got, err := repo.GetExptTurnAnnotateRecordRefsByTagKeyID(context.Background(), tt.exptID, tt.spaceID, tt.tagKeyID)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantLen, len(got))
			}
		})
	}
}

func TestExptAnnotateRepoImpl_UpdateCompleteCount(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTagRefDAO := daoMocks.NewMockIExptTurnResultTagRefDAO(ctrl)
	mockAnnotateRecordRefDAO := daoMocks.NewMockIExptTurnAnnotateRecordRefDAO(ctrl)
	mockAnnotateRecordDAO := daoMocks.NewMockIAnnotateRecordDAO(ctrl)
	mockIDGen := idgenMocks.NewMockIIDGenerator(ctrl)

	repo := &ExptAnnotateRepoImpl{
		exptTurnAnnotateRecordRefDAO: mockAnnotateRecordRefDAO,
		exptTurnResultTagRefDAO:      mockTagRefDAO,
		annotateRecordDAO:            mockAnnotateRecordDAO,
		idgenerator:                  mockIDGen,
	}

	tests := []struct {
		name         string
		exptID       int64
		spaceID      int64
		tagKeyID     int64
		mockSetup    func()
		wantTotal    int32
		wantComplete int32
		wantErr      bool
	}{{
		name:     "成功更新完成计数",
		exptID:   1,
		spaceID:  1,
		tagKeyID: 10,
		mockSetup: func() {
			mockTagRefDAO.EXPECT().UpdateCompleteCount(gomock.Any(), int64(1), int64(1), int64(10), gomock.Any()).
				Return(int32(100), int32(30), nil)
		},
		wantTotal:    100,
		wantComplete: 30,
		wantErr:      false,
	}, {
		name:     "更新失败-DAO错误",
		exptID:   2,
		spaceID:  2,
		tagKeyID: 20,
		mockSetup: func() {
			mockTagRefDAO.EXPECT().UpdateCompleteCount(gomock.Any(), int64(2), int64(2), int64(20), gomock.Any()).
				Return(int32(0), int32(0), errors.New("更新失败"))
		},
		wantTotal:    0,
		wantComplete: 0,
		wantErr:      true,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			total, complete, err := repo.UpdateCompleteCount(context.Background(), tt.exptID, tt.spaceID, tt.tagKeyID)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantTotal, total)
				assert.Equal(t, tt.wantComplete, complete)
			}
		})
	}
}

func TestExptAnnotateRepoImpl_GetExptTurnResultTagRefs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTagRefDAO := daoMocks.NewMockIExptTurnResultTagRefDAO(ctrl)
	mockAnnotateRecordRefDAO := daoMocks.NewMockIExptTurnAnnotateRecordRefDAO(ctrl)
	mockAnnotateRecordDAO := daoMocks.NewMockIAnnotateRecordDAO(ctrl)
	mockIDGen := idgenMocks.NewMockIIDGenerator(ctrl)

	repo := &ExptAnnotateRepoImpl{
		exptTurnAnnotateRecordRefDAO: mockAnnotateRecordRefDAO,
		exptTurnResultTagRefDAO:      mockTagRefDAO,
		annotateRecordDAO:            mockAnnotateRecordDAO,
		idgenerator:                  mockIDGen,
	}

	tests := []struct {
		name      string
		exptID    int64
		spaceID   int64
		mockSetup func()
		wantLen   int
		wantErr   bool
	}{{
		name:    "成功获取标签引用列表",
		exptID:  1,
		spaceID: 1,
		mockSetup: func() {
			mockTagRefDAO.EXPECT().GetByExptID(gomock.Any(), int64(1), int64(1)).Return([]*model.ExptTurnResultTagRef{
				{
					ID:          1,
					SpaceID:     1,
					ExptID:      1,
					TagKeyID:    10,
					TotalCnt:    100,
					CompleteCnt: 30,
				},
				{
					ID:          2,
					SpaceID:     1,
					ExptID:      1,
					TagKeyID:    11,
					TotalCnt:    50,
					CompleteCnt: 20,
				},
			}, nil)
		},
		wantLen: 2,
		wantErr: false,
	}, {
		name:    "获取失败-DAO错误",
		exptID:  2,
		spaceID: 2,
		mockSetup: func() {
			mockTagRefDAO.EXPECT().GetByExptID(gomock.Any(), int64(2), int64(2)).
				Return(nil, errors.New("查询失败"))
		},
		wantLen: 0,
		wantErr: true,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			got, err := repo.GetExptTurnResultTagRefs(context.Background(), tt.exptID, tt.spaceID)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantLen, len(got))
			}
		})
	}
}

func TestExptAnnotateRepoImpl_BatchGetExptTurnResultTagRefs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTagRefDAO := daoMocks.NewMockIExptTurnResultTagRefDAO(ctrl)
	mockAnnotateRecordRefDAO := daoMocks.NewMockIExptTurnAnnotateRecordRefDAO(ctrl)
	mockAnnotateRecordDAO := daoMocks.NewMockIAnnotateRecordDAO(ctrl)
	mockIDGen := idgenMocks.NewMockIIDGenerator(ctrl)

	repo := &ExptAnnotateRepoImpl{
		exptTurnAnnotateRecordRefDAO: mockAnnotateRecordRefDAO,
		exptTurnResultTagRefDAO:      mockTagRefDAO,
		annotateRecordDAO:            mockAnnotateRecordDAO,
		idgenerator:                  mockIDGen,
	}

	tests := []struct {
		name      string
		exptIDs   []int64
		spaceID   int64
		mockSetup func()
		wantLen   int
		wantErr   bool
	}{{
		name:    "成功批量获取标签引用",
		exptIDs: []int64{1, 2},
		spaceID: 1,
		mockSetup: func() {
			mockTagRefDAO.EXPECT().BatchGetByExptIDs(gomock.Any(), []int64{1, 2}, int64(1)).Return([]*model.ExptTurnResultTagRef{
				{
					ID:          1,
					SpaceID:     1,
					ExptID:      1,
					TagKeyID:    10,
					TotalCnt:    100,
					CompleteCnt: 30,
				},
				{
					ID:          2,
					SpaceID:     1,
					ExptID:      2,
					TagKeyID:    11,
					TotalCnt:    50,
					CompleteCnt: 20,
				},
			}, nil)
		},
		wantLen: 2,
		wantErr: false,
	}, {
		name:    "批量获取失败-DAO错误",
		exptIDs: []int64{3, 4},
		spaceID: 2,
		mockSetup: func() {
			mockTagRefDAO.EXPECT().BatchGetByExptIDs(gomock.Any(), []int64{3, 4}, int64(2)).
				Return(nil, errors.New("批量查询失败"))
		},
		wantLen: 0,
		wantErr: true,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			got, err := repo.BatchGetExptTurnResultTagRefs(context.Background(), tt.exptIDs, tt.spaceID)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantLen, len(got))
			}
		})
	}
}

func TestExptAnnotateRepoImpl_SaveAnnotateRecord(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTagRefDAO := daoMocks.NewMockIExptTurnResultTagRefDAO(ctrl)
	mockAnnotateRecordRefDAO := daoMocks.NewMockIExptTurnAnnotateRecordRefDAO(ctrl)
	mockAnnotateRecordDAO := daoMocks.NewMockIAnnotateRecordDAO(ctrl)
	mockIDGen := idgenMocks.NewMockIIDGenerator(ctrl)

	repo := &ExptAnnotateRepoImpl{
		exptTurnAnnotateRecordRefDAO: mockAnnotateRecordRefDAO,
		exptTurnResultTagRefDAO:      mockTagRefDAO,
		annotateRecordDAO:            mockAnnotateRecordDAO,
		idgenerator:                  mockIDGen,
	}

	record := &entity.AnnotateRecord{
		ID:           1000,
		SpaceID:      1,
		ExperimentID: 1,
		TagKeyID:     10,
		AnnotateData: &entity.AnnotateData{
			TagContentType: entity.TagContentTypeFreeText,
			TextValue:      ptr.Of("测试内容"),
		},
	}

	tests := []struct {
		name             string
		exptTurnResultID int64
		record           *entity.AnnotateRecord
		mockSetup        func()
		wantErr          bool
	}{{
		name:             "成功保存标注记录",
		exptTurnResultID: 100,
		record:           record,
		mockSetup: func() {
			mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(123), nil)
			mockAnnotateRecordDAO.EXPECT().Save(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			mockAnnotateRecordRefDAO.EXPECT().Save(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		},
		wantErr: false,
	}, {
		name:             "ID生成失败",
		exptTurnResultID: 101,
		record:           record,
		mockSetup: func() {
			mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(0), errors.New("ID生成失败"))
		},
		wantErr: true,
	}, {
		name:             "保存记录失败-DAO错误",
		exptTurnResultID: 102,
		record:           record,
		mockSetup: func() {
			mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(124), nil)
			mockAnnotateRecordDAO.EXPECT().Save(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("保存记录失败"))
		},
		wantErr: true,
	}, {
		name:             "保存引用失败-DAO错误",
		exptTurnResultID: 103,
		record:           record,
		mockSetup: func() {
			mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(125), nil)
			mockAnnotateRecordDAO.EXPECT().Save(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			mockAnnotateRecordRefDAO.EXPECT().Save(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("保存引用失败"))
		},
		wantErr: true,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			err := repo.SaveAnnotateRecord(context.Background(), tt.exptTurnResultID, tt.record)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptAnnotateRepoImpl_UpdateAnnotateRecord(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTagRefDAO := daoMocks.NewMockIExptTurnResultTagRefDAO(ctrl)
	mockAnnotateRecordRefDAO := daoMocks.NewMockIExptTurnAnnotateRecordRefDAO(ctrl)
	mockAnnotateRecordDAO := daoMocks.NewMockIAnnotateRecordDAO(ctrl)
	mockIDGen := idgenMocks.NewMockIIDGenerator(ctrl)

	repo := &ExptAnnotateRepoImpl{
		exptTurnAnnotateRecordRefDAO: mockAnnotateRecordRefDAO,
		exptTurnResultTagRefDAO:      mockTagRefDAO,
		annotateRecordDAO:            mockAnnotateRecordDAO,
		idgenerator:                  mockIDGen,
	}

	record := &entity.AnnotateRecord{
		ID:           1000,
		SpaceID:      1,
		ExperimentID: 1,
		TagKeyID:     10,
		AnnotateData: &entity.AnnotateData{
			TagContentType: entity.TagContentTypeFreeText,
			TextValue:      ptr.Of("测试内容"),
		},
	}

	tests := []struct {
		name      string
		record    *entity.AnnotateRecord
		mockSetup func()
		wantErr   bool
	}{{
		name:   "成功更新标注记录",
		record: record,
		mockSetup: func() {
			mockAnnotateRecordDAO.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
		},
		wantErr: false,
	}, {
		name:   "更新失败-DAO错误",
		record: record,
		mockSetup: func() {
			mockAnnotateRecordDAO.EXPECT().Update(gomock.Any(), gomock.Any()).Return(errors.New("更新失败"))
		},
		wantErr: true,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			err := repo.UpdateAnnotateRecord(context.Background(), tt.record)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptAnnotateRepoImpl_GetAnnotateRecordsByIDs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTagRefDAO := daoMocks.NewMockIExptTurnResultTagRefDAO(ctrl)
	mockAnnotateRecordRefDAO := daoMocks.NewMockIExptTurnAnnotateRecordRefDAO(ctrl)
	mockAnnotateRecordDAO := daoMocks.NewMockIAnnotateRecordDAO(ctrl)
	mockIDGen := idgenMocks.NewMockIIDGenerator(ctrl)

	repo := &ExptAnnotateRepoImpl{
		exptTurnAnnotateRecordRefDAO: mockAnnotateRecordRefDAO,
		exptTurnResultTagRefDAO:      mockTagRefDAO,
		annotateRecordDAO:            mockAnnotateRecordDAO,
		idgenerator:                  mockIDGen,
	}

	annotateData := &entity.AnnotateData{
		TagContentType: entity.TagContentTypeFreeText,
		TextValue:      ptr.Of("测试内容"),
	}
	annotateDataBytes, _ := json.Marshal(annotateData)

	tests := []struct {
		name      string
		spaceID   int64
		recordIDs []int64
		mockSetup func()
		wantLen   int
		wantErr   bool
	}{{
		name:      "成功批量获取标注记录",
		spaceID:   1,
		recordIDs: []int64{1000, 1001},
		mockSetup: func() {
			mockAnnotateRecordDAO.EXPECT().MGetByID(gomock.Any(), []int64{1000, 1001}).Return([]*model.AnnotateRecord{
				{
					ID:           1000,
					SpaceID:      1,
					ExperimentID: 1,
					TagKeyID:     10,
					AnnotateData: annotateDataBytes,
				},
				{
					ID:           1001,
					SpaceID:      1,
					ExperimentID: 1,
					TagKeyID:     10,
					AnnotateData: annotateDataBytes,
				},
			}, nil)
		},
		wantLen: 2,
		wantErr: false,
	}, {
		name:      "批量获取失败-DAO错误",
		spaceID:   2,
		recordIDs: []int64{2000, 2001},
		mockSetup: func() {
			mockAnnotateRecordDAO.EXPECT().MGetByID(gomock.Any(), []int64{2000, 2001}).
				Return(nil, errors.New("批量查询失败"))
		},
		wantLen: 0,
		wantErr: true,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			got, err := repo.GetAnnotateRecordsByIDs(context.Background(), tt.spaceID, tt.recordIDs)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantLen, len(got))
			}
		})
	}
}

func TestExptAnnotateRepoImpl_GetAnnotateRecordByID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTagRefDAO := daoMocks.NewMockIExptTurnResultTagRefDAO(ctrl)
	mockAnnotateRecordRefDAO := daoMocks.NewMockIExptTurnAnnotateRecordRefDAO(ctrl)
	mockAnnotateRecordDAO := daoMocks.NewMockIAnnotateRecordDAO(ctrl)
	mockIDGen := idgenMocks.NewMockIIDGenerator(ctrl)

	repo := &ExptAnnotateRepoImpl{
		exptTurnAnnotateRecordRefDAO: mockAnnotateRecordRefDAO,
		exptTurnResultTagRefDAO:      mockTagRefDAO,
		annotateRecordDAO:            mockAnnotateRecordDAO,
		idgenerator:                  mockIDGen,
	}

	annotateData := &entity.AnnotateData{
		TagContentType: entity.TagContentTypeFreeText,
		TextValue:      ptr.Of("测试内容"),
	}
	annotateDataBytes, _ := json.Marshal(annotateData)

	tests := []struct {
		name      string
		spaceID   int64
		recordID  int64
		mockSetup func()
		wantErr   bool
	}{{
		name:     "成功获取单个标注记录",
		spaceID:  1,
		recordID: 1000,
		mockSetup: func() {
			mockAnnotateRecordDAO.EXPECT().MGetByID(gomock.Any(), []int64{1000}).Return([]*model.AnnotateRecord{
				{
					ID:           1000,
					SpaceID:      1,
					ExperimentID: 1,
					TagKeyID:     10,
					AnnotateData: annotateDataBytes,
				},
			}, nil)
		},
		wantErr: false,
	}, {
		name:     "记录不存在",
		spaceID:  1,
		recordID: 9999,
		mockSetup: func() {
			mockAnnotateRecordDAO.EXPECT().MGetByID(gomock.Any(), []int64{9999}).Return([]*model.AnnotateRecord{}, nil)
		},
		wantErr: true,
	}, {
		name:     "查询失败-DAO错误",
		spaceID:  2,
		recordID: 2000,
		mockSetup: func() {
			mockAnnotateRecordDAO.EXPECT().MGetByID(gomock.Any(), []int64{2000}).
				Return(nil, errors.New("查询失败"))
		},
		wantErr: true,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			got, err := repo.GetAnnotateRecordByID(context.Background(), tt.spaceID, tt.recordID)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, got)
				assert.Equal(t, tt.recordID, got.ID)
			}
		})
	}
}
