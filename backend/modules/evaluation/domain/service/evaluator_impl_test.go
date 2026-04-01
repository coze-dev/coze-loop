// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	mqmocks "github.com/coze-dev/coze-loop/backend/infra/mq/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	idemmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem/mocks"
	componentMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	confmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/conf/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestNewEvaluatorServiceImpl(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIdgen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockLimiter := repomocks.NewMockRateLimiter(ctrl)
	mockMQ := mqmocks.NewMockIFactory(ctrl)
	mockEvaluatorRepo := repomocks.NewMockIEvaluatorRepo(ctrl)
	mockEvaluatorRecordRepo := repomocks.NewMockIEvaluatorRecordRepo(ctrl)
	mockIdem := idemmocks.NewMockIdempotentService(ctrl)
	mockConfiger := confmocks.NewMockIConfiger(ctrl)
	mockSourceService := mocks.NewMockEvaluatorSourceService(ctrl)
	mockPlainLimiter := repomocks.NewMockIPlainRateLimiter(ctrl)
	mockErrConfiger := componentMocks.NewMockIConfiger(ctrl)

	service := NewEvaluatorServiceImpl(
		mockIdgen,
		mockLimiter,
		mockMQ,
		mockEvaluatorRepo,
		mockEvaluatorRecordRepo,
		mockIdem,
		mockConfiger,
		map[entity.EvaluatorType]EvaluatorSourceService{
			entity.EvaluatorTypePrompt: mockSourceService,
		},
		mockPlainLimiter,
		mockErrConfiger,
	)

	assert.IsType(t, &EvaluatorServiceImpl{}, service)
}

// Test_GetBuiltinEvaluator 覆盖预置评估器按 builtin_visible_version 组装逻辑
func Test_GetBuiltinEvaluator(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repomocks.NewMockIEvaluatorRepo(ctrl)
	s := &EvaluatorServiceImpl{evaluatorRepo: mockRepo}

	ctx := context.Background()

	t.Run("evaluatorID为0返回nil", func(t *testing.T) {
		got, err := s.GetBuiltinEvaluator(ctx, 0)
		assert.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("非builtin或无visibleVersion返回nil", func(t *testing.T) {
		mockRepo.EXPECT().BatchGetEvaluatorMetaByID(gomock.Any(), []int64{101}, false).Return([]*entity.Evaluator{
			{ID: 101, Builtin: false},
		}, nil)
		got, err := s.GetBuiltinEvaluator(ctx, 101)
		assert.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("正常返回visible版本并回填元信息", func(t *testing.T) {
		meta := &entity.Evaluator{ID: 201, SpaceID: 9, Name: "builtin", Description: "desc", Builtin: true, BuiltinVisibleVersion: "1.2.3", EvaluatorType: entity.EvaluatorTypePrompt, LatestVersion: "2.0.0"}
		mockRepo.EXPECT().BatchGetEvaluatorMetaByID(gomock.Any(), []int64{201}, false).Return([]*entity.Evaluator{meta}, nil)
		mockRepo.EXPECT().BatchGetEvaluatorVersionsByEvaluatorIDAndVersions(gomock.Any(), [][2]interface{}{{int64(201), "1.2.3"}}).Return([]*entity.Evaluator{
			{PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{EvaluatorID: 201, Version: "1.2.3"}},
		}, nil)
		got, err := s.GetBuiltinEvaluator(ctx, 201)
		assert.NoError(t, err)
		assert.NotNil(t, got)
		assert.Equal(t, int64(201), got.ID)
		assert.Equal(t, int64(9), got.SpaceID)
		assert.Equal(t, "builtin", got.Name)
		assert.Equal(t, "1.2.3", got.GetVersion())
		assert.Equal(t, entity.EvaluatorTypePrompt, got.EvaluatorType)
		assert.True(t, got.Builtin)
		assert.Equal(t, "1.2.3", got.BuiltinVisibleVersion)
	})

	t.Run("visible版本不存在返回nil", func(t *testing.T) {
		meta := &entity.Evaluator{ID: 301, Builtin: true, BuiltinVisibleVersion: "0.1.0"}
		mockRepo.EXPECT().BatchGetEvaluatorMetaByID(gomock.Any(), []int64{301}, false).Return([]*entity.Evaluator{meta}, nil)
		mockRepo.EXPECT().BatchGetEvaluatorVersionsByEvaluatorIDAndVersions(gomock.Any(), [][2]interface{}{{int64(301), "0.1.0"}}).Return([]*entity.Evaluator{}, nil)
		got, err := s.GetBuiltinEvaluator(ctx, 301)
		assert.NoError(t, err)
		assert.Nil(t, got)
	})
}

func Test_EvaluatorServiceImpl_BatchGetBuiltinEvaluator(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		evaluatorIDs []int64
		setup        func(repo *repomocks.MockIEvaluatorRepo)
		wantLen      int
		wantErr      bool
	}{
		{
			name:         "empty input",
			evaluatorIDs: []int64{},
			setup:        func(*repomocks.MockIEvaluatorRepo) {},
			wantLen:      0,
		},
		{
			name:         "batch get meta error",
			evaluatorIDs: []int64{1, 2},
			setup: func(repo *repomocks.MockIEvaluatorRepo) {
				repo.EXPECT().BatchGetEvaluatorMetaByID(gomock.Any(), []int64{1, 2}, false).Return(nil, errors.New("meta error"))
			},
			wantErr: true,
		},
		{
			name:         "no valid builtin visible version pairs",
			evaluatorIDs: []int64{1, 2},
			setup: func(repo *repomocks.MockIEvaluatorRepo) {
				metas := []*entity.Evaluator{
					{ID: 1, Builtin: false},
					{ID: 2, Builtin: true, BuiltinVisibleVersion: ""},
					nil,
				}
				repo.EXPECT().BatchGetEvaluatorMetaByID(gomock.Any(), []int64{1, 2}, false).Return(metas, nil)
			},
			wantLen: 0,
		},
		{
			name:         "batch get versions error",
			evaluatorIDs: []int64{1},
			setup: func(repo *repomocks.MockIEvaluatorRepo) {
				metas := []*entity.Evaluator{
					{ID: 1, Builtin: true, BuiltinVisibleVersion: "1.0.0"},
				}
				repo.EXPECT().BatchGetEvaluatorMetaByID(gomock.Any(), []int64{1}, false).Return(metas, nil)
				repo.EXPECT().BatchGetEvaluatorVersionsByEvaluatorIDAndVersions(gomock.Any(), [][2]interface{}{{int64(1), "1.0.0"}}).Return(nil, errors.New("version error"))
			},
			wantErr: true,
		},
		{
			name:         "success",
			evaluatorIDs: []int64{1, 2},
			setup: func(repo *repomocks.MockIEvaluatorRepo) {
				metas := []*entity.Evaluator{
					{ID: 1, Builtin: true, BuiltinVisibleVersion: "1.0.0", Name: "eval1"},
					{ID: 2, Builtin: true, BuiltinVisibleVersion: "2.0.0", Name: "eval2"},
				}
				repo.EXPECT().BatchGetEvaluatorMetaByID(gomock.Any(), []int64{1, 2}, false).Return(metas, nil)

				versions := []*entity.Evaluator{
					{ID: 101, EvaluatorType: entity.EvaluatorTypePrompt, PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{EvaluatorID: 1, Version: "1.0.0"}},
					{ID: 102, EvaluatorType: entity.EvaluatorTypeCode, CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{EvaluatorID: 2, Version: "2.0.0"}},
				}
				repo.EXPECT().BatchGetEvaluatorVersionsByEvaluatorIDAndVersions(gomock.Any(), [][2]interface{}{{int64(1), "1.0.0"}, {int64(2), "2.0.0"}}).Return(versions, nil)
			},
			wantLen: 2,
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			repo := repomocks.NewMockIEvaluatorRepo(ctrl)
			tc.setup(repo)

			s := &EvaluatorServiceImpl{
				evaluatorRepo: repo,
			}

			got, err := s.BatchGetBuiltinEvaluator(context.Background(), tc.evaluatorIDs)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, got, tc.wantLen)
			}
		})
	}
}

func Test_EvaluatorServiceImpl_UpdateBuiltinEvaluatorTags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		evaluatorID int64
		tags        map[entity.EvaluatorTagLangType]map[entity.EvaluatorTagKey][]string
		setup       func(repo *repomocks.MockIEvaluatorRepo)
		wantErr     bool
	}{
		{
			name:        "success",
			evaluatorID: 1,
			tags: map[entity.EvaluatorTagLangType]map[entity.EvaluatorTagKey][]string{
				entity.EvaluatorTagLangType_Zh: {
					entity.EvaluatorTagKey_Category: {"llm"},
				},
			},
			setup: func(repo *repomocks.MockIEvaluatorRepo) {
				repo.EXPECT().UpdateEvaluatorTags(gomock.Any(), int64(1), gomock.Any()).Return(nil)
			},
			wantErr: false,
		},
		{
			name:        "error",
			evaluatorID: 1,
			tags:        nil,
			setup: func(repo *repomocks.MockIEvaluatorRepo) {
				repo.EXPECT().UpdateEvaluatorTags(gomock.Any(), int64(1), gomock.Any()).Return(errors.New("update error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			repo := repomocks.NewMockIEvaluatorRepo(ctrl)
			tc.setup(repo)

			s := &EvaluatorServiceImpl{
				evaluatorRepo: repo,
			}

			err := s.UpdateBuiltinEvaluatorTags(context.Background(), tc.evaluatorID, tc.tags)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_EvaluatorServiceImpl_CheckNameExist(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		spaceID     int64
		evaluatorID int64
		evalName    string
		setup       func(repo *repomocks.MockIEvaluatorRepo)
		wantExist   bool
		wantErr     bool
	}{
		{
			name:        "exist",
			spaceID:     1,
			evaluatorID: 2,
			evalName:    "test_name",
			setup: func(repo *repomocks.MockIEvaluatorRepo) {
				repo.EXPECT().CheckNameExist(gomock.Any(), int64(1), int64(2), "test_name").Return(true, nil)
			},
			wantExist: true,
			wantErr:   false,
		},
		{
			name:        "not exist",
			spaceID:     1,
			evaluatorID: 2,
			evalName:    "test_name",
			setup: func(repo *repomocks.MockIEvaluatorRepo) {
				repo.EXPECT().CheckNameExist(gomock.Any(), int64(1), int64(2), "test_name").Return(false, nil)
			},
			wantExist: false,
			wantErr:   false,
		},
		{
			name:        "error",
			spaceID:     1,
			evaluatorID: 2,
			evalName:    "test_name",
			setup: func(repo *repomocks.MockIEvaluatorRepo) {
				repo.EXPECT().CheckNameExist(gomock.Any(), int64(1), int64(2), "test_name").Return(false, errors.New("check error"))
			},
			wantExist: false,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			repo := repomocks.NewMockIEvaluatorRepo(ctrl)
			tc.setup(repo)

			s := &EvaluatorServiceImpl{
				evaluatorRepo: repo,
			}

			got, err := s.CheckNameExist(context.Background(), tc.spaceID, tc.evaluatorID, tc.evalName)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.wantExist, got)
			}
		})
	}
}

func Test_EvaluatorServiceImpl_ResolveBuiltinEvaluatorVisibleVersionID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		evaluatorID   int64
		evaluatorName string
		setup         func(ctx context.Context, cfg *confmocks.MockIConfiger, repo *repomocks.MockIEvaluatorRepo)
		wantID        int64
		wantErrCode   int32
		wantErr       bool
	}{
		{
			name:        "neither id nor name",
			evaluatorID: 0,
			setup:       func(context.Context, *confmocks.MockIConfiger, *repomocks.MockIEvaluatorRepo) {},
			wantErrCode: errno.CommonInvalidParamCode,
			wantErr:     true,
		},
		{
			name:          "empty builtin space config",
			evaluatorID:   1,
			evaluatorName: "",
			setup: func(ctx context.Context, cfg *confmocks.MockIConfiger, _ *repomocks.MockIEvaluatorRepo) {
				cfg.EXPECT().GetBuiltinEvaluatorSpaceConf(ctx).Return([]string{})
			},
		},
		{
			name:        "invalid builtin space id",
			evaluatorID: 1,
			setup: func(ctx context.Context, cfg *confmocks.MockIConfiger, _ *repomocks.MockIEvaluatorRepo) {
				cfg.EXPECT().GetBuiltinEvaluatorSpaceConf(ctx).Return([]string{"abc"})
			},
			wantErr: true,
		},
		{
			name:        "resolve by id success",
			evaluatorID: 11,
			setup: func(ctx context.Context, cfg *confmocks.MockIConfiger, repo *repomocks.MockIEvaluatorRepo) {
				cfg.EXPECT().GetBuiltinEvaluatorSpaceConf(ctx).Return([]string{"100"})
				meta := &entity.Evaluator{ID: 11, SpaceID: 100, Name: "builtin", Builtin: true, BuiltinVisibleVersion: "1.0.0"}
				repo.EXPECT().BatchGetEvaluatorMetaByID(gomock.Any(), []int64{int64(11)}, false).Return([]*entity.Evaluator{meta}, nil)
				ver := &entity.Evaluator{EvaluatorType: entity.EvaluatorTypePrompt, PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{ID: 333, EvaluatorID: 11, Version: "1.0.0"}}
				repo.EXPECT().BatchGetEvaluatorVersionsByEvaluatorIDAndVersions(gomock.Any(), [][2]interface{}{{int64(11), "1.0.0"}}).Return([]*entity.Evaluator{ver}, nil)
			},
			wantID: 333,
		},
		{
			name:        "resolve by id failed to get meta",
			evaluatorID: 11,
			setup: func(ctx context.Context, cfg *confmocks.MockIConfiger, repo *repomocks.MockIEvaluatorRepo) {
				cfg.EXPECT().GetBuiltinEvaluatorSpaceConf(ctx).Return([]string{"100"})
				repo.EXPECT().BatchGetEvaluatorMetaByID(gomock.Any(), []int64{int64(11)}, false).Return(nil, errors.New("get meta failed"))
			},
			wantErr: true,
		},
		{
			name:        "resolve by id empty meta",
			evaluatorID: 11,
			setup: func(ctx context.Context, cfg *confmocks.MockIConfiger, repo *repomocks.MockIEvaluatorRepo) {
				cfg.EXPECT().GetBuiltinEvaluatorSpaceConf(ctx).Return([]string{"100"})
				repo.EXPECT().BatchGetEvaluatorMetaByID(gomock.Any(), []int64{int64(11)}, false).Return([]*entity.Evaluator{}, nil)
			},
			wantID: 0,
		},
		{
			name:          "resolve by id and name mismatch",
			evaluatorID:   11,
			evaluatorName: "other",
			setup: func(ctx context.Context, cfg *confmocks.MockIConfiger, repo *repomocks.MockIEvaluatorRepo) {
				cfg.EXPECT().GetBuiltinEvaluatorSpaceConf(ctx).Return([]string{"100"})
				meta := &entity.Evaluator{ID: 11, SpaceID: 100, Name: "builtin", Builtin: true, BuiltinVisibleVersion: "1.0.0"}
				repo.EXPECT().BatchGetEvaluatorMetaByID(gomock.Any(), []int64{int64(11)}, false).Return([]*entity.Evaluator{meta}, nil)
			},
			wantErrCode: errno.CommonInvalidParamCode,
			wantErr:     true,
		},
		{
			name:        "resolve by id not in allowed space",
			evaluatorID: 11,
			setup: func(ctx context.Context, cfg *confmocks.MockIConfiger, repo *repomocks.MockIEvaluatorRepo) {
				cfg.EXPECT().GetBuiltinEvaluatorSpaceConf(ctx).Return([]string{"100"})
				meta := &entity.Evaluator{ID: 11, SpaceID: 200, Name: "builtin", Builtin: true, BuiltinVisibleVersion: "1.0.0"}
				repo.EXPECT().BatchGetEvaluatorMetaByID(gomock.Any(), []int64{int64(11)}, false).Return([]*entity.Evaluator{meta}, nil)
			},
		},
		{
			name:          "resolve by name success",
			evaluatorName: "builtin",
			setup: func(ctx context.Context, cfg *confmocks.MockIConfiger, repo *repomocks.MockIEvaluatorRepo) {
				cfg.EXPECT().GetBuiltinEvaluatorSpaceConf(ctx).Return([]string{"100", "200"})
				repo.EXPECT().GetEvaluatorMetaBySpaceIDAndName(gomock.Any(), int64(100), "builtin", false).Return(nil, nil)
				meta := &entity.Evaluator{ID: 22, SpaceID: 200, Name: "builtin", Builtin: true, BuiltinVisibleVersion: "2.0.0"}
				repo.EXPECT().GetEvaluatorMetaBySpaceIDAndName(gomock.Any(), int64(200), "builtin", false).Return(meta, nil)
				ver := &entity.Evaluator{EvaluatorType: entity.EvaluatorTypePrompt, PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{ID: 444, EvaluatorID: 22, Version: "2.0.0"}}
				repo.EXPECT().BatchGetEvaluatorVersionsByEvaluatorIDAndVersions(gomock.Any(), [][2]interface{}{{int64(22), "2.0.0"}}).Return([]*entity.Evaluator{ver}, nil)
			},
			wantID: 444,
		},
		{
			name:          "resolve by name failed to get meta",
			evaluatorName: "builtin",
			setup: func(ctx context.Context, cfg *confmocks.MockIConfiger, repo *repomocks.MockIEvaluatorRepo) {
				cfg.EXPECT().GetBuiltinEvaluatorSpaceConf(ctx).Return([]string{"100"})
				repo.EXPECT().GetEvaluatorMetaBySpaceIDAndName(gomock.Any(), int64(100), "builtin", false).Return(nil, errors.New("get meta failed"))
			},
			wantErr: true,
		},
		{
			name:          "resolve by name not found in any space",
			evaluatorName: "builtin",
			setup: func(ctx context.Context, cfg *confmocks.MockIConfiger, repo *repomocks.MockIEvaluatorRepo) {
				cfg.EXPECT().GetBuiltinEvaluatorSpaceConf(ctx).Return([]string{"100", "200"})
				repo.EXPECT().GetEvaluatorMetaBySpaceIDAndName(gomock.Any(), int64(100), "builtin", false).Return(nil, nil)
				repo.EXPECT().GetEvaluatorMetaBySpaceIDAndName(gomock.Any(), int64(200), "builtin", false).Return(nil, nil)
			},
			wantID: 0,
		},
		{
			name:          "resolve by name batch get versions failed",
			evaluatorName: "builtin",
			setup: func(ctx context.Context, cfg *confmocks.MockIConfiger, repo *repomocks.MockIEvaluatorRepo) {
				cfg.EXPECT().GetBuiltinEvaluatorSpaceConf(ctx).Return([]string{"100"})
				meta := &entity.Evaluator{ID: 22, SpaceID: 100, Name: "builtin", Builtin: true, BuiltinVisibleVersion: "2.0.0"}
				repo.EXPECT().GetEvaluatorMetaBySpaceIDAndName(gomock.Any(), int64(100), "builtin", false).Return(meta, nil)
				repo.EXPECT().BatchGetEvaluatorVersionsByEvaluatorIDAndVersions(gomock.Any(), [][2]interface{}{{int64(22), "2.0.0"}}).Return(nil, errors.New("get version failed"))
			},
			wantErr: true,
		},
		{
			name:          "resolve by name version not found",
			evaluatorName: "builtin",
			setup: func(ctx context.Context, cfg *confmocks.MockIConfiger, repo *repomocks.MockIEvaluatorRepo) {
				cfg.EXPECT().GetBuiltinEvaluatorSpaceConf(ctx).Return([]string{"100"})
				meta := &entity.Evaluator{ID: 22, SpaceID: 100, Name: "builtin", Builtin: true, BuiltinVisibleVersion: "2.0.0"}
				repo.EXPECT().GetEvaluatorMetaBySpaceIDAndName(gomock.Any(), int64(100), "builtin", false).Return(meta, nil)
				repo.EXPECT().BatchGetEvaluatorVersionsByEvaluatorIDAndVersions(gomock.Any(), [][2]interface{}{{int64(22), "2.0.0"}}).Return([]*entity.Evaluator{}, nil)
			},
			wantID: 0,
		},
		{
			name:          "resolve by name code evaluator success",
			evaluatorName: "builtin",
			setup: func(ctx context.Context, cfg *confmocks.MockIConfiger, repo *repomocks.MockIEvaluatorRepo) {
				cfg.EXPECT().GetBuiltinEvaluatorSpaceConf(ctx).Return([]string{"100"})
				meta := &entity.Evaluator{ID: 22, SpaceID: 100, Name: "builtin", Builtin: true, BuiltinVisibleVersion: "2.0.0"}
				repo.EXPECT().GetEvaluatorMetaBySpaceIDAndName(gomock.Any(), int64(100), "builtin", false).Return(meta, nil)
				ver := &entity.Evaluator{EvaluatorType: entity.EvaluatorTypeCode, CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{ID: 555, EvaluatorID: 22, Version: "2.0.0"}}
				repo.EXPECT().BatchGetEvaluatorVersionsByEvaluatorIDAndVersions(gomock.Any(), [][2]interface{}{{int64(22), "2.0.0"}}).Return([]*entity.Evaluator{ver}, nil)
			},
			wantID: 555,
		},
		{
			name:          "resolve by name not builtin",
			evaluatorName: "builtin",
			setup: func(ctx context.Context, cfg *confmocks.MockIConfiger, repo *repomocks.MockIEvaluatorRepo) {
				cfg.EXPECT().GetBuiltinEvaluatorSpaceConf(ctx).Return([]string{"100"})
				meta := &entity.Evaluator{ID: 22, SpaceID: 100, Name: "builtin", Builtin: false, BuiltinVisibleVersion: "1.0.0"}
				repo.EXPECT().GetEvaluatorMetaBySpaceIDAndName(gomock.Any(), int64(100), "builtin", false).Return(meta, nil)
			},
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.Background()
			mockRepo := repomocks.NewMockIEvaluatorRepo(ctrl)
			mockCfg := confmocks.NewMockIConfiger(ctrl)

			s := &EvaluatorServiceImpl{
				evaluatorRepo: mockRepo,
				configer:      mockCfg,
			}

			tc.setup(ctx, mockCfg, mockRepo)
			got, err := s.ResolveBuiltinEvaluatorVisibleVersionID(ctx, tc.evaluatorID, tc.evaluatorName)
			if tc.wantErr {
				assert.Error(t, err)
				if tc.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tc.wantErrCode, statusErr.Code())
				}
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.wantID, got)
		})
	}
}

