// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package span_processor

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	benefitmock "github.com/coze-dev/coze-loop/backend/infra/external/benefit/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
)

func TestExpireErrorProcessor_Transform(t *testing.T) {
	type fields struct {
		platformType loop_span.PlatformType
		queryEndTime int64
		workspaceId  int64
		benefitSvc   benefit.IBenefitService
	}
	type args struct {
		ctx   context.Context
		spans loop_span.SpanList
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         loop_span.SpanList
		wantErr      bool
	}{
		{
			name: "transform successfully when spans not empty",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{
					platformType: loop_span.PlatformCozeLoop,
					queryEndTime: time.Now().UnixMilli(),
					workspaceId:  1,
				}
			},
			args: args{
				ctx: context.Background(),
				spans: loop_span.SpanList{{
					TraceID: "123",
				}},
			},
			want: loop_span.SpanList{{
				TraceID: "123",
			}},
			wantErr: false,
		},
		{
			name: "transform failed when platform type not supported",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{
					platformType: loop_span.PlatformType("unsupported"),
					queryEndTime: time.Now().UnixMilli(),
					workspaceId:  1,
				}
			},
			args: args{
				ctx: context.Background(),
				spans: loop_span.SpanList{{
					TraceID: "123",
				}},
			},
			want: loop_span.SpanList{{
				TraceID: "123",
			}},
			wantErr: false,
		},
		{
			name: "transform returns spans when benefit check returns error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				benefitMock := benefitmock.NewMockIBenefitService(ctrl)
				benefitMock.EXPECT().CheckTraceBenefit(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("benefit error"))
				return fields{
					platformType: loop_span.PlatformCozeLoop,
					queryEndTime: time.Now().UnixMilli(),
					workspaceId:  1,
					benefitSvc:   benefitMock,
				}
			},
			args: args{
				ctx:   context.Background(),
				spans: loop_span.SpanList{},
			},
			want:    loop_span.SpanList{},
			wantErr: false,
		},
		{
			name: "transform returns spans when benefit check returns nil response",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				benefitMock := benefitmock.NewMockIBenefitService(ctrl)
				benefitMock.EXPECT().CheckTraceBenefit(gomock.Any(), gomock.Any()).Return(nil, nil)
				return fields{
					platformType: loop_span.PlatformCozeLoop,
					queryEndTime: time.Now().UnixMilli(),
					workspaceId:  1,
					benefitSvc:   benefitMock,
				}
			},
			args: args{
				ctx:   context.Background(),
				spans: loop_span.SpanList{},
			},
			want:    loop_span.SpanList{},
			wantErr: false,
		},
		{
			name: "transform failed when query time expired",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				benefitMock := benefitmock.NewMockIBenefitService(ctrl)
				benefitMock.EXPECT().CheckTraceBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckTraceBenefitResult{
					StorageDuration: 1,
				}, nil)
				return fields{
					platformType: loop_span.PlatformCozeLoop,
					queryEndTime: time.Now().Add(-25 * time.Hour).UnixMilli(),
					workspaceId:  1,
					benefitSvc:   benefitMock,
				}
			},
			args: args{
				ctx:   context.Background(),
				spans: loop_span.SpanList{},
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			e := &ExpireErrorProcessor{
				platformType: fields.platformType,
				queryEndTime: fields.queryEndTime,
				workspaceId:  fields.workspaceId,
				benefitSvc:   fields.benefitSvc,
			}
			got, err := e.Transform(tt.args.ctx, tt.args.spans)
			assert.Equal(t, err != nil, tt.wantErr)
			assert.Equal(t, got, tt.want)
		})
	}
}

func TestExpireErrorProcessorFactory_CreateProcessor(t *testing.T) {
	type fields struct {
		benefitSvc benefit.IBenefitService
	}
	type args struct {
		ctx context.Context
		set Settings
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         Processor
		wantErr      bool
	}{
		{
			name: "create processor successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{
					benefitSvc: benefitmock.NewMockIBenefitService(ctrl),
				}
			},
			args: args{
				ctx: context.Background(),
				set: Settings{
					PlatformType: loop_span.PlatformCozeLoop,
					QueryEndTime: time.Now().UnixMilli(),
					WorkspaceId:  1,
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			e := &ExpireErrorProcessorFactory{
				benefitSvc: fields.benefitSvc,
			}
			_, err := e.CreateProcessor(tt.args.ctx, tt.args.set)
			assert.Equal(t, err != nil, tt.wantErr)
		})
	}
}
