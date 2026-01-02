// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/bytedance/gg/gslice"
	"github.com/bytedance/sonic"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/xeipuuv/gojsonschema"

	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/consts"
)

type DatasetSchema struct {
	ID        int64
	AppID     int32
	SpaceID   int64
	DatasetID int64

	Fields    []*FieldSchema // 字段格式
	Immutable bool           // 是否不允许编辑

	CreatedBy     string
	CreatedAt     time.Time
	UpdatedBy     string
	UpdatedAt     time.Time
	UpdateVersion int64
}

func (s *DatasetSchema) AvailableFields() []*FieldSchema {
	return gslice.Filter(s.Fields, func(s *FieldSchema) bool { return s.Status == FieldStatusAvailable || s.Status == "" })
}

func (s *DatasetSchema) GetID() int64 {
	return s.ID
}

func (s *DatasetSchema) SetID(id int64) {
	s.ID = id
}

type FieldSchema struct {
	Key            string             `json:"key"`                        // 数据集 Schema 版本变化中 Key 唯一
	Name           string             `json:"name,omitempty"`             // 展示名称
	Description    string             `json:"description,omitempty"`      // 描述
	ContentType    ContentType        `json:"content_type,omitempty"`     // 类型，如文本，图片
	DefaultFormat  FieldDisplayFormat `json:"default_format,omitempty"`   // 默认展示格式
	SchemaKey      SchemaKey          `json:"schema_key,omitempty"`       // 内置格式 key
	TextSchema     *JSONSchema        `json:"text_schema"`                // 文本内容格式限制，JSON schema 格式
	MultiModelSpec *MultiModalSpec    `json:"multi_model_spec,omitempty"` // 多模态规格限制
	Status         FieldStatus        `json:"status,omitempty"`           // 状态
	Hidden         bool               `json:"hidden"`                     // 用户不可见
}

func (s *FieldSchema) ValidateData(d *FieldData) error {
	if d == nil {
		return errors.Errorf("nil field data")
	}
	if d.Key != s.Key {
		return errors.Errorf("key mismatch, schema_key=%s, data_key=%s", d.Key, s.Key)
	}

	switch s.ContentType {
	case ContentTypeText:
		return s.validateTextData(d)

	case ContentTypeImage, ContentTypeAudio, ContentTypeVideo:
		spec := s.MultiModelSpec
		if spec == nil {
			return nil
		}
		if spec.MaxFileCount > 0 && int(spec.MaxFileCount) < len(d.Attachments) {
			return errors.Errorf(`file count out of range, max_file_count=%d, file_count=%d`, spec.MaxFileCount, len(d.Attachments))
		}
		// Notice: 暂不校验文件大小与格式

	case ContentTypeMultiPart:
		return errors.Errorf("multipart content type not supported")
	}
	return nil
}

func (s *FieldSchema) validateTextData(d *FieldData) error {
	schema := s.TextSchema
	if s.SchemaKey != "" {
		schema = builtinSchemas[s.SchemaKey]
	}
	if schema == nil || schema.Schema == nil {
		return nil
	}

	return schema.Validate(d.Content)
}

func (s *FieldSchema) Available() bool {
	return s.Status == FieldStatusAvailable || s.Status == ""
}

func (s *FieldSchema) CompatibleWith(other *FieldSchema) bool {
	if s.ContentType != other.ContentType {
		return false
	}
	if s.SchemaKey != other.SchemaKey && other.SchemaKey != "" { // 添加 schema key 或修改 schema key 可能不兼容
		return false
	}
	if s.TextSchema == nil && other.TextSchema != nil { // 添加 text schema 可能不兼容
		return false
	}
	if s.TextSchema != nil && other.TextSchema != nil {
		return s.TextSchema.CompatibleWith(other.TextSchema)
	}
	return true
}

type JSONSchema struct {
	Raw    json.RawMessage      `json:"raw"`
	Schema *gojsonschema.Schema `json:"-"` // DB 中不存该字段，需要在 unmarshal 时初始化
}

func NewJSONSchema(raw string) (*JSONSchema, error) {
	data := json.RawMessage(raw)
	s, err := gojsonschema.NewSchema(gojsonschema.NewBytesLoader(data))
	if err != nil {
		return nil, err
	}
	return &JSONSchema{Schema: s, Raw: data}, nil
}

func (s *JSONSchema) MarshalJSON() ([]byte, error) {
	return s.Raw, nil
}

func (s *JSONSchema) UnmarshalJSON(data []byte) error {
	js, err := gojsonschema.NewSchema(gojsonschema.NewBytesLoader(data))
	if err != nil {
		return err
	}
	s.Raw = data
	s.Schema = js
	return nil
}

func (s *JSONSchema) CompatibleWith(other *JSONSchema) bool {
	// 仅校验类型
	types := s.getTypes()
	otherTypes := other.getTypes()
	return gslice.ContainsAll(otherTypes, types...)
}

func (s *JSONSchema) Validate(content string) error {
	switch t := s.GetSingleType(); t {
	case consts.TypeString:
		marshaled, err := json.Marshal(content) // make unquoted content a valid json string
		if err == nil {
			content = string(marshaled)
		}
	case consts.TypeBoolean:
		content = strings.ToLower(content)
	}

	if !json.Valid([]byte(content)) { // avoid string like '2024-01-01' becomes a valid integer
		return errors.New("content is not a valid json")
	}

	doc := gojsonschema.NewStringLoader(content)
	result, err := s.Schema.Validate(doc)
	if err != nil {
		return errors.Wrap(err, "load json content")
	}

	es := result.Errors()
	if len(es) == 0 {
		return nil
	}
	errs := gslice.Map(es, func(e gojsonschema.ResultError) error {
		return errors.New(e.Description())
	})
	merr := &multierror.Error{}
	merr = multierror.Append(merr, errs...)
	return merr.ErrorOrNil()
}

// GetSingleType 获取 JSON schema 的类型，如果类型为单个字符串，则返回该字符串，否则返回空字符串
func (s *JSONSchema) GetSingleType() string {
	types := s.getTypes()
	if len(types) == 1 {
		return types[0]
	}
	return ""
}

func (s *JSONSchema) getTypes() []string {
	type t struct {
		Type any `json:"type"`
	}

	var v t
	_ = sonic.Unmarshal(s.Raw, &v)
	switch v := v.Type.(type) {
	case string:
		return []string{v}
	case []any:
		return gslice.Map(v, func(t any) string {
			vv, _ := t.(string) // avoid panic
			return vv
		})
	}
	return nil
}

type MultiModalSpec struct {
	MaxFileCount           int64                    `json:"max_file_count,omitempty"`            // 文件数量上限
	MaxFileSize            int64                    `json:"max_file_size,omitempty"`             // 文件大小上限
	SupportedFormats       []string                 `json:"supported_formats,omitempty"`         // 文件格式
	MaxPartCount           int64                    `json:"max_part_count,omitempty"`            // 多文件数量上限
	SupportedFormatsByType map[ContentType][]string `json:"supported_formats_by_type,omitempty"` // 文件格式, 按内容类型分类
	MaxFileSizeByType      map[ContentType]int64    `json:"max_file_size_by_type,omitempty"`     // 文件大小上限, 按内容类型分类
}
