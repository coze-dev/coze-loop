// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package conf

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
	mock_conf "github.com/coze-dev/coze-loop/backend/pkg/conf/mocks"
)

func TestConfiger_GetEvaluationRecordStorage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLoader := mock_conf.NewMockIConfigLoader(ctrl)
	c := &configer{loader: mockLoader}
	ctx := context.Background()
	const key = "evaluation_record_storage"

	tests := []struct {
		name           string
		mockSetup      func()
		expectedRDS    int64
		expectedS3     int64
		expectedCustom bool
	}{
		{
			name: "解析成功返回配置",
			mockSetup: func() {
				mockLoader.EXPECT().UnmarshalKey(ctx, key, gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, _ string, out any, _ ...conf.DecodeOptionFn) error {
						ptr := out.(**component.EvaluationRecordStorage)
						*ptr = &component.EvaluationRecordStorage{
							Providers: []*component.EvaluationRecordProviderConfig{
								{Provider: "RDS", MaxSize: 1024},
								{Provider: "S3", MaxSize: 2048},
							},
						}
						return nil
					},
				)
			},
			expectedRDS:    1024,
			expectedS3:     2048,
			expectedCustom: true,
		},
		{
			name: "UnmarshalKey失败返回默认",
			mockSetup: func() {
				mockLoader.EXPECT().UnmarshalKey(ctx, key, gomock.Any(), gomock.Any()).Return(errors.New("parse fail"))
			},
			expectedRDS:    204800,
			expectedS3:     1 << 30,
			expectedCustom: false,
		},
		{
			name: "cfg为nil返回默认",
			mockSetup: func() {
				mockLoader.EXPECT().UnmarshalKey(ctx, key, gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, _ string, out any, _ ...conf.DecodeOptionFn) error {
						ptr := out.(**component.EvaluationRecordStorage)
						*ptr = nil
						return nil
					},
				)
			},
			expectedRDS:    204800,
			expectedS3:     1 << 30,
			expectedCustom: false,
		},
		{
			name: "Providers为空返回默认",
			mockSetup: func() {
				mockLoader.EXPECT().UnmarshalKey(ctx, key, gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, _ string, out any, _ ...conf.DecodeOptionFn) error {
						ptr := out.(**component.EvaluationRecordStorage)
						*ptr = &component.EvaluationRecordStorage{Providers: nil}
						return nil
					},
				)
			},
			expectedRDS:    204800,
			expectedS3:     1 << 30,
			expectedCustom: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			result := c.GetEvaluationRecordStorage(ctx)
			assert.NotNil(t, result)
			assert.NotEmpty(t, result.Providers)
			if tt.expectedCustom {
				assert.Len(t, result.Providers, 2)
				assert.Equal(t, "RDS", result.Providers[0].Provider)
				assert.Equal(t, tt.expectedRDS, result.Providers[0].MaxSize)
				assert.Equal(t, "S3", result.Providers[1].Provider)
				assert.Equal(t, tt.expectedS3, result.Providers[1].MaxSize)
			} else {
				assert.Len(t, result.Providers, 2)
				assert.Equal(t, "RDS", result.Providers[0].Provider)
				assert.Equal(t, int64(204800), result.Providers[0].MaxSize)
				assert.Equal(t, "S3", result.Providers[1].Provider)
				assert.Equal(t, int64(1<<30), result.Providers[1].MaxSize)
			}
		})
	}
}

func TestConfiger_GetExptTemplateUpdateEvalSetWhiteList(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLoader := mock_conf.NewMockIConfigLoader(ctrl)
	c := &configer{loader: mockLoader}
	ctx := context.Background()
	const key = "expt_template_update_eval_set_white_list"

	t.Run("解析成功返回配置", func(t *testing.T) {
		mockLoader.EXPECT().UnmarshalKey(ctx, key, gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, _ string, out any, _ ...conf.DecodeOptionFn) error {
				ptr := out.(**entity.ExptTemplateUpdateEvalSetWhiteList)
				*ptr = &entity.ExptTemplateUpdateEvalSetWhiteList{
					SpaceIDs: []int64{7533126599059701761, 7485358401870888962},
				}
				return nil
			},
		)
		result := c.GetExptTemplateUpdateEvalSetWhiteList(ctx)
		assert.NotNil(t, result)
		assert.False(t, result.AllowAll)
		assert.Equal(t, []int64{7533126599059701761, 7485358401870888962}, result.SpaceIDs)
		assert.True(t, result.IsSpaceAllowed(7533126599059701761))
		assert.False(t, result.IsSpaceAllowed(1))
	})

	t.Run("UnmarshalKey失败返回默认", func(t *testing.T) {
		mockLoader.EXPECT().UnmarshalKey(ctx, key, gomock.Any(), gomock.Any()).Return(errors.New("parse fail"))
		result := c.GetExptTemplateUpdateEvalSetWhiteList(ctx)
		assert.NotNil(t, result)
		assert.False(t, result.AllowAll)
		assert.Empty(t, result.SpaceIDs)
	})

	t.Run("解析成功且 allow_all=true", func(t *testing.T) {
		mockLoader.EXPECT().UnmarshalKey(ctx, key, gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, _ string, out any, _ ...conf.DecodeOptionFn) error {
				ptr := out.(**entity.ExptTemplateUpdateEvalSetWhiteList)
				*ptr = &entity.ExptTemplateUpdateEvalSetWhiteList{AllowAll: true}
				return nil
			},
		)
		result := c.GetExptTemplateUpdateEvalSetWhiteList(ctx)
		assert.NotNil(t, result)
		assert.True(t, result.AllowAll)
		assert.True(t, result.IsSpaceAllowed(999))
	})
}

