// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"context"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
	"github.com/coze-dev/cozeloop-go/spec/tracespec"
)

type DatasetCategory string

const (
	DatasetCategory_General    DatasetCategory = "general"
	DatasetCategory_Evaluation DatasetCategory = "evaluation"
)

type ContentType string

const (
	/* 基础类型 */
	ContentType_Text  ContentType = "Text"
	ContentType_Image ContentType = "Image"
	ContentType_Audio ContentType = "Audio"
	// 图文混排
	ContentType_MultiPart ContentType = "MultiPart"
)

type FieldDisplayFormat int64

const (
	FieldDisplayFormat_PlainText FieldDisplayFormat = 1
	FieldDisplayFormat_Markdown  FieldDisplayFormat = 2
	FieldDisplayFormat_JSON      FieldDisplayFormat = 3
	FieldDisplayFormat_YAML      FieldDisplayFormat = 4
	FieldDisplayFormat_Code      FieldDisplayFormat = 5
)

type EvaluationBizCategory string

type Dataset struct {
	// 主键&外键
	ID          int64
	WorkspaceID int64
	// 基础信息
	Name        string
	Description string
	// 业务分类
	DatasetCategory DatasetCategory
	// 版本信息
	DatasetVersion DatasetVersion
	// 评测集属性
	EvaluationBizCategory *EvaluationBizCategory
}

type DatasetVersion struct {
	// 主键&外键
	ID          int64
	WorkspaceID int64
	DatasetID   int64
	// 版本信息
	Version string
	// 版本描述
	Description string
	// schema
	DatasetSchema DatasetSchema
}

type DatasetSchema struct {
	// 主键&外键
	ID          int64
	WorkspaceID int64
	DatasetID   int64
	// 数据集字段约束
	FieldSchemas []FieldSchema
}

type FieldSchema struct {
	// 唯一键
	Key *string
	// 展示名称
	Name string
	// 描述
	Description string
	// 类型，如 文本，图片，etc.
	ContentType ContentType
	// [20,50) 内容格式限制相关
	TextSchema    string
	DisplayFormat FieldDisplayFormat
}

func NewDataset(id, spaceID int64, name string, category DatasetCategory, schema DatasetSchema) *Dataset {
	dataset := &Dataset{
		ID:          id,
		WorkspaceID: spaceID,
		Name:        name,
		DatasetVersion: DatasetVersion{
			DatasetSchema: schema,
		},
		DatasetCategory: category,
	}
	return dataset
}

func (d *Dataset) GetFieldSchemaKeyByName(fieldSchemaName string) string {
	for _, fieldSchema := range d.DatasetVersion.DatasetSchema.FieldSchemas {
		if fieldSchema.Name == fieldSchemaName {
			return *fieldSchema.Key
		}
	}
	return ""
}

type DatasetItem struct {
	ID          int64
	WorkspaceID int64
	DatasetID   int64
	SpanID      string
	ItemKey     *string
	FieldData   []*FieldData
	Error       []*ItemError
}

type ItemError struct {
	Message    string
	Type       int64
	FieldNames []string
}

type FieldData struct {
	Key     string // 评测集的唯一键
	Name    string // 用于展现的列名
	Content *Content
}

type Content struct {
	ContentType ContentType
	Text        string
	Image       *Image
	MultiPart   []*Content
}
type Image struct {
	Name string
	Url  string
}

// GetName returns the name of the image
func (i *Image) GetName() string {
	if i == nil {
		return ""
	}
	return i.Name
}

// GetUrl returns the URL of the image
func (i *Image) GetUrl() string {
	if i == nil {
		return ""
	}
	return i.Url
}

// GetContentType returns the content type of the content
func (c *Content) GetContentType() ContentType {
	if c == nil {
		return ""
	}
	return c.ContentType
}

// GetText returns the text content
func (c *Content) GetText() string {
	if c == nil {
		return ""
	}
	return c.Text
}

// GetImage returns the image content
func (c *Content) GetImage() *Image {
	if c == nil {
		return nil
	}
	return c.Image
}

