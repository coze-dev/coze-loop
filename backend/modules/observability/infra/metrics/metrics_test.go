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
