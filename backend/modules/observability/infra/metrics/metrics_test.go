// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"errors"
	"sync"
	"testing"
	"time"

	infraMetrics "github.com/coze-dev/coze-loop/backend/infra/metrics"
	"github.com/coze-dev/coze-loop/backend/infra/metrics/mocks"
	metrics2 "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/metrics"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestTraceMetricsImpl_NewTraceMetricsImpl(t *testing.T) {
	type fields struct {
		meter infraMetrics.Meter
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		want         metrics2.ITraceMetrics
		wantErr      bool
	}{
		{
			name: "should return a valid instance when meter is not nil and no error occurs",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				meter := mocks.NewMockMeter(ctrl)
				meter.EXPECT().NewMetric(gomock.Any(), gomock.Any(), gomock.Any()).Return(mocks.NewMockMetric(ctrl), nil)
				return fields{
					meter: meter,
				}
			},
		},
		{
			name: "should return a valid instance when meter is not nil and an error occurs",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				meter := mocks.NewMockMeter(ctrl)
				meter.EXPECT().NewMetric(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("some error"))
				return fields{
					meter: meter,
				}
			},
		},
		{
			name: "should return a valid instance when meter is nil",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{
					meter: nil,
				}
			},
		},
		{
			name: "should return the same instance when called multiple times",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				meter := mocks.NewMockMeter(ctrl)
				meter.EXPECT().NewMetric(gomock.Any(), gomock.Any(), gomock.Any()).Return(mocks.NewMockMetric(ctrl), nil).Times(1)
				return fields{
					meter: meter,
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {
				singletonTraceMetrics = nil
				traceMetricsOnce = sync.Once{}
			})
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			got := NewTraceMetricsImpl(fields.meter)
			assert.NotNil(t, got)

			if tt.name == "should return the same instance when called multiple times" {
				got2 := NewTraceMetricsImpl(fields.meter)
				assert.Same(t, got, got2)
			}
		})
	}
}

func TestTraceMetricsImpl_EmitListSpans(t *testing.T) {
	type fields struct {
		spansMetrics infraMetrics.Metric
	}
	type args struct {
		workspaceId int64
		spanType    string
		start       time.Time
		isError     bool
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
	}{
		{
			name: "should not panic when spansMetrics is nil",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{
					spansMetrics: nil,
				}
			},
			args: args{1, "test", time.Now(), false},
		},
		{
			name: "should emit metrics when spansMetrics is not nil",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				m := mocks.NewMockMetric(ctrl)
				m.EXPECT().Emit(gomock.Any(), gomock.Any()).Times(1)
				return fields{
					spansMetrics: m,
				}
			},
			args: args{1, "test", time.Now(), false},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {
				singletonTraceMetrics = nil
				traceMetricsOnce = sync.Once{}
			})
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			tr := &TraceMetricsImpl{
				spansMetrics: fields.spansMetrics,
			}
			assert.NotPanics(t, func() {
				tr.EmitListSpans(tt.args.workspaceId, tt.args.spanType, tt.args.start, tt.args.isError)
			})
		})
	}
}

func TestTraceMetricsImpl_EmitGetTrace(t *testing.T) {
	type fields struct {
		spansMetrics infraMetrics.Metric
	}
	type args struct {
		workspaceId int64
		start       time.Time
		isError     bool
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
	}{
		{
			name: "should not panic when spansMetrics is nil",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{
					spansMetrics: nil,
				}
			},
			args: args{1, time.Now(), false},
		},
		{
			name: "should emit metrics when spansMetrics is not nil",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				m := mocks.NewMockMetric(ctrl)
				m.EXPECT().Emit(gomock.Any(), gomock.Any()).Times(1)
				return fields{
					spansMetrics: m,
				}
			},
			args: args{1, time.Now(), false},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {
				singletonTraceMetrics = nil
				traceMetricsOnce = sync.Once{}
			})
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			tr := &TraceMetricsImpl{
				spansMetrics: fields.spansMetrics,
			}
			assert.NotPanics(t, func() {
				tr.EmitGetTrace(tt.args.workspaceId, tt.args.start, tt.args.isError)
			})
		})
	}
}