// GetMultiPart returns the multi-part content
func (c *Content) GetMultiPart() []*Content {
	if c == nil {
		return nil
	}
	return c.MultiPart
}

func NewDatasetItem(workspaceID int64, datasetID int64, spanID string) *DatasetItem {
	return &DatasetItem{
		WorkspaceID: workspaceID,
		DatasetID:   datasetID,
		SpanID:      spanID,
		FieldData:   make([]*FieldData, 0),
	}
}

func (e *DatasetItem) AddFieldData(key string, name string, content *Content) {
	if e.FieldData == nil {
		e.FieldData = make([]*FieldData, 0)
	}
	e.FieldData = append(e.FieldData, &FieldData{
		Key:     key,
		Name:    name,
		Content: content,
	})
}

func (e *DatasetItem) AddError(message string, errorType int64, fieldNames []string) {
	if e.Error == nil {
		e.Error = make([]*ItemError, 0)
	}
	e.Error = append(e.Error, &ItemError{
		Message:    message,
		Type:       errorType,
		FieldNames: fieldNames,
	})
}

type FieldMapping struct {
	// 数据集字段约束
	FieldSchema        FieldSchema
	TraceFieldKey      string
	TraceFieldJsonpath string
}

type ItemErrorGroup struct {
	Type    int64
	Summary string
	// 错误条数
	ErrorCount int32
	// 批量写入时，每类错误至多提供 5 个错误详情；导入任务，至多提供 10 个错误详情
	Details []*ItemErrorDetail
}

type ItemErrorDetail struct {
	Message string
	// 单条错误数据在输入数据中的索引。从 0 开始，下同
	Index *int32
	// [startIndex, endIndex] 表示区间错误范围, 如 ExceedDatasetCapacity 错误时
	StartIndex *int32
	EndIndex   *int32
}

const (
	DatasetErrorType_MismatchSchema int64 = 1
	DatasetErrorType_InternalError  int64 = 100
)

func GetContentInfo(ctx context.Context, contentType ContentType, value string) (*Content, int64) {
	var content *Content
	switch contentType {
	case ContentType_MultiPart:
		var parts []tracespec.ModelMessagePart
		err := json.Unmarshal([]byte(value), &parts)
		if err != nil {
			logs.CtxInfo(ctx, "Unmarshal multi part failed, err:%v", err)
			return nil, DatasetErrorType_MismatchSchema
		}
		var multiPart []*Content
		for _, part := range parts {
			// 本期仅支持回流图片的多模态数据，非ImageURL信息的，打包放进text
			switch part.Type {
			case tracespec.ModelMessagePartTypeImage:
				if part.ImageURL == nil {
					continue
				}
				multiPart = append(multiPart, &Content{
					ContentType: ContentType_Image,
					Image: &Image{
						Name: part.ImageURL.Name,
						Url:  part.ImageURL.URL,
					},
				})
			case tracespec.ModelMessagePartTypeText, tracespec.ModelMessagePartTypeFile:
				multiPart = append(multiPart, &Content{
					ContentType: ContentType_Text,
					Text:        part.Text,
				})
			default:
				logs.CtxWarn(ctx, "Unsupported part type: %s", part.Type)
				return nil, DatasetErrorType_MismatchSchema
			}
		}
		content = &Content{
			ContentType: ContentType_MultiPart,
			MultiPart:   multiPart,
		}
	default:
		content = &Content{
			ContentType: ContentType_Text,
			Text:        value,
		}
	}
	return content, 0
}

func CommonContentTypeDO2DTO(contentType ContentType) *common.ContentType {
	switch contentType {
	case ContentType_Text:
		return gptr.Of(common.ContentTypeText)
	case ContentType_Image:
		return gptr.Of(common.ContentTypeImage)
	case ContentType_Audio:
		return gptr.Of(common.ContentTypeAudio)
	case ContentType_MultiPart:
		return gptr.Of(common.ContentTypeMultiPart)
	default:
		return gptr.Of(common.ContentTypeText)
	}
}
