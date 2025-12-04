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
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/mq"
	mqmock "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/mq/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/storage"
	metric_entity "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	metric_repo "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/dao"
	daomock "github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/dao/mocks"
	redis_dao_mock "github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/redis/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

type mockStorageProvider struct{}

func (m *mockStorageProvider) GetTraceStorage(ctx context.Context, workSpaceID string, tenants []string) storage.Storage {
	return storage.Storage{
		StorageName:   "ck",
		StorageConfig: map[string]string{},
	}
}

func (m *mockStorageProvider) PrepareStorageForTask(ctx context.Context, workspaceID string, tenants []string) error {
	return nil
}

func (m *mockStorageProvider) GetSpanDao(tenant string) dao.ISpansDao {
	return nil
}

func (m *mockStorageProvider) GetAnnotationDao(tenant string) dao.IAnnotationDao {
	return nil
}

func TestTraceRepoImpl_InsertSpans(t *testing.T) {
	type fields struct {
		spansDao    dao.ISpansDao
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
				spansDaoMock := daomock.NewMockISpansDao(ctrl)
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
				spansDaoMock := daomock.NewMockISpansDao(ctrl)
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
			r, err := NewTraceRepoImpl(
				fields.traceConfig,
				&mockStorageProvider{},
				nil,
				nil,
				WithTraceStorageSpanDao("ck", fields.spansDao),
			)
			assert.NoError(t, err)
			err = r.InsertSpans(tt.args.ctx, tt.args.param)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func TestTraceRepoImpl_ListSpans(t *testing.T) {
	type fields struct {
		spansDao    dao.ISpansDao
		annoDao     dao.IAnnotationDao
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
				spansDaoMock := daomock.NewMockISpansDao(ctrl)
				spansDaoMock.EXPECT().Get(gomock.Any(), gomock.Any()).Return([]*dao.Span{
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
					annoDao:     daomock.NewMockIAnnotationDao(ctrl),
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
					spansDao:    daomock.NewMockISpansDao(ctrl),
					annoDao:     daomock.NewMockIAnnotationDao(ctrl),
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
				spansDaoMock := daomock.NewMockISpansDao(ctrl)
				spansDaoMock.EXPECT().Get(gomock.Any(), gomock.Any()).Return([]*dao.Span{
					{
						SpanID: "span1",
					},
				}, nil)
				annoDaoMock := daomock.NewMockIAnnotationDao(ctrl)
				annoDaoMock.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*dao.Annotation{
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
			r, err := NewTraceRepoImpl(
				fields.traceConfig,
				&mockStorageProvider{},
				nil,
				nil,
				WithTraceStorageDaos("ck", fields.spansDao, fields.annoDao),
			)
			assert.NoError(t, err)
			got, err := r.ListSpans(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
			if tt.want != nil && got != nil {
				tt.want.PageToken = got.PageToken
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTraceRepoImpl_GetTrace(t *testing.T) {
	type fields struct {
		spansDao    dao.ISpansDao
		annoDao     dao.IAnnotationDao
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
				spansDaoMock := daomock.NewMockISpansDao(ctrl)
				// 期望的QueryParam应该包含TraceID过滤条件
				spansDaoMock.EXPECT().Get(gomock.Any(), gomock.Any()).Return([]*dao.Span{
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
					annoDao:     daomock.NewMockIAnnotationDao(ctrl),
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
				spansDaoMock := daomock.NewMockISpansDao(ctrl)
				spansDaoMock.EXPECT().Get(gomock.Any(), gomock.Any()).Return([]*dao.Span{
					{
						SpanID: "span1",
					},
				}, nil)
				annoDaoMock := daomock.NewMockIAnnotationDao(ctrl)
				annoDaoMock.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*dao.Annotation{
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
					spansDao:    daomock.NewMockISpansDao(ctrl),
					annoDao:     daomock.NewMockIAnnotationDao(ctrl),
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
				spansDaoMock := daomock.NewMockISpansDao(ctrl)
				spansDaoMock.EXPECT().Get(gomock.Any(), gomock.Any()).Return([]*dao.Span{
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
					annoDao:     daomock.NewMockIAnnotationDao(ctrl),
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
			r, err := NewTraceRepoImpl(
				fields.traceConfig,
				&mockStorageProvider{},
				nil,
				nil,
				WithTraceStorageDaos("ck", fields.spansDao, fields.annoDao),
			)
			assert.NoError(t, err)
			got, err := r.GetTrace(tt.args.ctx, tt.args.req)
			assert.Equal(t, err != nil, tt.wantErr)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTraceRepoImpl_GetMetrics(t *testing.T) {
	t.Run("get metrics successfully", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		spansDaoMock := daomock.NewMockISpansDao(ctrl)
		traceConfigMock := confmocks.NewMockITraceConfig(ctrl)

		aggregations := []*metric_entity.Dimension{
			{
				Expression: &metric_entity.Expression{
					Expression: "count(*)",
				},
				Alias: "count",
			},
		}
		groupBys := []*metric_entity.Dimension{
			{
				Field: &loop_span.FilterField{
					FieldName: loop_span.SpanFieldPSM,
					FieldType: loop_span.FieldTypeString,
				},
				Alias: "psm",
			},
		}
		filters := &loop_span.FilterFields{
			FilterFields: []*loop_span.FilterField{
				{
					FieldName: loop_span.SpanFieldStatusCode,
					FieldType: loop_span.FieldTypeLong,
					Values:    []string{"200"},
					QueryType: ptr.Of(loop_span.QueryTypeEnumEq),
				},
			},
		}
		metricsData := []map[string]any{
			{
				"count": 1,
			},
		}
		spansDaoMock.EXPECT().GetMetrics(gomock.Any(), gomock.Any()).Return(metricsData, nil)
		traceConfigMock.EXPECT().GetTenantConfig(gomock.Any()).Return(&config.TenantCfg{
			TenantTables: map[string]map[loop_span.TTL]config.TableCfg{
				"tenant": {
					loop_span.TTL3d: {
						SpanTable: "spans",
					},
				},
			},
		}, nil)

		repoImpl, err := NewTraceMetricCKRepoImpl(
			traceConfigMock,
			&mockStorageProvider{},
			WithTraceStorageSpanDao("ck", spansDaoMock),
		)
		assert.NoError(t, err)
		result, err := repoImpl.GetMetrics(context.Background(), &metric_repo.GetMetricsParam{
			Tenants:      []string{"tenant"},
			Aggregations: aggregations,
			GroupBys:     groupBys,
			Filters:      filters,
			StartAt:      1000,
			EndAt:        2000,
			Granularity:  metric_entity.MetricGranularity1Min,
		})
		assert.NoError(t, err)
		assert.Equal(t, &metric_repo.GetMetricsResult{Data: metricsData}, result)
	})

	t.Run("get metrics failed due to config error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		traceConfigMock := confmocks.NewMockITraceConfig(ctrl)
		traceConfigMock.EXPECT().GetTenantConfig(gomock.Any()).Return(nil, assert.AnError)

		spansDaoMock := daomock.NewMockISpansDao(ctrl)
		repoImpl, err := NewTraceMetricCKRepoImpl(
			traceConfigMock,
			&mockStorageProvider{},
			WithTraceStorageSpanDao("ck", spansDaoMock),
		)
		assert.NoError(t, err)
		result, err := repoImpl.GetMetrics(context.Background(), &metric_repo.GetMetricsParam{
			Tenants: []string{"tenant"},
		})
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("get metrics failed due to dao error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		spansDaoMock := daomock.NewMockISpansDao(ctrl)
		traceConfigMock := confmocks.NewMockITraceConfig(ctrl)

		traceConfigMock.EXPECT().GetTenantConfig(gomock.Any()).Return(&config.TenantCfg{
			TenantTables: map[string]map[loop_span.TTL]config.TableCfg{
				"tenant": {
					loop_span.TTL3d: {
						SpanTable: "spans",
					},
				},
			},
		}, nil)
		spansDaoMock.EXPECT().GetMetrics(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)

		repoImpl, err := NewTraceMetricCKRepoImpl(
			traceConfigMock,
			&mockStorageProvider{},
			WithTraceStorageSpanDao("ck", spansDaoMock),
		)
		assert.NoError(t, err)
		result, err := repoImpl.GetMetrics(context.Background(), &metric_repo.GetMetricsParam{
			Tenants: []string{"tenant"},
		})
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestTraceRepoImpl_InsertAnnotation(t *testing.T) {
	type fields struct {
		annoDao      dao.IAnnotationDao
		traceConfig  config.ITraceConfig
		spanProducer mq.ISpanProducer
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
				annoDaoMock := daomock.NewMockIAnnotationDao(ctrl)
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
				spanProducerMock := mqmock.NewMockISpanProducer(ctrl)
				spanProducerMock.EXPECT().SendSpanWithAnnotation(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				return fields{
					annoDao:      annoDaoMock,
					traceConfig:  traceConfigMock,
					spanProducer: spanProducerMock,
				}
			},
			args: args{
				ctx: context.Background(),
				param: &repo.InsertAnnotationParam{
					Tenant: "test",
					TTL:    loop_span.TTL3d,
					Span: &loop_span.Span{
						Annotations: []*loop_span.Annotation{
							{
								ID: "anno1",
							},
						},
					},
					AnnotationType: ptr.Of(loop_span.AnnotationTypeOpenAPIFeedback),
				},
			},
			wantErr: false,
		},
		{
			name: "insert annotation failed due to config error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceConfigMock := confmocks.NewMockITraceConfig(ctrl)
				traceConfigMock.EXPECT().GetTenantConfig(gomock.Any()).Return(nil, assert.AnError)
				spanProducerMock := mqmock.NewMockISpanProducer(ctrl)
				return fields{
					annoDao:      daomock.NewMockIAnnotationDao(ctrl),
					traceConfig:  traceConfigMock,
					spanProducer: spanProducerMock,
				}
			},
			args: args{
				ctx: context.Background(),
				param: &repo.InsertAnnotationParam{
					Tenant: "test",
					TTL:    loop_span.TTL3d,
					Span: &loop_span.Span{
						Annotations: []*loop_span.Annotation{
							{
								ID: "anno1",
							},
						},
					},
					AnnotationType: ptr.Of(loop_span.AnnotationTypeOpenAPIFeedback),
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
			r, err := NewTraceRepoImpl(
				fields.traceConfig,
				&mockStorageProvider{},
				nil,
				fields.spanProducer,
				WithTraceStorageAnnotationDao("ck", fields.annoDao),
			)
			assert.NoError(t, err)
			err = r.InsertAnnotations(tt.args.ctx, tt.args.param)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func TestTraceRepoImpl_GetAnnotation(t *testing.T) {
	type fields struct {
		annoDao      dao.IAnnotationDao
		traceConfig  config.ITraceConfig
		spanRedisDao *redis_dao_mock.MockISpansRedisDao
		spanProducer mq.ISpanProducer
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
				annoDaoMock := daomock.NewMockIAnnotationDao(ctrl)
				annoDaoMock.EXPECT().Get(gomock.Any(), gomock.Any()).Return(&dao.Annotation{
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
					annoDao:      annoDaoMock,
					traceConfig:  traceConfigMock,
					spanRedisDao: nil,
					spanProducer: nil,
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
					annoDao:      daomock.NewMockIAnnotationDao(ctrl),
					traceConfig:  traceConfigMock,
					spanRedisDao: nil,
					spanProducer: nil,
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
			r, err := NewTraceRepoImpl(
				fields.traceConfig,
				&mockStorageProvider{},
				fields.spanRedisDao,
				fields.spanProducer,
				WithTraceStorageAnnotationDao("ck", fields.annoDao),
			)
			assert.NoError(t, err)
			got, err := r.GetAnnotation(tt.args.ctx, tt.args.param)
			assert.Equal(t, tt.wantErr, err != nil)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTraceRepoImpl_ListAnnotations(t *testing.T) {
	type fields struct {
		annoDao      dao.IAnnotationDao
		traceConfig  config.ITraceConfig
		spanRedisDao *redis_dao_mock.MockISpansRedisDao
		spanProducer mq.ISpanProducer
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
				annoDaoMock := daomock.NewMockIAnnotationDao(ctrl)
				annoDaoMock.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*dao.Annotation{
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
					annoDao:      annoDaoMock,
					traceConfig:  traceConfigMock,
					spanRedisDao: nil,
					spanProducer: nil,
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
			r, err := NewTraceRepoImpl(
				fields.traceConfig,
				&mockStorageProvider{},
				fields.spanRedisDao,
				fields.spanProducer,
				WithTraceStorageAnnotationDao("ck", fields.annoDao),
			)
			assert.NoError(t, err)
			got, err := r.ListAnnotations(tt.args.ctx, tt.args.param)
			assert.Equal(t, tt.wantErr, err != nil)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTraceRepoImpl_GetPreSpanIDs(t *testing.T) {
	type fields struct {
		spanRedisDao *redis_dao_mock.MockISpansRedisDao
	}
	type args struct {
		ctx   context.Context
		param *repo.GetPreSpanIDsParam
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantPre      []string
		wantResp     []string
		wantErr      bool
	}{
		{
			name: "get pre span IDs successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				spanRedisDaoMock := redis_dao_mock.NewMockISpansRedisDao(ctrl)
				spanRedisDaoMock.EXPECT().GetPreSpans(gomock.Any(), "resp123").Return([]string{"pre1", "pre2"}, []string{"resp1", "resp2"}, nil)
				return fields{
					spanRedisDao: spanRedisDaoMock,
				}
			},
			args: args{
				ctx: context.Background(),
				param: &repo.GetPreSpanIDsParam{
					PreRespID: "resp123",
				},
			},
			wantPre:  []string{"pre1", "pre2"},
			wantResp: []string{"resp1", "resp2"},
			wantErr:  false,
		},
		{
			name: "get pre span IDs with error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				spanRedisDaoMock := redis_dao_mock.NewMockISpansRedisDao(ctrl)
				spanRedisDaoMock.EXPECT().GetPreSpans(gomock.Any(), "resp123").Return(nil, nil, assert.AnError)
				return fields{
					spanRedisDao: spanRedisDaoMock,
				}
			},
			args: args{
				ctx: context.Background(),
				param: &repo.GetPreSpanIDsParam{
					PreRespID: "resp123",
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
			r := &TraceRepoImpl{
				spanRedisDao: fields.spanRedisDao,
			}
			pre, resp, err := r.GetPreSpanIDs(tt.args.ctx, tt.args.param)
			assert.Equal(t, tt.wantErr, err != nil)
			if !tt.wantErr {
				assert.Equal(t, tt.wantPre, pre)
				assert.Equal(t, tt.wantResp, resp)
			}
		})
	}
}

func TestTraceRepoImpl_addPageTokenFilter(t *testing.T) {
	type args struct {
		pageToken *PageToken
		filter    *loop_span.FilterFields
	}
	tests := []struct {
		name string
		args args
		want *loop_span.FilterFields
	}{
		{
			name: "add page token filter with nil filter",
			args: args{
				pageToken: &PageToken{
					StartTime: 1234567890,
					SpanID:    "span123",
				},
				filter: nil,
			},
			want: &loop_span.FilterFields{
				QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumOr),
				FilterFields: []*loop_span.FilterField{
					{
						FieldName: loop_span.SpanFieldStartTime,
						FieldType: loop_span.FieldTypeLong,
						Values:    []string{"1234567890"},
						QueryType: ptr.Of(loop_span.QueryTypeEnumLt),
					},
					{
						FieldName:  loop_span.SpanFieldStartTime,
						FieldType:  loop_span.FieldTypeLong,
						Values:     []string{"1234567890"},
						QueryType:  ptr.Of(loop_span.QueryTypeEnumEq),
						QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
						SubFilter: &loop_span.FilterFields{
							QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
							FilterFields: []*loop_span.FilterField{
								{
									FieldName: loop_span.SpanFieldSpanId,
									FieldType: loop_span.FieldTypeString,
									Values:    []string{"span123"},
									QueryType: ptr.Of(loop_span.QueryTypeEnumLt),
								},
							},
						},
					},
				},
			},
		},
		{
			name: "add page token filter with existing filter",
			args: args{
				pageToken: &PageToken{
					StartTime: 1234567890,
					SpanID:    "span123",
				},
				filter: &loop_span.FilterFields{
					QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
					FilterFields: []*loop_span.FilterField{
						{
							FieldName: loop_span.SpanFieldSpanType,
							FieldType: loop_span.FieldTypeString,
							Values:    []string{"http"},
							QueryType: ptr.Of(loop_span.QueryTypeEnumEq),
						},
					},
				},
			},
			want: &loop_span.FilterFields{
				QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
				FilterFields: []*loop_span.FilterField{
					{
						SubFilter: &loop_span.FilterFields{
							QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumOr),
							FilterFields: []*loop_span.FilterField{
								{
									FieldName: loop_span.SpanFieldStartTime,
									FieldType: loop_span.FieldTypeLong,
									Values:    []string{"1234567890"},
									QueryType: ptr.Of(loop_span.QueryTypeEnumLt),
								},
								{
									FieldName:  loop_span.SpanFieldStartTime,
									FieldType:  loop_span.FieldTypeLong,
									Values:     []string{"1234567890"},
									QueryType:  ptr.Of(loop_span.QueryTypeEnumEq),
									QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
									SubFilter: &loop_span.FilterFields{
										QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
										FilterFields: []*loop_span.FilterField{
											{
												FieldName: loop_span.SpanFieldSpanId,
												FieldType: loop_span.FieldTypeString,
												Values:    []string{"span123"},
												QueryType: ptr.Of(loop_span.QueryTypeEnumLt),
											},
										},
									},
								},
							},
						},
					},
					{
						SubFilter: &loop_span.FilterFields{
							QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
							FilterFields: []*loop_span.FilterField{
								{
									FieldName: loop_span.SpanFieldSpanType,
									FieldType: loop_span.FieldTypeString,
									Values:    []string{"http"},
									QueryType: ptr.Of(loop_span.QueryTypeEnumEq),
								},
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &TraceRepoImpl{}
			got := r.addPageTokenFilter(tt.args.pageToken, tt.args.filter)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_parsePageToken(t *testing.T) {
	type args struct {
		pageToken string
	}
	tests := []struct {
		name    string
		args    args
		want    *PageToken
		wantErr bool
	}{
		{
			name: "parse empty page token",
			args: args{
				pageToken: "",
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "parse valid page token",
			args: args{
				pageToken: "eyJTdGFydFRpbWUiOjEyMzQ1Njc4OTAsIlNwYW5JRCI6InNwYW4xMjMifQ==",
			},
			want: &PageToken{
				StartTime: 1234567890,
				SpanID:    "span123",
			},
			wantErr: false,
		},
		{
			name: "parse invalid base64 page token",
			args: args{
				pageToken: "invalid-base64!",
			},
			wantErr: true,
		},
		{
			name: "parse invalid json page token",
			args: args{
				pageToken: "eyJpbnZhbGlkIjogImpzb24ifQ==", // {"invalid": "json"}
			},
			want:    &PageToken{}, // Will unmarshal to empty struct
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePageToken(tt.args.pageToken)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestWithTraceStorageSpanDao(t *testing.T) {
	tests := []struct {
		name        string
		storageType string
		spanDao     dao.ISpansDao
		wantNil     bool
	}{
		{
			name:        "with valid storage type and dao",
			storageType: "ck",
			spanDao:     &daomock.MockISpansDao{},
			wantNil:     false,
		},
		{
			name:        "with empty storage type",
			storageType: "",
			spanDao:     &daomock.MockISpansDao{},
			wantNil:     true,
		},
		{
			name:        "with nil dao",
			storageType: "ck",
			spanDao:     nil,
			wantNil:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opt := WithTraceStorageSpanDao(tt.storageType, tt.spanDao)
			impl := &TraceRepoImpl{
				spanDaos: make(map[string]dao.ISpansDao),
			}
			opt(impl)
			if tt.wantNil {
				assert.Nil(t, impl.spanDaos[tt.storageType])
			} else {
				assert.Equal(t, tt.spanDao, impl.spanDaos[tt.storageType])
			}
		})
	}
}

func TestWithTraceStorageAnnotationDao(t *testing.T) {
	tests := []struct {
		name        string
		storageType string
		annoDao     dao.IAnnotationDao
		wantNil     bool
	}{
		{
			name:        "with valid storage type and dao",
			storageType: "ck",
			annoDao:     &daomock.MockIAnnotationDao{},
			wantNil:     false,
		},
		{
			name:        "with empty storage type",
			storageType: "",
			annoDao:     &daomock.MockIAnnotationDao{},
			wantNil:     true,
		},
		{
			name:        "with nil dao",
			storageType: "ck",
			annoDao:     nil,
			wantNil:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opt := WithTraceStorageAnnotationDao(tt.storageType, tt.annoDao)
			impl := &TraceRepoImpl{
				annoDaos: make(map[string]dao.IAnnotationDao),
			}
			opt(impl)
			if tt.wantNil {
				assert.Nil(t, impl.annoDaos[tt.storageType])
			} else {
				assert.Equal(t, tt.annoDao, impl.annoDaos[tt.storageType])
			}
		})
	}
}

func TestTraceRepoImpl_getSpanInsertTable(t *testing.T) {
	type fields struct {
		traceConfig config.ITraceConfig
	}
	type args struct {
		ctx    context.Context
		tenant string
		ttl    loop_span.TTL
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         string
		wantErr      bool
	}{
		{
			name: "get span insert table successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceConfigMock := confmocks.NewMockITraceConfig(ctrl)
				traceConfigMock.EXPECT().GetTenantConfig(gomock.Any()).Return(&config.TenantCfg{
					TenantTables: map[string]map[loop_span.TTL]config.TableCfg{
						"test": {
							loop_span.TTL3d: {
								SpanTable: "spans_test",
							},
						},
					},
				}, nil)
				return fields{
					traceConfig: traceConfigMock,
				}
			},
			args: args{
				ctx:    context.Background(),
				tenant: "test",
				ttl:    loop_span.TTL3d,
			},
			want:    "spans_test",
			wantErr: false,
		},
		{
			name: "get span insert table with config error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceConfigMock := confmocks.NewMockITraceConfig(ctrl)
				traceConfigMock.EXPECT().GetTenantConfig(gomock.Any()).Return(nil, assert.AnError)
				return fields{
					traceConfig: traceConfigMock,
				}
			},
			args: args{
				ctx:    context.Background(),
				tenant: "test",
				ttl:    loop_span.TTL3d,
			},
			wantErr: true,
		},
		{
			name: "get span insert table not found",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceConfigMock := confmocks.NewMockITraceConfig(ctrl)
				traceConfigMock.EXPECT().GetTenantConfig(gomock.Any()).Return(&config.TenantCfg{
					TenantTables: map[string]map[loop_span.TTL]config.TableCfg{
						"other": {
							loop_span.TTL3d: {
								SpanTable: "spans_other",
							},
						},
					},
				}, nil)
				return fields{
					traceConfig: traceConfigMock,
				}
			},
			args: args{
				ctx:    context.Background(),
				tenant: "test",
				ttl:    loop_span.TTL3d,
			},
			wantErr: true,
		},
		{
			name: "get span insert table with empty table name",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceConfigMock := confmocks.NewMockITraceConfig(ctrl)
				traceConfigMock.EXPECT().GetTenantConfig(gomock.Any()).Return(&config.TenantCfg{
					TenantTables: map[string]map[loop_span.TTL]config.TableCfg{
						"test": {
							loop_span.TTL3d: {
								SpanTable: "",
							},
						},
					},
				}, nil)
				return fields{
					traceConfig: traceConfigMock,
				}
			},
			args: args{
				ctx:    context.Background(),
				tenant: "test",
				ttl:    loop_span.TTL3d,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			r := &TraceRepoImpl{
				traceConfig: fields.traceConfig,
			}
			got, err := r.getSpanInsertTable(tt.args.ctx, tt.args.tenant, tt.args.ttl)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
