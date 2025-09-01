// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/metrics"
	metricsmocks "github.com/coze-dev/coze-loop/backend/infra/metrics/mocks"
)

func TestNewPromptLabelVersionCacheMetrics(t *testing.T) {
	type args struct {
		meter metrics.Meter
	}

	tests := []struct {
		name         string
		args         args
		setupMocks   func(ctrl *gomock.Controller) metrics.Meter
		want         *PromptLabelVersionCacheMetrics
		expectNonNil bool
	}{
		{
			name: "success - create new metrics",
			args: args{},
			setupMocks: func(ctrl *gomock.Controller) metrics.Meter {
				mockMeter := metricsmocks.NewMockMeter(ctrl)
				mockMetric := metricsmocks.NewMockMetric(ctrl)

				mockMeter.EXPECT().NewMetric(
					promptLabelVersionCacheMetricsName,
					[]metrics.MetricType{metrics.MetricTypeCounter},
					promptLabelVersionCacheMtrTags(),
				).Return(mockMetric, nil)

				return mockMeter
			},
			expectNonNil: true,
		},
		{
			name: "meter is nil",
			args: args{
				meter: nil,
			},
			setupMocks: func(ctrl *gomock.Controller) metrics.Meter {
				return nil
			},
			want: nil,
		},
		{
			name: "new metric error",
			args: args{},
			setupMocks: func(ctrl *gomock.Controller) metrics.Meter {
				mockMeter := metricsmocks.NewMockMeter(ctrl)

				mockMeter.EXPECT().NewMetric(
					promptLabelVersionCacheMetricsName,
					[]metrics.MetricType{metrics.MetricTypeCounter},
					promptLabelVersionCacheMtrTags(),
				).Return(nil, errors.New("create metric failed"))

				return mockMeter
			},
			expectNonNil: true, // 即使创建失败，也会返回一个PromptLabelVersionCacheMetrics对象，但metric字段为nil
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 重置全局变量，确保每个测试用例独立
			promptLabelVersionCacheMetrics = nil
			promptLabelVersionCacheMetricsInitOnce = sync.Once{}

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			var meter metrics.Meter
			if tt.setupMocks != nil {
				meter = tt.setupMocks(ctrl)
			}
			tt.args.meter = meter

			got := NewPromptLabelVersionCacheMetrics(tt.args.meter)

			if tt.want != nil {
				assert.Equal(t, tt.want, got)
			} else if tt.expectNonNil {
				assert.NotNil(t, got)
			} else {
				assert.Nil(t, got)
			}

			// 验证单例模式 - 再次调用应该返回相同的实例
			if tt.args.meter != nil {
				got2 := NewPromptLabelVersionCacheMetrics(tt.args.meter)
				assert.Equal(t, got, got2)
			}
		})
	}
}

func TestPromptLabelVersionCacheMetrics_MEmit(t *testing.T) {
	type fields struct {
		metric metrics.Metric
	}
	type args struct {
		ctx   context.Context
		param PromptLabelVersionCacheMetricsParam
	}

	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		expectEmit   bool
	}{
		{
			name: "success - emit hit and miss metrics",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockMetric := metricsmocks.NewMockMetric(ctrl)

				// 期望调用两次Emit，一次为hit，一次为miss
				mockMetric.EXPECT().Emit(
					[]metrics.T{
						{Name: tagMethod, Value: "unknown"}, // kitexutil.GetMethod返回空字符串时会使用"unknown"
						{Name: tagHit, Value: "true"},
					},
					metrics.Counter(int64(5), metrics.WithSuffix(getSuffix+throughputSuffix)),
				).Times(1)

				mockMetric.EXPECT().Emit(
					[]metrics.T{
						{Name: tagMethod, Value: "unknown"},
						{Name: tagHit, Value: "false"},
					},
					metrics.Counter(int64(3), metrics.WithSuffix(getSuffix+throughputSuffix)),
				).Times(1)

				return fields{
					metric: mockMetric,
				}
			},
			args: args{
				ctx: context.Background(),
				param: PromptLabelVersionCacheMetricsParam{
					HitNum:  5,
					MissNum: 3,
				},
			},
			expectEmit: true,
		},
		{
			name: "success - only hit metrics",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockMetric := metricsmocks.NewMockMetric(ctrl)

				// 只期望调用一次Emit，为hit
				mockMetric.EXPECT().Emit(
					[]metrics.T{
						{Name: tagMethod, Value: "unknown"},
						{Name: tagHit, Value: "true"},
					},
					metrics.Counter(int64(2), metrics.WithSuffix(getSuffix+throughputSuffix)),
				).Times(1)

				return fields{
					metric: mockMetric,
				}
			},
			args: args{
				ctx: context.Background(),
				param: PromptLabelVersionCacheMetricsParam{
					HitNum:  2,
					MissNum: 0,
				},
			},
			expectEmit: true,
		},
		{
			name: "success - only miss metrics",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockMetric := metricsmocks.NewMockMetric(ctrl)

				// 只期望调用一次Emit，为miss
				mockMetric.EXPECT().Emit(
					[]metrics.T{
						{Name: tagMethod, Value: "unknown"},
						{Name: tagHit, Value: "false"},
					},
					metrics.Counter(int64(1), metrics.WithSuffix(getSuffix+throughputSuffix)),
				).Times(1)

				return fields{
					metric: mockMetric,
				}
			},
			args: args{
				ctx: context.Background(),
				param: PromptLabelVersionCacheMetricsParam{
					HitNum:  0,
					MissNum: 1,
				},
			},
			expectEmit: true,
		},
		{
			name: "success - zero hit and miss numbers",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockMetric := metricsmocks.NewMockMetric(ctrl)

				// 不期望调用Emit，因为HitNum和MissNum都为0
				return fields{
					metric: mockMetric,
				}
			},
			args: args{
				ctx: context.Background(),
				param: PromptLabelVersionCacheMetricsParam{
					HitNum:  0,
					MissNum: 0,
				},
			},
			expectEmit: false,
		},
		{
			name: "metrics is nil - no emit",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{
					metric: nil,
				}
			},
			args: args{
				ctx: context.Background(),
				param: PromptLabelVersionCacheMetricsParam{
					HitNum:  1,
					MissNum: 1,
				},
			},
			expectEmit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			var fields fields
			if tt.fieldsGetter != nil {
				fields = tt.fieldsGetter(ctrl)
			}

			p := &PromptLabelVersionCacheMetrics{
				metric: fields.metric,
			}

			// 测试nil receiver
			if tt.name == "metrics is nil - no emit" {
				var nilMetrics *PromptLabelVersionCacheMetrics
				nilMetrics.MEmit(tt.args.ctx, tt.args.param)
			} else {
				p.MEmit(tt.args.ctx, tt.args.param)
			}
		})
	}
}

func Test_promptLabelVersionCacheMtrTags(t *testing.T) {
	expected := []string{
		tagMethod,
		tagHit,
	}

	result := promptLabelVersionCacheMtrTags()
	assert.Equal(t, expected, result)
}

func TestPromptLabelVersionCacheConstants(t *testing.T) {
	// 测试常量值是否正确
	assert.Equal(t, "prompt_label_version_cache", promptLabelVersionCacheMetricsName)
}
