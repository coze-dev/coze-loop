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

func TestNewPromptCacheMetrics(t *testing.T) {
	type args struct {
		meter metrics.Meter
	}

	tests := []struct {
		name         string
		args         args
		setupMocks   func(ctrl *gomock.Controller) metrics.Meter
		want         *PromptCacheMetrics
		expectNonNil bool
	}{
		{
			name: "success - create new metrics",
			args: args{},
			setupMocks: func(ctrl *gomock.Controller) metrics.Meter {
				mockMeter := metricsmocks.NewMockMeter(ctrl)
				mockMetric := metricsmocks.NewMockMetric(ctrl)

				mockMeter.EXPECT().NewMetric(
					promptCacheMetricsName,
					[]metrics.MetricType{metrics.MetricTypeCounter},
					promptCacheMtrTags(),
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
					promptCacheMetricsName,
					[]metrics.MetricType{metrics.MetricTypeCounter},
					promptCacheMtrTags(),
				).Return(nil, errors.New("create metric failed"))

				return mockMeter
			},
			expectNonNil: true, // 即使创建失败，也会返回一个PromptCacheMetrics对象，但metric字段为nil
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 重置全局变量，确保每个测试用例独立
			promptCacheMetrics = nil
			promptCacheMetricsInitOnce = sync.Once{}

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			var meter metrics.Meter
			if tt.setupMocks != nil {
				meter = tt.setupMocks(ctrl)
			}
			tt.args.meter = meter

			got := NewPromptCacheMetrics(tt.args.meter)

			if tt.want != nil {
				assert.Equal(t, tt.want, got)
			} else if tt.expectNonNil {
				assert.NotNil(t, got)
			} else {
				assert.Nil(t, got)
			}

			// 验证单例模式 - 再次调用应该返回相同的实例
			if tt.args.meter != nil {
				got2 := NewPromptCacheMetrics(tt.args.meter)
				assert.Equal(t, got, got2)
			}
		})
	}
}

func TestPromptCacheMetrics_MEmit(t *testing.T) {
	type fields struct {
		metric metrics.Metric
	}
	type args struct {
		ctx   context.Context
		param PromptCacheMetricsParam
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
						{Name: tagQueryType, Value: string(QueryTypePromptKey)},
						{Name: tagWithCommit, Value: "true"},
						{Name: tagMethod, Value: "unknown"}, // kitexutil.GetMethod返回空字符串时会使用"unknown"
						{Name: tagHit, Value: "true"},
					},
					metrics.Counter(int64(5), metrics.WithSuffix(getSuffix+throughputSuffix)),
				).Times(1)

				mockMetric.EXPECT().Emit(
					[]metrics.T{
						{Name: tagQueryType, Value: string(QueryTypePromptKey)},
						{Name: tagWithCommit, Value: "true"},
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
				param: PromptCacheMetricsParam{
					QueryType:  QueryTypePromptKey,
					WithCommit: true,
					HitNum:     5,
					MissNum:    3,
				},
			},
			expectEmit: true,
		},
		{
			name: "success - emit with prompt_id query type",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockMetric := metricsmocks.NewMockMetric(ctrl)

				mockMetric.EXPECT().Emit(
					[]metrics.T{
						{Name: tagQueryType, Value: string(QueryTypePromptID)},
						{Name: tagWithCommit, Value: "false"},
						{Name: tagMethod, Value: "unknown"},
						{Name: tagHit, Value: "true"},
					},
					metrics.Counter(int64(2), metrics.WithSuffix(getSuffix+throughputSuffix)),
				).Times(1)

				mockMetric.EXPECT().Emit(
					[]metrics.T{
						{Name: tagQueryType, Value: string(QueryTypePromptID)},
						{Name: tagWithCommit, Value: "false"},
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
				param: PromptCacheMetricsParam{
					QueryType:  QueryTypePromptID,
					WithCommit: false,
					HitNum:     2,
					MissNum:    1,
				},
			},
			expectEmit: true,
		},
		{
			name: "success - zero hit and miss numbers",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockMetric := metricsmocks.NewMockMetric(ctrl)

				mockMetric.EXPECT().Emit(
					[]metrics.T{
						{Name: tagQueryType, Value: string(QueryTypePromptKey)},
						{Name: tagWithCommit, Value: "true"},
						{Name: tagMethod, Value: "unknown"},
						{Name: tagHit, Value: "true"},
					},
					metrics.Counter(int64(0), metrics.WithSuffix(getSuffix+throughputSuffix)),
				).Times(1)

				mockMetric.EXPECT().Emit(
					[]metrics.T{
						{Name: tagQueryType, Value: string(QueryTypePromptKey)},
						{Name: tagWithCommit, Value: "true"},
						{Name: tagMethod, Value: "unknown"},
						{Name: tagHit, Value: "false"},
					},
					metrics.Counter(int64(0), metrics.WithSuffix(getSuffix+throughputSuffix)),
				).Times(1)

				return fields{
					metric: mockMetric,
				}
			},
			args: args{
				ctx: context.Background(),
				param: PromptCacheMetricsParam{
					QueryType:  QueryTypePromptKey,
					WithCommit: true,
					HitNum:     0,
					MissNum:    0,
				},
			},
			expectEmit: true,
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
				param: PromptCacheMetricsParam{
					QueryType:  QueryTypePromptKey,
					WithCommit: true,
					HitNum:     1,
					MissNum:    1,
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

			p := &PromptCacheMetrics{
				metric: fields.metric,
			}

			// 测试nil receiver
			if tt.name == "metrics is nil - no emit" {
				var nilMetrics *PromptCacheMetrics
				nilMetrics.MEmit(tt.args.ctx, tt.args.param)
			} else {
				p.MEmit(tt.args.ctx, tt.args.param)
			}
		})
	}
}

func Test_promptCacheMtrTags(t *testing.T) {
	expected := []string{
		tagQueryType,
		tagWithCommit,
		tagMethod,
		tagHit,
	}

	result := promptCacheMtrTags()
	assert.Equal(t, expected, result)
}

func TestConstants(t *testing.T) {
	// 测试常量值是否正确
	assert.Equal(t, "prompt_cache", promptCacheMetricsName)
	assert.Equal(t, "get", getSuffix)
	assert.Equal(t, ".throughput", throughputSuffix)
	assert.Equal(t, "query_type", tagQueryType)
	assert.Equal(t, "with_commit", tagWithCommit)
	assert.Equal(t, "method", tagMethod)
	assert.Equal(t, "hit", tagHit)

	// 测试QueryType常量
	assert.Equal(t, QueryType("prompt_key"), QueryTypePromptKey)
	assert.Equal(t, QueryType("prompt_id"), QueryTypePromptID)
}