// Test_BatchGetBuiltinEvaluator 覆盖批量可见版本查询与元信息回填
func Test_BatchGetBuiltinEvaluator(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repomocks.NewMockIEvaluatorRepo(ctrl)
	s := &EvaluatorServiceImpl{evaluatorRepo: mockRepo}
	ctx := context.Background()

	t.Run("空入参返回空切片", func(t *testing.T) {
		list, err := s.BatchGetBuiltinEvaluator(ctx, []int64{})
		assert.NoError(t, err)
		assert.Equal(t, 0, len(list))
	})

	t.Run("过滤非builtin与无visibleVersion并回填元信息", func(t *testing.T) {
		metas := []*entity.Evaluator{
			{ID: 1, Builtin: true, BuiltinVisibleVersion: "1.0.0", Name: "A", SpaceID: 7, EvaluatorType: entity.EvaluatorTypePrompt},
			{ID: 2, Builtin: false},
			{ID: 3, Builtin: true, BuiltinVisibleVersion: ""},
			{ID: 4, Builtin: true, BuiltinVisibleVersion: "2.0.0", Name: "B", SpaceID: 8, EvaluatorType: entity.EvaluatorTypeCode},
		}
		mockRepo.EXPECT().BatchGetEvaluatorMetaByID(gomock.Any(), []int64{1, 2, 3, 4}, false).Return(metas, nil)
		mockRepo.EXPECT().BatchGetEvaluatorVersionsByEvaluatorIDAndVersions(gomock.Any(), [][2]interface{}{{int64(1), "1.0.0"}, {int64(4), "2.0.0"}}).Return([]*entity.Evaluator{
			{PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{EvaluatorID: 1, Version: "1.0.0"}},
			{CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{EvaluatorID: 4, Version: "2.0.0"}},
		}, nil)
		list, err := s.BatchGetBuiltinEvaluator(ctx, []int64{1, 2, 3, 4})
		assert.NoError(t, err)
		assert.Equal(t, 2, len(list))
		// 回填校验
		for _, ev := range list {
			if ev.GetEvaluatorID() == 1 {
				assert.Equal(t, int64(1), ev.ID)
				assert.Equal(t, int64(7), ev.SpaceID)
				assert.Equal(t, "A", ev.Name)
				assert.Equal(t, entity.EvaluatorTypePrompt, ev.EvaluatorType)
			}
			if ev.GetEvaluatorID() == 4 {
				assert.Equal(t, int64(4), ev.ID)
				assert.Equal(t, int64(8), ev.SpaceID)
				assert.Equal(t, "B", ev.Name)
				assert.Equal(t, entity.EvaluatorTypeCode, ev.EvaluatorType)
			}
		}
	})
}

// Test_BatchGetEvaluatorByIDAndVersion 回填 evaluator 元信息
func Test_BatchGetEvaluatorByIDAndVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repomocks.NewMockIEvaluatorRepo(ctrl)
	s := &EvaluatorServiceImpl{evaluatorRepo: mockRepo}
	ctx := context.Background()

	pairs := [][2]interface{}{{int64(11), "0.1.0"}, {int64(22), "1.0.0"}}
	// 版本侧仅带有 EvaluatorID（置于具体版本字段中）
	versions := []*entity.Evaluator{
		{EvaluatorType: entity.EvaluatorTypePrompt, PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{EvaluatorID: 11}},
		{EvaluatorType: entity.EvaluatorTypePrompt, PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{EvaluatorID: 22}},
	}
	mockRepo.EXPECT().BatchGetEvaluatorVersionsByEvaluatorIDAndVersions(gomock.Any(), pairs).Return(versions, nil)
	// 元信息补充
	metas := []*entity.Evaluator{
		{ID: 11, SpaceID: 101, Name: "evA", EvaluatorType: entity.EvaluatorTypePrompt},
		{ID: 22, SpaceID: 202, Name: "evB", EvaluatorType: entity.EvaluatorTypeCode},
	}
	mockRepo.EXPECT().BatchGetEvaluatorMetaByID(gomock.Any(), []int64{11, 22}, false).Return(metas, nil)

	got, err := s.BatchGetEvaluatorByIDAndVersion(ctx, pairs)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(got))
	// 校验已回填
	if got[0].GetEvaluatorID() == 11 {
		assert.Equal(t, int64(11), got[0].ID)
		assert.Equal(t, int64(101), got[0].SpaceID)
		assert.Equal(t, "evA", got[0].Name)
	}
	if got[1].GetEvaluatorID() == 22 {
		assert.Equal(t, int64(22), got[1].ID)
		assert.Equal(t, int64(202), got[1].SpaceID)
		assert.Equal(t, "evB", got[1].Name)
	}

	// 空入参
	got2, err2 := s.BatchGetEvaluatorByIDAndVersion(ctx, [][2]interface{}{})
	assert.NoError(t, err2)
	assert.Equal(t, 0, len(got2))
}

// TestEvaluatorServiceImpl_ListEvaluator 使用 gomock 对 ListEvaluator 方法进行单元测试
func TestEvaluatorServiceImpl_ListEvaluator(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvaluatorRepo := repomocks.NewMockIEvaluatorRepo(ctrl)
	s := &EvaluatorServiceImpl{
		evaluatorRepo: mockEvaluatorRepo,
	}

	ctx := context.Background()

	// 定义测试用例
	testCases := []struct {
		name          string
		request       *entity.ListEvaluatorRequest // 注意：这里的 ListEvaluatorRequest 是 service 包内的，不是 repo 包内的
		setupMock     func(mockRepo *repomocks.MockIEvaluatorRepo)
		expectedList  []*entity.Evaluator
		expectedTotal int64
		expectedErr   error
	}{
		{
			name: "成功 - 不带版本信息 (WithVersion = false)",
			request: &entity.ListEvaluatorRequest{
				SpaceID:     1,
				PageSize:    10,
				PageNum:     1,
				WithVersion: false,
			},
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo) {
				// buildListEvaluatorRequest 会将 service.ListEvaluatorRequest 转换为 repo.ListEvaluatorRequest
				// 这里我们模拟 repo.ListEvaluator 的行为，其输入参数是转换后的 repo.ListEvaluatorRequest
				expectedRepoReq := &repo.ListEvaluatorRequest{
					SpaceID:       1,
					PageSize:      10,
					PageNum:       1,
					EvaluatorType: []entity.EvaluatorType{}, // 假设 request.EvaluatorType 为空
					OrderBy:       []*entity.OrderBy{{Field: ptr.Of("updated_at"), IsAsc: ptr.Of(false)}},
				}
				mockRepo.EXPECT().ListEvaluator(gomock.Any(), gomock.Eq(expectedRepoReq)).Return(
					&repo.ListEvaluatorResponse{
						Evaluators: []*entity.Evaluator{
							{ID: 1, Name: "Eval1", SpaceID: 1, Description: "Desc1"},
							{ID: 2, Name: "Eval2", SpaceID: 1, Description: "Desc2"},
						},
						TotalCount: 2,
					}, nil)
			},
			expectedList: []*entity.Evaluator{
				{ID: 1, Name: "Eval1", SpaceID: 1, Description: "Desc1"},
				{ID: 2, Name: "Eval2", SpaceID: 1, Description: "Desc2"},
			},
			expectedTotal: 2,
			expectedErr:   nil,
		},
		{
			name: "成功 - 带版本信息 (WithVersion = true)",
			request: &entity.ListEvaluatorRequest{
				SpaceID:     1,
				PageSize:    10,
				PageNum:     1,
				WithVersion: true,
			},
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo) {
				expectedRepoReq := &repo.ListEvaluatorRequest{
					SpaceID:       1,
					PageSize:      10,
					PageNum:       1,
					EvaluatorType: []entity.EvaluatorType{},
					OrderBy:       []*entity.OrderBy{{Field: ptr.Of("updated_at"), IsAsc: ptr.Of(false)}},
				}
				// 模拟 ListEvaluator 返回结果 (元数据)
				mockRepo.EXPECT().ListEvaluator(gomock.Any(), gomock.Eq(expectedRepoReq)).Return(
					&repo.ListEvaluatorResponse{
						Evaluators: []*entity.Evaluator{
							{ID: 101, Name: "Eval101", SpaceID: 1, EvaluatorType: entity.EvaluatorTypePrompt, LatestVersion: "v1", Description: "Meta Desc 101", BaseInfo: &entity.BaseInfo{UpdatedAt: ptr.Of(int64(1))}},
							{ID: 102, Name: "Eval102", SpaceID: 1, EvaluatorType: entity.EvaluatorTypePrompt, LatestVersion: "v2", Description: "Meta Desc 102", BaseInfo: &entity.BaseInfo{UpdatedAt: ptr.Of(int64(2))}},
						},
						TotalCount: 2,
					}, nil)

				// 模拟 BatchGetEvaluatorVersionsByEvaluatorIDs 返回结果 (版本详情)
				evaluatorIDs := []int64{101, 102}
				mockRepo.EXPECT().BatchGetEvaluatorVersionsByEvaluatorIDs(gomock.Any(), gomock.Eq(evaluatorIDs), false).Return(
					[]*entity.Evaluator{
						{ // 版本信息属于 Evaluator 101
							EvaluatorType: entity.EvaluatorTypePrompt, // 必须与元数据中的 EvaluatorType 一致
							PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{ // 实际版本数据
								EvaluatorID: 101, Version: "v1.0-version", Description: "Version specific desc 1",
							},
						},
						{ // 版本信息属于 Evaluator 102
							EvaluatorType: entity.EvaluatorTypePrompt,
							PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
								EvaluatorID: 102, Version: "v2.0-version", Description: "Version specific desc 2",
							},
						},
					}, nil)
			},
			expectedList: []*entity.Evaluator{
				{ // 结果是元数据和版本详情的合并
					ID: 101, Name: "Eval101", SpaceID: 1, EvaluatorType: entity.EvaluatorTypePrompt, LatestVersion: "v1",
					Description:    "Meta Desc 101",                               // 来自 ListEvaluator 的元数据
					BaseInfo:       &entity.BaseInfo{UpdatedAt: ptr.Of(int64(1))}, // 来自 ListEvaluator 的元数据
					DraftSubmitted: false,                                         // 默认值或来自 ListEvaluator
					PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{ // 来自 BatchGet
						EvaluatorID: 101, Version: "v1.0-version", Description: "Version specific desc 1",
					},
				},
				{
					ID: 102, Name: "Eval102", SpaceID: 1, EvaluatorType: entity.EvaluatorTypePrompt, LatestVersion: "v2",
					Description:    "Meta Desc 102",
					BaseInfo:       &entity.BaseInfo{UpdatedAt: ptr.Of(int64(2))},
					DraftSubmitted: false,
					PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
						EvaluatorID: 102, Version: "v2.0-version", Description: "Version specific desc 2",
					},
				},
			},
			expectedTotal: 2, // 当 WithVersion 为 true 时，返回的是版本数量
			expectedErr:   nil,
		},
		{
			name: "失败 - evaluatorRepo.ListEvaluator 返回错误",
			request: &entity.ListEvaluatorRequest{
				SpaceID:     1,
				WithVersion: false,
			},
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo) {
				expectedRepoReq := &repo.ListEvaluatorRequest{
					SpaceID:       1,
					EvaluatorType: []entity.EvaluatorType{},
					OrderBy:       []*entity.OrderBy{{Field: ptr.Of("updated_at"), IsAsc: ptr.Of(false)}},
				}
				mockRepo.EXPECT().ListEvaluator(gomock.Any(), gomock.Eq(expectedRepoReq)).Return(nil, errors.New("db error from ListEvaluator"))
			},
			expectedList:  nil,
			expectedTotal: 0,
			expectedErr:   errors.New("db error from ListEvaluator"),
		},
		{
			name: "失败 - WithVersion=true 时 evaluatorRepo.BatchGetEvaluatorVersionsByEvaluatorIDs 返回错误",
			request: &entity.ListEvaluatorRequest{
				SpaceID:     1,
				WithVersion: true,
			},
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo) {
				expectedRepoReq := &repo.ListEvaluatorRequest{
					SpaceID:       1,
					EvaluatorType: []entity.EvaluatorType{},
					OrderBy:       []*entity.OrderBy{{Field: ptr.Of("updated_at"), IsAsc: ptr.Of(false)}},
				}
				mockRepo.EXPECT().ListEvaluator(gomock.Any(), gomock.Eq(expectedRepoReq)).Return(
					&repo.ListEvaluatorResponse{
						Evaluators: []*entity.Evaluator{{ID: 1, EvaluatorType: entity.EvaluatorTypePrompt}}, // 提供基础数据
						TotalCount: 1,
					}, nil)
				mockRepo.EXPECT().BatchGetEvaluatorVersionsByEvaluatorIDs(gomock.Any(), gomock.Eq([]int64{1}), false).Return(
					nil, errors.New("db error from BatchGetVersions"))
			},
			expectedList:  nil,
			expectedTotal: 0,
			expectedErr:   errors.New("db error from BatchGetVersions"),
		},
		{
			name: "成功 - ListEvaluator 返回空列表 (WithVersion = false)",
			request: &entity.ListEvaluatorRequest{
				SpaceID:     1,
				WithVersion: false,
			},
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo) {
				expectedRepoReq := &repo.ListEvaluatorRequest{
					SpaceID:       1,
					EvaluatorType: []entity.EvaluatorType{},
					OrderBy:       []*entity.OrderBy{{Field: ptr.Of("updated_at"), IsAsc: ptr.Of(false)}},
				}
				mockRepo.EXPECT().ListEvaluator(gomock.Any(), gomock.Eq(expectedRepoReq)).Return(
					&repo.ListEvaluatorResponse{
						Evaluators: []*entity.Evaluator{},
						TotalCount: 0,
					}, nil)
			},
			expectedList:  []*entity.Evaluator{},
			expectedTotal: 0,
			expectedErr:   nil,
		},
		{
			name: "成功 - ListEvaluator 返回空列表 (WithVersion = true)",
			request: &entity.ListEvaluatorRequest{
				SpaceID:     1,
				WithVersion: true,
			},
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo) {
				expectedRepoReq := &repo.ListEvaluatorRequest{
					SpaceID:       1,
					EvaluatorType: []entity.EvaluatorType{},
					OrderBy:       []*entity.OrderBy{{Field: ptr.Of("updated_at"), IsAsc: ptr.Of(false)}},
				}
				mockRepo.EXPECT().ListEvaluator(gomock.Any(), gomock.Eq(expectedRepoReq)).Return(
					&repo.ListEvaluatorResponse{
						Evaluators: []*entity.Evaluator{},
						TotalCount: 0,
					}, nil)
				// BatchGetEvaluatorVersionsByEvaluatorIDs 应该传入空 evaluatorIDs
				mockRepo.EXPECT().BatchGetEvaluatorVersionsByEvaluatorIDs(gomock.Any(), gomock.Eq([]int64{}), false).Return(
					[]*entity.Evaluator{}, nil)
			},
			expectedList:  []*entity.Evaluator{},
			expectedTotal: 0,
			expectedErr:   nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setupMock(mockEvaluatorRepo)

			list, total, err := s.ListEvaluator(ctx, tc.request)

			if tc.expectedErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tc.expectedErr, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.expectedList, list)
			assert.Equal(t, tc.expectedTotal, total)
		})
	}
}

// TestEvaluatorServiceImpl_ListEvaluator_WithVersion_skipsOrphanBatchVersion 覆盖 BatchGet 返回的版本中父评估器不在 List 结果中时的 continue 分支（导出/列元数据场景下防御脏数据）。
func TestEvaluatorServiceImpl_ListEvaluator_WithVersion_skipsOrphanBatchVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repomocks.NewMockIEvaluatorRepo(ctrl)
	s := &EvaluatorServiceImpl{evaluatorRepo: mockRepo}
	ctx := context.Background()

	mockRepo.EXPECT().ListEvaluator(gomock.Any(), gomock.Any()).Return(
		&repo.ListEvaluatorResponse{
			Evaluators: []*entity.Evaluator{
				{ID: 101, Name: "Meta", SpaceID: 1, EvaluatorType: entity.EvaluatorTypePrompt, Description: "D"},
			},
			TotalCount: 1,
		}, nil)
	mockRepo.EXPECT().BatchGetEvaluatorVersionsByEvaluatorIDs(gomock.Any(), []int64{101}, false).Return(
		[]*entity.Evaluator{
			{
				EvaluatorType: entity.EvaluatorTypePrompt,
				PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
					EvaluatorID: 101, Version: "v1",
				},
			},
			{
				EvaluatorType: entity.EvaluatorTypePrompt,
				PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
					EvaluatorID: 999, Version: "orphan",
				},
			},
		}, nil)

	list, total, err := s.ListEvaluator(ctx, &entity.ListEvaluatorRequest{SpaceID: 1, WithVersion: true})
	assert.NoError(t, err)
	assert.Equal(t, int64(2), total)
	require.Len(t, list, 2)
	assert.Equal(t, int64(101), list[0].ID)
	assert.Equal(t, "Meta", list[0].Name)
	assert.Equal(t, int64(999), list[1].PromptEvaluatorVersion.EvaluatorID)
	assert.Empty(t, list[1].Name)
}

