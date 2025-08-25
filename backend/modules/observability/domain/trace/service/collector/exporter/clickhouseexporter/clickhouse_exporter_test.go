// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/collector/consumer"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo/mocks"
)

func TestCkExporter_Start(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(ctrl *gomock.Controller) *ckExporter
		wantErr bool
	}{
		{
			name: "start successfully",
			setup: func(ctrl *gomock.Controller) *ckExporter {
				return &ckExporter{}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			exporter := tt.setup(ctrl)
			err := exporter.Start(context.Background())
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func TestCkExporter_Shutdown(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(ctrl *gomock.Controller) *ckExporter
		wantErr bool
	}{
		{
			name: "shutdown successfully",
			setup: func(ctrl *gomock.Controller) *ckExporter {
				return &ckExporter{}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			exporter := tt.setup(ctrl)
			err := exporter.Shutdown(context.Background())
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func TestCkExporter_ConsumeTraces(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(ctrl *gomock.Controller) *ckExporter
		input   consumer.Traces
		wantErr bool
	}{
		{
			name: "consume traces successfully",
			setup: func(ctrl *gomock.Controller) *ckExporter {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				repoMock.EXPECT().InsertSpans(gomock.Any(), gomock.Any()).Return(nil)
				return &ckExporter{
					traceRepo: repoMock,
				}
			},
			input: consumer.Traces{
				TraceData: []*entity.TraceData{{
					TenantInfo: entity.TenantInfo{TTL: loop_span.TTL3d},
					SpanList: loop_span.SpanList{{
						TraceID: "123",
						SpanID:  "456",
					}},
				}},
			},
			wantErr: false,
		},
		{
			name: "consume traces with repo error",
			setup: func(ctrl *gomock.Controller) *ckExporter {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				repoMock.EXPECT().InsertSpans(gomock.Any(), gomock.Any()).Return(assert.AnError)
				return &ckExporter{
					traceRepo: repoMock,
				}
			},
			input: consumer.Traces{
				TraceData: []*entity.TraceData{{
					TenantInfo: entity.TenantInfo{TTL: loop_span.TTL3d},
					SpanList: loop_span.SpanList{{
						TraceID: "123",
						SpanID:  "456",
					}},
				}},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			exporter := tt.setup(ctrl)
			err := exporter.ConsumeTraces(context.Background(), tt.input)
			t.Log(err)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}
