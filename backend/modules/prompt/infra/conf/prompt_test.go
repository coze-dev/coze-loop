// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package conf

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/pkg/conf"
	confmocks "github.com/coze-dev/coze-loop/backend/pkg/conf/mocks"
)

func TestPromptConfigProvider_GetPTaaSMaxQPSByPromptKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		spaceID    int64
		promptKey  string
		configData interface{}
		mockErr    error
		wantQPS    int
		wantErr    bool
	}{
		{
			name:      "使用默认QPS - space_id和prompt_key都不存在",
			spaceID:   12345,
			promptKey: "non_existent_key",
			configData: &ptaasRateLimitConfig{
				DefaultMaxQPS:   100,
				PromptKeyMaxQPS: map[string]map[string]int{},
			},
			wantQPS: 100,
			wantErr: false,
		},
		{
			name:      "使用特定space_id和prompt_key的QPS",
			spaceID:   12345,
			promptKey: "special_prompt",
			configData: &ptaasRateLimitConfig{
				DefaultMaxQPS: 100,
				PromptKeyMaxQPS: map[string]map[string]int{
					"12345": {
						"special_prompt": 200,
					},
				},
			},
			wantQPS: 200,
			wantErr: false,
		},
		{
			name:      "space_id存在但prompt_key不存在时使用默认QPS",
			spaceID:   12345,
			promptKey: "non_existent_prompt",
			configData: &ptaasRateLimitConfig{
				DefaultMaxQPS: 150,
				PromptKeyMaxQPS: map[string]map[string]int{
					"12345": {
						"other_prompt": 300,
					},
				},
			},
			wantQPS: 150,
			wantErr: false,
		},
		{
			name:      "space_id不存在时使用默认QPS",
			spaceID:   99999,
			promptKey: "any_prompt",
			configData: &ptaasRateLimitConfig{
				DefaultMaxQPS: 120,
				PromptKeyMaxQPS: map[string]map[string]int{
					"12345": {
						"some_prompt": 400,
					},
				},
			},
			wantQPS: 120,
			wantErr: false,
		},
		{
			name:      "配置加载失败",
			spaceID:   12345,
			promptKey: "any_key",
			mockErr:   assert.AnError,
			wantQPS:   0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockConfigLoader := confmocks.NewMockIConfigLoader(ctrl)

			if tt.mockErr != nil {
				mockConfigLoader.EXPECT().
					UnmarshalKey(gomock.Any(), "ptaas_rate_limit_config", gomock.Any()).
					Return(tt.mockErr).AnyTimes()
			} else {
				mockConfigLoader.EXPECT().
					UnmarshalKey(gomock.Any(), "ptaas_rate_limit_config", gomock.Any()).
					DoAndReturn(func(ctx context.Context, key string, target interface{}, opts ...conf.DecodeOptionFn) error {
						if config, ok := target.(*ptaasRateLimitConfig); ok {
							*config = *tt.configData.(*ptaasRateLimitConfig)
						}
						return nil
					}).AnyTimes()
			}

			provider := &PromptConfigProvider{
				ConfigLoader: mockConfigLoader,
			}

			qps, err := provider.GetPTaaSMaxQPSByPromptKey(context.Background(), tt.spaceID, tt.promptKey)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantQPS, qps)
			}
		})
	}
}