// TestEvaluatorServiceImpl_BatchGetEvaluator 使用 gomock 对 BatchGetEvaluator 方法进行单元测试
func TestEvaluatorServiceImpl_BatchGetEvaluator(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvaluatorRepo := repomocks.NewMockIEvaluatorRepo(ctrl)

	// 被测服务实例
	s := &EvaluatorServiceImpl{
		evaluatorRepo: mockEvaluatorRepo,
		// 其他依赖项对于 BatchGetEvaluator 方法不是必需的
	}

	ctx := context.Background()

	// 定义测试用例
	testCases := []struct {
		name             string
		spaceID          int64
		evaluatorIDs     []int64
		includeDeleted   bool
		setupMock        func(mockRepo *repomocks.MockIEvaluatorRepo)
		expectedResponse []*entity.Evaluator
		expectedErr      error
	}{
		{
			name:           "成功 - 返回多个评估器",
			spaceID:        1,
			evaluatorIDs:   []int64{10, 20},
			includeDeleted: false,
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo) {
				mockRepo.EXPECT().BatchGetEvaluatorDraftByEvaluatorID(gomock.Any(), int64(1), []int64{10, 20}, false).Return(
					[]*entity.Evaluator{
						{ID: 10, Name: "Evaluator10"},
						{ID: 20, Name: "Evaluator20"},
					}, nil)
			},
			expectedResponse: []*entity.Evaluator{
				{ID: 10, Name: "Evaluator10"},
				{ID: 20, Name: "Evaluator20"},
			},
			expectedErr: nil,
		},
		{
			name:           "成功 - 返回空列表",
			spaceID:        2,
			evaluatorIDs:   []int64{30},
			includeDeleted: true,
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo) {
				mockRepo.EXPECT().BatchGetEvaluatorDraftByEvaluatorID(gomock.Any(), int64(2), []int64{30}, true).Return(
					[]*entity.Evaluator{}, nil)
			},
			expectedResponse: []*entity.Evaluator{},
			expectedErr:      nil,
		},
		{
			name:           "成功 - evaluatorIDs 为空",
			spaceID:        3,
			evaluatorIDs:   []int64{},
			includeDeleted: false,
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo) {
				mockRepo.EXPECT().BatchGetEvaluatorDraftByEvaluatorID(gomock.Any(), int64(3), []int64{}, false).Return(
					[]*entity.Evaluator{}, nil)
			},
			expectedResponse: []*entity.Evaluator{},
			expectedErr:      nil,
		},
		{
			name:           "失败 - evaluatorRepo 返回错误",
			spaceID:        4,
			evaluatorIDs:   []int64{40},
			includeDeleted: false,
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo) {
				mockRepo.EXPECT().BatchGetEvaluatorDraftByEvaluatorID(gomock.Any(), int64(4), []int64{40}, false).Return(
					nil, errors.New("database error"))
			},
			expectedResponse: nil,
			expectedErr:      errors.New("database error"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setupMock(mockEvaluatorRepo)

			actualResponse, actualErr := s.BatchGetEvaluator(ctx, tc.spaceID, tc.evaluatorIDs, tc.includeDeleted)

			if tc.expectedErr != nil {
				assert.Error(t, actualErr)
				assert.Equal(t, tc.expectedErr, actualErr)
			} else {
				assert.NoError(t, actualErr)
			}
			assert.Equal(t, tc.expectedResponse, actualResponse)
		})
	}
}

// TestEvaluatorServiceImpl_GetEvaluator 使用gomock 对 GetEvaluator 方法进行单元测试
func TestEvaluatorServiceImpl_GetEvaluator(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvaluatorRepo := repomocks.NewMockIEvaluatorRepo(ctrl)
	s := &EvaluatorServiceImpl{
		evaluatorRepo: mockEvaluatorRepo,
	}
	ctx := context.Background()

	testCases := []struct {
		name              string
		spaceID           int64
		evaluatorID       int64
		includeDeleted    bool
		setupMock         func(mockRepo *repomocks.MockIEvaluatorRepo)
		expectedEvaluator *entity.Evaluator
		expectedErr       error
		expectedErrCode   int32 // 用于校验 errorx 错误码
	}{
		{
			name:           "失败 - evaluatorRepo.BatchGetEvaluatorDraftByEvaluatorID 返回错误",
			spaceID:        1,
			evaluatorID:    100,
			includeDeleted: false,
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo) {
				mockRepo.EXPECT().BatchGetEvaluatorDraftByEvaluatorID(gomock.Any(), int64(1), gomock.Eq([]int64{100}), false).
					Return(nil, errors.New("db error"))
			},
			expectedErr: errors.New("db error"),
		},
		{
			name:           "成功 - evaluatorRepo.BatchGetEvaluatorDraftByEvaluatorID 返回空列表",
			spaceID:        1,
			evaluatorID:    101,
			includeDeleted: false,
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo) {
				mockRepo.EXPECT().BatchGetEvaluatorDraftByEvaluatorID(gomock.Any(), int64(1), gomock.Eq([]int64{101}), false).
					Return([]*entity.Evaluator{}, nil)
			},
			expectedEvaluator: nil,
			expectedErr:       nil,
		},
		{
			name:           "成功 - evaluatorRepo.BatchGetEvaluatorDraftByEvaluatorID 返回一个 evaluator",
			spaceID:        1,
			evaluatorID:    102,
			includeDeleted: false,
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo) {
				mockRepo.EXPECT().BatchGetEvaluatorDraftByEvaluatorID(gomock.Any(), int64(1), gomock.Eq([]int64{102}), false).
					Return([]*entity.Evaluator{{ID: 102, SpaceID: 1, Name: "Test Eval"}}, nil)
			},
			expectedEvaluator: &entity.Evaluator{ID: 102, SpaceID: 1, Name: "Test Eval"},
			expectedErr:       nil,
		},
		{
			name:           "成功 - evaluatorRepo.BatchGetEvaluatorDraftByEvaluatorID 返回多个 evaluators, 取第一个",
			spaceID:        1,
			evaluatorID:    103,
			includeDeleted: true,
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo) {
				mockRepo.EXPECT().BatchGetEvaluatorDraftByEvaluatorID(gomock.Any(), int64(1), gomock.Eq([]int64{103}), true).
					Return([]*entity.Evaluator{
						{ID: 103, SpaceID: 1, Name: "First Eval"},
						{ID: 10301, SpaceID: 1, Name: "Second Eval"},
					}, nil)
			},
			expectedEvaluator: &entity.Evaluator{ID: 103, SpaceID: 1, Name: "First Eval"},
			expectedErr:       nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setupMock(mockEvaluatorRepo)

			evaluator, err := s.GetEvaluator(ctx, tc.spaceID, tc.evaluatorID, tc.includeDeleted)

			if tc.expectedErr != nil {
				assert.Error(t, err)
				if tc.expectedErrCode != 0 {
					e, ok := err.(interface{ GetCode() int32 })
					assert.True(t, ok)
					assert.Equal(t, tc.expectedErrCode, e.GetCode())
				}
				assert.Contains(t, err.Error(), tc.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.expectedEvaluator, evaluator)
		})
	}
}

// TestEvaluatorServiceImpl_CreateEvaluator 使用 gomock 对 CreateEvaluator 方法进行单元测试
func TestEvaluatorServiceImpl_CreateEvaluator(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvaluatorRepo := repomocks.NewMockIEvaluatorRepo(ctrl)
	mockIdemService := idemmocks.NewMockIdempotentService(ctrl)

	s := &EvaluatorServiceImpl{
		evaluatorRepo: mockEvaluatorRepo,
		idem:          mockIdemService,
	}

	ctx := context.Background()
	fixedTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	testUserID := "test_user_123"

	// 准备一个基础的 evaluatorDO 用于测试
	baseEvaluatorDO := func() *entity.Evaluator {
		return &entity.Evaluator{
			SpaceID:       int64(1),
			Name:          "Test Evaluator",
			EvaluatorType: entity.EvaluatorTypePrompt,
			PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
				MessageList: []*entity.Message{},
				ModelConfig: &entity.ModelConfig{},
			},
		}
	}

	testCases := []struct {
		name            string
		evaluatorDO     *entity.Evaluator
		cid             string
		setupMock       func(evaluatorDO *entity.Evaluator, cid string, mockIdem *idemmocks.MockIdempotentService, mockRepo *repomocks.MockIEvaluatorRepo)
		expectedID      int64
		expectedErr     error
		expectedErrCode int32
	}{
		{
			name: "失败 - validateCreateEvaluatorRequest - CheckNameExist 返回错误",
			evaluatorDO: func() *entity.Evaluator {
				e := baseEvaluatorDO()
				e.Name = "check_name_err_eval"
				return e
			}(),
			cid: "validate_checkname_err_cid",
			setupMock: func(evaluatorDO *entity.Evaluator, cid string, mockIdem *idemmocks.MockIdempotentService, mockRepo *repomocks.MockIEvaluatorRepo) {
				expectedKey := "create_evaluator_idem" + cid
				mockIdem.EXPECT().Set(gomock.Any(), expectedKey, time.Second*10).Return(nil)
				mockRepo.EXPECT().CheckNameExist(gomock.Any(), evaluatorDO.SpaceID, int64(-1), evaluatorDO.Name).
					Return(false, errors.New("db check name error"))
			},
			expectedID:  0,
			expectedErr: errors.New("db check name error"),
		},
		{
			name:        "失败 - evaluatorRepo.CreateEvaluator 返回错误",
			evaluatorDO: baseEvaluatorDO(),
			cid:         "repo_create_err_cid",
			setupMock: func(evaluatorDO *entity.Evaluator, cid string, mockIdem *idemmocks.MockIdempotentService, mockRepo *repomocks.MockIEvaluatorRepo) {
				expectedKey := "create_evaluator_idem" + cid
				mockIdem.EXPECT().Set(gomock.Any(), expectedKey, time.Second*10).Return(nil)
				if evaluatorDO.Name != "" {
					mockEvaluatorRepo.EXPECT().CheckNameExist(gomock.Any(), evaluatorDO.SpaceID, int64(-1), evaluatorDO.Name).
						Return(false, nil)
				}
				session.WithCtxUser(ctx, &session.User{ID: testUserID})

				expectedInjectedDO := *evaluatorDO
				expectedInjectedDO.BaseInfo = &entity.BaseInfo{
					CreatedBy: &entity.UserInfo{UserID: ptr.Of(testUserID)},
					UpdatedBy: &entity.UserInfo{UserID: ptr.Of(testUserID)},
					CreatedAt: ptr.Of(fixedTime.UnixMilli()),
					UpdatedAt: ptr.Of(fixedTime.UnixMilli()),
				}
				if expectedInjectedDO.PromptEvaluatorVersion != nil {
					expectedInjectedDO.PromptEvaluatorVersion.BaseInfo = &entity.BaseInfo{
						CreatedBy: &entity.UserInfo{UserID: ptr.Of(testUserID)},
						UpdatedBy: &entity.UserInfo{UserID: ptr.Of(testUserID)},
						CreatedAt: ptr.Of(fixedTime.UnixMilli()),
						UpdatedAt: ptr.Of(fixedTime.UnixMilli()),
					}
				}

				mockRepo.EXPECT().CreateEvaluator(gomock.Any(), gomock.Any()).
					Return(int64(0), errors.New("db create error"))
			},
			expectedID:  int64(0),
			expectedErr: errors.New("db create error"),
		},
		{
			name:        "成功 - 创建 Evaluator",
			evaluatorDO: baseEvaluatorDO(),
			cid:         "success_cid",
			setupMock: func(evaluatorDO *entity.Evaluator, cid string, mockIdem *idemmocks.MockIdempotentService, mockRepo *repomocks.MockIEvaluatorRepo) {
				expectedKey := "create_evaluator_idem" + cid
				mockIdem.EXPECT().Set(gomock.Any(), expectedKey, time.Second*10).Return(nil)
				if evaluatorDO.Name != "" {
					mockEvaluatorRepo.EXPECT().CheckNameExist(gomock.Any(), evaluatorDO.SpaceID, int64(-1), evaluatorDO.Name).
						Return(false, nil)
				}
				session.WithCtxUser(ctx, &session.User{ID: testUserID})

				expectedInjectedDO := *evaluatorDO
				expectedInjectedDO.BaseInfo = &entity.BaseInfo{
					CreatedBy: &entity.UserInfo{UserID: ptr.Of(testUserID)},
					UpdatedBy: &entity.UserInfo{UserID: ptr.Of(testUserID)},
					CreatedAt: ptr.Of(fixedTime.UnixMilli()),
					UpdatedAt: ptr.Of(fixedTime.UnixMilli()),
				}
				if expectedInjectedDO.PromptEvaluatorVersion != nil {
					expectedInjectedDO.PromptEvaluatorVersion.BaseInfo = &entity.BaseInfo{
						CreatedBy: &entity.UserInfo{UserID: ptr.Of(testUserID)},
						UpdatedBy: &entity.UserInfo{UserID: ptr.Of(testUserID)},
						CreatedAt: ptr.Of(fixedTime.UnixMilli()),
						UpdatedAt: ptr.Of(fixedTime.UnixMilli()),
					}
				}

				mockRepo.EXPECT().CreateEvaluator(gomock.Any(), gomock.Any()).
					Return(int64(12345), nil)
			},
			expectedID:  int64(12345),
			expectedErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setupMock(tc.evaluatorDO, tc.cid, mockIdemService, mockEvaluatorRepo)

			id, err := s.CreateEvaluator(ctx, tc.evaluatorDO, tc.cid)

			if tc.expectedErr != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.expectedID, id)
		})
	}
}

func TestEvaluatorServiceImpl_UpdateEvaluatorMeta(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvaluatorRepo := repomocks.NewMockIEvaluatorRepo(ctrl)

	// 创建被测服务实例，并注入 mock 依赖
	s := &EvaluatorServiceImpl{
		evaluatorRepo: mockEvaluatorRepo,
		// 其他依赖项对于此方法不是必需的，可以省略或设为 nil
	}

	ctx := context.Background()

	// 定义测试用例
	tests := []struct {
		name        string
		id          int64
		spaceID     int64
		evalName    string // 对应 UpdateEvaluatorMeta 的 name 参数
		description string
		userID      string
		setupMock   func(repoMock *repomocks.MockIEvaluatorRepo) // 用于设置 mock 期望
		wantErr     bool
		expectedErr error // 期望的错误，用于更精确的错误断言
	}{
		{
			name:        "成功 - 名称为空字符串，不校验名称是否存在，更新成功",
			id:          1,
			spaceID:     100,
			evalName:    "", // 名称为空
			description: "new description",
			userID:      "user123",
			setupMock: func(repoMock *repomocks.MockIEvaluatorRepo) {
				// CheckNameExist 不应该被调用
				repoMock.EXPECT().UpdateEvaluatorMeta(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: false,
		},
		{
			name:        "成功 - 名称不为空，且名称不存在，更新成功",
			id:          2,
			spaceID:     101,
			evalName:    "newName",
			description: "another description",
			userID:      "user456",
			setupMock: func(repoMock *repomocks.MockIEvaluatorRepo) {
				repoMock.EXPECT().CheckNameExist(gomock.Any(), int64(101), int64(2), "newName").Return(false, nil)
				repoMock.EXPECT().UpdateEvaluatorMeta(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: false,
		},
		{
			name:        "失败 - 名称不为空，CheckNameExist 返回错误",
			id:          3,
			spaceID:     102,
			evalName:    "checkFailName",
			description: "desc",
			userID:      "user789",
			setupMock: func(repoMock *repomocks.MockIEvaluatorRepo) {
				repoMock.EXPECT().CheckNameExist(gomock.Any(), int64(102), int64(3), "checkFailName").Return(false, errors.New("db check error"))
				// UpdateEvaluatorMeta 不应该被调用
			},
			wantErr:     true,
			expectedErr: errors.New("db check error"),
		},
		{
			name:        "失败 - 名称不为空，且名称已存在",
			id:          4,
			spaceID:     103,
			evalName:    "existingName",
			description: "desc",
			userID:      "userABC",
			setupMock: func(repoMock *repomocks.MockIEvaluatorRepo) {
				repoMock.EXPECT().CheckNameExist(gomock.Any(), int64(103), int64(4), "existingName").Return(true, nil)
				// UpdateEvaluatorMeta 不应该被调用
			},
			wantErr:     true,
			expectedErr: errorx.NewByCode(errno.EvaluatorNameExistCode), // 假设 errorx 和 errno 包已正确导入
		},
		{
			name:        "失败 - 名称为空，但 UpdateEvaluatorMeta 返回错误",
			id:          5,
			spaceID:     104,
			evalName:    "",
			description: "update fail desc",
			userID:      "userDEF",
			setupMock: func(repoMock *repomocks.MockIEvaluatorRepo) {
				// CheckNameExist 不应该被调用
				repoMock.EXPECT().UpdateEvaluatorMeta(gomock.Any(), gomock.Any()).Return(errors.New("db update error"))
			},
			wantErr:     true,
			expectedErr: errors.New("db update error"),
		},
		{
			name:        "失败 - 名称不为空且不存在，但 UpdateEvaluatorMeta 返回错误",
			id:          6,
			spaceID:     105,
			evalName:    "validNameButFailUpdate",
			description: "desc",
			userID:      "userGHI",
			setupMock: func(repoMock *repomocks.MockIEvaluatorRepo) {
				repoMock.EXPECT().CheckNameExist(gomock.Any(), int64(105), int64(6), "validNameButFailUpdate").Return(false, nil)
				repoMock.EXPECT().UpdateEvaluatorMeta(gomock.Any(), gomock.Any()).Return(errors.New("db update error after check"))
			},
			wantErr:     true,
			expectedErr: errors.New("db update error after check"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 为每个子测试重置/设置 mock 期望
			if tt.setupMock != nil {
				tt.setupMock(mockEvaluatorRepo)
			}

			err := s.UpdateEvaluatorMeta(ctx, &entity.UpdateEvaluatorMetaRequest{
				ID:          tt.id,
				SpaceID:     tt.spaceID,
				Name:        &tt.evalName,
				Description: &tt.description,
				UpdatedBy:   tt.userID,
			})

			if tt.wantErr {
				assert.Error(t, err, "期望得到一个错误")
			} else {
				assert.NoError(t, err, "不期望得到错误")
			}
		})
	}
}

// TestEvaluatorServiceImpl_UpdateEvaluatorDraft 测试 UpdateEvaluatorDraft 方法
func TestEvaluatorServiceImpl_UpdateEvaluatorDraft(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvaluatorRepo := repomocks.NewMockIEvaluatorRepo(ctrl)

	s := &EvaluatorServiceImpl{
		evaluatorRepo: mockEvaluatorRepo,
	}

	ctx := context.Background()
	testEvaluator := &entity.Evaluator{
		ID:      1,
		SpaceID: 100,
		Name:    "Test Evaluator",
		PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
			ID:       10,
			BaseInfo: &entity.BaseInfo{},
		},
		BaseInfo: &entity.BaseInfo{},
	}

	tests := []struct {
		name          string
		evaluatorDO   *entity.Evaluator
		setupMock     func(mockRepo *repomocks.MockIEvaluatorRepo)
		expectedError error
	}{
		{
			name:        "成功更新评估器草稿",
			evaluatorDO: testEvaluator,
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo) {
				mockRepo.EXPECT().UpdateEvaluatorDraft(gomock.Any(), gomock.Any()).Return(nil)
			},
			expectedError: nil,
		},
		{
			name:        "更新评估器草稿失败 - repo返回错误",
			evaluatorDO: testEvaluator,
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo) {
				mockRepo.EXPECT().UpdateEvaluatorDraft(gomock.Any(), testEvaluator).Return(errors.New("repo error"))
			},
			expectedError: errors.New("repo error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock(mockEvaluatorRepo)
			err := s.UpdateEvaluatorDraft(ctx, tt.evaluatorDO)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestEvaluatorServiceImpl_UpdateBuiltinEvaluatorDraft 测试 UpdateBuiltinEvaluatorDraft 方法
func TestEvaluatorServiceImpl_UpdateBuiltinEvaluatorDraft(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvaluatorRepo := repomocks.NewMockIEvaluatorRepo(ctrl)

	s := &EvaluatorServiceImpl{
		evaluatorRepo: mockEvaluatorRepo,
	}

	ctx := context.Background()
	testEvaluator := &entity.Evaluator{
		ID:      1,
		SpaceID: 100,
		Name:    "Test Builtin Evaluator",
		Tags: map[entity.EvaluatorTagLangType]map[entity.EvaluatorTagKey][]string{
			entity.EvaluatorTagLangType_En: {
				entity.EvaluatorTagKey_Category:  {"LLM", "Code"},
				entity.EvaluatorTagKey_Objective: {"Quality"},
			},
		},
		PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
			ID:       10,
			BaseInfo: &entity.BaseInfo{},
		},
		BaseInfo: &entity.BaseInfo{},
	}

	tests := []struct {
		name          string
		evaluatorDO   *entity.Evaluator
		setupMock     func(mockRepo *repomocks.MockIEvaluatorRepo)
		expectedError error
	}{
		{
			name:        "成功更新内置评估器草稿",
			evaluatorDO: testEvaluator,
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo) {
				mockRepo.EXPECT().UpdateEvaluatorDraft(gomock.Any(), gomock.Any()).Return(nil)
			},
			expectedError: nil,
		},
		{
			name:        "更新内置评估器草稿失败 - repo返回错误",
			evaluatorDO: testEvaluator,
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo) {
				mockRepo.EXPECT().UpdateEvaluatorDraft(gomock.Any(), testEvaluator).Return(errors.New("repo error"))
			},
			expectedError: errors.New("repo error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock(mockEvaluatorRepo)
			err := s.UpdateEvaluatorDraft(ctx, tt.evaluatorDO)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError, err)
			} else {
				assert.NoError(t, err, "不期望得到错误")
			}
		})
	}
}

