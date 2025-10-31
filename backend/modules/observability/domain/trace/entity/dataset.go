// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"context"
	"strconv"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
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

const (
	BizCategoryFromOnlineTrace EvaluationBizCategory = "from_online_trace"
)

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
	Seesion               *common.Session
	UserID                *string
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

func NewDataset(id, spaceID int64, name string, category DatasetCategory, schema DatasetSchema, session *common.Session, evaluationBizCategory *EvaluationBizCategory) *Dataset {
	var userID *string
	if session != nil {
		userID = ptr.Of(strconv.FormatInt(*session.UserID, 10))
	}
	dataset := &Dataset{
		ID:          id,
		WorkspaceID: spaceID,
		Name:        name,
		DatasetVersion: DatasetVersion{
			DatasetSchema: schema,
		},
		EvaluationBizCategory: evaluationBizCategory,
		DatasetCategory:       category,
		Seesion:               session,
		UserID:                userID,
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
	TraceID     string
	SpanID      string
	ItemKey     *string
	FieldData   []*FieldData
	Error       []*ItemError
	SpanType    string
	SpanName    string
	Source      *ItemSource
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

type ItemSource struct {
	Type LineageSourceType
	// 任务类型，根据该字段区分数据导入任务/数据回流任务/...
	JobType *TrackedJobType
	// item 关联的任务 id，为 0 表示无相应任务(例如数据是通过克隆另一数据行产生的)
	JobID *int64
	// type = DataReflow 时，从该字段获取 span 信息
	Span *TrackedTraceSpan
}
type LineageSourceType int64

const (
	// 数据回流，需要根据 ItemSource.span.isManual 是否是手动回流。如果是自动回流，则 ItemSource.jobID 中会包含对应的任务 ID
	LineageSourceType_DataReflow LineageSourceType = 4
)

type TrackedJobType int64

const (
	// 数据导入任务
	TrackedJobType_DatasetIOJob TrackedJobType = 1
	// 数据回流任务
	TrackedJobType_DataReflow TrackedJobType = 2
)

type TrackedTraceSpan struct {
	TraceID  *string
	SpanID   *string
	SpanName *string
	SpanType *string
	// 是否手工回流
	IsManual *bool
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

func NewDatasetItem(workspaceID int64, datasetID int64, span *loop_span.Span, source *ItemSource) *DatasetItem {
	if span == nil {
		return nil
	}
	return &DatasetItem{
		WorkspaceID: workspaceID,
		DatasetID:   datasetID,
		TraceID:     span.TraceID,
		SpanID:      span.SpanID,
		FieldData:   make([]*FieldData, 0),
		SpanType:    span.SpanType,
		SpanName:    span.SpanName,
		Source:      source,
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
