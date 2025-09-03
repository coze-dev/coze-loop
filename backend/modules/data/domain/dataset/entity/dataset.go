// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"fmt"
	"time"

	"github.com/bytedance/gg/gcond"
	"github.com/bytedance/gg/gptr"
)

type Dataset struct {
	ID       int64
	AppID    int32
	SpaceID  int64
	SchemaID int64

	Name           string
	Description    *string
	Category       DatasetCategory   // 业务场景分类
	BizCategory    string            // 业务场景下自定义分类
	Status         DatasetStatus     // 状态
	SecurityLevel  SecurityLevel     // 安全等级
	Visibility     DatasetVisibility // 可见性
	Spec           *DatasetSpec      // 规格
	Features       *DatasetFeatures  // 功能开关
	LatestVersion  string            // 最新的版本号
	NextVersionNum int64             // 下一个版本的数字版本号
	LastOperation  DatasetOpType     // 最近一次操作

	CreatedBy string
	CreatedAt time.Time
	UpdatedBy string
	UpdatedAt time.Time
	ExpiredAt *time.Time
}

func (d *Dataset) IsChangeUncommitted() bool {
	switch d.LastOperation {
	case "", DatasetOpTypeCreateDataset, DatasetOpTypeCreateVersion:
		return false
	default:
		return true
	}
}

func (d *Dataset) GetDescription() string {
	return gcond.If(d.Description == nil, "", gptr.Indirect(d.Description))
}

func (d *Dataset) CanWriteItem() bool {
	switch d.Status {
	case DatasetStatusDeleted, DatasetStatusExpired:
		return false
	default:
		return true
	}
}

func (d *Dataset) GetID() int64 {
	return d.ID
}

func (d *Dataset) SetID(id int64) {
	d.ID = id
}

type DatasetSpec struct {
	MaxItemCount  int64 `mapstructure:"max_item_count" json:"max_item_count,omitempty"`   // 条数上限
	MaxFieldCount int32 `mapstructure:"max_field_count" json:"max_field_count,omitempty"` // 字段上限
	MaxItemSize   int64 `mapstructure:"max_item_size" json:"max_item_size,omitempty"`     // 单条数据字数上限
}

type DatasetFeatures struct {
	EditSchema   bool `mapstructure:"edit_schema" json:"edit_schema,omitempty"`     // 变更 schema
	RepeatedData bool `mapstructure:"repeated_data" json:"repeated_data,omitempty"` // 多轮数据
	MultiModal   bool `mapstructure:"multi_modal" json:"multi_modal,omitempty"`     // 多模态
}

type DatasetCategory string

const (
	DatasetCategoryUnknown    DatasetCategory = ""
	DatasetCategoryGeneral    DatasetCategory = "general"
	DatasetCategoryTraining   DatasetCategory = "training"
	DatasetCategoryValidation DatasetCategory = "validation"
	DatasetCategoryEvaluation DatasetCategory = "evaluation"
)

type DatasetStatus string

const (
	DatasetStatusUnknown   DatasetStatus = ""
	DatasetStatusAvailable DatasetStatus = "available"
	DatasetStatusDeleted   DatasetStatus = "deleted"
	DatasetStatusExpired   DatasetStatus = "expired"
	DatasetStatusImporting DatasetStatus = "importing"
	DatasetStatusExporting DatasetStatus = "exporting"
	DatasetStatusIndexing  DatasetStatus = "indexing"
)

type SecurityLevel string

const (
	SecurityLevelUnknown SecurityLevel = ""
	SecurityLevelL1      SecurityLevel = "l1"
	SecurityLevelL2      SecurityLevel = "l2"
	SecurityLevelL3      SecurityLevel = "l3"
	SecurityLevelL4      SecurityLevel = "l4"
)

type DatasetVisibility string

const (
	DatasetVisibilityUnknown DatasetVisibility = ""
	DatasetVisibilityPublic  DatasetVisibility = "public"
	DatasetVisibilitySpace   DatasetVisibility = "space"
	DatasetVisibilitySystem  DatasetVisibility = "system"
)

type ContentType string

const (
	ContentTypeUnknown   ContentType = ""
	ContentTypeText      ContentType = `text`
	ContentTypeImage     ContentType = `image`
	ContentTypeAudio     ContentType = `audio`
	ContentTypeVideo     ContentType = `video`
	ContentTypeMultiPart ContentType = `multipart`
)

func (ct ContentType) IsMultiModal() bool {
	switch ct {
	case ContentTypeImage, ContentTypeAudio, ContentTypeVideo, ContentTypeMultiPart:
		return true
	default:
		return false
	}
}

type FieldDisplayFormat string

const (
	FieldDisplayFormatUnknown   FieldDisplayFormat = ""
	FieldDisplayFormatPlainText FieldDisplayFormat = "plain-text"
	FieldDisplayFormatMarkdown  FieldDisplayFormat = "markdown"
	FieldDisplayFormatJSON      FieldDisplayFormat = "json"
	FieldDisplayFormatYAML      FieldDisplayFormat = "yaml"
	FieldDisplayFormatCode      FieldDisplayFormat = "code" // todo: language
)

type SchemaKey string

const (
	SchemaKeyUnknown SchemaKey = ""
	SchemaKeyString  SchemaKey = "string"
	SchemaKeyInteger SchemaKey = "integer"
	SchemaKeyFloat   SchemaKey = "float"
	SchemaKeyBool    SchemaKey = "bool"
	SchemaKeyMessage SchemaKey = "message"
)

type FieldStatus string

const (
	FieldStatusUnknown   FieldStatus = ""
	FieldStatusAvailable FieldStatus = "available"
	FieldStatusDeleted   FieldStatus = "deleted"
)

type DatasetOpType string

const (
	DatasetOpTypeCreateDataset DatasetOpType = "create_dataset"
	DatasetOpTypeImport        DatasetOpType = "import"
	DatasetOpTypeCreateVersion DatasetOpType = "create_version"
	DatasetOpTypeUpdateSchema  DatasetOpType = "update_schema"
	DatasetOpTypeWriteItem     DatasetOpType = "write_item"    // 增删改 item
	DatasetOpTypeClearDataset  DatasetOpType = "clear_dataset" // 清空 dataset
)

type DatasetOperation struct {
	ID   string        `json:"-"`
	Type DatasetOpType `json:"-"`
	TS   time.Time     `json:"ts"`
	TTL  time.Duration `json:"ttl"`
}

type DatasetLastOperation struct {
	OP             DatasetOpType
	LastOperatedAt time.Time
}

func (o *DatasetOperation) String() string {
	return fmt.Sprintf(`{id=%s, type=%s, ts=%s, ttl=%s}`, o.ID, o.Type, o.TS, o.TTL)
}