// TestEvaluatorServiceImpl_DeleteEvaluator 测试 DeleteEvaluator 方法
func TestEvaluatorServiceImpl_DeleteEvaluator(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvaluatorRepo := repomocks.NewMockIEvaluatorRepo(ctrl)

	s := &EvaluatorServiceImpl{
		evaluatorRepo: mockEvaluatorRepo,
	}

	ctx := context.Background()
	testEvaluatorIDs := []int64{1, 2, 3}
	testUserID := "test_user_id"

	tests := []struct {
		name          string
		evaluatorIDs  []int64
		userID        string
		setupMock     func(mockRepo *repomocks.MockIEvaluatorRepo)
		expectedError error
	}{
		{
			name:         "成功删除评估器",
			evaluatorIDs: testEvaluatorIDs,
			userID:       testUserID,
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo) {
				mockRepo.EXPECT().BatchDeleteEvaluator(gomock.Any(), testEvaluatorIDs, testUserID).Return(nil)
			},
			expectedError: nil,
		},
		{
			name:         "删除评估器失败 - repo返回错误",
			evaluatorIDs: testEvaluatorIDs,
			userID:       testUserID,
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo) {
				mockRepo.EXPECT().BatchDeleteEvaluator(gomock.Any(), testEvaluatorIDs, testUserID).Return(errors.New("repo delete error"))
			},
			expectedError: errors.New("repo delete error"),
		},
		{
			name:         "删除评估器 - 空ID列表",
			evaluatorIDs: []int64{},
			userID:       testUserID,
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo) {
				mockRepo.EXPECT().BatchDeleteEvaluator(gomock.Any(), []int64{}, testUserID).Return(nil) // 假设repo层允许空列表
			},
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock(mockEvaluatorRepo)
			err := s.DeleteEvaluator(ctx, tt.evaluatorIDs, tt.userID)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestEvaluatorServiceImpl_ListEvaluatorVersion 测试 ListEvaluatorVersion 方法
func TestEvaluatorServiceImpl_ListEvaluatorVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvaluatorRepo := repomocks.NewMockIEvaluatorRepo(ctrl)

	s := &EvaluatorServiceImpl{
		evaluatorRepo: mockEvaluatorRepo,
	}

	ctx := context.Background()

	// 辅助函数，用于创建 entity.Evaluator 实例
	newEvaluator := func(id int64, version string) *entity.Evaluator {
		return &entity.Evaluator{
			PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
				ID:      id,
				Version: version,
			},
		}
	}

	tests := []struct {
		name              string
		request           *entity.ListEvaluatorVersionRequest // service 包内的 ListEvaluatorVersionRequest
		setupMock         func(mockRepo *repomocks.MockIEvaluatorRepo, serviceReq *entity.ListEvaluatorVersionRequest)
		expectedVersions  []*entity.Evaluator
		expectedTotal     int64
		expectedError     error
		expectedErrorMsg  string // 用于 errorx 类型的错误消息比较
		expectedErrorCode int    // 用于 errorx 类型的错误码比较
	}{
		{
			name: "成功获取评估器版本列表 - 带自定义排序",
			request: &entity.ListEvaluatorVersionRequest{
				EvaluatorID: 1,
				PageSize:    10,
				PageNum:     1,
				OrderBys: []*entity.OrderBy{
					{Field: ptr.Of(entity.OrderByCreatedAt), IsAsc: ptr.Of(true)},
				},
			},
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo, serviceReq *entity.ListEvaluatorVersionRequest) {
				// 预期 buildListEvaluatorVersionRequest 会转换的 repo.ListEvaluatorVersionRequest
				expectedRepoReq := &repo.ListEvaluatorVersionRequest{
					EvaluatorID:   serviceReq.EvaluatorID,
					QueryVersions: serviceReq.QueryVersions,
					PageSize:      serviceReq.PageSize,
					PageNum:       serviceReq.PageNum,
					OrderBy: []*entity.OrderBy{ // 确保 OrderBySet 包含 OrderByCreatedAt
						{Field: ptr.Of(entity.OrderByCreatedAt), IsAsc: ptr.Of(true)},
					},
				}
				// Mock entity.OrderBySet 使得自定义排序生效
				// 注意：直接修改全局变量 entity.OrderBySet 可能会影响其他测试，更好的方式是确保其已正确初始化
				// 这里假设 entity.OrderBySet["created_at"] 存在

				mockRepo.EXPECT().ListEvaluatorVersion(gomock.Any(), gomock.Eq(expectedRepoReq)).Return(
					&repo.ListEvaluatorVersionResponse{
						Versions:   []*entity.Evaluator{newEvaluator(101, "v1.0"), newEvaluator(102, "v1.1")},
						TotalCount: 2,
					}, nil)
			},
			expectedVersions: []*entity.Evaluator{newEvaluator(101, "v1.0"), newEvaluator(102, "v1.1")},
			expectedTotal:    2,
			expectedError:    nil,
		},
		{
			name: "成功获取评估器版本列表 - 默认排序 (updated_at desc)",
			request: &entity.ListEvaluatorVersionRequest{
				EvaluatorID: 2,
				PageSize:    5,
				PageNum:     2,
			},
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo, serviceReq *entity.ListEvaluatorVersionRequest) {
				expectedRepoReq := &repo.ListEvaluatorVersionRequest{
					EvaluatorID:   serviceReq.EvaluatorID,
					QueryVersions: serviceReq.QueryVersions,
					PageSize:      serviceReq.PageSize,
					PageNum:       serviceReq.PageNum,
					OrderBy: []*entity.OrderBy{ // 默认排序
						{Field: ptr.Of(entity.OrderByUpdatedAt), IsAsc: ptr.Of(false)},
					},
				}

				mockRepo.EXPECT().ListEvaluatorVersion(gomock.Any(), gomock.Eq(expectedRepoReq)).Return(
					&repo.ListEvaluatorVersionResponse{
						Versions:   []*entity.Evaluator{newEvaluator(201, "v2.0")},
						TotalCount: 1,
					}, nil)
			},
			expectedVersions: []*entity.Evaluator{newEvaluator(201, "v2.0")},
			expectedTotal:    1,
			expectedError:    nil,
		},
		{
			name: "成功获取评估器版本列表 - repo返回空列表",
			request: &entity.ListEvaluatorVersionRequest{
				EvaluatorID: 3,
			},
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo, serviceReq *entity.ListEvaluatorVersionRequest) {
				expectedRepoReq := &repo.ListEvaluatorVersionRequest{
					EvaluatorID: serviceReq.EvaluatorID,
					OrderBy:     []*entity.OrderBy{{Field: ptr.Of(entity.OrderByUpdatedAt), IsAsc: ptr.Of(false)}},
				}

				mockRepo.EXPECT().ListEvaluatorVersion(gomock.Any(), gomock.Eq(expectedRepoReq)).Return(
					&repo.ListEvaluatorVersionResponse{
						Versions:   []*entity.Evaluator{},
						TotalCount: 0,
					}, nil)
			},
			expectedVersions: []*entity.Evaluator{},
			expectedTotal:    0,
			expectedError:    nil,
		},
		{
			name: "获取评估器版本列表失败 - repo返回错误",
			request: &entity.ListEvaluatorVersionRequest{
				EvaluatorID: 4,
			},
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo, serviceReq *entity.ListEvaluatorVersionRequest) {
				expectedRepoReq := &repo.ListEvaluatorVersionRequest{
					EvaluatorID: serviceReq.EvaluatorID,
					OrderBy:     []*entity.OrderBy{{Field: ptr.Of(entity.OrderByUpdatedAt), IsAsc: ptr.Of(false)}},
				}

				mockRepo.EXPECT().ListEvaluatorVersion(gomock.Any(), gomock.Eq(expectedRepoReq)).Return(
					nil, errors.New("db query error"))
			},
			expectedVersions: nil,
			expectedTotal:    0,
			expectedError:    errors.New("db query error"),
		},
		{
			name: "获取评估器版本列表失败 - 无效的OrderBy字段 (buildListEvaluatorVersionRequest内部过滤)",
			request: &entity.ListEvaluatorVersionRequest{
				EvaluatorID: 1,
				OrderBys: []*entity.OrderBy{
					{Field: ptr.Of("invalid_field"), IsAsc: ptr.Of(true)}, // 这个字段会被过滤掉
					{Field: ptr.Of(entity.OrderByCreatedAt), IsAsc: ptr.Of(false)},
				},
			},
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo, serviceReq *entity.ListEvaluatorVersionRequest) {
				// 预期 buildListEvaluatorVersionRequest 会过滤掉 "invalid_field"
				expectedRepoReq := &repo.ListEvaluatorVersionRequest{
					EvaluatorID: serviceReq.EvaluatorID,
					OrderBy: []*entity.OrderBy{
						{Field: ptr.Of(entity.OrderByCreatedAt), IsAsc: ptr.Of(false)},
					},
				}

				mockRepo.EXPECT().ListEvaluatorVersion(gomock.Any(), gomock.Eq(expectedRepoReq)).Return(
					&repo.ListEvaluatorVersionResponse{
						Versions:   []*entity.Evaluator{newEvaluator(101, "v1.0")},
						TotalCount: 1,
					}, nil)
			},
			expectedVersions: []*entity.Evaluator{newEvaluator(101, "v1.0")},
			expectedTotal:    1,
			expectedError:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 在每个 PatchConvey 内部设置 mock，以确保隔离
			// 对于依赖全局变量的 buildListEvaluatorVersionRequest，需要在这里 mock entity.OrderBySet
			// 如果 entity.OrderBySet 是在包初始化时就固定的，则不需要每次都 mock
			// 但为了测试的确定性，这里显式 mock
			originalOrderBySet := entity.OrderBySet                   // 备份原始值
			defer func() { entity.OrderBySet = originalOrderBySet }() // 恢复原始值

			tt.setupMock(mockEvaluatorRepo, tt.request)

			versions, total, err := s.ListEvaluatorVersion(ctx, tt.request)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.expectedVersions, versions)
			assert.Equal(t, tt.expectedTotal, total)
		})
	}
}

// TestEvaluatorServiceImpl_GetEvaluatorVersion 测试 GetEvaluatorVersion 方法
func TestEvaluatorServiceImpl_GetEvaluatorVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvaluatorRepo := repomocks.NewMockIEvaluatorRepo(ctrl)
	// 被测服务实例
	s := &EvaluatorServiceImpl{
		evaluatorRepo: mockEvaluatorRepo,
		// 其他依赖项如果该方法不需要，可以为 nil 或默认值
	}
	ctx := context.Background() // 标准的上下文

	// 定义输入参数结构体
	type args struct {
		evaluatorVersionID int64
		includeDeleted     bool
	}
	// 定义测试用例表格
	testCases := []struct {
		name      string                                                  // 测试用例名称
		args      args                                                    // 输入参数
		setupMock func(mockRepo *repomocks.MockIEvaluatorRepo, args args) // mock设置函数
		want      *entity.Evaluator                                       // 期望得到的评估器实体
		wantErr   error                                                   // 期望得到的错误
	}{
		{
			name: "成功 - 找到评估器版本",
			args: args{evaluatorVersionID: 1, includeDeleted: false},
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo, args args) {
				// 期望 evaluatorRepo.BatchGetEvaluatorByVersionID 被调用一次
				// 参数为：任意上下文, ID切片, 是否包含删除
				// 返回预设的评估器列表和nil错误
				mockRepo.EXPECT().BatchGetEvaluatorByVersionID(gomock.Any(), gomock.Any(), gomock.Eq([]int64{args.evaluatorVersionID}), args.includeDeleted, gomock.Any()).
					Return([]*entity.Evaluator{{ID: 1, Name: "Test Evaluator Version 1"}}, nil)
			},
			want:    &entity.Evaluator{ID: 1, Name: "Test Evaluator Version 1"},
			wantErr: nil,
		},
		{
			name: "成功 - 未找到评估器版本 (repo返回空列表)",
			args: args{evaluatorVersionID: 2, includeDeleted: false},
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo, args args) {
				mockRepo.EXPECT().BatchGetEvaluatorByVersionID(gomock.Any(), gomock.Any(), gomock.Eq([]int64{args.evaluatorVersionID}), args.includeDeleted, gomock.Any()).
					Return([]*entity.Evaluator{}, nil) // Repo返回空列表表示未找到
			},
			want:    nil, // 期望返回nil实体
			wantErr: nil, // 期望返回nil错误
		},
		{
			name: "失败 - evaluatorRepo.BatchGetEvaluatorByVersionID 返回错误",
			args: args{evaluatorVersionID: 3, includeDeleted: true},
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo, args args) {
				mockRepo.EXPECT().BatchGetEvaluatorByVersionID(gomock.Any(), gomock.Any(), gomock.Eq([]int64{args.evaluatorVersionID}), args.includeDeleted, gomock.Any()).
					Return(nil, errors.New("repo database error")) // Repo返回错误
			},
			want:    nil,
			wantErr: errors.New("repo database error"), // 期望透传错误
		},
		{
			name: "成功 - repo返回多个评估器版本 (应返回第一个)",
			args: args{evaluatorVersionID: 4, includeDeleted: false},
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo, args args) {
				mockRepo.EXPECT().BatchGetEvaluatorByVersionID(gomock.Any(), gomock.Any(), gomock.Eq([]int64{args.evaluatorVersionID}), args.includeDeleted, gomock.Any()).
					Return([]*entity.Evaluator{
						{ID: 4, Name: "First Evaluator Version"},
						{ID: 5, Name: "Second Evaluator Version"}, // 即使返回多个，方法也只取第一个
					}, nil)
			},
			want:    &entity.Evaluator{ID: 4, Name: "First Evaluator Version"},
			wantErr: nil,
		},
	}

	// 遍历执行测试用例
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setupMock(mockEvaluatorRepo, tc.args)

			got, err := s.GetEvaluatorVersion(ctx, nil, tc.args.evaluatorVersionID, tc.args.includeDeleted, false)

			// 断言错误
			if tc.wantErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tc.wantErr, err)
			} else {
				assert.NoError(t, err)
			}
			// 断言返回的实体
			assert.Equal(t, tc.want, got) // ShouldResemble用于比较结构体内容
		})
	}
}

// TestEvaluatorServiceImpl_BatchGetEvaluatorVersion 测试 BatchGetEvaluatorVersion 方法
func TestEvaluatorServiceImpl_BatchGetEvaluatorVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvaluatorRepo := repomocks.NewMockIEvaluatorRepo(ctrl)
	s := &EvaluatorServiceImpl{
		evaluatorRepo: mockEvaluatorRepo,
	}
	ctx := context.Background()

	type args struct {
		evaluatorVersionIDs []int64
		includeDeleted      bool
	}
	testCases := []struct {
		name      string
		args      args
		setupMock func(mockRepo *repomocks.MockIEvaluatorRepo, args args)
		want      []*entity.Evaluator
		wantErr   error
	}{
		{
			name: "成功 - 找到多个评估器版本",
			args: args{evaluatorVersionIDs: []int64{10, 20}, includeDeleted: false},
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo, args args) {
				mockRepo.EXPECT().BatchGetEvaluatorByVersionID(gomock.Any(), gomock.Any(), gomock.Eq(args.evaluatorVersionIDs), args.includeDeleted, gomock.Any()).
					Return([]*entity.Evaluator{
						{ID: 10, Name: "Evaluator Version 10"},
						{ID: 20, Name: "Evaluator Version 20"},
					}, nil)
			},
			want: []*entity.Evaluator{
				{ID: 10, Name: "Evaluator Version 10"},
				{ID: 20, Name: "Evaluator Version 20"},
			},
			wantErr: nil,
		},
		{
			name: "成功 - 传入空ID列表 (repo应返回空列表)",
			args: args{evaluatorVersionIDs: []int64{}, includeDeleted: false},
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo, args args) {
				mockRepo.EXPECT().BatchGetEvaluatorByVersionID(gomock.Any(), gomock.Any(), gomock.Eq(args.evaluatorVersionIDs), args.includeDeleted, gomock.Any()).
					Return([]*entity.Evaluator{}, nil) // 期望repo对于空ID列表返回空列表
			},
			want:    []*entity.Evaluator{},
			wantErr: nil,
		},
		{
			name: "成功 - 未找到任何评估器版本 (repo返回空列表)",
			args: args{evaluatorVersionIDs: []int64{999}, includeDeleted: true},
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo, args args) {
				mockRepo.EXPECT().BatchGetEvaluatorByVersionID(gomock.Any(), gomock.Any(), gomock.Eq(args.evaluatorVersionIDs), args.includeDeleted, gomock.Any()).
					Return([]*entity.Evaluator{}, nil)
			},
			want:    []*entity.Evaluator{},
			wantErr: nil,
		},
		{
			name: "失败 - evaluatorRepo.BatchGetEvaluatorByVersionID 返回错误",
			args: args{evaluatorVersionIDs: []int64{30}, includeDeleted: false},
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo, args args) {
				mockRepo.EXPECT().BatchGetEvaluatorByVersionID(gomock.Any(), gomock.Any(), gomock.Eq(args.evaluatorVersionIDs), args.includeDeleted, gomock.Any()).
					Return(nil, errors.New("batch repo database error"))
			},
			want:    nil,
			wantErr: errors.New("batch repo database error"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setupMock(mockEvaluatorRepo, tc.args)

			got, err := s.BatchGetEvaluatorVersion(ctx, nil, tc.args.evaluatorVersionIDs, tc.args.includeDeleted)

			if tc.wantErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tc.wantErr, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.want, got) // ShouldResemble用于比较slice内容
		})
	}
}

