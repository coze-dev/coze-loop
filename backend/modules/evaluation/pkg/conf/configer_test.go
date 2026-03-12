// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package conf

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
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