func TestConfiger_BuildEvalExt(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLoader := mock_conf.NewMockIConfigLoader(ctrl)
	c := &configer{loader: mockLoader}
	ctx := context.Background()

	tests := []struct {
		name    string
		spaceID int64
		turn    *entity.Turn
	}{
		{
			name:    "nil turn returns nil",
			spaceID: 100,
			turn:    nil,
		},
		{
			name:    "non-nil turn returns nil",
			spaceID: 200,
			turn:    &entity.Turn{ID: 1},
		},
		{
			name:    "zero space id returns nil",
			spaceID: 0,
			turn:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.BuildEvalExt(ctx, tt.spaceID, tt.turn)
			assert.Nil(t, got)
		})
	}
}

// GetEvalAsyncCtxTTL 打通「TCC 配置 → 空间级 conf → TTL」这条读取链。
// 这是需求侧最关心的一环：在 TCC 里给某空间配长 async_zombie_second 后，
// EvalAsyncCtx 的 Redis TTL 要真的跟着变长（而非仍取 12h 硬编码）。
func TestConfiger_GetEvalAsyncCtxTTL(t *testing.T) {
	const key = "expt_consumer_conf"
	const spaceID int64 = 7590110994886651906

	// 用真实 TCC 形状构造 conf：space_expt_exec_conf.<space_id>.expt_item_eval_conf
	mkConf := func(itemEvalConf *entity.ExptItemEvalConf) func(_ context.Context, _ string, out any, _ ...conf.DecodeOptionFn) error {
		return func(_ context.Context, _ string, out any, _ ...conf.DecodeOptionFn) error {
			ptr := out.(**entity.ExptConsumerConf)
			*ptr = &entity.ExptConsumerConf{
				SpaceExptExecConf: map[int64]*entity.ExptExecConf{
					spaceID: {ExptItemEvalConf: itemEvalConf},
				},
			}
			return nil
		}
	}

	tests := []struct {
		name      string
		spaceID   int64
		mockSetup func(*mock_conf.MockIConfigLoader, context.Context)
		expected  time.Duration
	}{
		{
			name:    "空间配了 async_zombie_second=24h，TTL 跟随抬到 24h30min",
			spaceID: spaceID,
			mockSetup: func(l *mock_conf.MockIConfigLoader, ctx context.Context) {
				l.EXPECT().UnmarshalKey(ctx, key, gomock.Any(), gomock.Any()).DoAndReturn(
					mkConf(&entity.ExptItemEvalConf{AsyncZombieSecond: 86400}))
			},
			expected: 86400*time.Second + 30*time.Minute,
		},
		{
			name:    "空间配了 async_zombie_second=5d，TTL 跟随抬到 5d30min",
			spaceID: spaceID,
			mockSetup: func(l *mock_conf.MockIConfigLoader, ctx context.Context) {
				l.EXPECT().UnmarshalKey(ctx, key, gomock.Any(), gomock.Any()).DoAndReturn(
					mkConf(&entity.ExptItemEvalConf{AsyncZombieSecond: 432000}))
			},
			expected: 432000*time.Second + 30*time.Minute,
		},
		{
			name:    "空间显式配了 eval_async_ctx_ttl_second，优先于按僵尸阈值推导",
			spaceID: spaceID,
			mockSetup: func(l *mock_conf.MockIConfigLoader, ctx context.Context) {
				l.EXPECT().UnmarshalKey(ctx, key, gomock.Any(), gomock.Any()).DoAndReturn(
					mkConf(&entity.ExptItemEvalConf{AsyncZombieSecond: 86400, EvalAsyncCtxTTLSecond: 90000}))
			},
			expected: 90000 * time.Second,
		},
		{
			name:    "该空间未配 async_zombie_second，取 12h 兜底",
			spaceID: spaceID,
			mockSetup: func(l *mock_conf.MockIConfigLoader, ctx context.Context) {
				l.EXPECT().UnmarshalKey(ctx, key, gomock.Any(), gomock.Any()).DoAndReturn(
					mkConf(&entity.ExptItemEvalConf{}))
			},
			expected: 12 * time.Hour,
		},
		{
			name:    "其它空间（未在 TCC 里列出）取 12h 兜底，不串用别人的配置",
			spaceID: 999,
			mockSetup: func(l *mock_conf.MockIConfigLoader, ctx context.Context) {
				l.EXPECT().UnmarshalKey(ctx, key, gomock.Any(), gomock.Any()).DoAndReturn(
					mkConf(&entity.ExptItemEvalConf{AsyncZombieSecond: 86400}))
			},
			expected: 12 * time.Hour,
		},
		{
			name:    "TCC 读取失败时回落默认 conf，仍给 12h 而非 0",
			spaceID: spaceID,
			mockSetup: func(l *mock_conf.MockIConfigLoader, ctx context.Context) {
				l.EXPECT().UnmarshalKey(ctx, key, gomock.Any(), gomock.Any()).Return(errors.New("tcc unavailable"))
			},
			expected: 12 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockLoader := mock_conf.NewMockIConfigLoader(ctrl)
			c := &configer{loader: mockLoader}
			ctx := context.Background()
			tt.mockSetup(mockLoader, ctx)

			assert.Equal(t, tt.expected, c.GetEvalAsyncCtxTTL(ctx, tt.spaceID))
		})
	}
}