// TestEvaluatorServiceImpl_SubmitEvaluatorVersion 使用 gomock 对 SubmitEvaluatorVersion 方法进行单元测试
func TestEvaluatorServiceImpl_SubmitEvaluatorVersion(t *testing.T) {
	mockUserID := "test-user-id"
	mockGeneratedVersionID := int64(12345)
	mockEvaluatorDO := &entity.Evaluator{
		ID:            100,
		SpaceID:       1,
		Name:          "Test Evaluator",
		EvaluatorType: entity.EvaluatorTypePrompt, // 确保 GetEvaluatorVersion 能工作
		// PromptEvaluatorVersion 直接使用具体实现
		PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
			ID:                100,
			EvaluatorID:       100,
			SpaceID:           1,
			PromptTemplateKey: "test-template-key",
			PromptSuffix:      "test-prompt-suffix",
			ModelConfig: &entity.ModelConfig{
				ModelID: gptr.Of(int64(1)),
			},
			ParseType: entity.ParseTypeFunctionCall,
			MessageList: []*entity.Message{
				{
					Role: entity.RoleSystem,
					Content: &entity.Content{
						ContentType: ptr.Of(entity.ContentTypeText),
						Text:        ptr.Of("test-content"),
					},
				},
			},
			InputSchemas: []*entity.ArgsSchema{
				{
					Key:        ptr.Of("test-input-key"),
					JsonSchema: ptr.Of("test-json-schema"),
					SupportContentTypes: []entity.ContentType{
						entity.ContentTypeText,
					},
				},
			},
		},
	}

	// Test cases
	testCases := []struct {
		name            string
		evaluatorDO     *entity.Evaluator // 输入的 Evaluator 实体
		version         string
		description     string
		cid             string
		setupMocks      func(ctrl *gomock.Controller, mockIdem *idemmocks.MockIdempotentService, mockIdgen *idgenmocks.MockIIDGenerator, mockRepo *repomocks.MockIEvaluatorRepo, inputEvaluatorDO *entity.Evaluator)
		expectedEvalDO  *entity.Evaluator // 期望返回的 Evaluator 实体
		expectedErrCode int32             // 期望的错误码，0表示无错误
		expectedErrMsg  string            // 期望的错误信息中的特定子串
		expectPanic     bool
	}{
		{
			name:        "成功提交新版本",
			evaluatorDO: mockEvaluatorDO,
			version:     "v1.0.0",
			description: "Initial version",
			cid:         "client-id-1",
			setupMocks: func(ctrl *gomock.Controller, mockIdem *idemmocks.MockIdempotentService, mockIdgen *idgenmocks.MockIIDGenerator, mockRepo *repomocks.MockIEvaluatorRepo, inputEvaluatorDO *entity.Evaluator) {
				// 1. Mock idem.Set
				mockIdem.EXPECT().Set(gomock.Any(), consts.IdemKeySubmitEvaluator+"client-id-1", time.Second*10).Return(nil)
				// 2. Mock idgen.GenID
				mockIdgen.EXPECT().GenID(gomock.Any()).Return(mockGeneratedVersionID, nil)
				session.WithCtxUser(context.Background(), &session.User{ID: mockUserID})

				mockRepo.EXPECT().CheckVersionExist(gomock.Any(), inputEvaluatorDO.ID, "v1.0.0").Return(false, nil)
				// 7. Mock time.Now
				// 10. Mock repo.SubmitEvaluatorVersion
				mockRepo.EXPECT().SubmitEvaluatorVersion(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, submittedDO *entity.Evaluator) error {
						return nil
					})
			},
			expectedEvalDO:  mockEvaluatorDO,
			expectedErrCode: 0,
		},
		{
			name: "失败 - 幂等性检查失败",
			evaluatorDO: &entity.Evaluator{
				ID:            101,
				EvaluatorType: entity.EvaluatorTypePrompt,
			},
			version:     "v1.0.0",
			description: "Desc",
			cid:         "client-id-2",
			setupMocks: func(ctrl *gomock.Controller, mockIdem *idemmocks.MockIdempotentService, mockIdgen *idgenmocks.MockIIDGenerator, mockRepo *repomocks.MockIEvaluatorRepo, inputEvaluatorDO *entity.Evaluator) {
				mockIdem.EXPECT().Set(gomock.Any(), consts.IdemKeySubmitEvaluator+"client-id-2", time.Second*10).Return(errors.New("idem set error"))
			},
			expectedErrCode: errno.ActionRepeatedCode,
			expectedErrMsg:  "idempotent error",
		},
		{
			name:        "失败 - ID生成失败",
			evaluatorDO: mockEvaluatorDO,
			version:     "v1.0.0",
			description: "Desc",
			cid:         "client-id-3",
			setupMocks: func(ctrl *gomock.Controller, mockIdem *idemmocks.MockIdempotentService, mockIdgen *idgenmocks.MockIIDGenerator, mockRepo *repomocks.MockIEvaluatorRepo, inputEvaluatorDO *entity.Evaluator) {
				mockIdem.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockIdgen.EXPECT().GenID(gomock.Any()).Return(int64(1), errors.New("gen id error"))
			},
			expectedErrCode: -1, // 函数直接返回 err，不是 errorx 类型
			expectedErrMsg:  "gen id error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockIdemService := idemmocks.NewMockIdempotentService(ctrl)
			mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
			mockEvalRepo := repomocks.NewMockIEvaluatorRepo(ctrl)

			s := &EvaluatorServiceImpl{
				evaluatorRepo: mockEvalRepo,
				idem:          mockIdemService,
				idgen:         mockIDGen,
			}

			if tc.setupMocks != nil {
				tc.setupMocks(ctrl, mockIdemService, mockIDGen, mockEvalRepo, tc.evaluatorDO)
			}

			returnedEvalDO, err := s.SubmitEvaluatorVersion(context.Background(), tc.evaluatorDO, tc.version, tc.description, tc.cid)

			if tc.expectedErrCode != 0 {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tc.expectedEvalDO != nil {
				assert.Equal(t, returnedEvalDO.ID, tc.expectedEvalDO.ID)
				assert.Equal(t, returnedEvalDO.LatestVersion, tc.expectedEvalDO.LatestVersion)
				assert.Equal(t, returnedEvalDO.DraftSubmitted, tc.expectedEvalDO.DraftSubmitted)
				if tc.expectedEvalDO.BaseInfo != nil && returnedEvalDO.BaseInfo != nil {
					if tc.expectedEvalDO.BaseInfo.UpdatedBy != nil && returnedEvalDO.BaseInfo.UpdatedBy != nil {
						assert.Equal(t, *returnedEvalDO.BaseInfo.UpdatedBy.UserID, *tc.expectedEvalDO.BaseInfo.UpdatedBy.UserID)
					}
				}
			}
		})
	}
}

func TestEvaluatorServiceImpl_RunEvaluator(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvaluatorRepo := repomocks.NewMockIEvaluatorRepo(ctrl)
	mockLimiter := repomocks.NewMockRateLimiter(ctrl)
	mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockEvaluatorRecordRepo := repomocks.NewMockIEvaluatorRecordRepo(ctrl)
	mockEvaluatorSourceService := mocks.NewMockEvaluatorSourceService(ctrl)
	mockPlainLimiter := repomocks.NewMockIPlainRateLimiter(ctrl)
	s := &EvaluatorServiceImpl{
		evaluatorRepo:       mockEvaluatorRepo,
		limiter:             mockLimiter,
		idgen:               mockIDGen,
		evaluatorRecordRepo: mockEvaluatorRecordRepo,
		// mqFactory, idem, configer 可以为 nil 或根据需要 mock
		evaluatorSourceServices: map[entity.EvaluatorType]EvaluatorSourceService{
			entity.EvaluatorTypePrompt: mockEvaluatorSourceService, // 使用生成的 mock
		},
		plainRateLimiter: mockPlainLimiter,
	}

	ctx := context.Background()

	defaultRequest := &entity.RunEvaluatorRequest{
		SpaceID:            1,
		EvaluatorVersionID: 101,
		InputData:          &entity.EvaluatorInputData{ /* ... */ },
		ExperimentID:       201,
		ItemID:             301,
		TurnID:             401,
		Ext:                map[string]string{"key": "value"},
	}

	defaultEvaluatorDO := &entity.Evaluator{
		ID:            100,
		SpaceID:       1,
		Name:          "Test Evaluator",
		EvaluatorType: entity.EvaluatorTypePrompt, // 确保 GetEvaluatorVersion 能工作
		// PromptEvaluatorVersion 直接使用具体实现
		PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
			ID:                100,
			EvaluatorID:       100,
			SpaceID:           1,
			PromptTemplateKey: "test-template-key",
			PromptSuffix:      "test-prompt-suffix",
			ModelConfig: &entity.ModelConfig{
				ModelID: gptr.Of(int64(1)),
			},
			ParseType: entity.ParseTypeFunctionCall,
			MessageList: []*entity.Message{
				{
					Role: entity.RoleSystem,
					Content: &entity.Content{
						ContentType: ptr.Of(entity.ContentTypeText),
						Text:        ptr.Of("test-content"),
					},
				},
			},
			InputSchemas: []*entity.ArgsSchema{
				{
					Key:        ptr.Of("test-input-key"),
					JsonSchema: ptr.Of("test-json-schema"),
					SupportContentTypes: []entity.ContentType{
						entity.ContentTypeText,
					},
				},
			},
		},
	}

	defaultOutputData := &entity.EvaluatorOutputData{ /* ... */ }
	defaultRunStatus := entity.EvaluatorRunStatusSuccess
	defaultRecordID := int64(999)
	defaultUserID := "user-test-id"
	defaultLogID := "log-id-abc"

	testCases := []struct {
		name            string
		request         *entity.RunEvaluatorRequest
		setupMocks      func(mockEvaluatorSourceService *mocks.MockEvaluatorSourceService)
		expectedRecord  *entity.EvaluatorRecord
		expectedErr     error
		expectedErrCode int32 // 用于校验 errorx 类型的错误
	}{
		{
			name:    "成功运行评估器",
			request: defaultRequest,
			setupMocks: func(mockEvaluatorSourceService *mocks.MockEvaluatorSourceService) {
				mockEvaluatorRepo.EXPECT().BatchGetEvaluatorByVersionID(gomock.Any(), nil, []int64{defaultRequest.EvaluatorVersionID}, false, false).Return([]*entity.Evaluator{defaultEvaluatorDO}, nil)
				mockLimiter.EXPECT().AllowInvoke(gomock.Any(), defaultRequest.SpaceID).Return(true)
				mockPlainLimiter.EXPECT().AllowInvokeWithKeyLimit(gomock.Any(), gomock.Any(), gomock.Any()).Return(true)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(defaultRecordID, nil)
				mockEvaluatorSourceService.EXPECT().PreHandle(gomock.Any(), defaultEvaluatorDO).Return(nil)
				mockEvaluatorSourceService.EXPECT().Run(gomock.Any(), defaultEvaluatorDO, defaultRequest.InputData, gomock.Any(), defaultRequest.SpaceID, defaultRequest.DisableTracing).Return(defaultOutputData, defaultRunStatus, "trace-id-123")

				mockEvaluatorRecordRepo.EXPECT().CreateEvaluatorRecord(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, record *entity.EvaluatorRecord) error {
						assert.Equal(t, record.ID, defaultRecordID)
						assert.Equal(t, record.SpaceID, defaultRequest.SpaceID)
						assert.Equal(t, record.EvaluatorVersionID, defaultRequest.EvaluatorVersionID)
						assert.Equal(t, record.Status, defaultRunStatus)
						return nil
					})
			},
			expectedRecord: &entity.EvaluatorRecord{
				ID:                  defaultRecordID,
				SpaceID:             defaultRequest.SpaceID,
				ExperimentID:        defaultRequest.ExperimentID,
				ExperimentRunID:     defaultRequest.ExperimentRunID,
				ItemID:              defaultRequest.ItemID,
				TurnID:              defaultRequest.TurnID,
				EvaluatorVersionID:  defaultRequest.EvaluatorVersionID,
				LogID:               defaultLogID,
				EvaluatorInputData:  defaultRequest.InputData,
				EvaluatorOutputData: defaultOutputData,
				Status:              defaultRunStatus,
				Ext:                 defaultRequest.Ext,
				BaseInfo: &entity.BaseInfo{
					CreatedBy: &entity.UserInfo{UserID: gptr.Of(defaultUserID)},
				},
			},
			expectedErr: nil,
		},
		{
			name: "成功运行评估器_DisableTracing为true",
			request: &entity.RunEvaluatorRequest{
				SpaceID:            1,
				EvaluatorVersionID: 101,
				InputData:          &entity.EvaluatorInputData{},
				ExperimentID:       201,
				ItemID:             301,
				TurnID:             401,
				Ext:                map[string]string{"key": "value"},
				DisableTracing:     true,
			},
			setupMocks: func(mockEvaluatorSourceService *mocks.MockEvaluatorSourceService) {
				mockEvaluatorRepo.EXPECT().BatchGetEvaluatorByVersionID(gomock.Any(), nil, []int64{101}, false, false).Return([]*entity.Evaluator{defaultEvaluatorDO}, nil)
				mockLimiter.EXPECT().AllowInvoke(gomock.Any(), int64(1)).Return(true)
				mockPlainLimiter.EXPECT().AllowInvokeWithKeyLimit(gomock.Any(), gomock.Any(), gomock.Any()).Return(true)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(defaultRecordID, nil)
				session.WithCtxUser(ctx, &session.User{ID: defaultUserID})
				mockEvaluatorSourceService.EXPECT().PreHandle(gomock.Any(), defaultEvaluatorDO).Return(nil)
				// 验证DisableTracing参数正确传递给EvaluatorSourceService.Run方法
				mockEvaluatorSourceService.EXPECT().Run(gomock.Any(), defaultEvaluatorDO, gomock.Any(), gomock.Nil(), int64(1), true).Return(defaultOutputData, defaultRunStatus, "trace-id-123")

				mockEvaluatorRecordRepo.EXPECT().CreateEvaluatorRecord(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, record *entity.EvaluatorRecord) error {
						assert.Equal(t, record.ID, defaultRecordID)
						assert.Equal(t, record.SpaceID, int64(1))
						assert.Equal(t, record.EvaluatorVersionID, int64(101))
						assert.Equal(t, record.Status, defaultRunStatus)
						return nil
					})
			},
			expectedRecord: &entity.EvaluatorRecord{
				ID:                  defaultRecordID,
				SpaceID:             1,
				ExperimentID:        201,
				ExperimentRunID:     0,
				ItemID:              301,
				TurnID:              401,
				EvaluatorVersionID:  101,
				LogID:               defaultLogID,
				EvaluatorInputData:  &entity.EvaluatorInputData{},
				EvaluatorOutputData: defaultOutputData,
				Status:              defaultRunStatus,
				Ext:                 map[string]string{"key": "value"},
				BaseInfo: &entity.BaseInfo{
					CreatedBy: &entity.UserInfo{UserID: gptr.Of(defaultUserID)},
				},
			},
			expectedErr: nil,
		},
		{
			name: "成功运行评估器_DisableTracing为false",
			request: &entity.RunEvaluatorRequest{
				SpaceID:            1,
				EvaluatorVersionID: 101,
				InputData:          &entity.EvaluatorInputData{},
				ExperimentID:       201,
				ItemID:             301,
				TurnID:             401,
				Ext:                map[string]string{"key": "value"},
				DisableTracing:     false,
			},
			setupMocks: func(mockEvaluatorSourceService *mocks.MockEvaluatorSourceService) {
				mockEvaluatorRepo.EXPECT().BatchGetEvaluatorByVersionID(gomock.Any(), nil, []int64{101}, false, false).Return([]*entity.Evaluator{defaultEvaluatorDO}, nil)
				mockLimiter.EXPECT().AllowInvoke(gomock.Any(), int64(1)).Return(true)
				mockPlainLimiter.EXPECT().AllowInvokeWithKeyLimit(gomock.Any(), gomock.Any(), gomock.Any()).Return(true)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(defaultRecordID, nil)
				session.WithCtxUser(ctx, &session.User{ID: defaultUserID})
				mockEvaluatorSourceService.EXPECT().PreHandle(gomock.Any(), defaultEvaluatorDO).Return(nil)
				// 验证DisableTracing参数正确传递给EvaluatorSourceService.Run方法
				mockEvaluatorSourceService.EXPECT().Run(gomock.Any(), defaultEvaluatorDO, gomock.Any(), gomock.Nil(), int64(1), false).Return(defaultOutputData, defaultRunStatus, "trace-id-123")

				mockEvaluatorRecordRepo.EXPECT().CreateEvaluatorRecord(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, record *entity.EvaluatorRecord) error {
						assert.Equal(t, record.ID, defaultRecordID)
						assert.Equal(t, record.SpaceID, int64(1))
						assert.Equal(t, record.EvaluatorVersionID, int64(101))
						assert.Equal(t, record.Status, defaultRunStatus)
						return nil
					})
			},
			expectedRecord: &entity.EvaluatorRecord{
				ID:                  defaultRecordID,
				SpaceID:             1,
				ExperimentID:        201,
				ExperimentRunID:     0,
				ItemID:              301,
				TurnID:              401,
				EvaluatorVersionID:  101,
				LogID:               defaultLogID,
				EvaluatorInputData:  &entity.EvaluatorInputData{},
				EvaluatorOutputData: defaultOutputData,
				Status:              defaultRunStatus,
				Ext:                 map[string]string{"key": "value"},
				BaseInfo: &entity.BaseInfo{
					CreatedBy: &entity.UserInfo{UserID: gptr.Of(defaultUserID)},
				},
			},
			expectedErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setupMocks != nil {
				tc.setupMocks(mockEvaluatorSourceService)
			}

			record, err := s.RunEvaluator(ctx, tc.request)

			if tc.expectedErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tc.expectedErr, err)
			} else {
				assert.NoError(t, err)
			}

			// 使用 ShouldResemble 比较结构体，它会递归比较字段值
			assert.Equal(t, tc.expectedRecord.ID, record.ID)
			assert.Equal(t, tc.expectedRecord.SpaceID, record.SpaceID)
			assert.Equal(t, tc.expectedRecord.EvaluatorVersionID, record.EvaluatorVersionID)
			assert.Equal(t, tc.expectedRecord.Status, record.Status)
			assert.Equal(t, tc.expectedRecord.Ext, record.Ext)
		})
	}
}

func Test_roundEvaluatorOutputScore(t *testing.T) {
	tests := []struct {
		name           string
		outputData     *entity.EvaluatorOutputData
		wantScore      *float64
		wantCorrection *float64
	}{
		{
			name:       "nil outputData",
			outputData: nil,
		},
		{
			name:       "nil EvaluatorResult",
			outputData: &entity.EvaluatorOutputData{},
		},
		{
			name: "round Score and Correction.Score",
			outputData: func() *entity.EvaluatorOutputData {
				score := 0.125
				cScore := 0.124
				return &entity.EvaluatorOutputData{
					EvaluatorResult: &entity.EvaluatorResult{
						Score: &score,
						Correction: &entity.Correction{
							Score: &cScore,
						},
					},
				}
			}(),
			wantScore:      gptr.Of(0.13),
			wantCorrection: gptr.Of(0.12),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			roundEvaluatorOutputScore(tt.outputData)
			if tt.outputData == nil {
				return
			}
			if tt.wantScore == nil && tt.wantCorrection == nil {
				return
			}
			if assert.NotNil(t, tt.outputData.EvaluatorResult) {
				if tt.wantScore != nil && assert.NotNil(t, tt.outputData.EvaluatorResult.Score) {
					assert.InDelta(t, *tt.wantScore, *tt.outputData.EvaluatorResult.Score, 1e-9)
				}
				if tt.wantCorrection != nil &&
					assert.NotNil(t, tt.outputData.EvaluatorResult.Correction) &&
					assert.NotNil(t, tt.outputData.EvaluatorResult.Correction.Score) {
					assert.InDelta(t, *tt.wantCorrection, *tt.outputData.EvaluatorResult.Correction.Score, 1e-9)
				}
			}
		})
	}
}