func TestTraceMetricsImpl_EmitTraceOapi(t *testing.T) {
	type fields struct {
		spansMetrics infraMetrics.Metric
	}
	type args struct {
		method       string
		workspaceId  int64
		platformType string
		spanType     string
		spanSize     int64
		errorCode    int
		start        time.Time
		isError      bool
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		expectTags   func(t *testing.T, tags []infraMetrics.T)
	}{
		{
			name: "should not panic when spansMetrics is nil",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{
					spansMetrics: nil,
				}
			},
			args: args{"ListSpans", 123, "coze", "llm", 1024, 0, time.Now(), false},
		},
		{
			name: "should emit metrics with correct tags for ListSpans",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				m := mocks.NewMockMetric(ctrl)
				m.EXPECT().Emit(gomock.Any(), gomock.Any()).Do(func(tags []infraMetrics.T, values ...interface{}) {
					// 验证标签
					expectedTags := map[string]string{
						tagMethod:       "ListSpans",
						tagSpaceID:      "123",
						tagIsErr:        "false",
						tagPlatformType: "coze",
						tagSpanType:     "llm",
						tagErrCode:      "0",
					}
					assert.Len(t, tags, 6)
					for _, tag := range tags {
						expectedValue, exists := expectedTags[tag.Name]
						assert.True(t, exists, "Unexpected tag: %s", tag.Name)
						assert.Equal(t, expectedValue, tag.Value, "Tag %s has wrong value", tag.Name)
					}
					// 验证指标值：3个指标（throughput counter, size counter, latency timer）
					assert.Len(t, values, 3)
				}).Times(1)
				return fields{
					spansMetrics: m,
				}
			},
			args: args{"ListSpans", 123, "coze", "llm", 1024, 0, time.Now(), false},
		},
		{
			name: "should emit metrics with correct tags for GetTrace",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				m := mocks.NewMockMetric(ctrl)
				m.EXPECT().Emit(gomock.Any(), gomock.Any()).Do(func(tags []infraMetrics.T, values ...interface{}) {
					expectedTags := map[string]string{
						tagMethod:       "GetTrace",
						tagSpaceID:      "456",
						tagIsErr:        "false",
						tagPlatformType: "dify",
						tagSpanType:     "workflow",
						tagErrCode:      "0",
					}
					assert.Len(t, tags, 6)
					for _, tag := range tags {
						expectedValue, exists := expectedTags[tag.Name]
						assert.True(t, exists, "Unexpected tag: %s", tag.Name)
						assert.Equal(t, expectedValue, tag.Value, "Tag %s has wrong value", tag.Name)
					}
				}).Times(1)
				return fields{
					spansMetrics: m,
				}
			},
			args: args{"GetTrace", 456, "dify", "workflow", 2048, 0, time.Now(), false},
		},
		{
			name: "should emit metrics with error tags",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				m := mocks.NewMockMetric(ctrl)
				m.EXPECT().Emit(gomock.Any(), gomock.Any()).Do(func(tags []infraMetrics.T, values ...interface{}) {
					expectedTags := map[string]string{
						tagMethod:       "ListSpans",
						tagSpaceID:      "789",
						tagIsErr:        "true",
						tagPlatformType: "openai",
						tagSpanType:     "chat",
						tagErrCode:      "500",
					}
					for _, tag := range tags {
						expectedValue, exists := expectedTags[tag.Name]
						assert.True(t, exists, "Unexpected tag: %s", tag.Name)
						assert.Equal(t, expectedValue, tag.Value, "Tag %s has wrong value", tag.Name)
					}
				}).Times(1)
				return fields{
					spansMetrics: m,
				}
			},
			args: args{"ListSpans", 789, "openai", "chat", 512, 500, time.Now(), true},
		},
		{
			name: "should handle empty method and platform type",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				m := mocks.NewMockMetric(ctrl)
				m.EXPECT().Emit(gomock.Any(), gomock.Any()).Do(func(tags []infraMetrics.T, values ...interface{}) {
					expectedTags := map[string]string{
						tagMethod:       "",
						tagSpaceID:      "0",
						tagIsErr:        "false",
						tagPlatformType: "",
						tagSpanType:     "",
						tagErrCode:      "0",
					}
					for _, tag := range tags {
						expectedValue, exists := expectedTags[tag.Name]
						assert.True(t, exists, "Unexpected tag: %s", tag.Name)
						assert.Equal(t, expectedValue, tag.Value, "Tag %s has wrong value", tag.Name)
					}
				}).Times(1)
				return fields{
					spansMetrics: m,
				}
			},
			args: args{"", 0, "", "", 0, 0, time.Now(), false},
		},
		{
			name: "should handle negative span size",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				m := mocks.NewMockMetric(ctrl)
				m.EXPECT().Emit(gomock.Any(), gomock.Any()).Do(func(tags []infraMetrics.T, values ...interface{}) {
					// 验证负数span size也能正确处理
					assert.Len(t, values, 3)
				}).Times(1)
				return fields{
					spansMetrics: m,
				}
			},
			args: args{"GetTrace", 999, "coze", "agent", -100, 0, time.Now(), false},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {
				singletonTraceMetrics = nil
				traceMetricsOnce = sync.Once{}
			})
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			tr := &TraceMetricsImpl{
				spansMetrics: fields.spansMetrics,
			}
			assert.NotPanics(t, func() {
				tr.EmitTraceOapi(tt.args.method, tt.args.workspaceId, tt.args.platformType, tt.args.spanType, tt.args.spanSize, tt.args.errorCode, tt.args.start, tt.args.isError)
			})
		})
	}
}

