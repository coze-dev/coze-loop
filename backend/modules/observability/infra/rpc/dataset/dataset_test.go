// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0			Type:       int64(dataset_domain.ItemErrorType_MismatchSchema),
package dataset

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/dataset"
	dataset_domain "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/dataset"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/rpc/dataset/mocks"
)

//go:generate mockgen -source=dataset.go -destination=mocks/mock_dataset.go
//go:generate mockgen -package=mocks -destination=mocks/mock_datasetservice_client.go github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/datasetservice Client

// Test helper functions
func createTestDataset() *entity.Dataset {
	return &entity.Dataset{
		ID:              1,
		WorkspaceID:     100,
		Name:            "test-dataset",
		Description:     "test description",
		DatasetCategory: entity.DatasetCategory_General,
		DatasetVersion: entity.DatasetVersion{
			DatasetSchema: entity.DatasetSchema{
				FieldSchemas: []entity.FieldSchema{
					{
						Key:           gptr.Of("input"),
						Name:          "Input",
						Description:   "Input field",
						ContentType:   entity.ContentType_Text,
						DisplayFormat: entity.FieldDisplayFormat_PlainText,
					},
					{
						Key:           gptr.Of("output"),
						Name:          "Output",
						Description:   "Output field",
						ContentType:   entity.ContentType_Text,
						DisplayFormat: entity.FieldDisplayFormat_Markdown,
					},
				},
			},
		},
	}
}

func createTestDatasetItems(count int) []*entity.DatasetItem {
	items := make([]*entity.DatasetItem, count)
	for i := 0; i < count; i++ {
		items[i] = &entity.DatasetItem{
			ID:          int64(i + 1),
			WorkspaceID: 100,
			DatasetID:   1,
			ItemKey:     gptr.Of(fmt.Sprintf("item-%d", i)),
			FieldData: []*entity.FieldData{
				{
					Key:  "input",
					Name: "Input",
					Content: &entity.Content{
						ContentType: entity.ContentType_Text,
						Text:        fmt.Sprintf("test input %d", i),
					},
				},
				{
					Key:  "output",
					Name: "Output",
					Content: &entity.Content{
						ContentType: entity.ContentType_Text,
						Text:        fmt.Sprintf("test output %d", i),
					},
				},
			},
		}
	}
	return items
}

func createTestItemErrorGroups() []*dataset_domain.ItemErrorGroup {
	return []*dataset_domain.ItemErrorGroup{
		{
			Type:       gptr.Of(dataset_domain.ItemErrorType_MismatchSchema),
			Summary:    gptr.Of("Validation failed"),
			ErrorCount: gptr.Of(int32(2)),
			Details: []*dataset_domain.ItemErrorDetail{
				{
					Message: gptr.Of("Invalid input format"),
					Index:   gptr.Of(int32(0)),
				},
				{
					Message:    gptr.Of("Range validation error"),
					StartIndex: gptr.Of(int32(1)),
					EndIndex:   gptr.Of(int32(2)),
				},
			},
		},
	}
}