func TestEvaluatorServiceImpl_RunEvaluator_RoundAndConvertErrMsg(t *testing.T) {
	tests := []struct {
		name           string
		outputData     *entity.EvaluatorOutputData
		wantMsg        string
		wantScore      *float64
		wantCorrection *float64
	}{
		{
			name: func() string {
				return "convert error message when code not custom and message non-empty"
			}(),
			outputData: func() *entity.EvaluatorOutputData {
				rawScore := 0.125
				rawCorrectionScore := 0.124
				return &entity.EvaluatorOutputData{
					EvaluatorResult: &entity.EvaluatorResult{
						Score: &rawScore,
						Correction: &entity.Correction{
							Score: &rawCorrectionScore,
						},
					},
					EvaluatorRunError: &entity.EvaluatorRunError{
						Code:    10001,
						Message: "raw-msg",
					},
				}
			}(),
			wantMsg:        "converted-msg",
			wantScore:      gptr.Of(0.13),
			wantCorrection: gptr.Of(0.12),
		},
		{
			name: "do not convert when code is custom rpc evaluator run failed",
			outputData: &entity.EvaluatorOutputData{
				EvaluatorRunError: &entity.EvaluatorRunError{
					Code:    int32(errno.CustomRPCEvaluatorRunFailedCode),
					Message: "raw-msg",
				},
			},
			wantMsg: "raw-msg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.Background()
			mockEvaluatorRepo := repomocks.NewMockIEvaluatorRepo(ctrl)
			mockLimiter := repomocks.NewMockRateLimiter(ctrl)
			mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
			mockEvaluatorRecordRepo := repomocks.NewMockIEvaluatorRecordRepo(ctrl)
			mockEvaluatorSourceService := mocks.NewMockEvaluatorSourceService(ctrl)
			mockPlainLimiter := repomocks.NewMockIPlainRateLimiter(ctrl)

			req := &entity.RunEvaluatorRequest{
				SpaceID:            1,
				EvaluatorVersionID: 101,
				InputData:          &entity.EvaluatorInputData{},
			}
			evaluatorDO := &entity.Evaluator{
				ID:            100,
				SpaceID:       1,
				Builtin:       false,
				EvaluatorType: entity.EvaluatorTypePrompt,
				PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
					ID:          101,
					EvaluatorID: 100,
					SpaceID:     1,
				},
			}

			mockErrConfiger := componentMocks.NewMockIConfiger(ctrl)
			if tt.wantMsg == "converted-msg" {
				mockErrConfiger.EXPECT().GetErrCtrl(gomock.Any()).Return(&entity.ExptErrCtrl{
					ResultErrConverts: []*entity.ResultErrConvert{
						{AsDefault: true, ToErrMsg: "converted-msg"},
					},
				})
			}

			s := &EvaluatorServiceImpl{
				evaluatorRepo:       mockEvaluatorRepo,
				limiter:             mockLimiter,
				idgen:               mockIDGen,
				evaluatorRecordRepo: mockEvaluatorRecordRepo,
				evaluatorSourceServices: map[entity.EvaluatorType]EvaluatorSourceService{
					entity.EvaluatorTypePrompt: mockEvaluatorSourceService,
				},
				plainRateLimiter: mockPlainLimiter,
				cConfiger:        mockErrConfiger,
			}

			mockEvaluatorRepo.EXPECT().BatchGetEvaluatorByVersionID(gomock.Any(), nil, []int64{req.EvaluatorVersionID}, false, false).
				Return([]*entity.Evaluator{evaluatorDO}, nil)
			mockLimiter.EXPECT().AllowInvoke(gomock.Any(), req.SpaceID).Return(true)
			mockPlainLimiter.EXPECT().AllowInvokeWithKeyLimit(gomock.Any(), "run_evaluator:100", gomock.Any()).Return(true)
			mockEvaluatorSourceService.EXPECT().PreHandle(gomock.Any(), evaluatorDO).Return(nil)
			mockEvaluatorSourceService.EXPECT().Run(gomock.Any(), evaluatorDO, req.InputData, req.EvaluatorRunConf, req.SpaceID, req.DisableTracing).
				Return(tt.outputData, entity.EvaluatorRunStatusFail, "trace-1")
			mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(999), nil)
			mockEvaluatorRecordRepo.EXPECT().CreateEvaluatorRecord(gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, record *entity.EvaluatorRecord) error {
					if assert.NotNil(t, record.EvaluatorOutputData) && assert.NotNil(t, record.EvaluatorOutputData.EvaluatorRunError) {
						assert.Equal(t, tt.wantMsg, record.EvaluatorOutputData.EvaluatorRunError.Message)
					}
					if tt.wantScore != nil &&
						assert.NotNil(t, record.EvaluatorOutputData) &&
						assert.NotNil(t, record.EvaluatorOutputData.EvaluatorResult) &&
						assert.NotNil(t, record.EvaluatorOutputData.EvaluatorResult.Score) {
						assert.InDelta(t, *tt.wantScore, *record.EvaluatorOutputData.EvaluatorResult.Score, 1e-9)
					}
					if tt.wantCorrection != nil &&
						assert.NotNil(t, record.EvaluatorOutputData) &&
						assert.NotNil(t, record.EvaluatorOutputData.EvaluatorResult) &&
						assert.NotNil(t, record.EvaluatorOutputData.EvaluatorResult.Correction) &&
						assert.NotNil(t, record.EvaluatorOutputData.EvaluatorResult.Correction.Score) {
						assert.InDelta(t, *tt.wantCorrection, *record.EvaluatorOutputData.EvaluatorResult.Correction.Score, 1e-9)
					}
					return nil
				},
			)

			_, err := s.RunEvaluator(ctx, req)
			assert.NoError(t, err)
		})
	}
}

func Test_EvaluatorServiceImpl_DebugEvaluator(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.Background()
	mockEvaluatorRepo := repomocks.NewMockIEvaluatorRepo(ctrl)
	mockLimiter := repomocks.NewMockRateLimiter(ctrl)
	mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockEvaluatorRecordRepo := repomocks.NewMockIEvaluatorRecordRepo(ctrl)
	mockEvaluatorSourceService := mocks.NewMockEvaluatorSourceService(ctrl)
	mockService := &EvaluatorServiceImpl{
		evaluatorRepo:       mockEvaluatorRepo,
		limiter:             mockLimiter,
		idgen:               mockIDGen,
		evaluatorRecordRepo: mockEvaluatorRecordRepo,
		// mqFactory, idem, configer 可以为 nil 或根据需要 mock
		evaluatorSourceServices: map[entity.EvaluatorType]EvaluatorSourceService{
			entity.EvaluatorTypePrompt: mockEvaluatorSourceService, // 假设这是一个 mock 的 PromptEvaluatorSourceService
		},
	}

	defaultOutputData := &entity.EvaluatorOutputData{ /* ... */ }
	mockEvaluator := &entity.Evaluator{
		ID:            100,
		SpaceID:       1,
		Name:          "Test Evaluator",
		EvaluatorType: entity.EvaluatorTypePrompt, // 确保 GetEvaluatorVersion 能工作
		// PromptEvaluatorVersion 直接使用具体实现
		PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
			ID:                100,
			EvaluatorID:       100,
			SpaceID:           1,
			PromptTemplateKey: "test-template-key",
			PromptSuffix:      "test-prompt-suffix",
			ModelConfig: &entity.ModelConfig{
				ModelID: gptr.Of(int64(1)),
			},
			ParseType: entity.ParseTypeFunctionCall,
			MessageList: []*entity.Message{
				{
					Role: entity.RoleSystem,
					Content: &entity.Content{
						ContentType: ptr.Of(entity.ContentTypeText),
						Text:        ptr.Of("test-content"),
					},
				},
			},
			InputSchemas: []*entity.ArgsSchema{
				{
					Key:        ptr.Of("test-input-key"),
					JsonSchema: ptr.Of("test-json-schema"),
					SupportContentTypes: []entity.ContentType{
						entity.ContentTypeText,
					},
				},
			},
		},
	}
	testCases := []struct {
		name            string
		request         *entity.RunEvaluatorRequest
		setupMocks      func(mockEvaluatorSourceService *mocks.MockEvaluatorSourceService)
		expectedErr     error
		expectedErrCode int32 // 用于校验 errorx 类型的错误
	}{
		{
			name: "成功调试评估器",
			request: &entity.RunEvaluatorRequest{
				SpaceID:            1,
				EvaluatorVersionID: 101,
				InputData:          &entity.EvaluatorInputData{ /*... */ },
				ExperimentID:       201,
				ItemID:             301,
				TurnID:             401,
				Ext:                map[string]string{"key": "value"},
			},
			setupMocks: func(mockEvaluatorSourceService *mocks.MockEvaluatorSourceService) {
				mockEvaluatorSourceService.EXPECT().PreHandle(ctx, mockEvaluator).Return(nil)
				mockEvaluatorSourceService.EXPECT().Validate(ctx, mockEvaluator).Return(nil)
				mockEvaluatorSourceService.EXPECT().Debug(ctx, mockEvaluator, gomock.Any(), gomock.Nil(), int64(0)).Return(defaultOutputData, nil)
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setupMocks != nil {
				tc.setupMocks(mockEvaluatorSourceService)
			}
			outputData, err := mockService.DebugEvaluator(ctx, mockEvaluator, tc.request.InputData, nil, int64(0))
			if tc.expectedErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tc.expectedErr, err)
			} else {
				assert.NoError(t, err)
			}
			assert.NotNil(t, outputData)
		})
	}
}

func Test_EvaluatorServiceImpl_injectUserInfo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.Background()
	mockEvaluatorRepo := repomocks.NewMockIEvaluatorRepo(ctrl)
	mockLimiter := repomocks.NewMockRateLimiter(ctrl)
	mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockEvaluatorRecordRepo := repomocks.NewMockIEvaluatorRecordRepo(ctrl)
	mockEvaluatorSourceService := mocks.NewMockEvaluatorSourceService(ctrl)
	mockService := &EvaluatorServiceImpl{
		evaluatorRepo:       mockEvaluatorRepo,
		limiter:             mockLimiter,
		idgen:               mockIDGen,
		evaluatorRecordRepo: mockEvaluatorRecordRepo,
		// mqFactory, idem, configer 可以为 nil 或根据需要 mock
		evaluatorSourceServices: map[entity.EvaluatorType]EvaluatorSourceService{
			entity.EvaluatorTypePrompt: mockEvaluatorSourceService, // 假设这是一个 mock 的 PromptEvaluatorSourceService
		},
	}
	mockEvaluator := &entity.Evaluator{
		EvaluatorType: entity.EvaluatorTypePrompt,
		PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
			BaseInfo: nil,
		},
		BaseInfo: &entity.BaseInfo{
			CreatedBy: &entity.UserInfo{UserID: gptr.Of("user-test-id")},
			UpdatedBy: &entity.UserInfo{UserID: gptr.Of("user-test-id")},
			UpdatedAt: gptr.Of(time.Now().UnixMilli()),
			CreatedAt: gptr.Of(time.Now().UnixMilli()),
		},
	}
	mockService.injectUserInfo(ctx, mockEvaluator)
	assert.NotNil(t, mockEvaluator.BaseInfo.CreatedBy.UserID)
	assert.NotNil(t, mockEvaluator.BaseInfo.UpdatedBy.UserID)
	assert.NotNil(t, mockEvaluator.BaseInfo.UpdatedAt)
	assert.NotNil(t, mockEvaluator.BaseInfo.CreatedAt)
}

// TestEvaluatorServiceImpl_RunEvaluator_DisableTracing 测试EvaluatorServiceImpl.RunEvaluator中DisableTracing参数传递
func TestEvaluatorServiceImpl_RunEvaluator_DisableTracing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvaluatorRepo := repomocks.NewMockIEvaluatorRepo(ctrl)
	mockLimiter := repomocks.NewMockRateLimiter(ctrl)
	mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockEvaluatorRecordRepo := repomocks.NewMockIEvaluatorRecordRepo(ctrl)
	mockEvaluatorSourceService := mocks.NewMockEvaluatorSourceService(ctrl)
	mockPlainLimiter := repomocks.NewMockIPlainRateLimiter(ctrl)

	s := &EvaluatorServiceImpl{
		evaluatorRepo:       mockEvaluatorRepo,
		limiter:             mockLimiter,
		idgen:               mockIDGen,
		evaluatorRecordRepo: mockEvaluatorRecordRepo,
		evaluatorSourceServices: map[entity.EvaluatorType]EvaluatorSourceService{
			entity.EvaluatorTypePrompt: mockEvaluatorSourceService,
		},
		plainRateLimiter: mockPlainLimiter,
	}

	ctx := context.Background()
	session.WithCtxUser(ctx, &session.User{ID: "test-user"})

	defaultEvaluatorDO := &entity.Evaluator{
		ID:            100,
		SpaceID:       1,
		Name:          "Test Evaluator",
		EvaluatorType: entity.EvaluatorTypePrompt,
		PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
			ID:                100,
			EvaluatorID:       100,
			SpaceID:           1,
			PromptTemplateKey: "test-template-key",
			PromptSuffix:      "test-prompt-suffix",
			ModelConfig: &entity.ModelConfig{
				ModelID: gptr.Of(int64(1)),
			},
			ParseType: entity.ParseTypeFunctionCall,
		},
	}

	defaultOutputData := &entity.EvaluatorOutputData{
		EvaluatorResult: &entity.EvaluatorResult{
			Score:     gptr.Of(0.85),
			Reasoning: "Test reasoning",
		},
	}
	defaultRunStatus := entity.EvaluatorRunStatusSuccess
	defaultRecordID := int64(999)

	tests := []struct {
		name           string
		disableTracing bool
		setupMocks     func()
	}{
		{
			name:           "DisableTracing为true时正确传递给EvaluatorSourceService.Run",
			disableTracing: true,
			setupMocks: func() {
				mockEvaluatorRepo.EXPECT().BatchGetEvaluatorByVersionID(gomock.Any(), nil, []int64{101}, false, false).Return([]*entity.Evaluator{defaultEvaluatorDO}, nil)
				mockLimiter.EXPECT().AllowInvoke(gomock.Any(), int64(1)).Return(true)
				mockPlainLimiter.EXPECT().AllowInvokeWithKeyLimit(gomock.Any(), gomock.Any(), gomock.Any()).Return(true)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(defaultRecordID, nil)
				mockEvaluatorSourceService.EXPECT().PreHandle(gomock.Any(), defaultEvaluatorDO).Return(nil)
				// 关键验证：确保DisableTracing参数正确传递
				mockEvaluatorSourceService.EXPECT().Run(gomock.Any(), defaultEvaluatorDO, gomock.Any(), gomock.Nil(), int64(1), true).Return(defaultOutputData, defaultRunStatus, "trace-id-123")
				mockEvaluatorRecordRepo.EXPECT().CreateEvaluatorRecord(gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		{
			name:           "DisableTracing为false时正确传递给EvaluatorSourceService.Run",
			disableTracing: false,
			setupMocks: func() {
				mockEvaluatorRepo.EXPECT().BatchGetEvaluatorByVersionID(gomock.Any(), nil, []int64{101}, false, false).Return([]*entity.Evaluator{defaultEvaluatorDO}, nil)
				mockLimiter.EXPECT().AllowInvoke(gomock.Any(), int64(1)).Return(true)
				mockPlainLimiter.EXPECT().AllowInvokeWithKeyLimit(gomock.Any(), gomock.Any(), gomock.Any()).Return(true)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(defaultRecordID, nil)
				mockEvaluatorSourceService.EXPECT().PreHandle(gomock.Any(), defaultEvaluatorDO).Return(nil)
				// 关键验证：确保DisableTracing参数正确传递
				mockEvaluatorSourceService.EXPECT().Run(gomock.Any(), defaultEvaluatorDO, gomock.Any(), gomock.Nil(), int64(1), false).Return(defaultOutputData, defaultRunStatus, "trace-id-123")
				mockEvaluatorRecordRepo.EXPECT().CreateEvaluatorRecord(gomock.Any(), gomock.Any()).Return(nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMocks()

			request := &entity.RunEvaluatorRequest{
				SpaceID:            1,
				EvaluatorVersionID: 101,
				InputData:          &entity.EvaluatorInputData{},
				DisableTracing:     tt.disableTracing,
			}

			record, err := s.RunEvaluator(ctx, request)

			assert.NoError(t, err)
			assert.NotNil(t, record)
			assert.Equal(t, defaultRecordID, record.ID)
			assert.Equal(t, defaultRunStatus, record.Status)
		})
	}
}

// TestEvaluatorServiceImpl_ListBuiltinEvaluator 测试 ListBuiltinEvaluator 方法
func TestEvaluatorServiceImpl_ListBuiltinEvaluator(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvaluatorRepo := repomocks.NewMockIEvaluatorRepo(ctrl)
	s := &EvaluatorServiceImpl{
		evaluatorRepo: mockEvaluatorRepo,
	}

	ctx := context.Background()

	// 定义测试用例
	testCases := []struct {
		name          string
		request       *entity.ListBuiltinEvaluatorRequest
		setupMock     func(mockRepo *repomocks.MockIEvaluatorRepo)
		expectedList  []*entity.Evaluator
		expectedTotal int64
		expectedErr   error
	}{
		{
			name: "成功 - 不带版本信息 (WithVersion = false)",
			request: &entity.ListBuiltinEvaluatorRequest{
				PageSize:    10,
				PageNum:     1,
				WithVersion: false,
			},
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo) {
				expectedRepoReq := &repo.ListBuiltinEvaluatorRequest{
					PageSize:       10,
					PageNum:        1,
					IncludeDeleted: false,
					FilterOption:   nil, // 没有筛选条件
				}
				mockRepo.EXPECT().ListBuiltinEvaluator(gomock.Any(), gomock.Eq(expectedRepoReq)).Return(
					&repo.ListBuiltinEvaluatorResponse{
						Evaluators: []*entity.Evaluator{
							{ID: 1, Name: "BuiltinEval1", SpaceID: 1, Description: "Builtin Desc1"},
							{ID: 2, Name: "BuiltinEval2", SpaceID: 1, Description: "Builtin Desc2"},
						},
						TotalCount: 2,
					}, nil)
			},
			expectedList: []*entity.Evaluator{
				{ID: 1, Name: "BuiltinEval1", SpaceID: 1, Description: "Builtin Desc1"},
				{ID: 2, Name: "BuiltinEval2", SpaceID: 1, Description: "Builtin Desc2"},
			},
			expectedTotal: 2,
			expectedErr:   nil,
		},
		{
			name: "成功 - 带BuiltinVisibleVersion并回填版本信息",
			request: &entity.ListBuiltinEvaluatorRequest{
				PageSize: 10,
				PageNum:  1,
			},
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo) {
				expectedRepoReq := &repo.ListBuiltinEvaluatorRequest{
					PageSize:       10,
					PageNum:        1,
					IncludeDeleted: false,
					FilterOption:   nil,
				}
				// 模拟返回有BuiltinVisibleVersion的评估器元数据
				mockRepo.EXPECT().ListBuiltinEvaluator(gomock.Any(), gomock.Eq(expectedRepoReq)).Return(
					&repo.ListBuiltinEvaluatorResponse{
						Evaluators: []*entity.Evaluator{
							{
								ID:                    1,
								Name:                  "BuiltinEval1",
								SpaceID:               1,
								Description:           "Builtin Desc1",
								BuiltinVisibleVersion: "1.0.0",
								EvaluatorType:         entity.EvaluatorTypePrompt,
							},
							{
								ID:                    2,
								Name:                  "BuiltinEval2",
								SpaceID:               1,
								Description:           "Builtin Desc2",
								BuiltinVisibleVersion: "2.0.0",
								EvaluatorType:         entity.EvaluatorTypeCode,
							},
						},
						TotalCount: 2,
					}, nil)

				// 模拟批量查询版本信息
				mockRepo.EXPECT().BatchGetEvaluatorVersionsByEvaluatorIDAndVersions(gomock.Any(), [][2]interface{}{
					{int64(1), "1.0.0"},
					{int64(2), "2.0.0"},
				}).Return([]*entity.Evaluator{
					{
						ID:            1,
						EvaluatorType: entity.EvaluatorTypePrompt,
						PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
							EvaluatorID: 1,
							Version:     "1.0.0",
							MessageList: []*entity.Message{},
						},
					},
					{
						ID:            2,
						EvaluatorType: entity.EvaluatorTypeCode,
						CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
							EvaluatorID: 2,
							Version:     "2.0.0",
							CodeContent: "def evaluate(): pass",
						},
					},
				}, nil)
			},
			expectedList: []*entity.Evaluator{
				{
					ID:                    1,
					Name:                  "BuiltinEval1",
					SpaceID:               1,
					Description:           "Builtin Desc1",
					BuiltinVisibleVersion: "1.0.0",
					EvaluatorType:         entity.EvaluatorTypePrompt,
				},
				{
					ID:                    2,
					Name:                  "BuiltinEval2",
					SpaceID:               1,
					Description:           "Builtin Desc2",
					BuiltinVisibleVersion: "2.0.0",
					EvaluatorType:         entity.EvaluatorTypeCode,
				},
			},
			expectedTotal: 2,
			expectedErr:   nil,
		},
		{
			name: "成功 - 部分评估器有BuiltinVisibleVersion",
			request: &entity.ListBuiltinEvaluatorRequest{
				PageSize: 10,
				PageNum:  1,
			},
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo) {
				expectedRepoReq := &repo.ListBuiltinEvaluatorRequest{
					PageSize:       10,
					PageNum:        1,
					IncludeDeleted: false,
					FilterOption:   nil,
				}
				mockRepo.EXPECT().ListBuiltinEvaluator(gomock.Any(), gomock.Eq(expectedRepoReq)).Return(
					&repo.ListBuiltinEvaluatorResponse{
						Evaluators: []*entity.Evaluator{
							{
								ID:                    1,
								Name:                  "BuiltinEval1",
								SpaceID:               1,
								BuiltinVisibleVersion: "1.0.0",
								EvaluatorType:         entity.EvaluatorTypePrompt,
							},
							{
								ID:                    2,
								Name:                  "BuiltinEval2",
								SpaceID:               1,
								BuiltinVisibleVersion: "", // 没有版本
								EvaluatorType:         entity.EvaluatorTypePrompt,
							},
						},
						TotalCount: 2,
					}, nil)

				// 只查询有BuiltinVisibleVersion的评估器版本
				mockRepo.EXPECT().BatchGetEvaluatorVersionsByEvaluatorIDAndVersions(gomock.Any(), [][2]interface{}{
					{int64(1), "1.0.0"},
				}).Return([]*entity.Evaluator{
					{
						ID:            1,
						EvaluatorType: entity.EvaluatorTypePrompt,
						PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
							EvaluatorID: 1,
							Version:     "1.0.0",
						},
					},
				}, nil)
			},
			expectedList: []*entity.Evaluator{
				{
					ID:                    1,
					Name:                  "BuiltinEval1",
					SpaceID:               1,
					BuiltinVisibleVersion: "1.0.0",
					EvaluatorType:         entity.EvaluatorTypePrompt,
				},
				{
					ID:            2,
					Name:          "BuiltinEval2",
					SpaceID:       1,
					EvaluatorType: entity.EvaluatorTypePrompt,
				},
			},
			expectedTotal: 2,
			expectedErr:   nil,
		},
		{
			name: "成功 - 空列表",
			request: &entity.ListBuiltinEvaluatorRequest{
				PageSize: 10,
				PageNum:  1,
			},
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo) {
				expectedRepoReq := &repo.ListBuiltinEvaluatorRequest{
					PageSize:       10,
					PageNum:        1,
					IncludeDeleted: false,
					FilterOption:   nil,
				}
				mockRepo.EXPECT().ListBuiltinEvaluator(gomock.Any(), gomock.Eq(expectedRepoReq)).Return(
					&repo.ListBuiltinEvaluatorResponse{
						Evaluators: []*entity.Evaluator{},
						TotalCount: 0,
					}, nil)
				// 空列表时不会调用BatchGetEvaluatorVersionsByEvaluatorIDAndVersions
			},
			expectedList:  []*entity.Evaluator{},
			expectedTotal: 0,
			expectedErr:   nil,
		},
		{
			name: "成功 - 带筛选条件",
			request: &entity.ListBuiltinEvaluatorRequest{
				PageSize:     10,
				PageNum:      1,
				FilterOption: &entity.EvaluatorFilterOption{},
			},
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo) {
				expectedRepoReq := &repo.ListBuiltinEvaluatorRequest{
					PageSize:       10,
					PageNum:        1,
					IncludeDeleted: false,
					FilterOption:   &entity.EvaluatorFilterOption{},
				}
				mockRepo.EXPECT().ListBuiltinEvaluator(gomock.Any(), gomock.Eq(expectedRepoReq)).Return(
					&repo.ListBuiltinEvaluatorResponse{
						Evaluators: []*entity.Evaluator{
							{ID: 1, Name: "BuiltinEval1", SpaceID: 1, Description: "Builtin Desc1"},
						},
						TotalCount: 1,
					}, nil)
			},
			expectedList: []*entity.Evaluator{
				{ID: 1, Name: "BuiltinEval1", SpaceID: 1, Description: "Builtin Desc1"},
			},
			expectedTotal: 1,
			expectedErr:   nil,
		},
		{
			name: "失败 - BatchGetEvaluatorVersionsByEvaluatorIDAndVersions返回错误",
			request: &entity.ListBuiltinEvaluatorRequest{
				PageSize: 10,
				PageNum:  1,
			},
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo) {
				expectedRepoReq := &repo.ListBuiltinEvaluatorRequest{
					PageSize:       10,
					PageNum:        1,
					IncludeDeleted: false,
					FilterOption:   nil,
				}
				mockRepo.EXPECT().ListBuiltinEvaluator(gomock.Any(), gomock.Eq(expectedRepoReq)).Return(
					&repo.ListBuiltinEvaluatorResponse{
						Evaluators: []*entity.Evaluator{
							{
								ID:                    1,
								Name:                  "BuiltinEval1",
								SpaceID:               1,
								BuiltinVisibleVersion: "1.0.0",
								EvaluatorType:         entity.EvaluatorTypePrompt,
							},
						},
						TotalCount: 1,
					}, nil)

				mockRepo.EXPECT().BatchGetEvaluatorVersionsByEvaluatorIDAndVersions(gomock.Any(), [][2]interface{}{
					{int64(1), "1.0.0"},
				}).Return(nil, errors.New("version query error"))
			},
			expectedList:  nil,
			expectedTotal: 0,
			expectedErr:   errors.New("version query error"),
		},
		{
			name: "失败 - repo返回错误",
			request: &entity.ListBuiltinEvaluatorRequest{
				PageSize:    10,
				PageNum:     1,
				WithVersion: false,
			},
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo) {
				expectedRepoReq := &repo.ListBuiltinEvaluatorRequest{
					PageSize:       10,
					PageNum:        1,
					IncludeDeleted: false,
					FilterOption:   nil,
				}
				mockRepo.EXPECT().ListBuiltinEvaluator(gomock.Any(), gomock.Eq(expectedRepoReq)).Return(
					nil, errors.New("repo error"))
			},
			expectedList:  nil,
			expectedTotal: 0,
			expectedErr:   errors.New("repo error"),
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock(mockEvaluatorRepo)

			result, total, err := s.ListBuiltinEvaluator(ctx, tt.request)

			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedErr.Error(), err.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedTotal, total)
				assert.Equal(t, len(tt.expectedList), len(result))
				for i, expected := range tt.expectedList {
					assert.Equal(t, expected.ID, result[i].ID)
					assert.Equal(t, expected.Name, result[i].Name)
					assert.Equal(t, expected.SpaceID, result[i].SpaceID)
					assert.Equal(t, expected.Description, result[i].Description)
				}
			}
		})
	}
}