func TestTraceMetricsImpl_EmitTraceOapi_MetricValues(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMetric := mocks.NewMockMetric(ctrl)

	// 测试指标值的正确性
	mockMetric.EXPECT().Emit(gomock.Any(), gomock.Any()).Do(func(tags []infraMetrics.T, values ...interface{}) {
		// 验证有3个指标值：throughput counter, size counter, latency timer
		assert.Len(t, values, 3)

		// 验证指标类型和后缀
		throughputCounter := values[0]
		sizeCounter := values[1]
		latencyTimer := values[2]

		assert.NotNil(t, throughputCounter)
		assert.NotNil(t, sizeCounter)
		assert.NotNil(t, latencyTimer)
	}).Times(1)

	tr := &TraceMetricsImpl{
		spansMetrics: mockMetric,
	}

	start := time.Now()
	tr.EmitTraceOapi("TestMethod", 12345, "test_platform", "test_span", 1024, 200, start, true)
}

func TestTraceMetricsImpl_EmitTraceOapi_Integration(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 测试与domain层接口的集成
	mockMetric := mocks.NewMockMetric(ctrl)
	mockMetric.EXPECT().Emit(gomock.Any(), gomock.Any()).AnyTimes()

	tr := &TraceMetricsImpl{
		spansMetrics: mockMetric,
	}

	// 验证实现了domain接口
	var domainMetrics metrics2.ITraceMetrics = tr
	assert.NotNil(t, domainMetrics)

	// 测试所有接口方法
	start := time.Now()
	domainMetrics.EmitListSpans(123, "llm", start, false)
	domainMetrics.EmitGetTrace(456, start, true)
	domainMetrics.EmitTraceOapi("TestMethod", 789, "coze", "workflow", 2048, 0, start, false)
}

func TestTraceMetricsImpl_ConcurrentAccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMetric := mocks.NewMockMetric(ctrl)
	mockMetric.EXPECT().Emit(gomock.Any(), gomock.Any()).AnyTimes()

	tr := &TraceMetricsImpl{
		spansMetrics: mockMetric,
	}

	// 并发安全性测试
	concurrency := 50
	done := make(chan bool, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(id int) {
			defer func() { done <- true }()

			start := time.Now()
			workspaceId := int64(id + 1000)

			// 并发调用EmitTraceOapi方法
			tr.EmitTraceOapi("ConcurrentTest", workspaceId, "test_platform", "test_span", int64(id*10), id%2, start, id%2 == 1)
		}(i)
	}

	// 等待所有goroutine完成
	timeout := time.After(5 * time.Second)
	for i := 0; i < concurrency; i++ {
		select {
		case <-done:
			// 成功完成
		case <-timeout:
			t.Fatal("Concurrent test timed out")
		}
	}
}

func TestTraceQueryTagNames(t *testing.T) {
	expected := []string{
		tagMethod,
		tagSpaceID,
		tagPlatformType,
		tagSpanType,
		tagIsErr,
		tagErrCode,
	}
	result := traceQueryTagNames()
	assert.Equal(t, expected, result)
	assert.Len(t, result, 6)
}
