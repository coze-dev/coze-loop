// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"database/sql"
	"database/sql/driver"
	"fmt"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/dataset"
)

type EvaluationSetSchema struct {
	ID              int64          `json:"id,omitempty"`
	AppID           int32          `json:"app_id,omitempty"`
	SpaceID         int64          `json:"space_id,omitempty"`
	EvaluationSetID int64          `json:"evaluation_set_id,omitempty"`
	FieldSchemas    []*FieldSchema `json:"field_schemas,omitempty"`
	BaseInfo        *BaseInfo      `json:"base_info,omitempty"`
}

type FieldSchema struct {
	Key                    string                               `json:"key,omitempty"`
	Name                   string                               `json:"name,omitempty"`
	Description            string                               `json:"description,omitempty"`
	ContentType            ContentType                          `json:"content_type,omitempty"`
	DefaultDisplayFormat   FieldDisplayFormat                   `json:"default_display_format,omitempty"`
	Status                 FieldStatus                          `json:"status,omitempty"`
	TextSchema             string                               `json:"text_schema,omitempty"`
	MultiModelSpec         *MultiModalSpec                      `json:"multi_model_spec,omitempty"`
	Hidden                 bool                                 `json:"hidden,omitempty"`
	IsRequired             bool                                 `json:"is_required,omitempty"`
	DefaultTransformations []*dataset.FieldTransformationConfig `json:"default_transformations,omitempty"`
}

type MultiModalSpec struct {
	MaxFileCount     int64    `json:"max_file_count,omitempty"`
	MaxFileSize      int64    `json:"max_file_size,omitempty"`
	SupportedFormats []string `json:"supported_formats,omitempty"`
}

type FieldDisplayFormat int64

const (
	FieldDisplayFormat_PlainText FieldDisplayFormat = 1
	FieldDisplayFormat_Markdown  FieldDisplayFormat = 2
	FieldDisplayFormat_JSON      FieldDisplayFormat = 3
	FieldDisplayFormat_YAML      FieldDisplayFormat = 4
	FieldDisplayFormat_Code      FieldDisplayFormat = 5
)

func (p FieldDisplayFormat) String() string {
	switch p {
	case FieldDisplayFormat_PlainText:
		return "PlainText"
	case FieldDisplayFormat_Markdown:
		return "Markdown"
	case FieldDisplayFormat_JSON:
		return "JSON"
	case FieldDisplayFormat_YAML:
		return "YAML"
	case FieldDisplayFormat_Code:
		return "Code"
	}
	return "<UNSET>"
}

func FieldDisplayFormatFromString(s string) (FieldDisplayFormat, error) {
	switch s {
	case "PlainText":
		return FieldDisplayFormat_PlainText, nil
	case "Markdown":
		return FieldDisplayFormat_Markdown, nil
	case "JSON":
		return FieldDisplayFormat_JSON, nil
	case "YAML":
		return FieldDisplayFormat_YAML, nil
	case "Code":
		return FieldDisplayFormat_Code, nil
	}
	return FieldDisplayFormat(0), fmt.Errorf("not a valid FieldDisplayFormat string")
}

func FieldDisplayFormatPtr(v FieldDisplayFormat) *FieldDisplayFormat { return &v }
func (p *FieldDisplayFormat) Scan(value interface{}) (err error) {
	var result sql.NullInt64
	err = result.Scan(value)
	*p = FieldDisplayFormat(result.Int64)
	return
}

func (p *FieldDisplayFormat) Value() (driver.Value, error) {
	if p == nil {
		return nil, nil
	}
	return int64(*p), nil
}

type FieldStatus int64

const (
	FieldStatus_Available FieldStatus = 1
	FieldStatus_Deleted   FieldStatus = 2
)

func (p FieldStatus) String() string {
	switch p {
	case FieldStatus_Available:
		return "Available"
	case FieldStatus_Deleted:
		return "Deleted"
	}
	return "<UNSET>"
}

func FieldStatusFromString(s string) (FieldStatus, error) {
	switch s {
	case "Available":
		return FieldStatus_Available, nil
	case "Deleted":
		return FieldStatus_Deleted, nil
	}
	return FieldStatus(0), fmt.Errorf("not a valid FieldStatus string")
}

func FieldStatusPtr(v FieldStatus) *FieldStatus { return &v }
func (p *FieldStatus) Scan(value interface{}) (err error) {
	var result sql.NullInt64
	err = result.Scan(value)
	*p = FieldStatus(result.Int64)
	return
}

func (p *FieldStatus) Value() (driver.Value, error) {
	if p == nil {
		return nil, nil
	}
	return int64(*p), nil
}