func TestDatasetProvider_ValidateDatasetItems(t *testing.T) {
	type fields struct {
		client *mocks.MockClient
	}
	type args struct {
		ctx                context.Context
		ds                 *entity.Dataset
		items              []*entity.DatasetItem
		ignoreCurrentCount *bool
	}
	tests := []struct {
		name            string
		fields          func(ctrl *gomock.Controller) fields
		args            args
		wantValidItems  int
		wantErrorGroups int
		wantErr         bool
		wantErrContains string
	}{
		{
			name: "validate dataset items successfully",
			fields: func(ctrl *gomock.Controller) fields {
				mockClient := mocks.NewMockClient(ctrl)
				mockClient.EXPECT().ValidateDatasetItems(gomock.Any(), gomock.Any()).
					Return(&dataset.ValidateDatasetItemsResp{
						ValidItemIndices: []int32{0, 1},
						Errors:           []*dataset_domain.ItemErrorGroup{},
					}, nil)
				return fields{client: mockClient}
			},
			args: args{
				ctx:                context.Background(),
				ds:                 createTestDataset(),
				items:              createTestDatasetItems(2),
				ignoreCurrentCount: gptr.Of(false),
			},
			wantValidItems:  2,
			wantErrorGroups: 0,
			wantErr:         false,
		},
		{
			name: "validate empty items list",
			fields: func(ctrl *gomock.Controller) fields {
				mockClient := mocks.NewMockClient(ctrl)
				// No RPC call expected for empty items
				return fields{client: mockClient}
			},
			args: args{
				ctx:                context.Background(),
				ds:                 createTestDataset(),
				items:              []*entity.DatasetItem{},
				ignoreCurrentCount: gptr.Of(false),
			},
			wantValidItems:  0,
			wantErrorGroups: 0,
			wantErr:         false,
		},
		{
			name: "validate with validation errors",
			fields: func(ctrl *gomock.Controller) fields {
				mockClient := mocks.NewMockClient(ctrl)
				mockClient.EXPECT().ValidateDatasetItems(gomock.Any(), gomock.Any()).
					Return(&dataset.ValidateDatasetItemsResp{
						ValidItemIndices: []int32{1},
						Errors:           createTestItemErrorGroups(),
					}, nil)
				return fields{client: mockClient}
			},
			args: args{
				ctx:                context.Background(),
				ds:                 createTestDataset(),
				items:              createTestDatasetItems(3),
				ignoreCurrentCount: gptr.Of(true),
			},
			wantValidItems:  1,
			wantErrorGroups: 1,
			wantErr:         false,
		},
		{
			name: "RPC call failure",
			fields: func(ctrl *gomock.Controller) fields {
				mockClient := mocks.NewMockClient(ctrl)
				mockClient.EXPECT().ValidateDatasetItems(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("RPC call failed"))
				return fields{client: mockClient}
			},
			args: args{
				ctx:                context.Background(),
				ds:                 createTestDataset(),
				items:              createTestDatasetItems(1),
				ignoreCurrentCount: gptr.Of(false),
			},
			wantValidItems:  0,
			wantErrorGroups: 0,
			wantErr:         true,
			wantErrContains: "RPC call failed",
		},
		{
			name: "validate with invalid indices",
			fields: func(ctrl *gomock.Controller) fields {
				mockClient := mocks.NewMockClient(ctrl)
				mockClient.EXPECT().ValidateDatasetItems(gomock.Any(), gomock.Any()).
					Return(&dataset.ValidateDatasetItemsResp{
						ValidItemIndices: []int32{0, 10}, // index 10 is out of bounds
						Errors: []*dataset_domain.ItemErrorGroup{
							{
								Type:       gptr.Of(dataset_domain.ItemErrorType_MismatchSchema),
								Summary:    gptr.Of("Invalid index"),
								ErrorCount: gptr.Of(int32(1)),
								Details: []*dataset_domain.ItemErrorDetail{
									{
										Message: gptr.Of("Out of bounds error"),
										Index:   gptr.Of(int32(10)), // invalid index
									},
								},
							},
						},
					}, nil)
				return fields{client: mockClient}
			},
			args: args{
				ctx:                context.Background(),
				ds:                 createTestDataset(),
				items:              createTestDatasetItems(2),
				ignoreCurrentCount: gptr.Of(false),
			},
			wantValidItems:  1, // only valid index 0
			wantErrorGroups: 1,
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			f := tt.fields(ctrl)
			d := &DatasetProvider{
				client: f.client,
			}

			gotValidItems, gotErrorGroups, err := d.ValidateDatasetItems(tt.args.ctx, tt.args.ds, tt.args.items, tt.args.ignoreCurrentCount)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrContains != "" {
					assert.Contains(t, err.Error(), tt.wantErrContains)
				}
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.wantValidItems, len(gotValidItems))
			assert.Equal(t, tt.wantErrorGroups, len(gotErrorGroups))
		})
	}
}

