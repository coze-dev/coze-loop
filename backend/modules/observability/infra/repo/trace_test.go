// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config"
	confmocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/ck"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/ck/gorm_gen/model"
	ckmock "github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/ck/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestTraceCkRepoImpl_InsertSpans(t *testing.T) {
	type fields struct {
		spansDao    ck.ISpansDao
		traceConfig config.ITraceConfig
	}
	type args struct {
		ctx   context.Context
		param *repo.InsertTraceParam
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantErr      bool
	}{
		{
			name: "insert spans successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				spansDaoMock := ckmock.NewMockISpansDao(ctrl)
				spansDaoMock.EXPECT().Insert(gomock.Any(), gomock.Any()).Return(nil)
				traceConfigMock := confmocks.NewMockITraceConfig(ctrl)
				traceConfigMock.EXPECT().GetTenantConfig(gomock.Any()).Return(&config.TenantCfg{
					TenantTables: map[string]map[loop_span.TTL]config.TableCfg{
						"test": {
							loop_span.TTL3d: {
								SpanTable: "spans",
							},
						},
					},
				}, nil)
				return fields{
					spansDao:    spansDaoMock,
					traceConfig: traceConfigMock,
				}
			},
			args: args{
				ctx: context.Background(),
				param: &repo.InsertTraceParam{
					Tenant: "test",
					TTL:    loop_span.TTL3d,
					Spans: loop_span.SpanList{
						{
							TagsBool: map[string]bool{
								"a": true,
								"b": false,
							},
							Method:        "a",
							CallType:      "z",
							ObjectStorage: "c",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "insert spans failed due to dao error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				spansDaoMock := ckmock.NewMockISpansDao(ctrl)
				spansDaoMock.EXPECT().Insert(gomock.Any(), gomock.Any()).Return(assert.AnError)
				traceConfigMock := confmocks.NewMockITraceConfig(ctrl)
				traceConfigMock.EXPECT().GetTenantConfig(gomock.Any()).Return(&config.TenantCfg{
					TenantTables: map[string]map[loop_span.TTL]config.TableCfg{
						"test": {
							loop_span.TTL7d: {
								SpanTable: "spans",
							},
						},
					},
				}, nil)
				return fields{
					spansDao:    spansDaoMock,
					traceConfig: traceConfigMock,
				}
			},
			args: args{
				ctx: context.Background(),
				param: &repo.InsertTraceParam{
					Tenant: "test",
					TTL:    loop_span.TTL7d,
					Spans: loop_span.SpanList{
						{
							TraceID: "123",
						},
						{
							SpanType: "test",
						},
					},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			r := &TraceCkRepoImpl{
				spansDao:    fields.spansDao,
				traceConfig: fields.traceConfig,
			}
			err := r.InsertSpans(tt.args.ctx, tt.args.param)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func TestTraceCkRepoImpl_ListSpans(t *testing.T) {
	type fields struct {
		spansDao    ck.ISpansDao
		annoDao     ck.IAnnotationDao
		traceConfig config.ITraceConfig
	}
	type args struct {
		ctx context.Context
		req *repo.ListSpansParam
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *repo.ListSpansResult
		wantErr      bool
	}{
		{
			name: "list spans successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				spansDaoMock := ckmock.NewMockISpansDao(ctrl)
				spansDaoMock.EXPECT().Get(gomock.Any(), gomock.Any()).Return([]*model.ObservabilitySpan{
					{
						TraceID: "123",
						SpanID:  "123",
						TagsBool: map[string]uint8{
							"a": 1,
							"b": 0,
						},
						Method:        ptr.Of("a"),
						CallType:      ptr.Of("z"),
						ObjectStorage: ptr.Of("c"),
					},
					{
						TraceID: "123",
						SpanID:  "123",
						TagsBool: map[string]uint8{
							"a": 1,
							"b": 0,
						},
						Method:        ptr.Of("a"),
						CallType:      ptr.Of("z"),
						ObjectStorage: ptr.Of("c"),
					},
				}, nil)
				traceConfigMock := confmocks.NewMockITraceConfig(ctrl)
				traceConfigMock.EXPECT().GetTenantConfig(gomock.Any()).Return(&config.TenantCfg{
					TenantTables: map[string]map[loop_span.TTL]config.TableCfg{
						"test": {
							loop_span.TTL3d: {
								SpanTable: "spans",
							},
						},
					},
				}, nil)
				return fields{
					spansDao:    spansDaoMock,
					annoDao:     ckmock.NewMockIAnnotationDao(ctrl),
					traceConfig: traceConfigMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &repo.ListSpansParam{
					Tenants: []string{"test"},
					Limit:   10,
				},
			},
			want: &repo.ListSpansResult{
				Spans: loop_span.SpanList{
					{
						TraceID: "123",
						SpanID:  "123",
						TagsBool: map[string]bool{
							"a": true,
							"b": false,
						},
						TagsString:       map[string]string{},
						TagsLong:         map[string]int64{},
						TagsByte:         map[string]string{},
						TagsDouble:       map[string]float64{},
						SystemTagsString: map[string]string{},
						SystemTagsLong:   map[string]int64{},
						SystemTagsDouble: map[string]float64{},
						Method:           "a",
						CallType:         "z",
						ObjectStorage:    "c",
					},
				},
				PageToken: "eyJTdGFydFRpbWUiOjAsIlNwYW5JRCI6IiJ9",
			},
		},
		{
			name: "list spans failed due to config error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceConfigMock := confmocks.NewMockITraceConfig(ctrl)
				traceConfigMock.EXPECT().GetTenantConfig(gomock.Any()).Return(nil, assert.AnError)
				return fields{
					spansDao:    ckmock.NewMockISpansDao(ctrl),
					annoDao:     ckmock.NewMockIAnnotationDao(ctrl),
					traceConfig: traceConfigMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &repo.ListSpansParam{
					Tenants: []string{"test"},
					Limit:   10,
				},
			},
			wantErr: true,
		},
		{
			name: "list spans with annotations successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				spansDaoMock := ckmock.NewMockISpansDao(ctrl)
				spansDaoMock.EXPECT().Get(gomock.Any(), gomock.Any()).Return([]*model.ObservabilitySpan{
					{
						SpanID: "span1",
					},
				}, nil)
				annoDaoMock := ckmock.NewMockIAnnotationDao(ctrl)
				annoDaoMock.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*model.ObservabilityAnnotation{
					{
						ID:     "anno1",
						SpanID: "span1",
					},
				}, nil)
				traceConfigMock := confmocks.NewMockITraceConfig(ctrl)
				traceConfigMock.EXPECT().GetTenantConfig(gomock.Any()).Return(&config.TenantCfg{
					TenantTables: map[string]map[loop_span.TTL]config.TableCfg{
						"test": {
							loop_span.TTL3d: {
								SpanTable: "spans",
								AnnoTable: "annotations",
							},
						},
					},
					TenantsSupportAnnotation: map[string]bool{
						"test": true,
					},
				}, nil)
				return fields{
					spansDao:    spansDaoMock,
					annoDao:     annoDaoMock,
					traceConfig: traceConfigMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &repo.ListSpansParam{
					Tenants:            []string{"test"},
					Limit:              10,
					NotQueryAnnotation: false,
				},
			},
			want: &repo.ListSpansResult{
				Spans: loop_span.SpanList{
					{
						SpanID: "span1",
						Annotations: []*loop_span.Annotation{
							{
								ID:        "anno1",
								SpanID:    "span1",
								StartTime: time.UnixMicro(0),
								UpdatedAt: time.UnixMicro(0),
								CreatedAt: time.UnixMicro(0),
							},
						},
						TagsBool:         map[string]bool{},
						TagsString:       map[string]string{},
						TagsLong:         map[string]int64{},
						TagsByte:         map[string]string{},
						TagsDouble:       map[string]float64{},
						SystemTagsString: map[string]string{},
						SystemTagsLong:   map[string]int64{},
						SystemTagsDouble: map[string]float64{},
					},
				},
				PageToken: "eyJTdGFydFRpbWUiOjAsIlNwYW5JRCI6InNwYW4xIn0=",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			r := &TraceCkRepoImpl{
				spansDao:    fields.spansDao,
				annoDao:     fields.annoDao,
				traceConfig: fields.traceConfig,
			}
			got, err := r.ListSpans(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
			if tt.want != nil && got != nil {
				tt.want.PageToken = got.PageToken
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTraceCkRepoImpl_GetTrace(t *testing.T) {
	type fields struct {
		spansDao    ck.ISpansDao
		annoDao     ck.IAnnotationDao
		traceConfig config.ITraceConfig
	}
	type args struct {
		ctx context.Context
		req *repo.GetTraceParam
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         loop_span.SpanList
		wantErr      bool
	}{
		{
			name: "get trace successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				spansDaoMock := ckmock.NewMockISpansDao(ctrl)
				spansDaoMock.EXPECT().Get(gomock.Any(), gomock.Any()).Return([]*model.ObservabilitySpan{
					{
						TraceID: "span1",
						SpanID:  "span1",
					},
					{
						TraceID: "span2",
						SpanID:  "span2",
					},
					{
						TraceID: "span1",
						SpanID:  "span1",
					},
					{
						TraceID: "span2",
						SpanID:  "span2",
					},
				}, nil)
				traceConfigMock := confmocks.NewMockITraceConfig(ctrl)
				traceConfigMock.EXPECT().GetTenantConfig(gomock.Any()).Return(&config.TenantCfg{
					TenantTables: map[string]map[loop_span.TTL]config.TableCfg{
						"test": {
							loop_span.TTL3d: {
								SpanTable: "spans",
							},
						},
					},
				}, nil)
				return fields{
					spansDao:    spansDaoMock,
					traceConfig: traceConfigMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &repo.GetTraceParam{
					TraceID: "123",
					Tenants: []string{"test"},
				},
			},
			want: loop_span.SpanList{
				{
					TraceID:          "span1",
					SpanID:           "span1",
					TagsString:       map[string]string{},
					TagsLong:         map[string]int64{},
					TagsByte:         map[string]string{},
					TagsDouble:       map[string]float64{},
					TagsBool:         map[string]bool{},
					SystemTagsString: map[string]string{},
					SystemTagsLong:   map[string]int64{},
					SystemTagsDouble: map[string]float64{},
				},
				{
					TraceID:          "span2",
					SpanID:           "span2",
					TagsString:       map[string]string{},
					TagsLong:         map[string]int64{},
					TagsByte:         map[string]string{},
					TagsDouble:       map[string]float64{},
					TagsBool:         map[string]bool{},
					SystemTagsString: map[string]string{},
					SystemTagsLong:   map[string]int64{},
					SystemTagsDouble: map[string]float64{},
				},
			},
		},
		{
			name: "get trace with annotations successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				spansDaoMock := ckmock.NewMockISpansDao(ctrl)
				spansDaoMock.EXPECT().Get(gomock.Any(), gomock.Any()).Return([]*model.ObservabilitySpan{
					{
						SpanID: "span1",
					},
				}, nil)
				annoDaoMock := ckmock.NewMockIAnnotationDao(ctrl)
				annoDaoMock.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*model.ObservabilityAnnotation{
					{
						ID:     "anno1",
						SpanID: "span1",
					},
				}, nil)
				traceConfigMock := confmocks.NewMockITraceConfig(ctrl)
				traceConfigMock.EXPECT().GetTenantConfig(gomock.Any()).Return(&config.TenantCfg{
					TenantTables: map[string]map[loop_span.TTL]config.TableCfg{
						"test": {
							loop_span.TTL3d: {
								SpanTable: "spans",
								AnnoTable: "annotations",
							},
						},
					},
					TenantsSupportAnnotation: map[string]bool{
						"test": true,
					},
				}, nil)
				return fields{
					spansDao:    spansDaoMock,
					annoDao:     annoDaoMock,
					traceConfig: traceConfigMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &repo.GetTraceParam{
					TraceID:            "123",
					Tenants:            []string{"test"},
					NotQueryAnnotation: false,
				},
			},
			want: loop_span.SpanList{
				{
					SpanID: "span1",
					Annotations: []*loop_span.Annotation{
						{
							ID:        "anno1",
							SpanID:    "span1",
							StartTime: time.UnixMicro(0),
							UpdatedAt: time.UnixMicro(0),
							CreatedAt: time.UnixMicro(0),
						},
					},
					TagsBool:         map[string]bool{},
					TagsString:       map[string]string{},
					TagsLong:         map[string]int64{},
					TagsByte:         map[string]string{},
					TagsDouble:       map[string]float64{},
					SystemTagsString: map[string]string{},
					SystemTagsLong:   map[string]int64{},
					SystemTagsDouble: map[string]float64{},
				},
			},
		},
		{
			name: "get trace failed due to config error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceConfigMock := confmocks.NewMockITraceConfig(ctrl)
				traceConfigMock.EXPECT().GetTenantConfig(gomock.Any()).Return(nil, assert.AnError)
				return fields{
					traceConfig: traceConfigMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &repo.GetTraceParam{
					TraceID: "123",
					Tenants: []string{"test"},
				},
			},
			wantErr: true,
		},
		{
			name: "get trace with span successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				spansDaoMock := ckmock.NewMockISpansDao(ctrl)
				spansDaoMock.EXPECT().Get(gomock.Any(), gomock.Any()).Return([]*model.ObservabilitySpan{
					{
						TraceID: "span1",
						SpanID:  "span1",
					},
					{
						TraceID: "span1",
						SpanID:  "span1",
					},
				}, nil)
				traceConfigMock := confmocks.NewMockITraceConfig(ctrl)
				traceConfigMock.EXPECT().GetTenantConfig(gomock.Any()).Return(&config.TenantCfg{
					TenantTables: map[string]map[loop_span.TTL]config.TableCfg{
						"test": {
							loop_span.TTL3d: {
								SpanTable: "spans",
							},
						},
					},
				}, nil)
				return fields{
					spansDao:    spansDaoMock,
					traceConfig: traceConfigMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &repo.GetTraceParam{
					TraceID: "123",
					Tenants: []string{"test"},
					SpanIDs: []string{"span1"},
				},
			},
			want: loop_span.SpanList{
				{
					TraceID:          "span1",
					SpanID:           "span1",
					TagsString:       map[string]string{},
					TagsLong:         map[string]int64{},
					TagsByte:         map[string]string{},
					TagsDouble:       map[string]float64{},
					TagsBool:         map[string]bool{},
					SystemTagsString: map[string]string{},
					SystemTagsLong:   map[string]int64{},
					SystemTagsDouble: map[string]float64{},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			r := &TraceCkRepoImpl{
				spansDao:    fields.spansDao,
				annoDao:     fields.annoDao,
				traceConfig: fields.traceConfig,
			}
			got, err := r.GetTrace(tt.args.ctx, tt.args.req)
			assert.Equal(t, err != nil, tt.wantErr)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTraceCkRepoImpl_InsertAnnotation(t *testing.T) {
	type fields struct {
		annoDao     ck.IAnnotationDao
		traceConfig config.ITraceConfig
	}
	type args struct {
		ctx   context.Context
		param *repo.InsertAnnotationParam
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantErr      bool
	}{
		{
			name: "insert annotation successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				annoDaoMock := ckmock.NewMockIAnnotationDao(ctrl)
				annoDaoMock.EXPECT().Insert(gomock.Any(), gomock.Any()).Return(nil)
				traceConfigMock := confmocks.NewMockITraceConfig(ctrl)
				traceConfigMock.EXPECT().GetTenantConfig(gomock.Any()).Return(&config.TenantCfg{
					TenantTables: map[string]map[loop_span.TTL]config.TableCfg{
						"test": {
							loop_span.TTL3d: {
								AnnoTable: "annotations",
							},
						},
					},
				}, nil)
				return fields{
					annoDao:     annoDaoMock,
					traceConfig: traceConfigMock,
				}
			},
			args: args{
				ctx: context.Background(),
				param: &repo.InsertAnnotationParam{
					Tenant: "test",
					TTL:    loop_span.TTL3d,
					Annotations: []*loop_span.Annotation{
						{
							ID: "anno1",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "insert annotation failed due to config error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceConfigMock := confmocks.NewMockITraceConfig(ctrl)
				traceConfigMock.EXPECT().GetTenantConfig(gomock.Any()).Return(nil, assert.AnError)
				return fields{
					traceConfig: traceConfigMock,
				}
			},
			args: args{
				ctx: context.Background(),
				param: &repo.InsertAnnotationParam{
					Tenant: "test",
					TTL:    loop_span.TTL3d,
					Annotations: []*loop_span.Annotation{
						{
							ID: "anno1",
						},
					},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			r := &TraceCkRepoImpl{
				annoDao:     fields.annoDao,
				traceConfig: fields.traceConfig,
			}
			err := r.InsertAnnotations(tt.args.ctx, tt.args.param)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func TestTraceCkRepoImpl_GetAnnotation(t *testing.T) {
	type fields struct {
		annoDao     ck.IAnnotationDao
		traceConfig config.ITraceConfig
	}
	type args struct {
		ctx   context.Context
		param *repo.GetAnnotationParam
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *loop_span.Annotation
		wantErr      bool
	}{
		{
			name: "get annotation successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				annoDaoMock := ckmock.NewMockIAnnotationDao(ctrl)
				annoDaoMock.EXPECT().Get(gomock.Any(), gomock.Any()).Return(&model.ObservabilityAnnotation{
					ID: "anno1",
				}, nil)
				traceConfigMock := confmocks.NewMockITraceConfig(ctrl)
				traceConfigMock.EXPECT().GetTenantConfig(gomock.Any()).Return(&config.TenantCfg{
					TenantTables: map[string]map[loop_span.TTL]config.TableCfg{
						"test": {
							loop_span.TTL3d: {
								AnnoTable: "annotations",
							},
						},
					},
				}, nil)
				return fields{
					annoDao:     annoDaoMock,
					traceConfig: traceConfigMock,
				}
			},
			args: args{
				ctx: context.Background(),
				param: &repo.GetAnnotationParam{
					ID:      "anno1",
					Tenants: []string{"test"},
				},
			},
			want: &loop_span.Annotation{
				ID:        "anno1",
				StartTime: time.UnixMicro(0),
				UpdatedAt: time.UnixMicro(0),
				CreatedAt: time.UnixMicro(0),
			},
		},
		{
			name: "get annotation failed due to config error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceConfigMock := confmocks.NewMockITraceConfig(ctrl)
				traceConfigMock.EXPECT().GetTenantConfig(gomock.Any()).Return(nil, assert.AnError)
				return fields{
					traceConfig: traceConfigMock,
				}
			},
			args: args{
				ctx: context.Background(),
				param: &repo.GetAnnotationParam{
					ID:      "anno1",
					Tenants: []string{"test"},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			r := &TraceCkRepoImpl{
				annoDao:     fields.annoDao,
				traceConfig: fields.traceConfig,
			}
			got, err := r.GetAnnotation(tt.args.ctx, tt.args.param)
			assert.Equal(t, tt.wantErr, err != nil)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTraceCkRepoImpl_ListAnnotations(t *testing.T) {
	type fields struct {
		annoDao     ck.IAnnotationDao
		traceConfig config.ITraceConfig
	}
	type args struct {
		ctx   context.Context
		param *repo.ListAnnotationsParam
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         loop_span.AnnotationList
		wantErr      bool
	}{
		{
			name: "list annotations successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				annoDaoMock := ckmock.NewMockIAnnotationDao(ctrl)
				annoDaoMock.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*model.ObservabilityAnnotation{
					{
						ID:      "anno1",
						TraceID: "trace1",
						SpaceID: "1",
					},
				}, nil)
				traceConfigMock := confmocks.NewMockITraceConfig(ctrl)
				traceConfigMock.EXPECT().GetTenantConfig(gomock.Any()).Return(&config.TenantCfg{
					TenantTables: map[string]map[loop_span.TTL]config.TableCfg{
						"test": {
							loop_span.TTL3d: {
								AnnoTable: "annotations",
							},
						},
					},
				}, nil)
				return fields{
					annoDao:     annoDaoMock,
					traceConfig: traceConfigMock,
				}
			},
			args: args{
				ctx: context.Background(),
				param: &repo.ListAnnotationsParam{
					SpanID:      "span1",
					TraceID:     "trace1",
					WorkspaceId: 1,
					Tenants:     []string{"test"},
				},
			},
			want: loop_span.AnnotationList{
				{
					ID:          "anno1",
					TraceID:     "trace1",
					WorkspaceID: "1",
					StartTime:   time.UnixMicro(0),
					UpdatedAt:   time.UnixMicro(0),
					CreatedAt:   time.UnixMicro(0),
				},
			},
		},
		{
			name: "list annotations with invalid param",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx:   context.Background(),
				param: &repo.ListAnnotationsParam{},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			r := &TraceCkRepoImpl{
				annoDao:     fields.annoDao,
				traceConfig: fields.traceConfig,
			}
			got, err := r.ListAnnotations(tt.args.ctx, tt.args.param)
			assert.Equal(t, tt.wantErr, err != nil)
			assert.Equal(t, tt.want, got)
		})
	}
}
