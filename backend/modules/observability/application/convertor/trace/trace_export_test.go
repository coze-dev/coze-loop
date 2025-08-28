// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0					TraceFieldJsonpath: "$.output",
package trace

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/dataset"
	eval_common "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	dataset0 "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/dataset"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/trace"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service"
	"github.com/stretchr/testify/assert"
)

func TestExportRequestDTO2DO(t *testing.T) {
	tests := []struct {
		name string
		req  *trace.ExportTracesToDatasetRequest
		want *service.ExportTracesToDatasetRequest
	}{
		{
			name: "nil request",
			req:  nil,
			want: nil,
		},
		{
			name: "basic request with required fields",
			req: &trace.ExportTracesToDatasetRequest{
				WorkspaceID: 100,
				SpanIds: []*trace.SpanID{
					{
						TraceID: "trace1",
						SpanID:  "span1",
					},
				},
				Category: dataset.DatasetCategory_General,
				Config: &trace.DatasetConfig{
					IsNewDataset: true,
					DatasetName:  gptr.Of("test dataset"),
				},
				StartTime: 1000,
				EndTime:   2000,
			},
			want: &service.ExportTracesToDatasetRequest{
				WorkspaceID: 100,
				SpanIds: []service.SpanID{
					{
						TraceID: "trace1",
						SpanID:  "span1",
					},
				},
				Category:     entity.DatasetCategory_General,
				Config:       service.DatasetConfig{IsNewDataset: true, DatasetName: gptr.Of("test dataset")},
				StartTime:    1000,
				EndTime:      2000,
				PlatformType: loop_span.PlatformCozeLoop,
				ExportType:   service.ExportType_Append,
			},
		},
		{
			name: "request with platform type",
			req: &trace.ExportTracesToDatasetRequest{
				WorkspaceID:  100,
				PlatformType: gptr.Of("cozeloop"),
				ExportType:   dataset0.ExportTypeOverwrite,
			},
			want: &service.ExportTracesToDatasetRequest{
				WorkspaceID:  100,
				SpanIds:      nil,
				Category:     entity.DatasetCategory_General,
				Config:       service.DatasetConfig{},
				PlatformType: loop_span.PlatformType("cozeloop"),
				ExportType:   service.ExportType_Overwrite,
			},
		},
		{
			name: "request with field mappings",
			req: &trace.ExportTracesToDatasetRequest{
				WorkspaceID: 100,
				FieldMappings: []*dataset0.FieldMapping{
					{
						FieldSchema: &dataset0.FieldSchema{
							Key:         gptr.Of("input"),
							Name:        gptr.Of("Input"),
							Description: gptr.Of("Input field"),
							ContentType: gptr.Of(eval_common.ContentTypeText),
							TextSchema:  gptr.Of("text schema"),
						},
						TraceFieldKey:      "trace_input",
						TraceFieldJsonpath: "$.input",
					},
				},
			},
			want: &service.ExportTracesToDatasetRequest{
				WorkspaceID:  100,
				SpanIds:      nil,
				Category:     entity.DatasetCategory_General,
				Config:       service.DatasetConfig{},
				PlatformType: loop_span.PlatformCozeLoop,
				ExportType:   service.ExportType_Append,
				FieldMappings: []entity.FieldMapping{
					{
						FieldSchema: entity.FieldSchema{
							Key:         gptr.Of("input"),
							Name:        "Input",
							Description: "Input field",
							ContentType: entity.ContentType_Text,
							TextSchema:  "text schema",
						},
						TraceFieldKey:      "trace_input",
						TraceFieldJsonpath: "$.input",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExportRequestDTO2DO(tt.req)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExportResponseDO2DTO(t *testing.T) {
	tests := []struct {
		name string
		resp *service.ExportTracesToDatasetResponse
		want *trace.ExportTracesToDatasetResponse
	}{
		{
			name: "nil response",
			resp: nil,
			want: nil,
		},
		{
			name: "basic response",
			resp: &service.ExportTracesToDatasetResponse{
				SuccessCount: 5,
				DatasetID:    100,
				DatasetName:  "test dataset",
			},
			want: &trace.ExportTracesToDatasetResponse{
				SuccessCount: gptr.Of(int32(5)),
				DatasetID:    gptr.Of(int64(100)),
				DatasetName:  gptr.Of("test dataset"),
			},
		},
		{
			name: "response with errors",
			resp: &service.ExportTracesToDatasetResponse{
				SuccessCount: 3,
				DatasetID:    100,
				DatasetName:  "test dataset",
				Errors: []entity.ItemErrorGroup{
					{
						Type:       int64(dataset.ItemErrorType_MismatchSchema),
						Summary:    "Validation failed",
						ErrorCount: 2,
						Details: []*entity.ItemErrorDetail{
							{
								Message: "Invalid field",
								Index:   gptr.Of(int32(1)),
							},
						},
					},
				},
			},
			want: &trace.ExportTracesToDatasetResponse{
				SuccessCount: gptr.Of(int32(3)),
				DatasetID:    gptr.Of(int64(100)),
				DatasetName:  gptr.Of("test dataset"),
				Errors: []*dataset.ItemErrorGroup{
					{
						Type:       gptr.Of(dataset.ItemErrorType_MismatchSchema),
						Summary:    gptr.Of("Validation failed"),
						ErrorCount: gptr.Of(int32(2)),
						Details: []*dataset.ItemErrorDetail{
							{
								Message: gptr.Of("Invalid field"),
								Index:   gptr.Of(int32(1)),
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExportResponseDO2DTO(tt.resp)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPreviewRequestDTO2DO(t *testing.T) {
	tests := []struct {
		name string
		req  *trace.PreviewExportTracesToDatasetRequest
		want *service.ExportTracesToDatasetRequest
	}{
		{
			name: "nil request",
			req:  nil,
			want: nil,
		},
		{
			name: "basic preview request",
			req: &trace.PreviewExportTracesToDatasetRequest{
				WorkspaceID: 200,
				SpanIds: []*trace.SpanID{
					{
						TraceID: "trace2",
						SpanID:  "span2",
					},
				},
				Category: dataset.DatasetCategory_Evaluation,
				Config: &trace.DatasetConfig{
					IsNewDataset: false,
					DatasetID:    gptr.Of(int64(50)),
				},
			},
			want: &service.ExportTracesToDatasetRequest{
				WorkspaceID: 200,
				SpanIds: []service.SpanID{
					{
						TraceID: "trace2",
						SpanID:  "span2",
					},
				},
				Category:     entity.DatasetCategory_Evaluation,
				Config:       service.DatasetConfig{IsNewDataset: false, DatasetID: gptr.Of(int64(50))},
				PlatformType: loop_span.PlatformCozeLoop,
				ExportType:   service.ExportType_Append,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PreviewRequestDTO2DO(tt.req)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPreviewResponseDO2DTO(t *testing.T) {
	tests := []struct {
		name string
		resp *service.PreviewExportTracesToDatasetResponse
		want *trace.PreviewExportTracesToDatasetResponse
	}{
		{
			name: "nil response",
			resp: nil,
			want: nil,
		},
		{
			name: "basic preview response",
			resp: &service.PreviewExportTracesToDatasetResponse{
				Items: []*entity.DatasetItem{
					{
						FieldData: []*entity.FieldData{
							{
								Key:  "input",
								Name: "Input",
								Content: &entity.Content{
									ContentType: entity.ContentType_Text,
									Text:        "test input",
								},
							},
						},
					},
				},
			},
			want: &trace.PreviewExportTracesToDatasetResponse{
				Items: []*dataset0.Item{
					{
						Status: dataset0.ItemStatusSuccess,
						FieldList: []*dataset0.FieldData{
							{
								Key:  gptr.Of("input"),
								Name: gptr.Of("Input"),
								Content: &dataset0.Content{
									ContentType: entity.CommonContentTypeDO2DTO(entity.ContentType_Text),
									Text:        gptr.Of("test input"),
									Image: &dataset0.Image{
										Name: gptr.Of(""),
										URL:  gptr.Of(""),
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "preview response with errors",
			resp: &service.PreviewExportTracesToDatasetResponse{
				Items: []*entity.DatasetItem{
					{
						Error: []*entity.ItemError{
							{
								Type:       int64(dataset.ItemErrorType_MismatchSchema),
								FieldNames: []string{"field1"},
							},
						},
					},
				},
				Errors: []entity.ItemErrorGroup{
					{
						Type:       int64(dataset.ItemErrorType_MismatchSchema),
						Summary:    "Preview validation failed",
						ErrorCount: 1,
					},
				},
			},
			want: &trace.PreviewExportTracesToDatasetResponse{
				Items: []*dataset0.Item{
					{
						Status: dataset0.ItemStatusError,
						Errors: []*dataset0.ItemError{
							{
								Type:       gptr.Of(dataset.ItemErrorType_MismatchSchema),
								FieldNames: []string{"field1"},
							},
						},
					},
				},
				Errors: []*dataset.ItemErrorGroup{
					{
						Type:       gptr.Of(dataset.ItemErrorType_MismatchSchema),
						Summary:    gptr.Of("Preview validation failed"),
						ErrorCount: gptr.Of(int32(1)),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PreviewResponseDO2DTO(tt.resp)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConvertDatasetConfigDTO2DO(t *testing.T) {
	tests := []struct {
		name   string
		config *trace.DatasetConfig
		want   service.DatasetConfig
	}{
		{
			name:   "nil config",
			config: nil,
			want:   service.DatasetConfig{},
		},
		{
			name: "basic config",
			config: &trace.DatasetConfig{
				IsNewDataset: true,
				DatasetName:  gptr.Of("new dataset"),
			},
			want: service.DatasetConfig{
				IsNewDataset: true,
				DatasetName:  gptr.Of("new dataset"),
			},
		},
		{
			name: "config with dataset ID",
			config: &trace.DatasetConfig{
				IsNewDataset: false,
				DatasetID:    gptr.Of(int64(123)),
			},
			want: service.DatasetConfig{
				IsNewDataset: false,
				DatasetID:    gptr.Of(int64(123)),
			},
		},
		{
			name: "config with schema",
			config: &trace.DatasetConfig{
				IsNewDataset: true,
				DatasetSchema: &dataset0.DatasetSchema{
					FieldSchemas: []*dataset0.FieldSchema{
						{
							Key:         gptr.Of("field1"),
							Name:        gptr.Of("Field 1"),
							Description: gptr.Of("Test field"),
							ContentType: gptr.Of(eval_common.ContentTypeText),
							TextSchema:  gptr.Of("text schema"),
						},
					},
				},
			},
			want: service.DatasetConfig{
				IsNewDataset: true,
				DatasetSchema: entity.DatasetSchema{
					FieldSchemas: []entity.FieldSchema{
						{
							Key:         gptr.Of("field1"),
							Name:        "Field 1",
							Description: "Test field",
							ContentType: entity.ContentType_Text,
							TextSchema:  "text schema",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertDatasetConfigDTO2DO(tt.config)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConvertDatasetSchemaDTO2DO(t *testing.T) {
	tests := []struct {
		name   string
		schema *dataset0.DatasetSchema
		want   entity.DatasetSchema
	}{
		{
			name:   "nil schema",
			schema: nil,
			want:   entity.DatasetSchema{},
		},
		{
			name: "empty schema",
			schema: &dataset0.DatasetSchema{
				FieldSchemas: []*dataset0.FieldSchema{},
			},
			want: entity.DatasetSchema{
				FieldSchemas: []entity.FieldSchema{},
			},
		},
		{
			name: "schema with multiple fields",
			schema: &dataset0.DatasetSchema{
				FieldSchemas: []*dataset0.FieldSchema{
					{
						Key:         gptr.Of("input"),
						Name:        gptr.Of("Input"),
						Description: gptr.Of("Input field"),
						ContentType: gptr.Of(eval_common.ContentTypeText),
						TextSchema:  gptr.Of("text schema"),
					},
					{
						Key:         gptr.Of("output"),
						Name:        gptr.Of("Output"),
						ContentType: gptr.Of(eval_common.ContentTypeImage),
					},
				},
			},
			want: entity.DatasetSchema{
				FieldSchemas: []entity.FieldSchema{
					{
						Key:         gptr.Of("input"),
						Name:        "Input",
						Description: "Input field",
						ContentType: entity.ContentType_Text,
						TextSchema:  "text schema",
					},
					{
						Key:         gptr.Of("output"),
						Name:        "Output",
						Description: "",
						ContentType: entity.ContentType_Image,
						TextSchema:  "",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertDatasetSchemaDTO2DO(tt.schema)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConvertFieldMappingsDTO2DO(t *testing.T) {
	tests := []struct {
		name     string
		mappings []*dataset0.FieldMapping
		want     []entity.FieldMapping
	}{
		{
			name:     "nil mappings",
			mappings: nil,
			want:     nil,
		},
		{
			name:     "empty mappings",
			mappings: []*dataset0.FieldMapping{},
			want:     nil,
		},
		{
			name: "single mapping",
			mappings: []*dataset0.FieldMapping{
				{
					FieldSchema: &dataset0.FieldSchema{
						Key:         gptr.Of("input"),
						Name:        gptr.Of("Input"),
						Description: gptr.Of("Input field"),
						ContentType: gptr.Of(eval_common.ContentTypeText),
						TextSchema:  gptr.Of("text schema"),
					},
					TraceFieldKey:      "trace_input",
					TraceFieldJsonpath: "$.input",
				},
			},
			want: []entity.FieldMapping{
				{
					FieldSchema: entity.FieldSchema{
						Key:         gptr.Of("input"),
						Name:        "Input",
						Description: "Input field",
						ContentType: entity.ContentType_Text,
						TextSchema:  "text schema",
					},
					TraceFieldKey:      "trace_input",
					TraceFieldJsonpath: "$.input",
				},
			},
		},
		{
			name: "multiple mappings",
			mappings: []*dataset0.FieldMapping{
				{
					FieldSchema: &dataset0.FieldSchema{
						Key:         gptr.Of("input"),
						Name:        gptr.Of("Input"),
						ContentType: gptr.Of(eval_common.ContentTypeText),
					},
					TraceFieldKey: "trace_input",
				},
				{
					FieldSchema: &dataset0.FieldSchema{
						Key:         gptr.Of("output"),
						Name:        gptr.Of("Output"),
						ContentType: gptr.Of(eval_common.ContentTypeImage),
					},
					TraceFieldJsonpath: "$.output",
				},
			},
			want: []entity.FieldMapping{
				{
					FieldSchema: entity.FieldSchema{
						Key:         gptr.Of("input"),
						Name:        "Input",
						Description: "",
						ContentType: entity.ContentType_Text,
						TextSchema:  "",
					},
					TraceFieldKey:      "trace_input",
					TraceFieldJsonpath: "",
				},
				{
					FieldSchema: entity.FieldSchema{
						Key:         gptr.Of("output"),
						Name:        "Output",
						Description: "",
						ContentType: entity.ContentType_Image,
						TextSchema:  "",
					},
					TraceFieldKey:      "",
					TraceFieldJsonpath: "$.output",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertFieldMappingsDTO2DO(tt.mappings)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConvertItemErrorGroupsDO2DTO(t *testing.T) {
	tests := []struct {
		name   string
		errors []entity.ItemErrorGroup
		want   []*dataset.ItemErrorGroup
	}{
		{
			name:   "nil errors",
			errors: nil,
			want:   nil,
		},
		{
			name:   "empty errors",
			errors: []entity.ItemErrorGroup{},
			want:   nil,
		},
		{
			name: "single error group",
			errors: []entity.ItemErrorGroup{
				{
					Type:       int64(dataset.ItemErrorType_MismatchSchema),
					Summary:    "Validation failed",
					ErrorCount: 2,
				},
			},
			want: []*dataset.ItemErrorGroup{
				{
					Type:       gptr.Of(dataset.ItemErrorType_MismatchSchema),
					Summary:    gptr.Of("Validation failed"),
					ErrorCount: gptr.Of(int32(2)),
				},
			},
		},
		{
			name: "error group with details",
			errors: []entity.ItemErrorGroup{
				{
					Type:       int64(dataset.ItemErrorType_MismatchSchema),
					Summary:    "Validation failed",
					ErrorCount: 3,
					Details: []*entity.ItemErrorDetail{
						{
							Message: "Invalid field value",
							Index:   gptr.Of(int32(1)),
						},
						{
							Message:    "Range error",
							StartIndex: gptr.Of(int32(2)),
							EndIndex:   gptr.Of(int32(4)),
						},
					},
				},
			},
			want: []*dataset.ItemErrorGroup{
				{
					Type:       gptr.Of(dataset.ItemErrorType_MismatchSchema),
					Summary:    gptr.Of("Validation failed"),
					ErrorCount: gptr.Of(int32(3)),
					Details: []*dataset.ItemErrorDetail{
						{
							Message: gptr.Of("Invalid field value"),
							Index:   gptr.Of(int32(1)),
						},
						{
							Message:    gptr.Of("Range error"),
							StartIndex: gptr.Of(int32(2)),
							EndIndex:   gptr.Of(int32(4)),
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertItemErrorGroupsDO2DTO(tt.errors)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConvertDatasetItemsDO2DTO(t *testing.T) {
	tests := []struct {
		name  string
		items []*entity.DatasetItem
		want  []*dataset0.Item
	}{
		{
			name:  "nil items",
			items: nil,
			want:  nil,
		},
		{
			name:  "empty items",
			items: []*entity.DatasetItem{},
			want:  nil,
		},
		{
			name: "single successful item",
			items: []*entity.DatasetItem{
				{
					FieldData: []*entity.FieldData{
						{
							Key:  "input",
							Name: "Input",
							Content: &entity.Content{
								ContentType: entity.ContentType_Text,
								Text:        "test input",
							},
						},
					},
				},
			},
			want: []*dataset0.Item{
				{
					Status: dataset0.ItemStatusSuccess,
					FieldList: []*dataset0.FieldData{
						{
							Key:  gptr.Of("input"),
							Name: gptr.Of("Input"),
							Content: &dataset0.Content{
								ContentType: entity.CommonContentTypeDO2DTO(entity.ContentType_Text),
								Text:        gptr.Of("test input"),
								Image: &dataset0.Image{
									Name: gptr.Of(""),
									URL:  gptr.Of(""),
								},
							},
						},
					},
				},
			},
		},
		{
			name: "item with errors",
			items: []*entity.DatasetItem{
				{
					Error: []*entity.ItemError{
						{
							Type:       int64(dataset.ItemErrorType_MismatchSchema),
							FieldNames: []string{"field1", "field2"},
						},
					},
				},
			},
			want: []*dataset0.Item{
				{
					Status: dataset0.ItemStatusError,
					Errors: []*dataset0.ItemError{
						{
							Type:       gptr.Of(dataset.ItemErrorType_MismatchSchema),
							FieldNames: []string{"field1", "field2"},
						},
					},
				},
			},
		},
		{
			name: "item with multipart content",
			items: []*entity.DatasetItem{
				{
					FieldData: []*entity.FieldData{
						{
							Key:  "multipart",
							Name: "Multipart Field",
							Content: &entity.Content{
								ContentType: entity.ContentType_MultiPart,
								MultiPart: []*entity.Content{
									{
										ContentType: entity.ContentType_Text,
										Text:        "text part",
									},
									{
										ContentType: entity.ContentType_Image,
										Image: &entity.Image{
											Name: "image.jpg",
											Url:  "http://example.com/image.jpg",
										},
									},
								},
							},
						},
					},
				},
			},
			want: []*dataset0.Item{
				{
					Status: dataset0.ItemStatusSuccess,
					FieldList: []*dataset0.FieldData{
						{
							Key:  gptr.Of("multipart"),
							Name: gptr.Of("Multipart Field"),
							Content: &dataset0.Content{
								ContentType: entity.CommonContentTypeDO2DTO(entity.ContentType_MultiPart),
								Text:        gptr.Of(""),
								Image: &dataset0.Image{
									Name: gptr.Of(""),
									URL:  gptr.Of(""),
								},
								MultiPart: []*dataset0.Content{
									{
										ContentType: entity.CommonContentTypeDO2DTO(entity.ContentType_Text),
										Text:        gptr.Of("text part"),
										Image: &dataset0.Image{
											Name: gptr.Of(""),
											URL:  gptr.Of(""),
										},
									},
									{
										ContentType: entity.CommonContentTypeDO2DTO(entity.ContentType_Image),
										Text:        gptr.Of(""),
										Image: &dataset0.Image{
											Name: gptr.Of("image.jpg"),
											URL:  gptr.Of("http://example.com/image.jpg"),
										},
									},
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
			got := convertDatasetItemsDO2DTO(tt.items)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConvertDatasetCategoryDTO2DO(t *testing.T) {
	tests := []struct {
		name     string
		category dataset.DatasetCategory
		want     entity.DatasetCategory
	}{
		{
			name:     "general category",
			category: dataset.DatasetCategory_General,
			want:     entity.DatasetCategory_General,
		},
		{
			name:     "evaluation category",
			category: dataset.DatasetCategory_Evaluation,
			want:     entity.DatasetCategory_Evaluation,
		},
		{
			name:     "unknown category defaults to general",
			category: dataset.DatasetCategory(999),
			want:     entity.DatasetCategory_General,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertDatasetCategoryDTO2DO(tt.category)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConvertSpanIdsDTO2DO(t *testing.T) {
	tests := []struct {
		name    string
		spanIDs []*trace.SpanID
		want    []service.SpanID
	}{
		{
			name:    "nil span IDs",
			spanIDs: nil,
			want:    nil,
		},
		{
			name:    "empty span IDs",
			spanIDs: []*trace.SpanID{},
			want:    []service.SpanID{},
		},
		{
			name: "single span ID",
			spanIDs: []*trace.SpanID{
				{
					TraceID: "trace123",
					SpanID:  "span456",
				},
			},
			want: []service.SpanID{
				{
					TraceID: "trace123",
					SpanID:  "span456",
				},
			},
		},
		{
			name: "multiple span IDs",
			spanIDs: []*trace.SpanID{
				{
					TraceID: "trace1",
					SpanID:  "span1",
				},
				{
					TraceID: "trace2",
					SpanID:  "span2",
				},
			},
			want: []service.SpanID{
				{
					TraceID: "trace1",
					SpanID:  "span1",
				},
				{
					TraceID: "trace2",
					SpanID:  "span2",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertSpanIdsDTO2DO(tt.spanIDs)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConvertContentTypeDTO2DO(t *testing.T) {
	tests := []struct {
		name        string
		contentType eval_common.ContentType
		want        entity.ContentType
	}{
		{
			name:        "text content type",
			contentType: eval_common.ContentTypeText,
			want:        entity.ContentType_Text,
		},
		{
			name:        "image content type",
			contentType: eval_common.ContentTypeImage,
			want:        entity.ContentType_Image,
		},
		{
			name:        "audio content type",
			contentType: eval_common.ContentTypeAudio,
			want:        entity.ContentType_Audio,
		},
		{
			name:        "multipart content type",
			contentType: eval_common.ContentTypeMultiPart,
			want:        entity.ContentType_MultiPart,
		},
		{
			name:        "unknown content type defaults to text",
			contentType: "unknown",
			want:        entity.ContentType_Text,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertContentTypeDTO2DO(tt.contentType)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConvertContentDO2DTO(t *testing.T) {
	tests := []struct {
		name    string
		content *entity.Content
		want    *dataset0.Content
	}{
		{
			name:    "nil content",
			content: nil,
			want:    nil,
		},
		{
			name: "text content",
			content: &entity.Content{
				ContentType: entity.ContentType_Text,
				Text:        "test text",
			},
			want: &dataset0.Content{
				ContentType: entity.CommonContentTypeDO2DTO(entity.ContentType_Text),
				Text:        gptr.Of("test text"),
				Image: &dataset0.Image{
					Name: gptr.Of(""),
					URL:  gptr.Of(""),
				},
			},
		},
		{
			name: "image content",
			content: &entity.Content{
				ContentType: entity.ContentType_Image,
				Image: &entity.Image{
					Name: "test.jpg",
					Url:  "http://example.com/test.jpg",
				},
			},
			want: &dataset0.Content{
				ContentType: entity.CommonContentTypeDO2DTO(entity.ContentType_Image),
				Text:        gptr.Of(""),
				Image: &dataset0.Image{
					Name: gptr.Of("test.jpg"),
					URL:  gptr.Of("http://example.com/test.jpg"),
				},
			},
		},
		{
			name: "multipart content",
			content: &entity.Content{
				ContentType: entity.ContentType_MultiPart,
				MultiPart: []*entity.Content{
					{
						ContentType: entity.ContentType_Text,
						Text:        "part1",
					},
					{
						ContentType: entity.ContentType_Image,
						Image: &entity.Image{
							Name: "part2.jpg",
							Url:  "http://example.com/part2.jpg",
						},
					},
				},
			},
			want: &dataset0.Content{
				ContentType: entity.CommonContentTypeDO2DTO(entity.ContentType_MultiPart),
				Text:        gptr.Of(""),
				Image: &dataset0.Image{
					Name: gptr.Of(""),
					URL:  gptr.Of(""),
				},
				MultiPart: []*dataset0.Content{
					{
						ContentType: entity.CommonContentTypeDO2DTO(entity.ContentType_Text),
						Text:        gptr.Of("part1"),
						Image: &dataset0.Image{
							Name: gptr.Of(""),
							URL:  gptr.Of(""),
						},
					},
					{
						ContentType: entity.CommonContentTypeDO2DTO(entity.ContentType_Image),
						Text:        gptr.Of(""),
						Image: &dataset0.Image{
							Name: gptr.Of("part2.jpg"),
							URL:  gptr.Of("http://example.com/part2.jpg"),
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertContentDO2DTO(tt.content)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConvertFieldListDO2DTO(t *testing.T) {
	tests := []struct {
		name      string
		fieldList []*entity.FieldData
		want      []*dataset0.FieldData
	}{
		{
			name:      "empty field list",
			fieldList: []*entity.FieldData{},
			want:      []*dataset0.FieldData{},
		},
		{
			name: "single field",
			fieldList: []*entity.FieldData{
				{
					Key:  "input",
					Name: "Input",
					Content: &entity.Content{
						ContentType: entity.ContentType_Text,
						Text:        "test input",
					},
				},
			},
			want: []*dataset0.FieldData{
				{
					Key:  gptr.Of("input"),
					Name: gptr.Of("Input"),
					Content: &dataset0.Content{
						ContentType: entity.CommonContentTypeDO2DTO(entity.ContentType_Text),
						Text:        gptr.Of("test input"),
						Image: &dataset0.Image{
							Name: gptr.Of(""),
							URL:  gptr.Of(""),
						},
					},
				},
			},
		},
		{
			name: "field with multipart content",
			fieldList: []*entity.FieldData{
				{
					Key:  "multipart",
					Name: "Multipart",
					Content: &entity.Content{
						ContentType: entity.ContentType_MultiPart,
						MultiPart: []*entity.Content{
							{
								ContentType: entity.ContentType_Text,
								Text:        "text part",
							},
						},
					},
				},
			},
			want: []*dataset0.FieldData{
				{
					Key:  gptr.Of("multipart"),
					Name: gptr.Of("Multipart"),
					Content: &dataset0.Content{
						ContentType: entity.CommonContentTypeDO2DTO(entity.ContentType_MultiPart),
						Text:        gptr.Of(""),
						Image: &dataset0.Image{
							Name: gptr.Of(""),
							URL:  gptr.Of(""),
						},
						MultiPart: []*dataset0.Content{
							{
								ContentType: entity.CommonContentTypeDO2DTO(entity.ContentType_Text),
								Text:        gptr.Of("text part"),
								Image: &dataset0.Image{
									Name: gptr.Of(""),
									URL:  gptr.Of(""),
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
			got := convertFieldListDO2DTO(tt.fieldList)
			assert.Equal(t, tt.want, got)
		})
	}
}
