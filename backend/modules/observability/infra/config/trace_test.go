// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	confmocks "github.com/coze-dev/coze-loop/backend/pkg/conf/mocks"
)

func TestTraceConfigCenter_GetSystemViews(t *testing.T) {
	type fields struct {
		configLoader *confmocks.MockIConfigLoader
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         []*config.SystemView
		wantErr      bool
	}{
		{
			name: "get system views successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockLoader := confmocks.NewMockIConfigLoader(ctrl)
				mockLoader.EXPECT().UnmarshalKey(gomock.Any(), systemViewsCfgKey, gomock.Any()).
					DoAndReturn(func(ctx context.Context, key string, v interface{}, opts ...interface{}) error {
						views := v.(*[]*config.SystemView)
						*views = []*config.SystemView{
							{ID: 1, ViewName: "View 1"},
							{ID: 2, ViewName: "View 2"},
						}
						return nil
					})
				return fields{configLoader: mockLoader}
			},
			args: args{ctx: context.Background()},
			want: []*config.SystemView{
				{ID: 1, ViewName: "View 1"},
				{ID: 2, ViewName: "View 2"},
			},
			wantErr: false,
		},
		{
			name: "unmarshal key failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockLoader := confmocks.NewMockIConfigLoader(ctrl)
				mockLoader.EXPECT().UnmarshalKey(gomock.Any(), systemViewsCfgKey, gomock.Any()).
					Return(fmt.Errorf("unmarshal error"))
				return fields{configLoader: mockLoader}
			},
			args:    args{ctx: context.Background()},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			f := tt.fieldsGetter(ctrl)
			tr := &TraceConfigCenter{
				IConfigLoader: f.configLoader,
			}
			got, err := tr.GetSystemViews(tt.args.ctx)
			assert.Equal(t, tt.wantErr, err != nil)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTraceConfigCenter_GetPlatformTenants(t *testing.T) {
	type fields struct {
		configLoader *confmocks.MockIConfigLoader
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *config.PlatformTenantsCfg
		wantErr      bool
	}{
		{
			name: "get platform tenants successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockLoader := confmocks.NewMockIConfigLoader(ctrl)
				mockLoader.EXPECT().UnmarshalKey(gomock.Any(), platformTenantCfgKey, gomock.Any()).
					DoAndReturn(func(ctx context.Context, key string, v interface{}, opts ...interface{}) error {
						cfg := v.(*config.PlatformTenantsCfg)
						cfg.Config = map[string][]string{"platform1": {"tenant1"}}
						return nil
					})
				return fields{configLoader: mockLoader}
			},
			args: args{ctx: context.Background()},
			want: &config.PlatformTenantsCfg{
				Config: map[string][]string{"platform1": {"tenant1"}},
			},
			wantErr: false,
		},
		{
			name: "unmarshal key failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockLoader := confmocks.NewMockIConfigLoader(ctrl)
				mockLoader.EXPECT().UnmarshalKey(gomock.Any(), platformTenantCfgKey, gomock.Any()).
					Return(fmt.Errorf("unmarshal error"))
				return fields{configLoader: mockLoader}
			},
			args:    args{ctx: context.Background()},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			f := tt.fieldsGetter(ctrl)
			tr := &TraceConfigCenter{
				IConfigLoader: f.configLoader,
			}
			got, err := tr.GetPlatformTenants(tt.args.ctx)
			assert.Equal(t, tt.wantErr, err != nil)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTraceConfigCenter_GetPlatformSpansTrans(t *testing.T) {
	type fields struct {
		configLoader *confmocks.MockIConfigLoader
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *config.SpanTransHandlerConfig
		wantErr      bool
	}{
		{
			name: "get platform spans trans successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockLoader := confmocks.NewMockIConfigLoader(ctrl)
				mockLoader.EXPECT().UnmarshalKey(gomock.Any(), platformSpanHandlerCfgKey, gomock.Any()).
					DoAndReturn(func(ctx context.Context, key string, v interface{}, opts ...interface{}) error {
						cfg := v.(*config.SpanTransHandlerConfig)
						cfg.PlatformCfg = make(map[string]loop_span.SpanTransCfgList)
						return nil
					})
				return fields{configLoader: mockLoader}
			},
			args: args{ctx: context.Background()},
			want: &config.SpanTransHandlerConfig{
				PlatformCfg: make(map[string]loop_span.SpanTransCfgList),
			},
			wantErr: false,
		},
		{
			name: "unmarshal key failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockLoader := confmocks.NewMockIConfigLoader(ctrl)
				mockLoader.EXPECT().UnmarshalKey(gomock.Any(), platformSpanHandlerCfgKey, gomock.Any()).
					Return(fmt.Errorf("unmarshal error"))
				return fields{configLoader: mockLoader}
			},
			args:    args{ctx: context.Background()},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			f := tt.fieldsGetter(ctrl)
			tr := &TraceConfigCenter{
				IConfigLoader: f.configLoader,
			}
			got, err := tr.GetPlatformSpansTrans(tt.args.ctx)
			assert.Equal(t, tt.wantErr, err != nil)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTraceConfigCenter_GetTraceIngestTenantProducerCfg(t *testing.T) {
	type fields struct {
		configLoader *confmocks.MockIConfigLoader
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         map[string]*config.IngestConfig
		wantErr      bool
	}{
		{
			name: "get trace ingest tenant producer cfg successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockLoader := confmocks.NewMockIConfigLoader(ctrl)
				mockLoader.EXPECT().UnmarshalKey(context.Background(), traceIngestTenantCfgKey, gomock.Any()).
					DoAndReturn(func(ctx context.Context, key string, v interface{}, opts ...interface{}) error {
						cfg := v.(*map[string]*config.IngestConfig)
						(*cfg)["tenant1"] = &config.IngestConfig{
							MaxSpanLength: 1000,
							MqProducer:    config.MqProducerCfg{Topic: "topic1"},
						}
						return nil
					})
				return fields{configLoader: mockLoader}
			},
			args: args{ctx: context.Background()},
			want: map[string]*config.IngestConfig{
				"tenant1": {
					MaxSpanLength: 1000,
					MqProducer:    config.MqProducerCfg{Topic: "topic1"},
				},
			},
			wantErr: false,
		},
		{
			name: "unmarshal key failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockLoader := confmocks.NewMockIConfigLoader(ctrl)
				mockLoader.EXPECT().UnmarshalKey(context.Background(), traceIngestTenantCfgKey, gomock.Any()).
					Return(fmt.Errorf("unmarshal error"))
				return fields{configLoader: mockLoader}
			},
			args:    args{ctx: context.Background()},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			f := tt.fieldsGetter(ctrl)
			tr := &TraceConfigCenter{
				IConfigLoader: f.configLoader,
			}
			got, err := tr.GetTraceIngestTenantProducerCfg(tt.args.ctx)
			assert.Equal(t, tt.wantErr, err != nil)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTraceConfigCenter_GetAnnotationMqProducerCfg(t *testing.T) {
	type fields struct {
		configLoader *confmocks.MockIConfigLoader
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *config.MqProducerCfg
		wantErr      bool
	}{
		{
			name: "get annotation mq producer cfg successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockLoader := confmocks.NewMockIConfigLoader(ctrl)
				mockLoader.EXPECT().UnmarshalKey(context.Background(), annotationMqProducerCfgKey, gomock.Any()).
					DoAndReturn(func(ctx context.Context, key string, v interface{}, opts ...interface{}) error {
						cfg := v.(*config.MqProducerCfg)
						cfg.Topic = "annotation_topic"
						return nil
					})
				return fields{configLoader: mockLoader}
			},
			args: args{ctx: context.Background()},
			want: &config.MqProducerCfg{
				Topic: "annotation_topic",
			},
			wantErr: false,
		},
		{
			name: "unmarshal key failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockLoader := confmocks.NewMockIConfigLoader(ctrl)
				mockLoader.EXPECT().UnmarshalKey(context.Background(), annotationMqProducerCfgKey, gomock.Any()).
					Return(fmt.Errorf("unmarshal error"))
				return fields{configLoader: mockLoader}
			},
			args:    args{ctx: context.Background()},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			f := tt.fieldsGetter(ctrl)
			tr := &TraceConfigCenter{
				IConfigLoader: f.configLoader,
			}
			got, err := tr.GetAnnotationMqProducerCfg(tt.args.ctx)
			assert.Equal(t, tt.wantErr, err != nil)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTraceConfigCenter_GetTraceCkCfg(t *testing.T) {
	type fields struct {
		configLoader *confmocks.MockIConfigLoader
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *config.TraceCKCfg
		wantErr      bool
	}{
		{
			name: "get trace ck cfg successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockLoader := confmocks.NewMockIConfigLoader(ctrl)
				mockLoader.EXPECT().UnmarshalKey(context.Background(), traceCkCfgKey, gomock.Any()).
					DoAndReturn(func(ctx context.Context, key string, v interface{}, opts ...interface{}) error {
						cfg := v.(*config.TraceCKCfg)
						cfg.DataBase = "trace_db"
						return nil
					})
				return fields{configLoader: mockLoader}
			},
			args: args{ctx: context.Background()},
			want: &config.TraceCKCfg{
				DataBase: "trace_db",
			},
			wantErr: false,
		},
		{
			name: "unmarshal key failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockLoader := confmocks.NewMockIConfigLoader(ctrl)
				mockLoader.EXPECT().UnmarshalKey(context.Background(), traceCkCfgKey, gomock.Any()).
					Return(fmt.Errorf("unmarshal error"))
				return fields{configLoader: mockLoader}
			},
			args:    args{ctx: context.Background()},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			f := tt.fieldsGetter(ctrl)
			tr := &TraceConfigCenter{
				IConfigLoader: f.configLoader,
			}
			got, err := tr.GetTraceCkCfg(tt.args.ctx)
			assert.Equal(t, tt.wantErr, err != nil)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTraceConfigCenter_GetTenantConfig(t *testing.T) {
	type fields struct {
		configLoader *confmocks.MockIConfigLoader
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *config.TenantCfg
		wantErr      bool
	}{
		{
			name: "get tenant config successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockLoader := confmocks.NewMockIConfigLoader(ctrl)
				mockLoader.EXPECT().UnmarshalKey(gomock.Any(), tenantTablesCfgKey, gomock.Any()).
					DoAndReturn(func(ctx context.Context, key string, v interface{}, opts ...interface{}) error {
						cfg := v.(**config.TenantCfg)
						*cfg = &config.TenantCfg{
							DefaultIngestTenant: "default_tenant",
						}
						return nil
					})
				return fields{configLoader: mockLoader}
			},
			args: args{ctx: context.Background()},
			want: &config.TenantCfg{
				DefaultIngestTenant: "default_tenant",
			},
			wantErr: false,
		},
		{
			name: "unmarshal key failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockLoader := confmocks.NewMockIConfigLoader(ctrl)
				mockLoader.EXPECT().UnmarshalKey(gomock.Any(), tenantTablesCfgKey, gomock.Any()).
					Return(fmt.Errorf("unmarshal error"))
				return fields{configLoader: mockLoader}
			},
			args:    args{ctx: context.Background()},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			f := tt.fieldsGetter(ctrl)
			tr := &TraceConfigCenter{
				IConfigLoader: f.configLoader,
			}
			got, err := tr.GetTenantConfig(tt.args.ctx)
			assert.Equal(t, tt.wantErr, err != nil)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTraceConfigCenter_GetTraceFieldMetaInfo(t *testing.T) {
	type fields struct {
		configLoader *confmocks.MockIConfigLoader
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *config.TraceFieldMetaInfoCfg
		wantErr      bool
	}{
		{
			name: "get trace field meta info successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockLoader := confmocks.NewMockIConfigLoader(ctrl)
				mockLoader.EXPECT().UnmarshalKey(gomock.Any(), traceFieldMetaInfoCfgKey, gomock.Any()).
					DoAndReturn(func(ctx context.Context, key string, v interface{}, opts ...interface{}) error {
						cfg := v.(**config.TraceFieldMetaInfoCfg)
						*cfg = &config.TraceFieldMetaInfoCfg{
							AvailableFields: map[string]*config.FieldMeta{"field1": {}},
						}
						return nil
					})
				return fields{configLoader: mockLoader}
			},
			args: args{ctx: context.Background()},
			want: &config.TraceFieldMetaInfoCfg{
				AvailableFields: map[string]*config.FieldMeta{"field1": {}},
			},
			wantErr: false,
		},
		{
			name: "unmarshal key failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockLoader := confmocks.NewMockIConfigLoader(ctrl)
				mockLoader.EXPECT().UnmarshalKey(gomock.Any(), traceFieldMetaInfoCfgKey, gomock.Any()).
					Return(fmt.Errorf("unmarshal error"))
				return fields{configLoader: mockLoader}
			},
			args:    args{ctx: context.Background()},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			f := tt.fieldsGetter(ctrl)
			tr := &TraceConfigCenter{
				IConfigLoader: f.configLoader,
			}
			got, err := tr.GetTraceFieldMetaInfo(tt.args.ctx)
			assert.Equal(t, tt.wantErr, err != nil)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTraceConfigCenter_GetTraceDataMaxDurationDay(t *testing.T) {
	type fields struct {
		configLoader *confmocks.MockIConfigLoader
	}
	type args struct {
		ctx         context.Context
		platformPtr *string
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         int64
	}{
		{
			name: "platform ptr is nil, return default duration",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockLoader := confmocks.NewMockIConfigLoader(ctrl)
				return fields{configLoader: mockLoader}
			},
			args: args{ctx: context.Background(), platformPtr: nil},
			want: 7,
		},
		{
			name: "get duration successfully with platform type",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockLoader := confmocks.NewMockIConfigLoader(ctrl)
				mockLoader.EXPECT().UnmarshalKey(gomock.Any(), traceMaxDurationDay, gomock.Any()).
					DoAndReturn(func(ctx context.Context, key string, v interface{}, opts ...interface{}) error {
						mp := v.(*map[string]int64)
						(*mp)["coze"] = 30
						return nil
					})
				return fields{configLoader: mockLoader}
			},
			args: args{ctx: context.Background(), platformPtr: stringPtr("coze")},
			want: 30,
		},
		{
			name: "platform type not found, return default duration",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockLoader := confmocks.NewMockIConfigLoader(ctrl)
				mockLoader.EXPECT().UnmarshalKey(gomock.Any(), traceMaxDurationDay, gomock.Any()).
					DoAndReturn(func(ctx context.Context, key string, v interface{}, opts ...interface{}) error {
						mp := v.(*map[string]int64)
						(*mp)["other"] = 15
						return nil
					})
				return fields{configLoader: mockLoader}
			},
			args: args{ctx: context.Background(), platformPtr: stringPtr("coze")},
			want: 7,
		},
		{
			name: "platform type has zero value, return default duration",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockLoader := confmocks.NewMockIConfigLoader(ctrl)
				mockLoader.EXPECT().UnmarshalKey(gomock.Any(), traceMaxDurationDay, gomock.Any()).
					DoAndReturn(func(ctx context.Context, key string, v interface{}, opts ...interface{}) error {
						mp := v.(*map[string]int64)
						(*mp)["coze"] = 0
						return nil
					})
				return fields{configLoader: mockLoader}
			},
			args: args{ctx: context.Background(), platformPtr: stringPtr("coze")},
			want: 7,
		},
		{
			name: "unmarshal key failed, return default duration",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockLoader := confmocks.NewMockIConfigLoader(ctrl)
				mockLoader.EXPECT().UnmarshalKey(gomock.Any(), traceMaxDurationDay, gomock.Any()).
					Return(fmt.Errorf("unmarshal error"))
				return fields{configLoader: mockLoader}
			},
			args: args{ctx: context.Background(), platformPtr: stringPtr("coze")},
			want: 7,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			f := tt.fieldsGetter(ctrl)
			tr := &TraceConfigCenter{
				IConfigLoader: f.configLoader,
			}
			got := tr.GetTraceDataMaxDurationDay(tt.args.ctx, tt.args.platformPtr)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTraceConfigCenter_GetDefaultTraceTenant(t *testing.T) {
	type fields struct {
		configLoader       *confmocks.MockIConfigLoader
		traceDefaultTenant string
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         string
	}{
		{
			name: "get default trace tenant successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockLoader := confmocks.NewMockIConfigLoader(ctrl)
				return fields{
					configLoader:       mockLoader,
					traceDefaultTenant: "default_tenant",
				}
			},
			args: args{ctx: context.Background()},
			want: "default_tenant",
		},
		{
			name: "get empty default trace tenant",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockLoader := confmocks.NewMockIConfigLoader(ctrl)
				return fields{
					configLoader:       mockLoader,
					traceDefaultTenant: "",
				}
			},
			args: args{ctx: context.Background()},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			f := tt.fieldsGetter(ctrl)
			tr := &TraceConfigCenter{
				IConfigLoader:      f.configLoader,
				traceDefaultTenant: f.traceDefaultTenant,
			}
			got := tr.GetDefaultTraceTenant(tt.args.ctx)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTraceConfigCenter_getDefaultTraceTenant(t *testing.T) {
	type fields struct {
		configLoader       *confmocks.MockIConfigLoader
		traceDefaultTenant string
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         string
		wantErr      bool
	}{
		{
			name: "trace default tenant already set",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockLoader := confmocks.NewMockIConfigLoader(ctrl)
				return fields{
					configLoader:       mockLoader,
					traceDefaultTenant: "existing_tenant",
				}
			},
			args:    args{ctx: context.Background()},
			want:    "existing_tenant",
			wantErr: false,
		},
		{
			name: "get tenant config successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockLoader := confmocks.NewMockIConfigLoader(ctrl)
				mockLoader.EXPECT().UnmarshalKey(gomock.Any(), tenantTablesCfgKey, gomock.Any()).
					DoAndReturn(func(ctx context.Context, key string, v interface{}, opts ...interface{}) error {
						cfg := v.(**config.TenantCfg)
						*cfg = &config.TenantCfg{
							DefaultIngestTenant: "new_tenant",
						}
						return nil
					})
				return fields{
					configLoader:       mockLoader,
					traceDefaultTenant: "",
				}
			},
			args:    args{ctx: context.Background()},
			want:    "new_tenant",
			wantErr: false,
		},
		{
			name: "get tenant config failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockLoader := confmocks.NewMockIConfigLoader(ctrl)
				mockLoader.EXPECT().UnmarshalKey(gomock.Any(), tenantTablesCfgKey, gomock.Any()).
					Return(fmt.Errorf("config error"))
				return fields{
					configLoader:       mockLoader,
					traceDefaultTenant: "",
				}
			},
			args:    args{ctx: context.Background()},
			want:    "",
			wantErr: true,
		},
		{
			name: "default ingest tenant is empty",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockLoader := confmocks.NewMockIConfigLoader(ctrl)
				mockLoader.EXPECT().UnmarshalKey(gomock.Any(), tenantTablesCfgKey, gomock.Any()).
					DoAndReturn(func(ctx context.Context, key string, v interface{}, opts ...interface{}) error {
						cfg := v.(**config.TenantCfg)
						*cfg = &config.TenantCfg{
							DefaultIngestTenant: "",
						}
						return nil
					})
				return fields{
					configLoader:       mockLoader,
					traceDefaultTenant: "",
				}
			},
			args:    args{ctx: context.Background()},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			f := tt.fieldsGetter(ctrl)
			tr := &TraceConfigCenter{
				IConfigLoader:      f.configLoader,
				traceDefaultTenant: f.traceDefaultTenant,
			}
			got, err := tr.getDefaultTraceTenant(tt.args.ctx)
			assert.Equal(t, tt.wantErr, err != nil)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTraceConfigCenter_GetAnnotationSourceCfg(t *testing.T) {
	type fields struct {
		configLoader *confmocks.MockIConfigLoader
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *config.AnnotationSourceConfig
		wantErr      bool
	}{
		{
			name: "get annotation source cfg successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockLoader := confmocks.NewMockIConfigLoader(ctrl)
				mockLoader.EXPECT().UnmarshalKey(gomock.Any(), annotationSourceCfgKey, gomock.Any()).
					DoAndReturn(func(ctx context.Context, key string, v interface{}, opts ...interface{}) error {
						cfg := v.(**config.AnnotationSourceConfig)
						*cfg = &config.AnnotationSourceConfig{
							SourceCfg: map[string]config.AnnotationConfig{"source1": {}},
						}
						return nil
					})
				return fields{configLoader: mockLoader}
			},
			args: args{ctx: context.Background()},
			want: &config.AnnotationSourceConfig{
				SourceCfg: map[string]config.AnnotationConfig{"source1": {}},
			},
			wantErr: false,
		},
		{
			name: "unmarshal key failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockLoader := confmocks.NewMockIConfigLoader(ctrl)
				mockLoader.EXPECT().UnmarshalKey(gomock.Any(), annotationSourceCfgKey, gomock.Any()).
					Return(fmt.Errorf("unmarshal error"))
				return fields{configLoader: mockLoader}
			},
			args:    args{ctx: context.Background()},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			f := tt.fieldsGetter(ctrl)
			tr := &TraceConfigCenter{
				IConfigLoader: f.configLoader,
			}
			got, err := tr.GetAnnotationSourceCfg(tt.args.ctx)
			assert.Equal(t, tt.wantErr, err != nil)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTraceConfigCenter_GetQueryMaxQPS(t *testing.T) {
	type fields struct {
		configLoader *confmocks.MockIConfigLoader
	}
	type args struct {
		ctx context.Context
		key string
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         int
		wantErr      bool
	}{
		{
			name: "get query max qps successfully with specific key",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockLoader := confmocks.NewMockIConfigLoader(ctrl)
				mockLoader.EXPECT().UnmarshalKey(gomock.Any(), queryTraceRateLimitCfgKey, gomock.Any()).
					DoAndReturn(func(ctx context.Context, key string, v interface{}, opts ...interface{}) error {
						cfg := v.(**config.QueryTraceRateLimitConfig)
						*cfg = &config.QueryTraceRateLimitConfig{
							SpaceMaxQPS:   map[string]int{"space1": 100},
							DefaultMaxQPS: 50,
						}
						return nil
					})
				return fields{configLoader: mockLoader}
			},
			args:    args{ctx: context.Background(), key: "space1"},
			want:    100,
			wantErr: false,
		},
		{
			name: "get query max qps with default value",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockLoader := confmocks.NewMockIConfigLoader(ctrl)
				mockLoader.EXPECT().UnmarshalKey(gomock.Any(), queryTraceRateLimitCfgKey, gomock.Any()).
					DoAndReturn(func(ctx context.Context, key string, v interface{}, opts ...interface{}) error {
						cfg := v.(**config.QueryTraceRateLimitConfig)
						*cfg = &config.QueryTraceRateLimitConfig{
							SpaceMaxQPS:   map[string]int{"space1": 100},
							DefaultMaxQPS: 50,
						}
						return nil
					})
				return fields{configLoader: mockLoader}
			},
			args:    args{ctx: context.Background(), key: "space2"},
			want:    50,
			wantErr: false,
		},
		{
			name: "unmarshal key failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockLoader := confmocks.NewMockIConfigLoader(ctrl)
				mockLoader.EXPECT().UnmarshalKey(gomock.Any(), queryTraceRateLimitCfgKey, gomock.Any()).
					Return(fmt.Errorf("unmarshal error"))
				return fields{configLoader: mockLoader}
			},
			args:    args{ctx: context.Background(), key: "space1"},
			want:    0,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			f := tt.fieldsGetter(ctrl)
			tr := &TraceConfigCenter{
				IConfigLoader: f.configLoader,
			}
			got, err := tr.GetQueryMaxQPS(tt.args.ctx, tt.args.key)
			assert.Equal(t, tt.wantErr, err != nil)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNewTraceConfigCenter(t *testing.T) {
	type args struct {
		confP *confmocks.MockIConfigLoader
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) args
		wantPanic    bool
	}{
		{
			name: "create trace config center successfully",
			fieldsGetter: func(ctrl *gomock.Controller) args {
				mockLoader := confmocks.NewMockIConfigLoader(ctrl)
				mockLoader.EXPECT().UnmarshalKey(context.Background(), tenantTablesCfgKey, gomock.Any()).
					DoAndReturn(func(ctx context.Context, key string, v interface{}, opts ...interface{}) error {
						cfg := v.(**config.TenantCfg)
						*cfg = &config.TenantCfg{
							DefaultIngestTenant: "test_tenant",
						}
						return nil
					})
				return args{confP: mockLoader}
			},
			wantPanic: false,
		},
		{
			name: "create trace config center with panic",
			fieldsGetter: func(ctrl *gomock.Controller) args {
				mockLoader := confmocks.NewMockIConfigLoader(ctrl)
				mockLoader.EXPECT().UnmarshalKey(context.Background(), tenantTablesCfgKey, gomock.Any()).
					Return(fmt.Errorf("config error"))
				return args{confP: mockLoader}
			},
			wantPanic: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			f := tt.fieldsGetter(ctrl)
			if tt.wantPanic {
				assert.Panics(t, func() {
					NewTraceConfigCenter(f.confP)
				})
			} else {
				got := NewTraceConfigCenter(f.confP)
				assert.NotNil(t, got)
				assert.IsType(t, &TraceConfigCenter{}, got)
			}
		})
	}
}

// Helper function to create string pointer
func stringPtr(s string) *string {
	return &s
}