func TestDatasetItemsDO2DTO(t *testing.T) {
	tests := []struct {
		name  string
		items []*entity.DatasetItem
		want  []*dataset_domain.DatasetItem
	}{
		{
			name:  "convert empty items list",
			items: []*entity.DatasetItem{},
			want:  nil,
		},
		{
			name:  "convert nil items",
			items: nil,
			want:  nil,
		},
		{
			name:  "convert valid items",
			items: createTestDatasetItems(2),
			want: []*dataset_domain.DatasetItem{
				{
					ID:        gptr.Of(int64(1)),
					SpaceID:   gptr.Of(int64(100)),
					DatasetID: gptr.Of(int64(1)),
					ItemKey:   gptr.Of("item-0"),
					Data: []*dataset_domain.FieldData{
						{
							Key:     gptr.Of("input"),
							Name:    gptr.Of("Input"),
							Content: gptr.Of(`{"ContentType":"Text","Text":"test input 0","Image":null,"MultiPart":null}`),
						},
						{
							Key:     gptr.Of("output"),
							Name:    gptr.Of("Output"),
							Content: gptr.Of(`{"ContentType":"Text","Text":"test output 0","Image":null,"MultiPart":null}`),
						},
					},
				},
				{
					ID:        gptr.Of(int64(2)),
					SpaceID:   gptr.Of(int64(100)),
					DatasetID: gptr.Of(int64(1)),
					ItemKey:   gptr.Of("item-1"),
					Data: []*dataset_domain.FieldData{
						{
							Key:     gptr.Of("input"),
							Name:    gptr.Of("Input"),
							Content: gptr.Of(`{"ContentType":"Text","Text":"test input 1","Image":null,"MultiPart":null}`),
						},
						{
							Key:     gptr.Of("output"),
							Name:    gptr.Of("Output"),
							Content: gptr.Of(`{"ContentType":"Text","Text":"test output 1","Image":null,"MultiPart":null}`),
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := datasetItemsDO2DTO(tt.items)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDatasetItemDO2DTO(t *testing.T) {
	tests := []struct {
		name string
		item *entity.DatasetItem
		want *dataset_domain.DatasetItem
	}{
		{
			name: "convert nil item",
			item: nil,
			want: nil,
		},
		{
			name: "convert valid item",
			item: createTestDatasetItems(1)[0],
			want: &dataset_domain.DatasetItem{
				ID:        gptr.Of(int64(1)),
				SpaceID:   gptr.Of(int64(100)),
				DatasetID: gptr.Of(int64(1)),
				ItemKey:   gptr.Of("item-0"),
				Data: []*dataset_domain.FieldData{
					{
						Key:     gptr.Of("input"),
						Name:    gptr.Of("Input"),
						Content: gptr.Of(`{"ContentType":"Text","Text":"test input 0","Image":null,"MultiPart":null}`),
					},
					{
						Key:     gptr.Of("output"),
						Name:    gptr.Of("Output"),
						Content: gptr.Of(`{"ContentType":"Text","Text":"test output 0","Image":null,"MultiPart":null}`),
					},
				},
			},
		},
		{
			name: "convert item with empty field data",
			item: &entity.DatasetItem{
				ID:          1,
				WorkspaceID: 100,
				DatasetID:   1,
				ItemKey:     gptr.Of("empty-item"),
				FieldData:   []*entity.FieldData{},
			},
			want: &dataset_domain.DatasetItem{
				ID:        gptr.Of(int64(1)),
				SpaceID:   gptr.Of(int64(100)),
				DatasetID: gptr.Of(int64(1)),
				ItemKey:   gptr.Of("empty-item"),
				Data:      nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := datasetItemDO2DTO(tt.item)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFieldDataListDO2DTO(t *testing.T) {
	tests := []struct {
		name string
		data []*entity.FieldData
		want []*dataset_domain.FieldData
	}{
		{
			name: "convert empty field data list",
			data: []*entity.FieldData{},
			want: nil,
		},
		{
			name: "convert nil field data",
			data: nil,
			want: nil,
		},
		{
			name: "convert valid field data",
			data: []*entity.FieldData{
				{
					Key:  "test-key",
					Name: "Test Name",
					Content: &entity.Content{
						ContentType: entity.ContentType_Text,
						Text:        "test content",
					},
				},
			},
			want: []*dataset_domain.FieldData{
				{
					Key:     gptr.Of("test-key"),
					Name:    gptr.Of("Test Name"),
					Content: gptr.Of(`{"ContentType":"Text","Text":"test content","Image":null,"MultiPart":null}`),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fieldDataListDO2DTO(tt.data)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFieldDataDO2DTO(t *testing.T) {
	tests := []struct {
		name      string
		fieldData *entity.FieldData
		want      *dataset_domain.FieldData
	}{
		{
			name:      "convert nil field data",
			fieldData: nil,
			want:      nil,
		},
		{
			name: "convert valid field data",
			fieldData: &entity.FieldData{
				Key:  "test-key",
				Name: "Test Name",
				Content: &entity.Content{
					ContentType: entity.ContentType_Text,
					Text:        "test content",
				},
			},
			want: &dataset_domain.FieldData{
				Key:     gptr.Of("test-key"),
				Name:    gptr.Of("Test Name"),
				Content: gptr.Of(`{"ContentType":"Text","Text":"test content","Image":null,"MultiPart":null}`),
			},
		},
		{
			name: "convert field data with complex content",
			fieldData: &entity.FieldData{
				Key:  "multipart-key",
				Name: "MultiPart Content",
				Content: &entity.Content{
					ContentType: entity.ContentType_MultiPart,
					MultiPart: []*entity.Content{
						{
							ContentType: entity.ContentType_Text,
							Text:        "part1",
						},
						{
							ContentType: entity.ContentType_Image,
							Image:       &entity.Image{Url: "http://example.com/image.jpg"},
						},
					},
				},
			},
			want: &dataset_domain.FieldData{
				Key:     gptr.Of("multipart-key"),
				Name:    gptr.Of("MultiPart Content"),
				Content: gptr.Of(`{"ContentType":"MultiPart","Text":"","Image":null,"MultiPart":[{"ContentType":"Text","Text":"part1","Image":null,"MultiPart":null},{"ContentType":"Image","Text":"","Image":{"Name":"","Url":"http://example.com/image.jpg"},"MultiPart":null}]}`),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fieldDataDO2DTO(tt.fieldData)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestItemErrorGroupsDTO2DO(t *testing.T) {
	tests := []struct {
		name   string
		errors []*dataset_domain.ItemErrorGroup
		want   []entity.ItemErrorGroup
	}{
		{
			name:   "convert empty error groups",
			errors: []*dataset_domain.ItemErrorGroup{},
			want:   nil,
		},
		{
			name:   "convert nil error groups",
			errors: nil,
			want:   nil,
		},
		{
			name:   "convert valid error groups",
			errors: createTestItemErrorGroups(),
			want: []entity.ItemErrorGroup{
				{
					Type:       int64(dataset_domain.ItemErrorType_MismatchSchema),
					Summary:    "Validation failed",
					ErrorCount: 2,
					Details: []*entity.ItemErrorDetail{
						{
							Message: "Invalid input format",
							Index:   gptr.Of(int32(0)),
						},
						{
							Message:    "Range validation error",
							StartIndex: gptr.Of(int32(1)),
							EndIndex:   gptr.Of(int32(2)),
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := itemErrorGroupsDTO2DO(tt.errors)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestItemErrorGroupDTO2DO(t *testing.T) {
	tests := []struct {
		name       string
		errorGroup *dataset_domain.ItemErrorGroup
		want       entity.ItemErrorGroup
	}{
		{
			name:       "convert nil error group",
			errorGroup: nil,
			want:       entity.ItemErrorGroup{},
		},
		{
			name:       "convert valid error group",
			errorGroup: createTestItemErrorGroups()[0],
			want: entity.ItemErrorGroup{
				Type:       int64(dataset_domain.ItemErrorType_MismatchSchema),
				Summary:    "Validation failed",
				ErrorCount: 2,
				Details: []*entity.ItemErrorDetail{
					{
						Message: "Invalid input format",
						Index:   gptr.Of(int32(0)),
					},
					{
						Message:    "Range validation error",
						StartIndex: gptr.Of(int32(1)),
						EndIndex:   gptr.Of(int32(2)),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := itemErrorGroupDTO2DO(tt.errorGroup)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestItemErrorTypeDTO2DO(t *testing.T) {
	tests := []struct {
		name      string
		errorType *dataset_domain.ItemErrorType
		want      int64
	}{
		{
			name:      "convert nil error type",
			errorType: nil,
			want:      entity.DatasetErrorType_InternalError,
		},
		{
			name:      "convert mismatch schema error type",
			errorType: gptr.Of(dataset_domain.ItemErrorType_MismatchSchema),
			want:      int64(dataset_domain.ItemErrorType_MismatchSchema),
		},
		{
			name:      "convert internal error type",
			errorType: gptr.Of(dataset_domain.ItemErrorType_InternalError),
			want:      int64(dataset_domain.ItemErrorType_InternalError),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := itemErrorTypeDTO2DO(tt.errorType)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestItemErrorDetailsDTO2DO(t *testing.T) {
	tests := []struct {
		name    string
		details []*dataset_domain.ItemErrorDetail
		want    []*entity.ItemErrorDetail
	}{
		{
			name:    "convert empty details",
			details: []*dataset_domain.ItemErrorDetail{},
			want:    nil,
		},
		{
			name:    "convert nil details",
			details: nil,
			want:    nil,
		},
		{
			name: "convert valid details",
			details: []*dataset_domain.ItemErrorDetail{
				{
					Message: gptr.Of("Test error"),
					Index:   gptr.Of(int32(1)),
				},
				{
					Message:    gptr.Of("Range error"),
					StartIndex: gptr.Of(int32(2)),
					EndIndex:   gptr.Of(int32(4)),
				},
			},
			want: []*entity.ItemErrorDetail{
				{
					Message: "Test error",
					Index:   gptr.Of(int32(1)),
				},
				{
					Message:    "Range error",
					StartIndex: gptr.Of(int32(2)),
					EndIndex:   gptr.Of(int32(4)),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := itemErrorDetailsDTO2DO(tt.details)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestItemErrorDetailDTO2DO(t *testing.T) {
	tests := []struct {
		name   string
		detail *dataset_domain.ItemErrorDetail
		want   *entity.ItemErrorDetail
	}{
		{
			name:   "convert nil detail",
			detail: nil,
			want:   nil,
		},
		{
			name: "convert detail with index",
			detail: &dataset_domain.ItemErrorDetail{
				Message: gptr.Of("Index error"),
				Index:   gptr.Of(int32(5)),
			},
			want: &entity.ItemErrorDetail{
				Message: "Index error",
				Index:   gptr.Of(int32(5)),
			},
		},
		{
			name: "convert detail with range",
			detail: &dataset_domain.ItemErrorDetail{
				Message:    gptr.Of("Range error"),
				StartIndex: gptr.Of(int32(1)),
				EndIndex:   gptr.Of(int32(3)),
			},
			want: &entity.ItemErrorDetail{
				Message:    "Range error",
				StartIndex: gptr.Of(int32(1)),
				EndIndex:   gptr.Of(int32(3)),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := itemErrorDetailDTO2DO(tt.detail)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFieldDisplayFormatDO2DTO(t *testing.T) {
	tests := []struct {
		name string
		df   entity.FieldDisplayFormat
		want dataset_domain.FieldDisplayFormat
	}{
		{
			name: "plain text format",
			df:   entity.FieldDisplayFormat_PlainText,
			want: dataset_domain.FieldDisplayFormat_PlainText,
		},
		{
			name: "markdown format",
			df:   entity.FieldDisplayFormat_Markdown,
			want: dataset_domain.FieldDisplayFormat_Markdown,
		},
		{
			name: "json format",
			df:   entity.FieldDisplayFormat_JSON,
			want: dataset_domain.FieldDisplayFormat_JSON,
		},
		{
			name: "yaml format",
			df:   entity.FieldDisplayFormat_YAML,
			want: dataset_domain.FieldDisplayFormat_YAML,
		},
		{
			name: "code format",
			df:   entity.FieldDisplayFormat_Code,
			want: dataset_domain.FieldDisplayFormat_Code,
		},
		{
			name: "unknown format defaults to plain text",
			df:   entity.FieldDisplayFormat(999),
			want: dataset_domain.FieldDisplayFormat_PlainText,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fieldDisplayFormatDO2DTO(tt.df)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDatasetCategoryDO2DTO(t *testing.T) {
	tests := []struct {
		name     string
		category entity.DatasetCategory
		want     *dataset_domain.DatasetCategory
	}{
		{
			name:     "evaluation category",
			category: entity.DatasetCategory_Evaluation,
			want:     gptr.Of(dataset_domain.DatasetCategory_Evaluation),
		},
		{
			name:     "general category",
			category: entity.DatasetCategory_General,
			want:     gptr.Of(dataset_domain.DatasetCategory_General),
		},
		{
			name:     "unknown category defaults to evaluation",
			category: entity.DatasetCategory("unknown"),
			want:     gptr.Of(dataset_domain.DatasetCategory_Evaluation),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := datasetCategoryDO2DTO(tt.category)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFieldSchemasDO2DTO(t *testing.T) {
	tests := []struct {
		name    string
		schemas []entity.FieldSchema
		want    []*dataset_domain.FieldSchema
	}{
		{
			name:    "convert empty schemas",
			schemas: []entity.FieldSchema{},
			want:    nil,
		},
		{
			name:    "convert nil schemas",
			schemas: nil,
			want:    nil,
		},
		{
			name: "convert valid schemas",
			schemas: []entity.FieldSchema{
				{
					Key:           gptr.Of("field1"),
					Name:          "Field 1",
					Description:   "First field",
					ContentType:   entity.ContentType_Text,
					DisplayFormat: entity.FieldDisplayFormat_PlainText,
				},
				{
					Key:           gptr.Of("field2"),
					Name:          "Field 2",
					Description:   "Second field",
					ContentType:   entity.ContentType_Image,
					DisplayFormat: entity.FieldDisplayFormat_Markdown,
				},
			},
			want: []*dataset_domain.FieldSchema{
				{
					Key:           gptr.Of("field1"),
					Name:          gptr.Of("Field 1"),
					Description:   gptr.Of("First field"),
					ContentType:   gptr.Of(dataset_domain.ContentType_Text),
					DefaultFormat: gptr.Of(dataset_domain.FieldDisplayFormat_PlainText),
				},
				{
					Key:           gptr.Of("field2"),
					Name:          gptr.Of("Field 2"),
					Description:   gptr.Of("Second field"),
					ContentType:   gptr.Of(dataset_domain.ContentType_Image),
					DefaultFormat: gptr.Of(dataset_domain.FieldDisplayFormat_Markdown),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fieldSchemasDO2DTO(tt.schemas)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFieldSchemaDO2DTO(t *testing.T) {
	tests := []struct {
		name   string
		schema *entity.FieldSchema
		want   *dataset_domain.FieldSchema
	}{
		{
			name:   "convert nil schema",
			schema: nil,
			want:   nil,
		},
		{
			name: "convert valid schema",
			schema: &entity.FieldSchema{
				Key:           gptr.Of("test-field"),
				Name:          "Test Field",
				Description:   "Test description",
				ContentType:   entity.ContentType_Audio,
				DisplayFormat: entity.FieldDisplayFormat_JSON,
			},
			want: &dataset_domain.FieldSchema{
				Key:           gptr.Of("test-field"),
				Name:          gptr.Of("Test Field"),
				Description:   gptr.Of("Test description"),
				ContentType:   gptr.Of(dataset_domain.ContentType_Audio),
				DefaultFormat: gptr.Of(dataset_domain.FieldDisplayFormat_JSON),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fieldSchemaDO2DTO(tt.schema)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestContentTypeDO2DTO(t *testing.T) {
	tests := []struct {
		name        string
		contentType entity.ContentType
		want        *dataset_domain.ContentType
	}{
		{
			name:        "text content type",
			contentType: entity.ContentType_Text,
			want:        gptr.Of(dataset_domain.ContentType_Text),
		},
		{
			name:        "image content type",
			contentType: entity.ContentType_Image,
			want:        gptr.Of(dataset_domain.ContentType_Image),
		},
		{
			name:        "audio content type",
			contentType: entity.ContentType_Audio,
			want:        gptr.Of(dataset_domain.ContentType_Audio),
		},
		{
			name:        "multipart content type",
			contentType: entity.ContentType_MultiPart,
			want:        gptr.Of(dataset_domain.ContentType_MultiPart),
		},
		{
			name:        "unknown content type defaults to text",
			contentType: entity.ContentType("unknown"),
			want:        gptr.Of(dataset_domain.ContentType_Text),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ContentTypeDO2DTO(tt.contentType)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNewDatasetProvider(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)
	provider := NewDatasetProvider(mockClient)

	assert.NotNil(t, provider)
	assert.Equal(t, mockClient, provider.client)
}