func TestEvaluatorServiceImpl_ListEvaluatorTags(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repomocks.NewMockIEvaluatorRepo(ctrl)
	s := &EvaluatorServiceImpl{evaluatorRepo: mockRepo}
	ctx := context.Background()

	tests := []struct {
		name           string
		tagType        entity.EvaluatorTagKeyType
		mockSetup      func()
		expectedResult map[entity.EvaluatorTagKey][]string
		expectedError  error
		description    string
	}{
		{
			name:    "成功 - 评估器标签类型",
			tagType: entity.EvaluatorTagKeyType_Evaluator,
			mockSetup: func() {
				mockRepo.EXPECT().
					ListEvaluatorTags(gomock.Any(), entity.EvaluatorTagKeyType_Evaluator).
					Return(map[entity.EvaluatorTagKey][]string{
						entity.EvaluatorTagKey_Category:   {"Code", "LLM"},
						entity.EvaluatorTagKey_TargetType: {"Image", "Text"},
					}, nil)
			},
			expectedResult: map[entity.EvaluatorTagKey][]string{
				entity.EvaluatorTagKey_Category:   {"Code", "LLM"},
				entity.EvaluatorTagKey_TargetType: {"Image", "Text"},
			},
			expectedError: nil,
			description:   "评估器标签类型时，应该正确返回并排序标签",
		},
		{
			name:    "成功 - 默认标签类型",
			tagType: 0,
			mockSetup: func() {
				mockRepo.EXPECT().
					ListEvaluatorTags(gomock.Any(), entity.EvaluatorTagKeyType_Evaluator).
					Return(map[entity.EvaluatorTagKey][]string{
						entity.EvaluatorTagKey_Category: {"LLM"},
					}, nil)
			},
			expectedResult: map[entity.EvaluatorTagKey][]string{
				entity.EvaluatorTagKey_Category: {"LLM"},
			},
			expectedError: nil,
			description:   "tagType为0时，应该默认使用评估器标签类型",
		},
		{
			name:    "成功 - 模板标签类型",
			tagType: entity.EvaluatorTagKeyType_Template,
			mockSetup: func() {
				mockRepo.EXPECT().
					ListEvaluatorTags(gomock.Any(), entity.EvaluatorTagKeyType_Template).
					Return(map[entity.EvaluatorTagKey][]string{
						entity.EvaluatorTagKey_Category: {"Code", "Prompt"},
					}, nil)
			},
			expectedResult: map[entity.EvaluatorTagKey][]string{
				entity.EvaluatorTagKey_Category: {"Code", "Prompt"},
			},
			expectedError: nil,
			description:   "模板标签类型时，应该正确返回并排序标签",
		},
		{
			name:    "成功 - 空结果",
			tagType: entity.EvaluatorTagKeyType_Evaluator,
			mockSetup: func() {
				mockRepo.EXPECT().
					ListEvaluatorTags(gomock.Any(), entity.EvaluatorTagKeyType_Evaluator).
					Return(map[entity.EvaluatorTagKey][]string{}, nil)
			},
			expectedResult: map[entity.EvaluatorTagKey][]string{},
			expectedError:  nil,
			description:    "无结果时，应该返回空map",
		},
		{
			name:    "成功 - 标签值按字母顺序排序",
			tagType: entity.EvaluatorTagKeyType_Evaluator,
			mockSetup: func() {
				mockRepo.EXPECT().
					ListEvaluatorTags(gomock.Any(), entity.EvaluatorTagKeyType_Evaluator).
					Return(map[entity.EvaluatorTagKey][]string{
						entity.EvaluatorTagKey_Category: {"z", "a", "m"},
					}, nil)
			},
			expectedResult: map[entity.EvaluatorTagKey][]string{
				entity.EvaluatorTagKey_Category: {"a", "m", "z"},
			},
			expectedError: nil,
			description:   "标签值应该按字母顺序排序",
		},
		{
			name:    "失败 - Repo错误",
			tagType: entity.EvaluatorTagKeyType_Evaluator,
			mockSetup: func() {
				mockRepo.EXPECT().
					ListEvaluatorTags(gomock.Any(), entity.EvaluatorTagKeyType_Evaluator).
					Return(nil, errors.New("repo error"))
			},
			expectedResult: nil,
			expectedError:  errors.New("repo error"),
			description:    "Repo错误时，应该返回错误",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			result, err := s.ListEvaluatorTags(ctx, tt.tagType)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, len(tt.expectedResult), len(result))
				for key, expectedValues := range tt.expectedResult {
					actualValues, ok := result[key]
					assert.True(t, ok, "key %s should exist", key)
					assert.Equal(t, expectedValues, actualValues)
				}
			}
		})
	}
}

func TestEvaluatorServiceImpl_AsyncRunEvaluator(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	mockEvaluatorRepo := repomocks.NewMockIEvaluatorRepo(ctrl)
	mockLimiter := repomocks.NewMockRateLimiter(ctrl)
	mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockEvaluatorRecordRepo := repomocks.NewMockIEvaluatorRecordRepo(ctrl)
	mockEvaluatorSourceService := mocks.NewMockEvaluatorSourceService(ctrl)
	mockPlainLimiter := repomocks.NewMockIPlainRateLimiter(ctrl)

	s := &EvaluatorServiceImpl{
		evaluatorRepo:       mockEvaluatorRepo,
		limiter:             mockLimiter,
		idgen:               mockIDGen,
		evaluatorRecordRepo: mockEvaluatorRecordRepo,
		evaluatorSourceServices: map[entity.EvaluatorType]EvaluatorSourceService{
			entity.EvaluatorTypeAgent: mockEvaluatorSourceService,
		},
		plainRateLimiter: mockPlainLimiter,
	}

	req := &entity.AsyncRunEvaluatorRequest{
		SpaceID:            2,
		EvaluatorVersionID: 101,
		InputData:          &entity.EvaluatorInputData{},
		ExperimentID:       1,
		ExperimentRunID:    2,
		ItemID:             3,
		TurnID:             4,
		Ext:                map[string]string{"k": "v"},
	}

	agentEvaluatorDO := &entity.Evaluator{
		ID:            100,
		SpaceID:       2,
		EvaluatorType: entity.EvaluatorTypeAgent,
		AgentEvaluatorVersion: &entity.AgentEvaluatorVersion{
			ID:          101,
			EvaluatorID: 100,
			SpaceID:     2,
		},
	}

	nonAgentEvaluatorDO := &entity.Evaluator{
		ID:            100,
		SpaceID:       2,
		EvaluatorType: entity.EvaluatorTypePrompt,
		PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
			ID:          101,
			EvaluatorID: 100,
			SpaceID:     2,
		},
	}

	tests := []struct {
		name            string
		setupMocks      func()
		expectedErrCode int32
	}{
		{
			name: "成功 - 异步运行 Agent 评估器",
			setupMocks: func() {
				mockEvaluatorRepo.EXPECT().BatchGetEvaluatorByVersionID(gomock.Any(), nil, []int64{req.EvaluatorVersionID}, false, false).Return([]*entity.Evaluator{agentEvaluatorDO}, nil)
				mockLimiter.EXPECT().AllowInvoke(gomock.Any(), req.SpaceID).Return(true)
				mockPlainLimiter.EXPECT().AllowInvokeWithKeyLimit(gomock.Any(), "async_run_evaluator:100", gomock.Any()).Return(true)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(999), nil)
				mockEvaluatorSourceService.EXPECT().AsyncRun(gomock.Any(), agentEvaluatorDO, req.InputData, req.EvaluatorRunConf, req.SpaceID, int64(999)).
					Return(map[string]string{"async": "1"}, "trace-1", nil)
				mockEvaluatorRecordRepo.EXPECT().CreateEvaluatorRecord(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, record *entity.EvaluatorRecord) error {
						assert.Equal(t, int64(999), record.ID)
						assert.Equal(t, req.SpaceID, record.SpaceID)
						assert.Equal(t, req.EvaluatorVersionID, record.EvaluatorVersionID)
						assert.Equal(t, entity.EvaluatorRunStatusAsyncInvoking, record.Status)
						assert.Equal(t, req.Ext, record.Ext)
						if assert.NotNil(t, record.EvaluatorOutputData) {
							assert.Equal(t, map[string]string{"async": "1"}, record.EvaluatorOutputData.Ext)
						}
						return nil
					},
				)
			},
			expectedErrCode: 0,
		},
		{
			name: "失败 - evaluator_version 不存在",
			setupMocks: func() {
				mockEvaluatorRepo.EXPECT().BatchGetEvaluatorByVersionID(gomock.Any(), nil, []int64{req.EvaluatorVersionID}, false, false).Return([]*entity.Evaluator{}, nil)
			},
			expectedErrCode: int32(errno.EvaluatorVersionNotFoundCode),
		},
		{
			name: "失败 - 非 Agent 类型不支持异步运行",
			setupMocks: func() {
				mockEvaluatorRepo.EXPECT().BatchGetEvaluatorByVersionID(gomock.Any(), nil, []int64{req.EvaluatorVersionID}, false, false).Return([]*entity.Evaluator{nonAgentEvaluatorDO}, nil)
			},
			expectedErrCode: int32(errno.InvalidEvaluatorTypeCode),
		},
		{
			name: "失败 - SpaceID 不匹配",
			setupMocks: func() {
				mismatch := *agentEvaluatorDO
				mismatch.SpaceID = 3
				mockEvaluatorRepo.EXPECT().BatchGetEvaluatorByVersionID(gomock.Any(), nil, []int64{req.EvaluatorVersionID}, false, false).Return([]*entity.Evaluator{&mismatch}, nil)
			},
			expectedErrCode: int32(errno.EvaluatorVersionNotFoundCode),
		},
		{
			name: "失败 - 触发 space 级限流",
			setupMocks: func() {
				mockEvaluatorRepo.EXPECT().BatchGetEvaluatorByVersionID(gomock.Any(), nil, []int64{req.EvaluatorVersionID}, false, false).Return([]*entity.Evaluator{agentEvaluatorDO}, nil)
				mockLimiter.EXPECT().AllowInvoke(gomock.Any(), req.SpaceID).Return(false)
			},
			expectedErrCode: int32(errno.EvaluatorQPSLimitCode),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMocks()
			_, err := s.AsyncRunEvaluator(ctx, req)

			if tt.expectedErrCode == 0 {
				assert.NoError(t, err)
				return
			}
			assert.Error(t, err)
			statusErr, ok := errorx.FromStatusError(err)
			assert.True(t, ok)
			assert.Equal(t, tt.expectedErrCode, statusErr.Code())
		})
	}
}

func TestEvaluatorServiceImpl_AsyncDebugEvaluator(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	mockLimiter := repomocks.NewMockRateLimiter(ctrl)
	mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockEvaluatorRecordRepo := repomocks.NewMockIEvaluatorRecordRepo(ctrl)
	mockEvaluatorSourceService := mocks.NewMockEvaluatorSourceService(ctrl)

	s := &EvaluatorServiceImpl{
		limiter:             mockLimiter,
		idgen:               mockIDGen,
		evaluatorRecordRepo: mockEvaluatorRecordRepo,
		evaluatorSourceServices: map[entity.EvaluatorType]EvaluatorSourceService{
			entity.EvaluatorTypeAgent: mockEvaluatorSourceService,
		},
	}

	agentEvaluatorDO := &entity.Evaluator{
		ID:            100,
		SpaceID:       2,
		EvaluatorType: entity.EvaluatorTypeAgent,
		AgentEvaluatorVersion: &entity.AgentEvaluatorVersion{
			ID:          101,
			EvaluatorID: 100,
			SpaceID:     2,
		},
	}

	tests := []struct {
		name            string
		req             *entity.AsyncDebugEvaluatorRequest
		setupMocks      func()
		expectedErrCode int32
	}{
		{
			name: "成功 - 异步调试 Agent 评估器",
			req: &entity.AsyncDebugEvaluatorRequest{
				SpaceID:     2,
				EvaluatorDO: agentEvaluatorDO,
				InputData:   &entity.EvaluatorInputData{},
			},
			setupMocks: func() {
				mockLimiter.EXPECT().AllowInvoke(gomock.Any(), int64(2)).Return(true)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(888), nil)
				mockEvaluatorSourceService.EXPECT().AsyncDebug(gomock.Any(), agentEvaluatorDO, gomock.Any(), gomock.Any(), int64(2), int64(888)).
					Return(map[string]string{"d": "1"}, "trace-d", nil)
				mockEvaluatorRecordRepo.EXPECT().CreateEvaluatorRecord(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, record *entity.EvaluatorRecord) error {
						assert.Equal(t, int64(888), record.ID)
						assert.Equal(t, int64(2), record.SpaceID)
						assert.Equal(t, entity.EvaluatorRunStatusAsyncInvoking, record.Status)
						if assert.NotNil(t, record.EvaluatorOutputData) {
							assert.Equal(t, map[string]string{"d": "1"}, record.EvaluatorOutputData.Ext)
						}
						return nil
					},
				)
			},
			expectedErrCode: 0,
		},
		{
			name: "失败 - evaluator 为空",
			req: &entity.AsyncDebugEvaluatorRequest{
				SpaceID:     2,
				EvaluatorDO: nil,
				InputData:   &entity.EvaluatorInputData{},
			},
			setupMocks:      func() {},
			expectedErrCode: int32(errno.EvaluatorNotExistCode),
		},
		{
			name: "失败 - 非 Agent 类型不支持异步调试",
			req: &entity.AsyncDebugEvaluatorRequest{
				SpaceID: 2,
				EvaluatorDO: &entity.Evaluator{
					ID:            100,
					SpaceID:       2,
					EvaluatorType: entity.EvaluatorTypePrompt,
					PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
						ID:          101,
						EvaluatorID: 100,
						SpaceID:     2,
					},
				},
				InputData: &entity.EvaluatorInputData{},
			},
			setupMocks:      func() {},
			expectedErrCode: int32(errno.InvalidEvaluatorTypeCode),
		},
		{
			name: "失败 - 触发 space 级限流",
			req: &entity.AsyncDebugEvaluatorRequest{
				SpaceID:     2,
				EvaluatorDO: agentEvaluatorDO,
				InputData:   &entity.EvaluatorInputData{},
			},
			setupMocks: func() {
				mockLimiter.EXPECT().AllowInvoke(gomock.Any(), int64(2)).Return(false)
			},
			expectedErrCode: int32(errno.EvaluatorQPSLimitCode),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMocks()
			_, err := s.AsyncDebugEvaluator(ctx, tt.req)

			if tt.expectedErrCode == 0 {
				assert.NoError(t, err)
				return
			}
			assert.Error(t, err)
			statusErr, ok := errorx.FromStatusError(err)
			assert.True(t, ok)
			assert.Equal(t, tt.expectedErrCode, statusErr.Code())
		})
	}
}

