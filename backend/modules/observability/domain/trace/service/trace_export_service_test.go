// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config"
	confmocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/metrics"
	metricmocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/mq"
	mqmocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/mq/mocks"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/tenant"
	tenantmocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/tenant/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/samber/lo"
)

func TestTraceExportServiceImpl_ExportTracesToDataset(t *testing.T) {
	type fields struct {
		traceRepo             repo.ITraceRepo
		traceConfig           config.ITraceConfig
		traceProducer         mq.ITraceProducer
		annotationProducer    mq.IAnnotationProducer
		metrics               metrics.ITraceMetrics
		tenantProvider        tenant.ITenantProvider
		DatasetServiceAdaptor *DatasetServiceAdaptor
	}
	type args struct {
		ctx context.Context
		req *ExportTracesToDatasetRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *ExportTracesToDatasetResponse
		wantErr      bool
	}{
		{
			name: "export traces to dataset successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				datasetProviderMock := rpcmocks.NewMockIDatasetProvider(ctrl)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				traceProducerMock := mqmocks.NewMockITraceProducer(ctrl)
				annotationProducerMock := mqmocks.NewMockIAnnotationProducer(ctrl)
				metricsMock := metricmocks.NewMockITraceMetrics(ctrl)

				adaptor := NewDatasetServiceAdaptor()
				adaptor.Register(entity.DatasetCategory_General, datasetProviderMock)

				tenantMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), loop_span.PlatformCozeLoop).Return([]string{"tenant1"}, nil)
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{
					Spans: []*loop_span.Span{
						{
							TraceID:     "trace-1",
							SpanID:      "span-1",
							WorkspaceID: "123",
							Input:       `{"name": "test"}`,
							Output:      `{"result": "success"}`,
						},
					},
				}, nil)
				datasetProviderMock.EXPECT().CreateDataset(gomock.Any(), gomock.Any()).Return(int64(100), nil)
				datasetProviderMock.EXPECT().GetDataset(gomock.Any(), int64(123), int64(100), entity.DatasetCategory_General).Return(&entity.Dataset{
					ID:              100,
					Name:            "test-dataset",
					DatasetCategory: entity.DatasetCategory_General,
					DatasetVersion: entity.DatasetVersion{
						DatasetSchema: entity.DatasetSchema{
							FieldSchemas: []entity.FieldSchema{
								{Name: "input", Key: ptr.Of("input")},
								{Name: "output", Key: ptr.Of("output")},
							},
						},
					},
				}, nil)
				datasetProviderMock.EXPECT().AddDatasetItems(gomock.Any(), int64(100), entity.DatasetCategory_General, gomock.Any()).Return([]*entity.DatasetItem{
					{SpanID: "span-1", DatasetID: 100},
				}, []entity.ItemErrorGroup{}, nil)

				return fields{
					traceRepo:             repoMock,
					traceConfig:           confMock,
					traceProducer:         traceProducerMock,
					annotationProducer:    annotationProducerMock,
					metrics:               metricsMock,
					tenantProvider:        tenantMock,
					DatasetServiceAdaptor: adaptor,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &ExportTracesToDatasetRequest{
					WorkspaceID:  123,
					SpanIds:      []SpanID{{TraceID: "trace-1", SpanID: "span-1"}},
					Category:     entity.DatasetCategory_General,
					Config:       DatasetConfig{IsNewDataset: true, DatasetName: ptr.Of("test-dataset"), DatasetSchema: entity.DatasetSchema{FieldSchemas: []entity.FieldSchema{{Name: "input"}, {Name: "output"}}}},
					StartTime:    time.Now().Unix() - 3600,
					EndTime:      time.Now().Unix(),
					PlatformType: loop_span.PlatformCozeLoop,
					ExportType:   ExportType_Append,
					FieldMappings: []entity.FieldMapping{
						{TraceFieldKey: "input", FieldSchema: entity.FieldSchema{Name: "input"}},
						{TraceFieldKey: "output", FieldSchema: entity.FieldSchema{Name: "output"}},
					},
				},
			},
			want: &ExportTracesToDatasetResponse{
				SuccessCount: 1,
				DatasetID:    100,
				DatasetName:  "test-dataset",
				Errors:       []entity.ItemErrorGroup{},
			},
			wantErr: false,
		},
		{
			name: "export traces to dataset with no spans found",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				traceProducerMock := mqmocks.NewMockITraceProducer(ctrl)
				annotationProducerMock := mqmocks.NewMockIAnnotationProducer(ctrl)
				metricsMock := metricmocks.NewMockITraceMetrics(ctrl)

				adaptor := NewDatasetServiceAdaptor()

				tenantMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), loop_span.PlatformCozeLoop).Return([]string{"tenant1"}, nil)
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{
					Spans: []*loop_span.Span{},
				}, nil)

				return fields{
					traceRepo:             repoMock,
					traceConfig:           confMock,
					traceProducer:         traceProducerMock,
					annotationProducer:    annotationProducerMock,
					metrics:               metricsMock,
					tenantProvider:        tenantMock,
					DatasetServiceAdaptor: adaptor,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &ExportTracesToDatasetRequest{
					WorkspaceID:  123,
					SpanIds:      []SpanID{{TraceID: "trace-1", SpanID: "span-1"}},
					Category:     entity.DatasetCategory_General,
					Config:       DatasetConfig{IsNewDataset: true, DatasetName: ptr.Of("test-dataset")},
					StartTime:    time.Now().Unix() - 3600,
					EndTime:      time.Now().Unix(),
					PlatformType: loop_span.PlatformCozeLoop,
					ExportType:   ExportType_Append,
				},
			},
			want:    &ExportTracesToDatasetResponse{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			fields := tt.fieldsGetter(ctrl)
			r := &TraceExportServiceImpl{
				traceRepo:             fields.traceRepo,
				traceConfig:           fields.traceConfig,
				traceProducer:         fields.traceProducer,
				annotationProducer:    fields.annotationProducer,
				metrics:               fields.metrics,
				tenantProvider:        fields.tenantProvider,
				DatasetServiceAdaptor: fields.DatasetServiceAdaptor,
			}

			got, err := r.ExportTracesToDataset(tt.args.ctx, tt.args.req)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestTraceExportServiceImpl_buildPreviewDataset(t *testing.T) {
	type args struct {
		ctx         context.Context
		workspaceID int64
		category    entity.DatasetCategory
		config      DatasetConfig
	}
	tests := []struct {
		name    string
		args    args
		want    *entity.Dataset
		wantErr bool
	}{
		{
			name: "build preview dataset with existing keys",
			args: args{
				ctx:         context.Background(),
				workspaceID: 123,
				category:    entity.DatasetCategory_General,
				config: DatasetConfig{
					DatasetID:   ptr.Of(int64(100)),
					DatasetName: ptr.Of("test-dataset"),
					DatasetSchema: entity.DatasetSchema{
						FieldSchemas: []entity.FieldSchema{
							{Name: "input", Key: ptr.Of("input_key")},
							{Name: "output", Key: ptr.Of("output_key")},
						},
					},
				},
			},
			want: &entity.Dataset{
				ID:              100,
				Name:            "test-dataset",
				WorkspaceID:     123,
				DatasetCategory: entity.DatasetCategory_General,
				DatasetVersion: entity.DatasetVersion{
					DatasetSchema: entity.DatasetSchema{
						FieldSchemas: []entity.FieldSchema{
							{Name: "input", Key: ptr.Of("input_key")},
							{Name: "output", Key: ptr.Of("output_key")},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "build preview dataset with nil keys - auto generate",
			args: args{
				ctx:         context.Background(),
				workspaceID: 456,
				category:    entity.DatasetCategory_Evaluation,
				config: DatasetConfig{
					DatasetSchema: entity.DatasetSchema{
						FieldSchemas: []entity.FieldSchema{
							{Name: "input", Key: nil},
							{Name: "output", Key: nil},
						},
					},
				},
			},
			want: &entity.Dataset{
				ID:              0,
				Name:            "",
				WorkspaceID:     456,
				DatasetCategory: entity.DatasetCategory_Evaluation,
				DatasetVersion: entity.DatasetVersion{
					DatasetSchema: entity.DatasetSchema{
						FieldSchemas: []entity.FieldSchema{
							{Name: "input", Key: ptr.Of("input")},
							{Name: "output", Key: ptr.Of("output")},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "build preview dataset with empty keys - auto generate",
			args: args{
				ctx:         context.Background(),
				workspaceID: 789,
				category:    entity.DatasetCategory_General,
				config: DatasetConfig{
					DatasetID:   ptr.Of(int64(200)),
					DatasetName: ptr.Of("another-dataset"),
					DatasetSchema: entity.DatasetSchema{
						FieldSchemas: []entity.FieldSchema{
							{Name: "question", Key: ptr.Of("")},
							{Name: "answer", Key: ptr.Of("")},
						},
					},
				},
			},
			want: &entity.Dataset{
				ID:              200,
				Name:            "another-dataset",
				WorkspaceID:     789,
				DatasetCategory: entity.DatasetCategory_General,
				DatasetVersion: entity.DatasetVersion{
					DatasetSchema: entity.DatasetSchema{
						FieldSchemas: []entity.FieldSchema{
							{Name: "question", Key: ptr.Of("question")},
							{Name: "answer", Key: ptr.Of("answer")},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "build preview dataset with mixed key states",
			args: args{
				ctx:         context.Background(),
				workspaceID: 999,
				category:    entity.DatasetCategory_General,
				config: DatasetConfig{
					DatasetSchema: entity.DatasetSchema{
						FieldSchemas: []entity.FieldSchema{
							{Name: "field1", Key: ptr.Of("existing_key")},
							{Name: "field2", Key: nil},
							{Name: "field3", Key: ptr.Of("")},
							{Name: "field4", Key: ptr.Of("another_key")},
						},
					},
				},
			},
			want: &entity.Dataset{
				ID:              0,
				Name:            "",
				WorkspaceID:     999,
				DatasetCategory: entity.DatasetCategory_General,
				DatasetVersion: entity.DatasetVersion{
					DatasetSchema: entity.DatasetSchema{
						FieldSchemas: []entity.FieldSchema{
							{Name: "field1", Key: ptr.Of("existing_key")},
							{Name: "field2", Key: ptr.Of("field2")},
							{Name: "field3", Key: ptr.Of("field3")},
							{Name: "field4", Key: ptr.Of("another_key")},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "build preview dataset with empty schema",
			args: args{
				ctx:         context.Background(),
				workspaceID: 111,
				category:    entity.DatasetCategory_General,
				config: DatasetConfig{
					DatasetID:   ptr.Of(int64(300)),
					DatasetName: ptr.Of("empty-schema-dataset"),
					DatasetSchema: entity.DatasetSchema{
						FieldSchemas: []entity.FieldSchema{},
					},
				},
			},
			want: &entity.Dataset{
				ID:              300,
				Name:            "empty-schema-dataset",
				WorkspaceID:     111,
				DatasetCategory: entity.DatasetCategory_General,
				DatasetVersion: entity.DatasetVersion{
					DatasetSchema: entity.DatasetSchema{
						FieldSchemas: []entity.FieldSchema{},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "build preview dataset without dataset id and name",
			args: args{
				ctx:         context.Background(),
				workspaceID: 222,
				category:    entity.DatasetCategory_Evaluation,
				config: DatasetConfig{
					DatasetID:   nil,
					DatasetName: nil,
					DatasetSchema: entity.DatasetSchema{
						FieldSchemas: []entity.FieldSchema{
							{Name: "prompt", Key: ptr.Of("prompt_key")},
						},
					},
				},
			},
			want: &entity.Dataset{
				ID:              0,
				Name:            "",
				WorkspaceID:     222,
				DatasetCategory: entity.DatasetCategory_Evaluation,
				DatasetVersion: entity.DatasetVersion{
					DatasetSchema: entity.DatasetSchema{
						FieldSchemas: []entity.FieldSchema{
							{Name: "prompt", Key: ptr.Of("prompt_key")},
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &TraceExportServiceImpl{}
			got, err := r.buildPreviewDataset(tt.args.ctx, tt.args.workspaceID, tt.args.category, tt.args.config)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want.ID, got.ID)
				assert.Equal(t, tt.want.Name, got.Name)
				assert.Equal(t, tt.want.WorkspaceID, got.WorkspaceID)
				assert.Equal(t, tt.want.DatasetCategory, got.DatasetCategory)
				assert.Equal(t, len(tt.want.DatasetVersion.DatasetSchema.FieldSchemas), len(got.DatasetVersion.DatasetSchema.FieldSchemas))

				// 检查每个字段的 key 是否正确设置
				for i, expectedField := range tt.want.DatasetVersion.DatasetSchema.FieldSchemas {
					gotField := got.DatasetVersion.DatasetSchema.FieldSchemas[i]
					assert.Equal(t, expectedField.Name, gotField.Name)
					assert.Equal(t, *expectedField.Key, *gotField.Key)
				}
			}
		})
	}
}

func TestTraceExportServiceImpl_PreviewExportTracesToDataset(t *testing.T) {
	type fields struct {
		traceRepo             repo.ITraceRepo
		traceConfig           config.ITraceConfig
		traceProducer         mq.ITraceProducer
		annotationProducer    mq.IAnnotationProducer
		metrics               metrics.ITraceMetrics
		tenantProvider        tenant.ITenantProvider
		DatasetServiceAdaptor *DatasetServiceAdaptor
	}
	type args struct {
		ctx context.Context
		req *ExportTracesToDatasetRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *PreviewExportTracesToDatasetResponse
		wantErr      bool
	}{
		{
			name: "preview export traces to new dataset successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				datasetProviderMock := rpcmocks.NewMockIDatasetProvider(ctrl)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				traceProducerMock := mqmocks.NewMockITraceProducer(ctrl)
				annotationProducerMock := mqmocks.NewMockIAnnotationProducer(ctrl)
				metricsMock := metricmocks.NewMockITraceMetrics(ctrl)

				adaptor := NewDatasetServiceAdaptor()
				adaptor.Register(entity.DatasetCategory_General, datasetProviderMock)

				testSpan := &loop_span.Span{
					TraceID:     "trace-123",
					SpanID:      "span-456",
					WorkspaceID: "123",
					Input:       `{"question": "test input"}`,
					Output:      `{"answer": "test output"}`,
				}

				testItem := &entity.DatasetItem{
					SpanID:      "span-456",
					WorkspaceID: 123,
					DatasetID:   0,
					FieldData: []*entity.FieldData{
						{Key: "input", Name: "input", Content: &entity.Content{ContentType: entity.ContentType_Text, Text: ""}},
						{Key: "output", Name: "output", Content: &entity.Content{ContentType: entity.ContentType_Text, Text: ""}},
					},
				}

				tenantMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), loop_span.PlatformCozeLoop).Return([]string{"tenant1"}, nil)
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{
					Spans: []*loop_span.Span{testSpan},
				}, nil)
				datasetProviderMock.EXPECT().ValidateDatasetItems(gomock.Any(), gomock.Any(), gomock.Any(), (*bool)(nil)).Return(
					[]*entity.DatasetItem{testItem}, []entity.ItemErrorGroup{}, nil)

				return fields{
					traceRepo:             repoMock,
					traceConfig:           confMock,
					traceProducer:         traceProducerMock,
					annotationProducer:    annotationProducerMock,
					metrics:               metricsMock,
					tenantProvider:        tenantMock,
					DatasetServiceAdaptor: adaptor,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &ExportTracesToDatasetRequest{
					WorkspaceID: 123,
					SpanIds:     []SpanID{{TraceID: "trace-123", SpanID: "span-456"}},
					Category:    entity.DatasetCategory_General,
					Config: DatasetConfig{
						IsNewDataset: true,
						DatasetName:  ptr.Of("test-dataset"),
						DatasetSchema: entity.DatasetSchema{
							FieldSchemas: []entity.FieldSchema{
								{Name: "input", ContentType: entity.ContentType_Text},
								{Name: "output", ContentType: entity.ContentType_Text},
							},
						},
					},
					StartTime:    time.Now().Unix() - 3600,
					EndTime:      time.Now().Unix(),
					PlatformType: loop_span.PlatformCozeLoop,
					ExportType:   ExportType_Append,
					FieldMappings: []entity.FieldMapping{
						{TraceFieldKey: "input", FieldSchema: entity.FieldSchema{Name: "input", ContentType: entity.ContentType_Text}},
						{TraceFieldKey: "output", FieldSchema: entity.FieldSchema{Name: "output", ContentType: entity.ContentType_Text}},
					},
				},
			},
			want: &PreviewExportTracesToDatasetResponse{
				Items: []*entity.DatasetItem{{
					SpanID:      "span-456",
					WorkspaceID: 123,
					DatasetID:   0,
					FieldData: []*entity.FieldData{
						{Key: "input", Name: "input", Content: &entity.Content{ContentType: entity.ContentType_Text, Text: ""}},
						{Key: "output", Name: "output", Content: &entity.Content{ContentType: entity.ContentType_Text, Text: ""}},
					},
				}},
				Errors: []entity.ItemErrorGroup{},
			},
			wantErr: false,
		},
		{
			name: "preview export traces to existing dataset with overwrite mode",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				datasetProviderMock := rpcmocks.NewMockIDatasetProvider(ctrl)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				traceProducerMock := mqmocks.NewMockITraceProducer(ctrl)
				annotationProducerMock := mqmocks.NewMockIAnnotationProducer(ctrl)
				metricsMock := metricmocks.NewMockITraceMetrics(ctrl)

				adaptor := NewDatasetServiceAdaptor()
				adaptor.Register(entity.DatasetCategory_General, datasetProviderMock)

				testSpan := &loop_span.Span{
					TraceID:     "trace-123",
					SpanID:      "span-456",
					WorkspaceID: "123",
					Input:       `{"question": "test input"}`,
					Output:      `{"answer": "test output"}`,
				}

				testItem := &entity.DatasetItem{
					SpanID:      "span-456",
					WorkspaceID: 123,
					DatasetID:   100,
					FieldData: []*entity.FieldData{
						{Key: "input", Name: "input", Content: &entity.Content{ContentType: entity.ContentType_Text, Text: ""}},
						{Key: "output", Name: "output", Content: &entity.Content{ContentType: entity.ContentType_Text, Text: ""}},
					},
				}

				tenantMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), loop_span.PlatformCozeLoop).Return([]string{"tenant1"}, nil)
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{
					Spans: []*loop_span.Span{testSpan},
				}, nil)
				// 关键点：验证 ignoreCurrentCount 参数为 true
				datasetProviderMock.EXPECT().ValidateDatasetItems(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Eq(lo.ToPtr(true))).Return(
					[]*entity.DatasetItem{testItem}, []entity.ItemErrorGroup{}, nil)

				return fields{
					traceRepo:             repoMock,
					traceConfig:           confMock,
					traceProducer:         traceProducerMock,
					annotationProducer:    annotationProducerMock,
					metrics:               metricsMock,
					tenantProvider:        tenantMock,
					DatasetServiceAdaptor: adaptor,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &ExportTracesToDatasetRequest{
					WorkspaceID: 123,
					SpanIds:     []SpanID{{TraceID: "trace-123", SpanID: "span-456"}},
					Category:    entity.DatasetCategory_General,
					Config: DatasetConfig{
						IsNewDataset: false,
						DatasetID:    ptr.Of(int64(100)),
						DatasetName:  ptr.Of("existing-dataset"),
						DatasetSchema: entity.DatasetSchema{
							FieldSchemas: []entity.FieldSchema{
								{Name: "input", ContentType: entity.ContentType_Text},
								{Name: "output", ContentType: entity.ContentType_Text},
							},
						},
					},
					StartTime:    time.Now().Unix() - 3600,
					EndTime:      time.Now().Unix(),
					PlatformType: loop_span.PlatformCozeLoop,
					ExportType:   ExportType_Overwrite, // 覆盖模式
					FieldMappings: []entity.FieldMapping{
						{TraceFieldKey: "input", FieldSchema: entity.FieldSchema{Name: "input", ContentType: entity.ContentType_Text}},
						{TraceFieldKey: "output", FieldSchema: entity.FieldSchema{Name: "output", ContentType: entity.ContentType_Text}},
					},
				},
			},
			want: &PreviewExportTracesToDatasetResponse{
				Items: []*entity.DatasetItem{{
					SpanID:      "span-456",
					WorkspaceID: 123,
					DatasetID:   100,
					FieldData: []*entity.FieldData{
						{Key: "input", Name: "input", Content: &entity.Content{ContentType: entity.ContentType_Text, Text: ""}},
						{Key: "output", Name: "output", Content: &entity.Content{ContentType: entity.ContentType_Text, Text: ""}},
					},
				}},
				Errors: []entity.ItemErrorGroup{},
			},
			wantErr: false,
		},
		{
			name: "preview export traces to existing dataset with append mode",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				datasetProviderMock := rpcmocks.NewMockIDatasetProvider(ctrl)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				traceProducerMock := mqmocks.NewMockITraceProducer(ctrl)
				annotationProducerMock := mqmocks.NewMockIAnnotationProducer(ctrl)
				metricsMock := metricmocks.NewMockITraceMetrics(ctrl)

				adaptor := NewDatasetServiceAdaptor()
				adaptor.Register(entity.DatasetCategory_General, datasetProviderMock)

				testSpan := &loop_span.Span{
					TraceID:     "trace-123",
					SpanID:      "span-456",
					WorkspaceID: "123",
					Input:       `{"question": "test input"}`,
					Output:      `{"answer": "test output"}`,
				}

				testItem := &entity.DatasetItem{
					SpanID:      "span-456",
					WorkspaceID: 123,
					DatasetID:   100,
					FieldData: []*entity.FieldData{
						{Key: "input", Name: "input", Content: &entity.Content{ContentType: entity.ContentType_Text, Text: ""}},
						{Key: "output", Name: "output", Content: &entity.Content{ContentType: entity.ContentType_Text, Text: ""}},
					},
				}

				tenantMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), loop_span.PlatformCozeLoop).Return([]string{"tenant1"}, nil)
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{
					Spans: []*loop_span.Span{testSpan},
				}, nil)
				// 关键点：验证 ignoreCurrentCount 参数为 nil
				datasetProviderMock.EXPECT().ValidateDatasetItems(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Eq((*bool)(nil))).Return(
					[]*entity.DatasetItem{testItem}, []entity.ItemErrorGroup{}, nil)

				return fields{
					traceRepo:             repoMock,
					traceConfig:           confMock,
					traceProducer:         traceProducerMock,
					annotationProducer:    annotationProducerMock,
					metrics:               metricsMock,
					tenantProvider:        tenantMock,
					DatasetServiceAdaptor: adaptor,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &ExportTracesToDatasetRequest{
					WorkspaceID: 123,
					SpanIds:     []SpanID{{TraceID: "trace-123", SpanID: "span-456"}},
					Category:    entity.DatasetCategory_General,
					Config: DatasetConfig{
						IsNewDataset: false,
						DatasetID:    ptr.Of(int64(100)),
						DatasetName:  ptr.Of("existing-dataset"),
						DatasetSchema: entity.DatasetSchema{
							FieldSchemas: []entity.FieldSchema{
								{Name: "input", ContentType: entity.ContentType_Text},
								{Name: "output", ContentType: entity.ContentType_Text},
							},
						},
					},
					StartTime:    time.Now().Unix() - 3600,
					EndTime:      time.Now().Unix(),
					PlatformType: loop_span.PlatformCozeLoop,
					ExportType:   ExportType_Append, // 追加模式
					FieldMappings: []entity.FieldMapping{
						{TraceFieldKey: "input", FieldSchema: entity.FieldSchema{Name: "input", ContentType: entity.ContentType_Text}},
						{TraceFieldKey: "output", FieldSchema: entity.FieldSchema{Name: "output", ContentType: entity.ContentType_Text}},
					},
				},
			},
			want: &PreviewExportTracesToDatasetResponse{
				Items: []*entity.DatasetItem{{
					SpanID:      "span-456",
					WorkspaceID: 123,
					DatasetID:   100,
					FieldData: []*entity.FieldData{
						{Key: "input", Name: "input", Content: &entity.Content{ContentType: entity.ContentType_Text, Text: ""}},
						{Key: "output", Name: "output", Content: &entity.Content{ContentType: entity.ContentType_Text, Text: ""}},
					},
				}},
				Errors: []entity.ItemErrorGroup{},
			},
			wantErr: false,
		},
		{
			name: "get tenants by platform type failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				traceProducerMock := mqmocks.NewMockITraceProducer(ctrl)
				annotationProducerMock := mqmocks.NewMockIAnnotationProducer(ctrl)
				metricsMock := metricmocks.NewMockITraceMetrics(ctrl)

				adaptor := NewDatasetServiceAdaptor()

				tenantMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), loop_span.PlatformCozeLoop).Return(nil, assert.AnError)

				return fields{
					traceRepo:             repoMock,
					traceConfig:           confMock,
					traceProducer:         traceProducerMock,
					annotationProducer:    annotationProducerMock,
					metrics:               metricsMock,
					tenantProvider:        tenantMock,
					DatasetServiceAdaptor: adaptor,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &ExportTracesToDatasetRequest{
					WorkspaceID:  123,
					SpanIds:      []SpanID{{TraceID: "trace-123", SpanID: "span-456"}},
					Category:     entity.DatasetCategory_General,
					Config:       DatasetConfig{IsNewDataset: true, DatasetName: ptr.Of("test-dataset")},
					StartTime:    time.Now().Unix() - 3600,
					EndTime:      time.Now().Unix(),
					PlatformType: loop_span.PlatformCozeLoop,
					ExportType:   ExportType_Append,
				},
			},
			want:    &PreviewExportTracesToDatasetResponse{},
			wantErr: true,
		},
		{
			name: "list spans failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				traceProducerMock := mqmocks.NewMockITraceProducer(ctrl)
				annotationProducerMock := mqmocks.NewMockIAnnotationProducer(ctrl)
				metricsMock := metricmocks.NewMockITraceMetrics(ctrl)

				adaptor := NewDatasetServiceAdaptor()

				tenantMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), loop_span.PlatformCozeLoop).Return([]string{"tenant1"}, nil)
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)

				return fields{
					traceRepo:             repoMock,
					traceConfig:           confMock,
					traceProducer:         traceProducerMock,
					annotationProducer:    annotationProducerMock,
					metrics:               metricsMock,
					tenantProvider:        tenantMock,
					DatasetServiceAdaptor: adaptor,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &ExportTracesToDatasetRequest{
					WorkspaceID:  123,
					SpanIds:      []SpanID{{TraceID: "trace-123", SpanID: "span-456"}},
					Category:     entity.DatasetCategory_General,
					Config:       DatasetConfig{IsNewDataset: true, DatasetName: ptr.Of("test-dataset")},
					StartTime:    time.Now().Unix() - 3600,
					EndTime:      time.Now().Unix(),
					PlatformType: loop_span.PlatformCozeLoop,
					ExportType:   ExportType_Append,
				},
			},
			want:    &PreviewExportTracesToDatasetResponse{},
			wantErr: true,
		},
		{
			name: "validate dataset items failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				datasetProviderMock := rpcmocks.NewMockIDatasetProvider(ctrl)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				traceProducerMock := mqmocks.NewMockITraceProducer(ctrl)
				annotationProducerMock := mqmocks.NewMockIAnnotationProducer(ctrl)
				metricsMock := metricmocks.NewMockITraceMetrics(ctrl)

				adaptor := NewDatasetServiceAdaptor()
				adaptor.Register(entity.DatasetCategory_General, datasetProviderMock)

				testSpan := &loop_span.Span{
					TraceID:     "trace-123",
					SpanID:      "span-456",
					WorkspaceID: "123",
					Input:       `{"question": "test input"}`,
					Output:      `{"answer": "test output"}`,
				}

				tenantMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), loop_span.PlatformCozeLoop).Return([]string{"tenant1"}, nil)
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{
					Spans: []*loop_span.Span{testSpan},
				}, nil)
				datasetProviderMock.EXPECT().ValidateDatasetItems(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, assert.AnError)

				return fields{
					traceRepo:             repoMock,
					traceConfig:           confMock,
					traceProducer:         traceProducerMock,
					annotationProducer:    annotationProducerMock,
					metrics:               metricsMock,
					tenantProvider:        tenantMock,
					DatasetServiceAdaptor: adaptor,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &ExportTracesToDatasetRequest{
					WorkspaceID: 123,
					SpanIds:     []SpanID{{TraceID: "trace-123", SpanID: "span-456"}},
					Category:    entity.DatasetCategory_General,
					Config: DatasetConfig{
						IsNewDataset: true,
						DatasetName:  ptr.Of("test-dataset"),
						DatasetSchema: entity.DatasetSchema{
							FieldSchemas: []entity.FieldSchema{
								{Name: "input", ContentType: entity.ContentType_Text},
								{Name: "output", ContentType: entity.ContentType_Text},
							},
						},
					},
					StartTime:    time.Now().Unix() - 3600,
					EndTime:      time.Now().Unix(),
					PlatformType: loop_span.PlatformCozeLoop,
					ExportType:   ExportType_Append,
					FieldMappings: []entity.FieldMapping{
						{TraceFieldKey: "input", FieldSchema: entity.FieldSchema{Name: "input", ContentType: entity.ContentType_Text}},
						{TraceFieldKey: "output", FieldSchema: entity.FieldSchema{Name: "output", ContentType: entity.ContentType_Text}},
					},
				},
			},
			want:    &PreviewExportTracesToDatasetResponse{},
			wantErr: true,
		},
		{
			name: "empty spans result",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				datasetProviderMock := rpcmocks.NewMockIDatasetProvider(ctrl)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				traceProducerMock := mqmocks.NewMockITraceProducer(ctrl)
				annotationProducerMock := mqmocks.NewMockIAnnotationProducer(ctrl)
				metricsMock := metricmocks.NewMockITraceMetrics(ctrl)

				adaptor := NewDatasetServiceAdaptor()
				adaptor.Register(entity.DatasetCategory_General, datasetProviderMock)

				tenantMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), loop_span.PlatformCozeLoop).Return([]string{"tenant1"}, nil)
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{
					Spans: []*loop_span.Span{},
				}, nil)
				datasetProviderMock.EXPECT().ValidateDatasetItems(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(
					[]*entity.DatasetItem{}, []entity.ItemErrorGroup{}, nil)

				return fields{
					traceRepo:             repoMock,
					traceConfig:           confMock,
					traceProducer:         traceProducerMock,
					annotationProducer:    annotationProducerMock,
					metrics:               metricsMock,
					tenantProvider:        tenantMock,
					DatasetServiceAdaptor: adaptor,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &ExportTracesToDatasetRequest{
					WorkspaceID: 123,
					SpanIds:     []SpanID{{TraceID: "trace-123", SpanID: "span-456"}},
					Category:    entity.DatasetCategory_General,
					Config: DatasetConfig{
						IsNewDataset: true,
						DatasetName:  ptr.Of("test-dataset"),
						DatasetSchema: entity.DatasetSchema{
							FieldSchemas: []entity.FieldSchema{
								{Name: "input", ContentType: entity.ContentType_Text},
								{Name: "output", ContentType: entity.ContentType_Text},
							},
						},
					},
					StartTime:    time.Now().Unix() - 3600,
					EndTime:      time.Now().Unix(),
					PlatformType: loop_span.PlatformCozeLoop,
					ExportType:   ExportType_Append,
					FieldMappings: []entity.FieldMapping{
						{TraceFieldKey: "input", FieldSchema: entity.FieldSchema{Name: "input", ContentType: entity.ContentType_Text}},
						{TraceFieldKey: "output", FieldSchema: entity.FieldSchema{Name: "output", ContentType: entity.ContentType_Text}},
					},
				},
			},
			want: &PreviewExportTracesToDatasetResponse{
				Items:  []*entity.DatasetItem{},
				Errors: []entity.ItemErrorGroup{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			fields := tt.fieldsGetter(ctrl)
			r := &TraceExportServiceImpl{
				traceRepo:             fields.traceRepo,
				traceConfig:           fields.traceConfig,
				traceProducer:         fields.traceProducer,
				annotationProducer:    fields.annotationProducer,
				metrics:               fields.metrics,
				tenantProvider:        fields.tenantProvider,
				DatasetServiceAdaptor: fields.DatasetServiceAdaptor,
			}

			got, err := r.PreviewExportTracesToDataset(tt.args.ctx, tt.args.req)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestTraceExportServiceImpl_ExportTracesToDataset_Additional(t *testing.T) {
	type fields struct {
		traceRepo             repo.ITraceRepo
		traceConfig           config.ITraceConfig
		traceProducer         mq.ITraceProducer
		annotationProducer    mq.IAnnotationProducer
		metrics               metrics.ITraceMetrics
		tenantProvider        tenant.ITenantProvider
		DatasetServiceAdaptor *DatasetServiceAdaptor
	}
	type args struct {
		ctx context.Context
		req *ExportTracesToDatasetRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *ExportTracesToDatasetResponse
		wantErr      bool
	}{
		// 测试用例：现有数据集DatasetID为nil的错误场景 (覆盖第220-222行)
		{
			name: "export to existing dataset with nil dataset id should fail",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				traceProducerMock := mqmocks.NewMockITraceProducer(ctrl)
				annotationProducerMock := mqmocks.NewMockIAnnotationProducer(ctrl)
				metricsMock := metricmocks.NewMockITraceMetrics(ctrl)

				adaptor := NewDatasetServiceAdaptor()

				// Mock GetTenantsByPlatformType 调用
				tenantMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), loop_span.PlatformCozeLoop).Return([]string{"tenant1"}, nil)
				// Mock ListSpans 调用，因为错误会在createOrUpdateDataset之前
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{
					Spans: []*loop_span.Span{},
				}, nil)

				return fields{
					traceRepo:             repoMock,
					traceConfig:           confMock,
					traceProducer:         traceProducerMock,
					annotationProducer:    annotationProducerMock,
					metrics:               metricsMock,
					tenantProvider:        tenantMock,
					DatasetServiceAdaptor: adaptor,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &ExportTracesToDatasetRequest{
					WorkspaceID: 123,
					SpanIds:     []SpanID{{TraceID: "trace-1", SpanID: "span-1"}},
					Category:    entity.DatasetCategory_General,
					Config: DatasetConfig{
						IsNewDataset: false,
						DatasetID:    nil, // 触发第220-222行错误
						DatasetSchema: entity.DatasetSchema{
							FieldSchemas: []entity.FieldSchema{
								{Name: "input", Key: nil},
							},
						},
					},
					StartTime:    time.Now().Unix() - 3600,
					EndTime:      time.Now().Unix(),
					PlatformType: loop_span.PlatformCozeLoop,
					ExportType:   ExportType_Append,
					FieldMappings: []entity.FieldMapping{
						{TraceFieldKey: "input", FieldSchema: entity.FieldSchema{Name: "input"}},
					},
				},
			},
			want:    &ExportTracesToDatasetResponse{},
			wantErr: true,
		},
		// 测试用例：现有数据集需要更新schema的成功场景 (覆盖第224-240行)
		{
			name: "export to existing dataset with schema update success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				datasetProviderMock := rpcmocks.NewMockIDatasetProvider(ctrl)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				traceProducerMock := mqmocks.NewMockITraceProducer(ctrl)
				annotationProducerMock := mqmocks.NewMockIAnnotationProducer(ctrl)
				metricsMock := metricmocks.NewMockITraceMetrics(ctrl)

				adaptor := NewDatasetServiceAdaptor()
				adaptor.Register(entity.DatasetCategory_General, datasetProviderMock)

				testSpan := &loop_span.Span{
					TraceID:     "trace-123",
					SpanID:      "span-456",
					WorkspaceID: "123",
					Input:       `{"question": "test input"}`,
					Output:      `{"answer": "test output"}`,
				}

				// Mock成功的导出流程
				tenantMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), loop_span.PlatformCozeLoop).Return([]string{"tenant1"}, nil)
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{
					Spans: []*loop_span.Span{testSpan},
				}, nil)

				// 关键：Mock数据集schema更新 (覆盖第232-240行)
				datasetProviderMock.EXPECT().UpdateDatasetSchema(gomock.Any(), gomock.Any()).Return(nil)
				datasetProviderMock.EXPECT().GetDataset(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.Dataset{
					ID:              100,
					DatasetCategory: entity.DatasetCategory_General,
					DatasetVersion: entity.DatasetVersion{
						DatasetSchema: entity.DatasetSchema{
							FieldSchemas: []entity.FieldSchema{
								{Name: "input", Key: ptr.Of("input")},
								{Name: "output", Key: ptr.Of("output")},
							},
						},
					},
				}, nil)

				// Mock数据集条目添加
				datasetProviderMock.EXPECT().AddDatasetItems(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(
					[]*entity.DatasetItem{{SpanID: "span-456", DatasetID: 100}}, []entity.ItemErrorGroup{}, nil)

				return fields{
					traceRepo:             repoMock,
					traceConfig:           confMock,
					traceProducer:         traceProducerMock,
					annotationProducer:    annotationProducerMock,
					metrics:               metricsMock,
					tenantProvider:        tenantMock,
					DatasetServiceAdaptor: adaptor,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &ExportTracesToDatasetRequest{
					WorkspaceID: 123,
					SpanIds:     []SpanID{{TraceID: "trace-123", SpanID: "span-456"}},
					Category:    entity.DatasetCategory_General,
					Config: DatasetConfig{
						IsNewDataset: false,
						DatasetID:    ptr.Of(int64(100)),
						DatasetSchema: entity.DatasetSchema{
							FieldSchemas: []entity.FieldSchema{
								{Name: "input", Key: nil},         // 触发第224-230行needUpdate=true
								{Name: "output", Key: ptr.Of("")}, // 触发第224-230行needUpdate=true
							},
						},
					},
					StartTime:    time.Now().Unix() - 3600,
					EndTime:      time.Now().Unix(),
					PlatformType: loop_span.PlatformCozeLoop,
					ExportType:   ExportType_Append,
					FieldMappings: []entity.FieldMapping{
						{TraceFieldKey: "input", FieldSchema: entity.FieldSchema{Name: "input", ContentType: entity.ContentType_Text}},
						{TraceFieldKey: "output", FieldSchema: entity.FieldSchema{Name: "output", ContentType: entity.ContentType_Text}},
					},
				},
			},
			want: &ExportTracesToDatasetResponse{
				SuccessCount: 1,
				DatasetID:    100,
				DatasetName:  "",
				Errors:       []entity.ItemErrorGroup{},
			},
			wantErr: false,
		},
		// 测试用例：现有数据集schema更新失败的错误处理场景 (覆盖第232-240行错误路径)
		{
			name: "export to existing dataset with schema update failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				datasetProviderMock := rpcmocks.NewMockIDatasetProvider(ctrl)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				traceProducerMock := mqmocks.NewMockITraceProducer(ctrl)
				annotationProducerMock := mqmocks.NewMockIAnnotationProducer(ctrl)
				metricsMock := metricmocks.NewMockITraceMetrics(ctrl)

				adaptor := NewDatasetServiceAdaptor()
				adaptor.Register(entity.DatasetCategory_General, datasetProviderMock)

				testSpan := &loop_span.Span{
					TraceID:     "trace-123",
					SpanID:      "span-456",
					WorkspaceID: "123",
					Input:       `{"question": "test input"}`,
					Output:      `{"answer": "test output"}`,
				}

				tenantMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), loop_span.PlatformCozeLoop).Return([]string{"tenant1"}, nil)
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{
					Spans: []*loop_span.Span{testSpan},
				}, nil)

				// Mock schema更新失败 (覆盖第238-240行错误处理)
				datasetProviderMock.EXPECT().UpdateDatasetSchema(gomock.Any(), gomock.Any()).Return(assert.AnError)

				return fields{
					traceRepo:             repoMock,
					traceConfig:           confMock,
					traceProducer:         traceProducerMock,
					annotationProducer:    annotationProducerMock,
					metrics:               metricsMock,
					tenantProvider:        tenantMock,
					DatasetServiceAdaptor: adaptor,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &ExportTracesToDatasetRequest{
					WorkspaceID: 123,
					SpanIds:     []SpanID{{TraceID: "trace-123", SpanID: "span-456"}},
					Category:    entity.DatasetCategory_General,
					Config: DatasetConfig{
						IsNewDataset: false,
						DatasetID:    ptr.Of(int64(100)),
						DatasetSchema: entity.DatasetSchema{
							FieldSchemas: []entity.FieldSchema{
								{Name: "input", Key: nil}, // 触发needUpdate=true
							},
						},
					},
					StartTime:    time.Now().Unix() - 3600,
					EndTime:      time.Now().Unix(),
					PlatformType: loop_span.PlatformCozeLoop,
					ExportType:   ExportType_Append,
					FieldMappings: []entity.FieldMapping{
						{TraceFieldKey: "input", FieldSchema: entity.FieldSchema{Name: "input"}},
					},
				},
			},
			want:    &ExportTracesToDatasetResponse{},
			wantErr: true,
		},
		// 测试用例：成功添加注解 - Evaluation类别 (覆盖第353-387行)
		{
			name: "export traces to evaluation dataset with annotations success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				datasetProviderMock := rpcmocks.NewMockIDatasetProvider(ctrl)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				traceProducerMock := mqmocks.NewMockITraceProducer(ctrl)
				annotationProducerMock := mqmocks.NewMockIAnnotationProducer(ctrl)
				metricsMock := metricmocks.NewMockITraceMetrics(ctrl)

				adaptor := NewDatasetServiceAdaptor()
				adaptor.Register(entity.DatasetCategory_Evaluation, datasetProviderMock)

				testSpan := &loop_span.Span{
					TraceID:     "trace-123",
					SpanID:      "span-456",
					WorkspaceID: "123",
					Input:       `{"question": "test input"}`,
					Output:      `{"answer": "test output"}`,
				}

				// Mock成功的导出流程
				tenantMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), loop_span.PlatformCozeLoop).Return([]string{"tenant1"}, nil)
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{
					Spans: []*loop_span.Span{testSpan},
				}, nil)

				// Mock数据集创建
				datasetProviderMock.EXPECT().CreateDataset(gomock.Any(), gomock.Any()).Return(int64(200), nil)
				datasetProviderMock.EXPECT().GetDataset(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.Dataset{
					ID:              200,
					Name:            "eval-dataset",
					DatasetCategory: entity.DatasetCategory_Evaluation,
					DatasetVersion: entity.DatasetVersion{
						DatasetSchema: entity.DatasetSchema{
							FieldSchemas: []entity.FieldSchema{
								{Name: "input", Key: ptr.Of("input")},
								{Name: "output", Key: ptr.Of("output")},
							},
						},
					},
				}, nil)

				// Mock数据集条目添加
				datasetProviderMock.EXPECT().AddDatasetItems(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(
					[]*entity.DatasetItem{{SpanID: "span-456", DatasetID: 200}}, []entity.ItemErrorGroup{}, nil)

				// Mock注解插入 (覆盖第353-387行)
				repoMock.EXPECT().InsertAnnotations(gomock.Any(), gomock.Any()).Return(nil)

				return fields{
					traceRepo:             repoMock,
					traceConfig:           confMock,
					traceProducer:         traceProducerMock,
					annotationProducer:    annotationProducerMock,
					metrics:               metricsMock,
					tenantProvider:        tenantMock,
					DatasetServiceAdaptor: adaptor,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "test-user"}), // 设置用户ID
				req: &ExportTracesToDatasetRequest{
					WorkspaceID: 123,
					SpanIds:     []SpanID{{TraceID: "trace-123", SpanID: "span-456"}},
					Category:    entity.DatasetCategory_Evaluation, // 触发第357-358行
					Config: DatasetConfig{
						IsNewDataset: true,
						DatasetName:  ptr.Of("eval-dataset"),
						DatasetSchema: entity.DatasetSchema{
							FieldSchemas: []entity.FieldSchema{
								{Name: "input", ContentType: entity.ContentType_Text},
								{Name: "output", ContentType: entity.ContentType_Text},
							},
						},
					},
					StartTime:    time.Now().Unix() - 3600,
					EndTime:      time.Now().Unix(),
					PlatformType: loop_span.PlatformCozeLoop,
					ExportType:   ExportType_Append,
					FieldMappings: []entity.FieldMapping{
						{TraceFieldKey: "input", FieldSchema: entity.FieldSchema{Name: "input", ContentType: entity.ContentType_Text}},
						{TraceFieldKey: "output", FieldSchema: entity.FieldSchema{Name: "output", ContentType: entity.ContentType_Text}},
					},
				},
			},
			want: &ExportTracesToDatasetResponse{
				SuccessCount: 1,
				DatasetID:    200,
				DatasetName:  "eval-dataset",
				Errors:       []entity.ItemErrorGroup{},
			},
			wantErr: false,
		},
		// 测试用例：Span未找到的边界情况 (覆盖第364-368行)
		{
			name: "export with span not found in span map",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				datasetProviderMock := rpcmocks.NewMockIDatasetProvider(ctrl)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				traceProducerMock := mqmocks.NewMockITraceProducer(ctrl)
				annotationProducerMock := mqmocks.NewMockIAnnotationProducer(ctrl)
				metricsMock := metricmocks.NewMockITraceMetrics(ctrl)

				adaptor := NewDatasetServiceAdaptor()
				adaptor.Register(entity.DatasetCategory_General, datasetProviderMock)

				testSpan := &loop_span.Span{
					TraceID:     "trace-123",
					SpanID:      "span-456",
					WorkspaceID: "123",
					Input:       `{"question": "test input"}`,
					Output:      `{"answer": "test output"}`,
				}

				// Mock成功的导出流程
				tenantMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), loop_span.PlatformCozeLoop).Return([]string{"tenant1"}, nil)
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{
					Spans: []*loop_span.Span{testSpan},
				}, nil)

				// Mock数据集创建
				datasetProviderMock.EXPECT().CreateDataset(gomock.Any(), gomock.Any()).Return(int64(300), nil)
				datasetProviderMock.EXPECT().GetDataset(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.Dataset{
					ID:              300,
					Name:            "test-dataset",
					DatasetCategory: entity.DatasetCategory_General,
					DatasetVersion: entity.DatasetVersion{
						DatasetSchema: entity.DatasetSchema{
							FieldSchemas: []entity.FieldSchema{
								{Name: "input", Key: ptr.Of("input")},
								{Name: "output", Key: ptr.Of("output")},
							},
						},
					},
				}, nil)

				// Mock数据集条目添加，返回不存在的SpanID (触发第364-368行)
				datasetProviderMock.EXPECT().AddDatasetItems(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(
					[]*entity.DatasetItem{{SpanID: "non-existent-span", DatasetID: 300}}, []entity.ItemErrorGroup{}, nil)

				// 不需要Mock注解相关操作，因为span未找到会跳过

				return fields{
					traceRepo:             repoMock,
					traceConfig:           confMock,
					traceProducer:         traceProducerMock,
					annotationProducer:    annotationProducerMock,
					metrics:               metricsMock,
					tenantProvider:        tenantMock,
					DatasetServiceAdaptor: adaptor,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "test-user"}),
				req: &ExportTracesToDatasetRequest{
					WorkspaceID: 123,
					SpanIds:     []SpanID{{TraceID: "trace-123", SpanID: "span-456"}},
					Category:    entity.DatasetCategory_General,
					Config: DatasetConfig{
						IsNewDataset: true,
						DatasetName:  ptr.Of("test-dataset"),
						DatasetSchema: entity.DatasetSchema{
							FieldSchemas: []entity.FieldSchema{
								{Name: "input", ContentType: entity.ContentType_Text},
								{Name: "output", ContentType: entity.ContentType_Text},
							},
						},
					},
					StartTime:    time.Now().Unix() - 3600,
					EndTime:      time.Now().Unix(),
					PlatformType: loop_span.PlatformCozeLoop,
					ExportType:   ExportType_Append,
					FieldMappings: []entity.FieldMapping{
						{TraceFieldKey: "input", FieldSchema: entity.FieldSchema{Name: "input", ContentType: entity.ContentType_Text}},
						{TraceFieldKey: "output", FieldSchema: entity.FieldSchema{Name: "output", ContentType: entity.ContentType_Text}},
					},
				},
			},
			want: &ExportTracesToDatasetResponse{
				SuccessCount: 1,
				DatasetID:    300,
				DatasetName:  "test-dataset",
				Errors:       []entity.ItemErrorGroup{},
			},
			wantErr: false,
		},
		// 测试用例：AddManualDatasetAnnotation失败的错误处理 (覆盖第369-374行)
		{
			name: "export with add manual dataset annotation failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				datasetProviderMock := rpcmocks.NewMockIDatasetProvider(ctrl)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				traceProducerMock := mqmocks.NewMockITraceProducer(ctrl)
				annotationProducerMock := mqmocks.NewMockIAnnotationProducer(ctrl)
				metricsMock := metricmocks.NewMockITraceMetrics(ctrl)

				adaptor := NewDatasetServiceAdaptor()
				adaptor.Register(entity.DatasetCategory_General, datasetProviderMock)

				testSpan := &loop_span.Span{
					TraceID:     "trace-123",
					SpanID:      "span-456",
					WorkspaceID: "123",
					Input:       `{"question": "test input"}`,
					Output:      `{"answer": "test output"}`,
				}

				// Mock成功的导出流程
				tenantMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), loop_span.PlatformCozeLoop).Return([]string{"tenant1"}, nil)
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{
					Spans: []*loop_span.Span{testSpan},
				}, nil)

				// Mock数据集创建
				datasetProviderMock.EXPECT().CreateDataset(gomock.Any(), gomock.Any()).Return(int64(400), nil)
				datasetProviderMock.EXPECT().GetDataset(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.Dataset{
					ID:              400,
					Name:            "test-dataset",
					DatasetCategory: entity.DatasetCategory_General,
					DatasetVersion: entity.DatasetVersion{
						DatasetSchema: entity.DatasetSchema{
							FieldSchemas: []entity.FieldSchema{
								{Name: "input", Key: ptr.Of("input")},
								{Name: "output", Key: ptr.Of("output")},
							},
						},
					},
				}, nil)

				// Mock数据集条目添加
				datasetProviderMock.EXPECT().AddDatasetItems(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(
					[]*entity.DatasetItem{{SpanID: "span-456", DatasetID: 400}}, []entity.ItemErrorGroup{}, nil)

				// Mock InsertAnnotations，会被调用但AddManualDatasetAnnotation会失败
				repoMock.EXPECT().InsertAnnotations(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

				return fields{
					traceRepo:             repoMock,
					traceConfig:           confMock,
					traceProducer:         traceProducerMock,
					annotationProducer:    annotationProducerMock,
					metrics:               metricsMock,
					tenantProvider:        tenantMock,
					DatasetServiceAdaptor: adaptor,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "test-user"}),
				req: &ExportTracesToDatasetRequest{
					WorkspaceID: 123,
					SpanIds:     []SpanID{{TraceID: "trace-123", SpanID: "span-456"}},
					Category:    entity.DatasetCategory_General,
					Config: DatasetConfig{
						IsNewDataset: true,
						DatasetName:  ptr.Of("test-dataset"),
						DatasetSchema: entity.DatasetSchema{
							FieldSchemas: []entity.FieldSchema{
								{Name: "input", ContentType: entity.ContentType_Text},
								{Name: "output", ContentType: entity.ContentType_Text},
							},
						},
					},
					StartTime:    time.Now().Unix() - 3600,
					EndTime:      time.Now().Unix(),
					PlatformType: loop_span.PlatformCozeLoop,
					ExportType:   ExportType_Append,
					FieldMappings: []entity.FieldMapping{
						{TraceFieldKey: "input", FieldSchema: entity.FieldSchema{Name: "input", ContentType: entity.ContentType_Text}},
						{TraceFieldKey: "output", FieldSchema: entity.FieldSchema{Name: "output", ContentType: entity.ContentType_Text}},
					},
				},
			},
			want: &ExportTracesToDatasetResponse{
				SuccessCount: 1,
				DatasetID:    400,
				DatasetName:  "test-dataset",
				Errors:       []entity.ItemErrorGroup{},
			},
			wantErr: false,
		},
		// 测试用例：InsertAnnotations失败的错误处理 (覆盖第375-384行)
		{
			name: "export with insert annotations failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				datasetProviderMock := rpcmocks.NewMockIDatasetProvider(ctrl)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				traceProducerMock := mqmocks.NewMockITraceProducer(ctrl)
				annotationProducerMock := mqmocks.NewMockIAnnotationProducer(ctrl)
				metricsMock := metricmocks.NewMockITraceMetrics(ctrl)

				adaptor := NewDatasetServiceAdaptor()
				adaptor.Register(entity.DatasetCategory_General, datasetProviderMock)

				testSpan := &loop_span.Span{
					TraceID:     "trace-123",
					SpanID:      "span-456",
					WorkspaceID: "123",
					Input:       `{"question": "test input"}`,
					Output:      `{"answer": "test output"}`,
				}

				// Mock成功的导出流程
				tenantMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), loop_span.PlatformCozeLoop).Return([]string{"tenant1"}, nil)
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{
					Spans: []*loop_span.Span{testSpan},
				}, nil)

				// Mock数据集创建
				datasetProviderMock.EXPECT().CreateDataset(gomock.Any(), gomock.Any()).Return(int64(500), nil)
				datasetProviderMock.EXPECT().GetDataset(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.Dataset{
					ID:              500,
					Name:            "test-dataset",
					DatasetCategory: entity.DatasetCategory_General,
					DatasetVersion: entity.DatasetVersion{
						DatasetSchema: entity.DatasetSchema{
							FieldSchemas: []entity.FieldSchema{
								{Name: "input", Key: ptr.Of("input")},
								{Name: "output", Key: ptr.Of("output")},
							},
						},
					},
				}, nil)

				// Mock数据集条目添加
				datasetProviderMock.EXPECT().AddDatasetItems(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(
					[]*entity.DatasetItem{{SpanID: "span-456", DatasetID: 500}}, []entity.ItemErrorGroup{}, nil)

				// Mock注解插入失败 (覆盖第375-384行)
				repoMock.EXPECT().InsertAnnotations(gomock.Any(), gomock.Any()).Return(assert.AnError)

				return fields{
					traceRepo:             repoMock,
					traceConfig:           confMock,
					traceProducer:         traceProducerMock,
					annotationProducer:    annotationProducerMock,
					metrics:               metricsMock,
					tenantProvider:        tenantMock,
					DatasetServiceAdaptor: adaptor,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "test-user"}),
				req: &ExportTracesToDatasetRequest{
					WorkspaceID: 123,
					SpanIds:     []SpanID{{TraceID: "trace-123", SpanID: "span-456"}},
					Category:    entity.DatasetCategory_General,
					Config: DatasetConfig{
						IsNewDataset: true,
						DatasetName:  ptr.Of("test-dataset"),
						DatasetSchema: entity.DatasetSchema{
							FieldSchemas: []entity.FieldSchema{
								{Name: "input", ContentType: entity.ContentType_Text},
								{Name: "output", ContentType: entity.ContentType_Text},
							},
						},
					},
					StartTime:    time.Now().Unix() - 3600,
					EndTime:      time.Now().Unix(),
					PlatformType: loop_span.PlatformCozeLoop,
					ExportType:   ExportType_Append,
					FieldMappings: []entity.FieldMapping{
						{TraceFieldKey: "input", FieldSchema: entity.FieldSchema{Name: "input", ContentType: entity.ContentType_Text}},
						{TraceFieldKey: "output", FieldSchema: entity.FieldSchema{Name: "output", ContentType: entity.ContentType_Text}},
					},
				},
			},
			want: &ExportTracesToDatasetResponse{
				SuccessCount: 1,
				DatasetID:    500,
				DatasetName:  "test-dataset",
				Errors:       []entity.ItemErrorGroup{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			fields := tt.fieldsGetter(ctrl)
			r := &TraceExportServiceImpl{
				traceRepo:             fields.traceRepo,
				traceConfig:           fields.traceConfig,
				traceProducer:         fields.traceProducer,
				annotationProducer:    fields.annotationProducer,
				metrics:               fields.metrics,
				tenantProvider:        fields.tenantProvider,
				DatasetServiceAdaptor: fields.DatasetServiceAdaptor,
			}

			got, err := r.ExportTracesToDataset(tt.args.ctx, tt.args.req)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestTraceExportServiceImpl_PreviewExportTracesToDataset_Additional(t *testing.T) {
	type fields struct {
		traceRepo             repo.ITraceRepo
		traceConfig           config.ITraceConfig
		traceProducer         mq.ITraceProducer
		annotationProducer    mq.IAnnotationProducer
		metrics               metrics.ITraceMetrics
		tenantProvider        tenant.ITenantProvider
		DatasetServiceAdaptor *DatasetServiceAdaptor
	}
	type args struct {
		ctx context.Context
		req *ExportTracesToDatasetRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *PreviewExportTracesToDatasetResponse
		wantErr      bool
	}{
		{
			name: "preview export traces to new dataset successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				datasetProviderMock := rpcmocks.NewMockIDatasetProvider(ctrl)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				traceProducerMock := mqmocks.NewMockITraceProducer(ctrl)
				annotationProducerMock := mqmocks.NewMockIAnnotationProducer(ctrl)
				metricsMock := metricmocks.NewMockITraceMetrics(ctrl)

				adaptor := NewDatasetServiceAdaptor()
				adaptor.Register(entity.DatasetCategory_General, datasetProviderMock)

				testSpan := &loop_span.Span{
					TraceID:     "trace-123",
					SpanID:      "span-456",
					WorkspaceID: "123",
					Input:       `{"question": "test input"}`,
					Output:      `{"answer": "test output"}`,
				}
				successItem := &entity.DatasetItem{
					SpanID:      "span-456",
					WorkspaceID: 123,
					DatasetID:   0,
					FieldData: []*entity.FieldData{
						{Key: "input", Name: "input", Content: &entity.Content{ContentType: entity.ContentType_Text, Text: ""}},
						{Key: "output", Name: "output", Content: &entity.Content{ContentType: entity.ContentType_Text, Text: ""}},
					},
				}

				tenantMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), loop_span.PlatformCozeLoop).Return([]string{"tenant1"}, nil)
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{
					Spans: []*loop_span.Span{testSpan},
				}, nil)
				datasetProviderMock.EXPECT().ValidateDatasetItems(gomock.Any(), gomock.Any(), gomock.Any(), (*bool)(nil)).Return(
					[]*entity.DatasetItem{successItem}, []entity.ItemErrorGroup{}, nil)

				return fields{
					traceRepo:             repoMock,
					traceConfig:           confMock,
					traceProducer:         traceProducerMock,
					annotationProducer:    annotationProducerMock,
					metrics:               metricsMock,
					tenantProvider:        tenantMock,
					DatasetServiceAdaptor: adaptor,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &ExportTracesToDatasetRequest{
					WorkspaceID: 123,
					SpanIds:     []SpanID{{TraceID: "trace-123", SpanID: "span-456"}},
					Category:    entity.DatasetCategory_General,
					Config: DatasetConfig{
						IsNewDataset: true,
						DatasetName:  ptr.Of("test-dataset"),
						DatasetSchema: entity.DatasetSchema{
							FieldSchemas: []entity.FieldSchema{
								{Name: "input", ContentType: entity.ContentType_Text},
								{Name: "output", ContentType: entity.ContentType_Text},
							},
						},
					},
					StartTime:    time.Now().Unix() - 3600,
					EndTime:      time.Now().Unix(),
					PlatformType: loop_span.PlatformCozeLoop,
					ExportType:   ExportType_Append,
					FieldMappings: []entity.FieldMapping{
						{TraceFieldKey: "input", FieldSchema: entity.FieldSchema{Name: "input", ContentType: entity.ContentType_Text}},
						{TraceFieldKey: "output", FieldSchema: entity.FieldSchema{Name: "output", ContentType: entity.ContentType_Text}},
					},
				},
			},
			want: &PreviewExportTracesToDatasetResponse{
				Items: []*entity.DatasetItem{{
					SpanID:      "span-456",
					WorkspaceID: 123,
					DatasetID:   0,
					FieldData: []*entity.FieldData{
						{Key: "input", Name: "input", Content: &entity.Content{ContentType: entity.ContentType_Text, Text: ""}},
						{Key: "output", Name: "output", Content: &entity.Content{ContentType: entity.ContentType_Text, Text: ""}},
					},
				}},
				Errors: []entity.ItemErrorGroup{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			fields := tt.fieldsGetter(ctrl)
			r := &TraceExportServiceImpl{
				traceRepo:             fields.traceRepo,
				traceConfig:           fields.traceConfig,
				traceProducer:         fields.traceProducer,
				annotationProducer:    fields.annotationProducer,
				metrics:               fields.metrics,
				tenantProvider:        fields.tenantProvider,
				DatasetServiceAdaptor: fields.DatasetServiceAdaptor,
			}

			got, err := r.PreviewExportTracesToDataset(tt.args.ctx, tt.args.req)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
