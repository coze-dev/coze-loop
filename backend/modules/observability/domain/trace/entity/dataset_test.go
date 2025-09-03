// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	"github.com/stretchr/testify/assert"
)

func TestNewDataset(t *testing.T) {
	type args struct {
		id       int64
		spaceID  int64
		name     string
		category DatasetCategory
		schema   DatasetSchema
	}
	tests := []struct {
		name string
		args args
		want *Dataset
	}{
		{
			name: "create dataset successfully",
			args: args{
				id:       1,
				spaceID:  100,
				name:     "test dataset",
				category: DatasetCategory_General,
				schema: DatasetSchema{
					ID:          1,
					WorkspaceID: 100,
					DatasetID:   1,
					FieldSchemas: []FieldSchema{
						{
							Key:         gptr.Of("field1"),
							Name:        "Field 1",
							Description: "Test field",
							ContentType: ContentType_Text,
						},
					},
				},
			},
			want: &Dataset{
				ID:          1,
				WorkspaceID: 100,
				Name:        "test dataset",
				DatasetVersion: DatasetVersion{
					DatasetSchema: DatasetSchema{
						ID:          1,
						WorkspaceID: 100,
						DatasetID:   1,
						FieldSchemas: []FieldSchema{
							{
								Key:         gptr.Of("field1"),
								Name:        "Field 1",
								Description: "Test field",
								ContentType: ContentType_Text,
							},
						},
					},
				},
				DatasetCategory: DatasetCategory_General,
			},
		},
		{
			name: "create evaluation dataset",
			args: args{
				id:       2,
				spaceID:  200,
				name:     "eval dataset",
				category: DatasetCategory_Evaluation,
				schema: DatasetSchema{
					FieldSchemas: []FieldSchema{},
				},
			},
			want: &Dataset{
				ID:          2,
				WorkspaceID: 200,
				Name:        "eval dataset",
				DatasetVersion: DatasetVersion{
					DatasetSchema: DatasetSchema{
						FieldSchemas: []FieldSchema{},
					},
				},
				DatasetCategory: DatasetCategory_Evaluation,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewDataset(tt.args.id, tt.args.spaceID, tt.args.name, tt.args.category, tt.args.schema)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDataset_GetFieldSchemaKeyByName(t *testing.T) {
	tests := []struct {
		name            string
		dataset         *Dataset
		fieldSchemaName string
		want            string
	}{
		{
			name: "field found",
			dataset: &Dataset{
				DatasetVersion: DatasetVersion{
					DatasetSchema: DatasetSchema{
						FieldSchemas: []FieldSchema{
							{
								Key:  gptr.Of("key1"),
								Name: "field1",
							},
							{
								Key:  gptr.Of("key2"),
								Name: "field2",
							},
						},
					},
				},
			},
			fieldSchemaName: "field1",
			want:            "key1",
		},
		{
			name: "field not found",
			dataset: &Dataset{
				DatasetVersion: DatasetVersion{
					DatasetSchema: DatasetSchema{
						FieldSchemas: []FieldSchema{
							{
								Key:  gptr.Of("key1"),
								Name: "field1",
							},
						},
					},
				},
			},
			fieldSchemaName: "nonexistent",
			want:            "",
		},
		{
			name: "empty field schemas",
			dataset: &Dataset{
				DatasetVersion: DatasetVersion{
					DatasetSchema: DatasetSchema{
						FieldSchemas: []FieldSchema{},
					},
				},
			},
			fieldSchemaName: "field1",
			want:            "",
		},
		{
			name: "nil field schemas",
			dataset: &Dataset{
				DatasetVersion: DatasetVersion{
					DatasetSchema: DatasetSchema{
						FieldSchemas: nil,
					},
				},
			},
			fieldSchemaName: "field1",
			want:            "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.dataset.GetFieldSchemaKeyByName(tt.fieldSchemaName)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNewDatasetItem(t *testing.T) {
	type args struct {
		workspaceID int64
		datasetID   int64
		spanID      string
	}
	tests := []struct {
		name string
		args args
		want *DatasetItem
	}{
		{
			name: "create dataset item successfully",
			args: args{
				workspaceID: 100,
				datasetID:   1,
				spanID:      "span123",
			},
			want: &DatasetItem{
				WorkspaceID: 100,
				DatasetID:   1,
				SpanID:      "span123",
				FieldData:   make([]*FieldData, 0),
			},
		},
		{
			name: "create with empty span id",
			args: args{
				workspaceID: 200,
				datasetID:   2,
				spanID:      "",
			},
			want: &DatasetItem{
				WorkspaceID: 200,
				DatasetID:   2,
				SpanID:      "",
				FieldData:   make([]*FieldData, 0),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewDatasetItem(tt.args.workspaceID, tt.args.datasetID, tt.args.spanID)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDatasetItem_AddFieldData(t *testing.T) {
	tests := []struct {
		name        string
		item        *DatasetItem
		key         string
		fieldName   string
		content     *Content
		wantLen     int
		wantLastKey string
	}{
		{
			name: "add field data to existing slice",
			item: &DatasetItem{
				FieldData: []*FieldData{
					{Key: "existing", Name: "existing", Content: &Content{Text: "existing"}},
				},
			},
			key:         "key1",
			fieldName:   "field1",
			content:     &Content{Text: "test"},
			wantLen:     2,
			wantLastKey: "key1",
		},
		{
			name:        "add field data to nil slice",
			item:        &DatasetItem{FieldData: nil},
			key:         "key2",
			fieldName:   "field2",
			content:     &Content{Text: "test2"},
			wantLen:     1,
			wantLastKey: "key2",
		},
		{
			name:        "add field data with nil content",
			item:        &DatasetItem{FieldData: make([]*FieldData, 0)},
			key:         "key3",
			fieldName:   "field3",
			content:     nil,
			wantLen:     1,
			wantLastKey: "key3",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.item.AddFieldData(tt.key, tt.fieldName, tt.content)
			assert.Equal(t, tt.wantLen, len(tt.item.FieldData))
			if tt.wantLen > 0 {
				lastData := tt.item.FieldData[tt.wantLen-1]
				assert.Equal(t, tt.wantLastKey, lastData.Key)
				assert.Equal(t, tt.fieldName, lastData.Name)
				assert.Equal(t, tt.content, lastData.Content)
			}
		})
	}
}

func TestDatasetItem_AddError(t *testing.T) {
	tests := []struct {
		name           string
		item           *DatasetItem
		message        string
		errorType      int64
		fieldNames     []string
		wantLen        int
		wantLastType   int64
		wantLastFields []string
	}{
		{
			name: "add error to existing slice",
			item: &DatasetItem{
				Error: []*ItemError{
					{Message: "existing", Type: 1, FieldNames: []string{"field1"}},
				},
			},
			message:        "new error",
			errorType:      2,
			fieldNames:     []string{"field2", "field3"},
			wantLen:        2,
			wantLastType:   2,
			wantLastFields: []string{"field2", "field3"},
		},
		{
			name:           "add error to nil slice",
			item:           &DatasetItem{Error: nil},
			message:        "first error",
			errorType:      DatasetErrorType_MismatchSchema,
			fieldNames:     []string{"field1"},
			wantLen:        1,
			wantLastType:   DatasetErrorType_MismatchSchema,
			wantLastFields: []string{"field1"},
		},
		{
			name:           "add error with empty field names",
			item:           &DatasetItem{Error: make([]*ItemError, 0)},
			message:        "error without fields",
			errorType:      DatasetErrorType_InternalError,
			fieldNames:     []string{},
			wantLen:        1,
			wantLastType:   DatasetErrorType_InternalError,
			wantLastFields: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.item.AddError(tt.message, tt.errorType, tt.fieldNames)
			assert.Equal(t, tt.wantLen, len(tt.item.Error))
			if tt.wantLen > 0 {
				lastError := tt.item.Error[tt.wantLen-1]
				assert.Equal(t, tt.message, lastError.Message)
				assert.Equal(t, tt.wantLastType, lastError.Type)
				assert.Equal(t, tt.wantLastFields, lastError.FieldNames)
			}
		})
	}
}

func TestImage_GetName(t *testing.T) {
	tests := []struct {
		name  string
		image *Image
		want  string
	}{
		{
			name:  "get name from valid image",
			image: &Image{Name: "test.jpg", Url: "http://example.com/test.jpg"},
			want:  "test.jpg",
		},
		{
			name:  "get name from image with empty name",
			image: &Image{Name: "", Url: "http://example.com/test.jpg"},
			want:  "",
		},
		{
			name:  "get name from nil image",
			image: nil,
			want:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.image.GetName()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestImage_GetUrl(t *testing.T) {
	tests := []struct {
		name  string
		image *Image
		want  string
	}{
		{
			name:  "get url from valid image",
			image: &Image{Name: "test.jpg", Url: "http://example.com/test.jpg"},
			want:  "http://example.com/test.jpg",
		},
		{
			name:  "get url from image with empty url",
			image: &Image{Name: "test.jpg", Url: ""},
			want:  "",
		},
		{
			name:  "get url from nil image",
			image: nil,
			want:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.image.GetUrl()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestContent_GetContentType(t *testing.T) {
	tests := []struct {
		name    string
		content *Content
		want    ContentType
	}{
		{
			name:    "get content type from text content",
			content: &Content{ContentType: ContentType_Text, Text: "hello"},
			want:    ContentType_Text,
		},
		{
			name:    "get content type from image content",
			content: &Content{ContentType: ContentType_Image, Image: &Image{Name: "test.jpg"}},
			want:    ContentType_Image,
		},
		{
			name:    "get content type from multipart content",
			content: &Content{ContentType: ContentType_MultiPart, MultiPart: []*Content{}},
			want:    ContentType_MultiPart,
		},
		{
			name:    "get content type from nil content",
			content: nil,
			want:    "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.content.GetContentType()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestContent_GetText(t *testing.T) {
	tests := []struct {
		name    string
		content *Content
		want    string
	}{
		{
			name:    "get text from text content",
			content: &Content{ContentType: ContentType_Text, Text: "hello world"},
			want:    "hello world",
		},
		{
			name:    "get text from content with empty text",
			content: &Content{ContentType: ContentType_Text, Text: ""},
			want:    "",
		},
		{
			name:    "get text from nil content",
			content: nil,
			want:    "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.content.GetText()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestContent_GetImage(t *testing.T) {
	testImage := &Image{Name: "test.jpg", Url: "http://example.com/test.jpg"}
	tests := []struct {
		name    string
		content *Content
		want    *Image
	}{
		{
			name:    "get image from image content",
			content: &Content{ContentType: ContentType_Image, Image: testImage},
			want:    testImage,
		},
		{
			name:    "get image from content with nil image",
			content: &Content{ContentType: ContentType_Image, Image: nil},
			want:    nil,
		},
		{
			name:    "get image from nil content",
			content: nil,
			want:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.content.GetImage()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestContent_GetMultiPart(t *testing.T) {
	testMultiPart := []*Content{
		{ContentType: ContentType_Text, Text: "hello"},
		{ContentType: ContentType_Image, Image: &Image{Name: "test.jpg"}},
	}
	tests := []struct {
		name    string
		content *Content
		want    []*Content
	}{
		{
			name:    "get multipart from multipart content",
			content: &Content{ContentType: ContentType_MultiPart, MultiPart: testMultiPart},
			want:    testMultiPart,
		},
		{
			name:    "get multipart from content with nil multipart",
			content: &Content{ContentType: ContentType_MultiPart, MultiPart: nil},
			want:    nil,
		},
		{
			name:    "get multipart from nil content",
			content: nil,
			want:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.content.GetMultiPart()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetContentInfo(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx         context.Context
		contentType ContentType
		value       string
	}
	tests := []struct {
		name        string
		args        args
		wantContent *Content
		wantError   int64
	}{
		{
			name: "text content",
			args: args{
				ctx:         ctx,
				contentType: ContentType_Text,
				value:       "hello world",
			},
			wantContent: &Content{
				ContentType: ContentType_Text,
				Text:        "hello world",
			},
			wantError: 0,
		},
		{
			name: "image content",
			args: args{
				ctx:         ctx,
				contentType: ContentType_Image,
				value:       "image data",
			},
			wantContent: &Content{
				ContentType: ContentType_Text,
				Text:        "image data",
			},
			wantError: 0,
		},
		{
			name: "audio content",
			args: args{
				ctx:         ctx,
				contentType: ContentType_Audio,
				value:       "audio data",
			},
			wantContent: &Content{
				ContentType: ContentType_Text,
				Text:        "audio data",
			},
			wantError: 0,
		},
		{
			name: "multipart content with unsupported type returns error",
			args: args{
				ctx:         ctx,
				contentType: ContentType_MultiPart,
				value: `[
					{
						"type": "unsupported"
					}
				]`,
			},
			wantContent: nil,
			wantError:   DatasetErrorType_MismatchSchema,
		},
		{
			name: "multipart content with image but no image_url returns error",
			args: args{
				ctx:         ctx,
				contentType: ContentType_MultiPart,
				value: `[
					{
						"type": "image"
					}
				]`,
			},
			wantContent: nil,
			wantError:   DatasetErrorType_MismatchSchema,
		},
		{
			name: "multipart content with invalid json",
			args: args{
				ctx:         ctx,
				contentType: ContentType_MultiPart,
				value:       "invalid json",
			},
			wantContent: nil,
			wantError:   DatasetErrorType_MismatchSchema,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotContent, gotError := GetContentInfo(tt.args.ctx, tt.args.contentType, tt.args.value)
			assert.Equal(t, tt.wantContent, gotContent)
			assert.Equal(t, tt.wantError, gotError)
		})
	}
}

func TestCommonContentTypeDO2DTO(t *testing.T) {
	tests := []struct {
		name        string
		contentType ContentType
		want        *common.ContentType
	}{
		{
			name:        "text content type",
			contentType: ContentType_Text,
			want:        gptr.Of(common.ContentTypeText),
		},
		{
			name:        "image content type",
			contentType: ContentType_Image,
			want:        gptr.Of(common.ContentTypeImage),
		},
		{
			name:        "audio content type",
			contentType: ContentType_Audio,
			want:        gptr.Of(common.ContentTypeAudio),
		},
		{
			name:        "multipart content type",
			contentType: ContentType_MultiPart,
			want:        gptr.Of(common.ContentTypeMultiPart),
		},
		{
			name:        "unknown content type",
			contentType: ContentType("unknown"),
			want:        gptr.Of(common.ContentTypeText),
		},
		{
			name:        "empty content type",
			contentType: ContentType(""),
			want:        gptr.Of(common.ContentTypeText),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CommonContentTypeDO2DTO(tt.contentType)
			assert.Equal(t, tt.want, got)
		})
	}
}
