// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	mysqlMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
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
	t.Run("returns complete basic experiments in requested order", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo, mockExptDAO, _, _ := newRepo(ctrl)

		createdAt202 := time.Date(2026, time.July, 24, 10, 11, 12, 0, time.UTC)
		createdAt101 := time.Date(2026, time.July, 23, 9, 8, 7, 0, time.UTC)
		itemConcurNum202 := 7
		itemRetryNum202 := 3
		enableExtractTrajectory202 := true
		evalConf202 := &entity.EvaluationConfiguration{
			ItemConcurNum:           &itemConcurNum202,
			ItemRetryNum:            &itemRetryNum202,
			EnableExtractTrajectory: &enableExtractTrajectory202,
			Ext:                     map[string]string{"source": "mget-basic"},
		}
		evalConf202JSON, err := json.Marshal(evalConf202)
		require.NoError(t, err)

		webhookURLs202 := "https://example.com/hooks/202"
		webhookEnvironment202 := entity.WebhookEnvironment_PPE
		webhookLane202 := "eval-202"
		feishuUserID202 := "ou_creator_202"
		notificationConf202 := &entity.ExptNotificationConf{
			Webhook: &entity.WebhookNotificationConf{
				Enable:      true,
				Urls:        &webhookURLs202,
				Environment: &webhookEnvironment202,
				Lane:        &webhookLane202,
			},
			FeishuNotification: &entity.FeishuNotificationConf{
				Enable: true,
				UserID: &feishuUserID202,
			},
		}
		notificationConf202JSON, err := json.Marshal(notificationConf202)
		require.NoError(t, err)

		itemConcurNum101 := 2
		evalConf101 := &entity.EvaluationConfiguration{
			ItemConcurNum: &itemConcurNum101,
			Ext:           map[string]string{"source": "second"},
		}
		evalConf101JSON, err := json.Marshal(evalConf101)
		require.NoError(t, err)

		webhookURLs101 := "https://example.com/hooks/101"
		notificationConf101 := &entity.ExptNotificationConf{
			Webhook: &entity.WebhookNotificationConf{
				Enable: true,
				Urls:   &webhookURLs101,
			},
		}
		notificationConf101JSON, err := json.Marshal(notificationConf101)
		require.NoError(t, err)

		ids := []int64{202, 101}
		mockExptDAO.EXPECT().MGetByID(gomock.Any(), ids).Return([]*model.Experiment{
			{
				ID:                 202,
				SpaceID:            42,
				CreatedBy:          "creator-202",
				Name:               "experiment-202",
				Description:        "description-202",
				ExperimentGroupKey: "group-202",
				EvalConf:           &evalConf202JSON,
				NotificationConf:   &notificationConf202JSON,
				CreatedAt:          createdAt202,
			},
			{
				ID:                 101,
				SpaceID:            42,
				CreatedBy:          "creator-101",
				Name:               "experiment-101",
				Description:        "description-101",
				ExperimentGroupKey: "group-101",
				EvalConf:           &evalConf101JSON,
				NotificationConf:   &notificationConf101JSON,
				CreatedAt:          createdAt101,
			},
		}, nil)

		got, err := repo.MGetBasicByID(context.Background(), ids)
		require.NoError(t, err)
		require.Len(t, got, 2)
		require.Equal(t, []int64{202, 101}, []int64{got[0].ID, got[1].ID})

		require.Equal(t, int64(42), got[0].SpaceID)
		require.Equal(t, "creator-202", got[0].CreatedBy)
		require.Equal(t, "experiment-202", got[0].Name)
		require.Equal(t, "description-202", got[0].Description)
		require.Equal(t, "group-202", got[0].ExperimentGroupKey)
		require.NotNil(t, got[0].CreatedAt)
		require.Equal(t, createdAt202, *got[0].CreatedAt)
		require.Equal(t, evalConf202, got[0].EvalConf)
		require.Equal(t, notificationConf202, got[0].NotificationConf)
		require.Empty(t, got[0].EvaluatorVersionRef)

		require.Equal(t, int64(42), got[1].SpaceID)
		require.Equal(t, "creator-101", got[1].CreatedBy)
		require.Equal(t, "experiment-101", got[1].Name)
		require.Equal(t, "description-101", got[1].Description)
		require.Equal(t, "group-101", got[1].ExperimentGroupKey)
		require.NotNil(t, got[1].CreatedAt)
		require.Equal(t, createdAt101, *got[1].CreatedAt)
		require.Equal(t, evalConf101, got[1].EvalConf)
		require.Equal(t, notificationConf101, got[1].NotificationConf)
		require.Empty(t, got[1].EvaluatorVersionRef)
	})

	t.Run("returns dao error unchanged", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo, mockExptDAO, _, _ := newRepo(ctrl)
		daoErr := errors.New("mget basic experiments failed")
		ids := []int64{303, 404}
		mockExptDAO.EXPECT().MGetByID(gomock.Any(), ids).Return(nil, daoErr)

		got, err := repo.MGetBasicByID(context.Background(), ids)
		require.ErrorIs(t, err, daoErr)
		require.Nil(t, got)
	})
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