func TestEvaluatorServiceImpl_ReportEvaluatorInvokeResult(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	mockEvaluatorRecordRepo := repomocks.NewMockIEvaluatorRecordRepo(ctrl)
	s := &EvaluatorServiceImpl{
		evaluatorRecordRepo: mockEvaluatorRecordRepo,
	}

	tests := []struct {
		name            string
		param           *entity.ReportEvaluatorRecordParam
		setupMocks      func()
		expectedErrCode int32
	}{
		{
			name: "成功 - 合并 Ext 并更新记录",
			param: &entity.ReportEvaluatorRecordParam{
				SpaceID:  2,
				RecordID: 100,
				Status:   entity.EvaluatorRunStatusSuccess,
				OutputData: &entity.EvaluatorOutputData{
					Ext: map[string]string{"new": "1"},
				},
			},
			setupMocks: func() {
				mockEvaluatorRecordRepo.EXPECT().GetEvaluatorRecord(gomock.Any(), int64(100), false).Return(
					&entity.EvaluatorRecord{
						ID:      100,
						SpaceID: 2,
						EvaluatorOutputData: &entity.EvaluatorOutputData{
							Ext: map[string]string{"old": "1"},
						},
					}, nil,
				)
				mockEvaluatorRecordRepo.EXPECT().UpdateEvaluatorRecordResult(gomock.Any(), int64(100), entity.EvaluatorRunStatusSuccess, gomock.Any()).
					DoAndReturn(func(_ context.Context, _ int64, _ entity.EvaluatorRunStatus, out *entity.EvaluatorOutputData) error {
						if assert.NotNil(t, out) {
							assert.Equal(t, "1", out.Ext["new"])
							assert.Equal(t, "1", out.Ext["old"])
						}
						return nil
					})
			},
			expectedErrCode: 0,
		},
		{
			name: "失败 - record 不存在",
			param: &entity.ReportEvaluatorRecordParam{
				SpaceID:  2,
				RecordID: 100,
				Status:   entity.EvaluatorRunStatusSuccess,
			},
			setupMocks: func() {
				mockEvaluatorRecordRepo.EXPECT().GetEvaluatorRecord(gomock.Any(), int64(100), false).Return(nil, nil)
			},
			expectedErrCode: int32(errno.EvaluatorRecordNotFoundCode),
		},
		{
			name: "失败 - SpaceID 不匹配",
			param: &entity.ReportEvaluatorRecordParam{
				SpaceID:  3,
				RecordID: 100,
				Status:   entity.EvaluatorRunStatusSuccess,
			},
			setupMocks: func() {
				mockEvaluatorRecordRepo.EXPECT().GetEvaluatorRecord(gomock.Any(), int64(100), false).Return(&entity.EvaluatorRecord{ID: 100, SpaceID: 2}, nil)
			},
			expectedErrCode: int32(errno.CommonInvalidParamCode),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMocks()
			err := s.ReportEvaluatorInvokeResult(ctx, tt.param)

			if tt.expectedErrCode == 0 {
				assert.NoError(t, err)
				return
			}
			assert.Error(t, err)
			statusErr, ok := errorx.FromStatusError(err)
			assert.True(t, ok)
			assert.Equal(t, tt.expectedErrCode, statusErr.Code())
		})
	}
}

func TestEvaluatorServiceImpl_ReportEvaluatorInvokeResult_OutputDataNilOrExtNil(t *testing.T) {
	tests := []struct {
		name       string
		outputData *entity.EvaluatorOutputData
	}{
		{
			name:       "param.OutputData is nil, keep existing ext",
			outputData: nil,
		},
		{
			name: "param.OutputData.Ext is nil, keep existing ext",
			outputData: &entity.EvaluatorOutputData{
				Ext: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.Background()
			mockEvaluatorRecordRepo := repomocks.NewMockIEvaluatorRecordRepo(ctrl)
			s := &EvaluatorServiceImpl{
				evaluatorRecordRepo: mockEvaluatorRecordRepo,
			}

			param := &entity.ReportEvaluatorRecordParam{
				SpaceID:    2,
				RecordID:   100,
				Status:     entity.EvaluatorRunStatusSuccess,
				OutputData: tt.outputData,
			}

			mockEvaluatorRecordRepo.EXPECT().GetEvaluatorRecord(gomock.Any(), int64(100), false).Return(
				&entity.EvaluatorRecord{
					ID:      100,
					SpaceID: 2,
					EvaluatorOutputData: &entity.EvaluatorOutputData{
						Ext: map[string]string{"old": "1"},
					},
				}, nil,
			)
			mockEvaluatorRecordRepo.EXPECT().UpdateEvaluatorRecordResult(gomock.Any(), int64(100), entity.EvaluatorRunStatusSuccess, gomock.Any()).
				DoAndReturn(func(_ context.Context, _ int64, _ entity.EvaluatorRunStatus, out *entity.EvaluatorOutputData) error {
					if assert.NotNil(t, out) && assert.NotNil(t, out.Ext) {
						assert.Equal(t, "1", out.Ext["old"])
					}
					return nil
				})

			err := s.ReportEvaluatorInvokeResult(ctx, param)
			assert.NoError(t, err)
		})
	}
}

func TestEvaluatorServiceImpl_AsyncRunEvaluator_EvaluatorLevelLimitAndSourceMissing(t *testing.T) {
	tests := []struct {
		name       string
		setupMocks func(ctx context.Context, s *EvaluatorServiceImpl, evaluatorDO *entity.Evaluator, req *entity.AsyncRunEvaluatorRequest, deps *struct {
			repo       *repomocks.MockIEvaluatorRepo
			limiter    *repomocks.MockRateLimiter
			idgen      *idgenmocks.MockIIDGenerator
			plain      *repomocks.MockIPlainRateLimiter
			source     *mocks.MockEvaluatorSourceService
			recordRepo *repomocks.MockIEvaluatorRecordRepo
		})
		expectedErrCode int32
	}{
		{
			name: "evaluator level limit",
			setupMocks: func(_ context.Context, _ *EvaluatorServiceImpl, evaluatorDO *entity.Evaluator, req *entity.AsyncRunEvaluatorRequest, deps *struct {
				repo       *repomocks.MockIEvaluatorRepo
				limiter    *repomocks.MockRateLimiter
				idgen      *idgenmocks.MockIIDGenerator
				plain      *repomocks.MockIPlainRateLimiter
				source     *mocks.MockEvaluatorSourceService
				recordRepo *repomocks.MockIEvaluatorRecordRepo
			},
			) {
				deps.repo.EXPECT().BatchGetEvaluatorByVersionID(gomock.Any(), nil, []int64{req.EvaluatorVersionID}, false, false).
					Return([]*entity.Evaluator{evaluatorDO}, nil)
				deps.limiter.EXPECT().AllowInvoke(gomock.Any(), req.SpaceID).Return(true)
				deps.plain.EXPECT().AllowInvokeWithKeyLimit(gomock.Any(), "async_run_evaluator:100", gomock.Any()).Return(false)
			},
			expectedErrCode: int32(errno.EvaluatorQPSLimitCode),
		},
		{
			name: "source service missing for agent type",
			setupMocks: func(_ context.Context, s *EvaluatorServiceImpl, evaluatorDO *entity.Evaluator, req *entity.AsyncRunEvaluatorRequest, deps *struct {
				repo       *repomocks.MockIEvaluatorRepo
				limiter    *repomocks.MockRateLimiter
				idgen      *idgenmocks.MockIIDGenerator
				plain      *repomocks.MockIPlainRateLimiter
				source     *mocks.MockEvaluatorSourceService
				recordRepo *repomocks.MockIEvaluatorRecordRepo
			},
			) {
				s.evaluatorSourceServices = map[entity.EvaluatorType]EvaluatorSourceService{}
				deps.repo.EXPECT().BatchGetEvaluatorByVersionID(gomock.Any(), nil, []int64{req.EvaluatorVersionID}, false, false).
					Return([]*entity.Evaluator{evaluatorDO}, nil)
				deps.limiter.EXPECT().AllowInvoke(gomock.Any(), req.SpaceID).Return(true)
				deps.plain.EXPECT().AllowInvokeWithKeyLimit(gomock.Any(), "async_run_evaluator:100", gomock.Any()).Return(true)
				deps.idgen.EXPECT().GenID(gomock.Any()).Return(int64(999), nil)
			},
			expectedErrCode: int32(errno.InvalidEvaluatorTypeCode),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.Background()
			mockEvaluatorRepo := repomocks.NewMockIEvaluatorRepo(ctrl)
			mockLimiter := repomocks.NewMockRateLimiter(ctrl)
			mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
			mockEvaluatorRecordRepo := repomocks.NewMockIEvaluatorRecordRepo(ctrl)
			mockEvaluatorSourceService := mocks.NewMockEvaluatorSourceService(ctrl)
			mockPlainLimiter := repomocks.NewMockIPlainRateLimiter(ctrl)

			s := &EvaluatorServiceImpl{
				evaluatorRepo:       mockEvaluatorRepo,
				limiter:             mockLimiter,
				idgen:               mockIDGen,
				evaluatorRecordRepo: mockEvaluatorRecordRepo,
				evaluatorSourceServices: map[entity.EvaluatorType]EvaluatorSourceService{
					entity.EvaluatorTypeAgent: mockEvaluatorSourceService,
				},
				plainRateLimiter: mockPlainLimiter,
			}

			req := &entity.AsyncRunEvaluatorRequest{
				SpaceID:            2,
				EvaluatorVersionID: 101,
				InputData:          &entity.EvaluatorInputData{},
			}
			agentEvaluatorDO := &entity.Evaluator{
				ID:            100,
				SpaceID:       2,
				EvaluatorType: entity.EvaluatorTypeAgent,
				AgentEvaluatorVersion: &entity.AgentEvaluatorVersion{
					ID:          101,
					EvaluatorID: 100,
					SpaceID:     2,
				},
			}

			tt.setupMocks(ctx, s, agentEvaluatorDO, req, &struct {
				repo       *repomocks.MockIEvaluatorRepo
				limiter    *repomocks.MockRateLimiter
				idgen      *idgenmocks.MockIIDGenerator
				plain      *repomocks.MockIPlainRateLimiter
				source     *mocks.MockEvaluatorSourceService
				recordRepo *repomocks.MockIEvaluatorRecordRepo
			}{
				repo:       mockEvaluatorRepo,
				limiter:    mockLimiter,
				idgen:      mockIDGen,
				plain:      mockPlainLimiter,
				source:     mockEvaluatorSourceService,
				recordRepo: mockEvaluatorRecordRepo,
			})

			_, err := s.AsyncRunEvaluator(ctx, req)
			assert.Error(t, err)
			statusErr, ok := errorx.FromStatusError(err)
			assert.True(t, ok)
			assert.Equal(t, tt.expectedErrCode, statusErr.Code())
		})
	}
}

func TestEvaluatorServiceImpl_SubmitEvaluatorVersion_ValidateAndVersionExist(t *testing.T) {
	evaluatorDO := &entity.Evaluator{
		ID:            100,
		SpaceID:       1,
		EvaluatorType: entity.EvaluatorTypePrompt,
		PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
			ID:                100,
			EvaluatorID:       100,
			SpaceID:           1,
			PromptTemplateKey: "test-template-key",
			PromptSuffix:      "test-prompt-suffix",
			ModelConfig: &entity.ModelConfig{
				ModelID: gptr.Of(int64(1)),
			},
			ParseType: entity.ParseTypeFunctionCall,
			MessageList: []*entity.Message{
				{
					Role: entity.RoleSystem,
					Content: &entity.Content{
						ContentType: ptr.Of(entity.ContentTypeText),
						Text:        ptr.Of("test-content"),
					},
				},
			},
			InputSchemas: []*entity.ArgsSchema{
				{
					Key:        ptr.Of("test-input-key"),
					JsonSchema: ptr.Of("test-json-schema"),
					SupportContentTypes: []entity.ContentType{
						entity.ContentTypeText,
					},
				},
			},
		},
	}

	tests := []struct {
		name            string
		setupMocks      func(mockIdem *idemmocks.MockIdempotentService, mockIdgen *idgenmocks.MockIIDGenerator, mockRepo *repomocks.MockIEvaluatorRepo, mockSource *mocks.MockEvaluatorSourceService)
		expectedErrCode int32
		expectedErrMsg  string
	}{
		{
			name: "validate fails when source service exists",
			setupMocks: func(mockIdem *idemmocks.MockIdempotentService, mockIdgen *idgenmocks.MockIIDGenerator, _ *repomocks.MockIEvaluatorRepo, mockSource *mocks.MockEvaluatorSourceService) {
				mockIdem.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockIdgen.EXPECT().GenID(gomock.Any()).Return(int64(123), nil)
				mockSource.EXPECT().Validate(gomock.Any(), evaluatorDO).Return(errors.New("validate error"))
			},
			expectedErrCode: -1,
			expectedErrMsg:  "validate error",
		},
		{
			name: "version exists",
			setupMocks: func(mockIdem *idemmocks.MockIdempotentService, mockIdgen *idgenmocks.MockIIDGenerator, mockRepo *repomocks.MockIEvaluatorRepo, mockSource *mocks.MockEvaluatorSourceService) {
				mockIdem.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockIdgen.EXPECT().GenID(gomock.Any()).Return(int64(123), nil)
				mockSource.EXPECT().Validate(gomock.Any(), evaluatorDO).Return(nil)
				mockRepo.EXPECT().CheckVersionExist(gomock.Any(), evaluatorDO.ID, "v1.0.0").Return(true, nil)
			},
			expectedErrCode: int32(errno.EvaluatorVersionExistCode),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.Background()
			mockIdemService := idemmocks.NewMockIdempotentService(ctrl)
			mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
			mockEvalRepo := repomocks.NewMockIEvaluatorRepo(ctrl)
			mockSourceService := mocks.NewMockEvaluatorSourceService(ctrl)

			s := &EvaluatorServiceImpl{
				evaluatorRepo: mockEvalRepo,
				idem:          mockIdemService,
				idgen:         mockIDGen,
				evaluatorSourceServices: map[entity.EvaluatorType]EvaluatorSourceService{
					entity.EvaluatorTypePrompt: mockSourceService,
				},
			}

			tt.setupMocks(mockIdemService, mockIDGen, mockEvalRepo, mockSourceService)

			_, err := s.SubmitEvaluatorVersion(ctx, evaluatorDO, "v1.0.0", "desc", "cid-1")
			assert.Error(t, err)
			if tt.expectedErrCode > 0 {
				statusErr, ok := errorx.FromStatusError(err)
				assert.True(t, ok)
				assert.Equal(t, tt.expectedErrCode, statusErr.Code())
				return
			}
			if tt.expectedErrMsg != "" {
				assert.Equal(t, tt.expectedErrMsg, err.Error())
			}
		})
	}
}

func TestEvaluatorServiceImpl_DebugEvaluator_InvalidAndRound(t *testing.T) {
	tests := []struct {
		name           string
		evaluatorDO    *entity.Evaluator
		setupMocks     func(mockSource *mocks.MockEvaluatorSourceService, evaluatorDO *entity.Evaluator)
		wantErrCode    int32
		wantErrMsg     string
		wantScore      *float64
		wantCorrection *float64
	}{
		{
			name:        "invalid evaluator is nil",
			evaluatorDO: nil,
			wantErrCode: int32(errno.EvaluatorNotExistCode),
		},
		{
			name:        "invalid prompt evaluator version is nil",
			evaluatorDO: &entity.Evaluator{EvaluatorType: entity.EvaluatorTypePrompt},
			wantErrCode: int32(errno.EvaluatorNotExistCode),
		},
		{
			name: "validate fails",
			evaluatorDO: &entity.Evaluator{
				EvaluatorType: entity.EvaluatorTypePrompt,
				PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
					ID:          101,
					EvaluatorID: 100,
				},
			},
			setupMocks: func(mockSource *mocks.MockEvaluatorSourceService, evaluatorDO *entity.Evaluator) {
				mockSource.EXPECT().PreHandle(gomock.Any(), evaluatorDO).Return(nil)
				mockSource.EXPECT().Validate(gomock.Any(), evaluatorDO).Return(errors.New("validate error"))
			},
			wantErrMsg: "validate error",
		},
		{
			name: "round output scores in debug",
			evaluatorDO: &entity.Evaluator{
				EvaluatorType: entity.EvaluatorTypePrompt,
				PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
					ID:          101,
					EvaluatorID: 100,
				},
			},
			setupMocks: func(mockSource *mocks.MockEvaluatorSourceService, evaluatorDO *entity.Evaluator) {
				rawScore := 0.125
				rawCorrectionScore := 0.124
				out := &entity.EvaluatorOutputData{
					EvaluatorResult: &entity.EvaluatorResult{
						Score: &rawScore,
						Correction: &entity.Correction{
							Score: &rawCorrectionScore,
						},
					},
				}
				mockSource.EXPECT().PreHandle(gomock.Any(), evaluatorDO).Return(nil)
				mockSource.EXPECT().Validate(gomock.Any(), evaluatorDO).Return(nil)
				mockSource.EXPECT().Debug(gomock.Any(), evaluatorDO, gomock.Any(), gomock.Any(), int64(0)).Return(out, nil)
			},
			wantScore:      gptr.Of(0.13),
			wantCorrection: gptr.Of(0.12),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.Background()

			var mockSource *mocks.MockEvaluatorSourceService
			sourceMap := map[entity.EvaluatorType]EvaluatorSourceService{}
			if tt.evaluatorDO != nil {
				mockSource = mocks.NewMockEvaluatorSourceService(ctrl)
				sourceMap[entity.EvaluatorTypePrompt] = mockSource
			}
			s := &EvaluatorServiceImpl{
				evaluatorSourceServices: sourceMap,
			}

			if tt.setupMocks != nil {
				tt.setupMocks(mockSource, tt.evaluatorDO)
			}

			got, err := s.DebugEvaluator(ctx, tt.evaluatorDO, &entity.EvaluatorInputData{}, &entity.EvaluatorRunConfig{}, int64(0))
			if tt.wantErrCode > 0 {
				assert.Error(t, err)
				statusErr, ok := errorx.FromStatusError(err)
				assert.True(t, ok)
				assert.Equal(t, tt.wantErrCode, statusErr.Code())
				return
			}
			if tt.wantErrMsg != "" {
				assert.Error(t, err)
				assert.Equal(t, tt.wantErrMsg, err.Error())
				return
			}

			assert.NoError(t, err)
			if tt.wantScore != nil && assert.NotNil(t, got) && assert.NotNil(t, got.EvaluatorResult) && assert.NotNil(t, got.EvaluatorResult.Score) {
				assert.InDelta(t, *tt.wantScore, *got.EvaluatorResult.Score, 1e-9)
			}
			if tt.wantCorrection != nil &&
				assert.NotNil(t, got) &&
				assert.NotNil(t, got.EvaluatorResult) &&
				assert.NotNil(t, got.EvaluatorResult.Correction) &&
				assert.NotNil(t, got.EvaluatorResult.Correction.Score) {
				assert.InDelta(t, *tt.wantCorrection, *got.EvaluatorResult.Correction.Score, 1e-9)
			}
		})
	}
}
