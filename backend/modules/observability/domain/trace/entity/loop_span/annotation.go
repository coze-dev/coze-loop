// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package loop_span

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/samber/lo"
)

type (
	AnnotationType           string
	AnnotationValueType      string
	AnnotationCorrectionType string
	AnnotationStatus         string
)

const (
	AnnotationValueTypeString AnnotationValueType = "string"
	AnnotationValueTypeLong   AnnotationValueType = "long"
	AnnotationValueTypeDouble AnnotationValueType = "double"
	AnnotationValueTypeBool   AnnotationValueType = "bool"

	AnnotationStatusNormal   AnnotationStatus = "normal"
	AnnotationStatusInactive AnnotationStatus = "inactive"
	AnnotationStatusDeleted  AnnotationStatus = "deleted"

	AnnotationCorrectionTypeLLM    AnnotationCorrectionType = "llm"
	AnnotationCorrectionTypeManual AnnotationCorrectionType = "manual"

	AnnotationTypeAutoEvaluate        AnnotationType = "auto_evaluate"
	AnnotationTypeManualEvaluationSet AnnotationType = "manual_evaluation_set"
	AnnotationTypeManualFeedback      AnnotationType = "manual_feedback"
	AnnotationTypeCozeFeedback        AnnotationType = "coze_feedback"
)

type AnnotationValue struct {
	ValueType   AnnotationValueType `json:"value_type,omitempty"` // 类型
	LongValue   int64               `json:"long_value,omitempty"`
	StringValue string              `json:"string_value,omitempty"`
	FloatValue  float64             `json:"float_value,omitempty"`
	BoolValue   bool                `json:"bool_value,omitempty"`
}

type AnnotationCorrection struct {
	Reasoning string                   `json:"reasoning,omitempty"`
	Value     AnnotationValue          `json:"value"`
	Type      AnnotationCorrectionType `json:"type"`
	UpdateAt  time.Time                `json:"update_at"`
	UpdatedBy string                   `json:"updated_by"`
}

type AutoEvaluateMetadata struct {
	TaskID             int64 `json:"task_id"`
	EvaluatorRecordID  int64 `json:"evaluator_record_id"`
	EvaluatorVersionID int64 `json:"evaluator_version_id"`
}

type AnnotationManualFeedback struct {
	TagKeyId   int64  // 标签Key的ID
	TagKeyName string // 标签Key的名称
	TagValueId *int64 // 标签值的名称，自由文本/数值没有ID
	TagValue   string // 显示的标签值
}

type ManualEvaluationSetMetadata struct{}

type AnnotationList []*Annotation

type Annotation struct {
	ID              string
	SpanID          string
	TraceID         string
	StartTime       time.Time // start time of span
	WorkspaceID     string
	AnnotationType  AnnotationType
	AnnotationIndex []string
	Key             string
	Value           AnnotationValue
	Reasoning       string
	Corrections     []AnnotationCorrection
	Metadata        any
	Status          AnnotationStatus
	CreatedAt       time.Time
	CreatedBy       string
	UpdatedAt       time.Time
	UpdatedBy       string
	IsDeleted       bool
}

func (a *Annotation) GenID() error {
	if a.SpanID == "" {
		return fmt.Errorf("spanID is empty")
	}
	if a.TraceID == "" {
		return fmt.Errorf("traceID is empty")
	}
	if a.AnnotationType == "" {
		return fmt.Errorf("annotationType is empty")
	}
	if a.Key == "" {
		return fmt.Errorf("key is empty")
	}
	input := fmt.Sprintf("%s:%s:%s:%s", a.SpanID, a.TraceID, a.AnnotationType, a.Key)
	hash := sha256.New()
	hash.Write([]byte(input))
	hashBytes := hash.Sum(nil)
	a.ID = hex.EncodeToString(hashBytes)
	return nil
}

func (a *Annotation) GetAutoEvaluateMetadata() *AutoEvaluateMetadata {
	if a.AnnotationType != AnnotationTypeAutoEvaluate {
		return nil
	}
	AnnotationMetaData := a.Metadata
	metadata, ok := AnnotationMetaData.(AutoEvaluateMetadata)
	if !ok {
		return nil
	}
	return &metadata
}

func (a *Annotation) GetEvaluationSetMetadata() *ManualEvaluationSetMetadata {
	if a.AnnotationType != AnnotationTypeManualEvaluationSet {
		return nil
	}
	metadata, ok := a.Metadata.(*ManualEvaluationSetMetadata)
	if !ok {
		return nil
	}
	return metadata
}

func (a AnnotationList) GetUserIDs() []string {
	if len(a) == 0 {
		return nil
	}
	result := make([]string, 0)
	seen := make(map[string]bool)
	for _, annotation := range a {
		userId := annotation.UpdatedBy
		if userId == "" {
			continue
		} else if seen[userId] {
			continue
		}
		seen[userId] = true
		result = append(result, userId)
	}
	return result
}

func (a AnnotationList) GetAnnotationTagIDs() []string {
	if len(a) == 0 {
		return nil
	}
	result := make([]string, 0)
	seen := make(map[string]bool)
	for _, annotation := range a {
		if annotation.AnnotationType != AnnotationTypeManualFeedback {
			continue
		}
		tagKeyId := annotation.Key
		if tagKeyId == "" {
			continue
		} else if seen[tagKeyId] {
			continue
		}
		seen[tagKeyId] = true
		result = append(result, tagKeyId)
	}
	return result
}

func (a AnnotationList) GetEvaluatorVersionIDs() []int64 {
	if len(a) == 0 {
		return nil
	}
	result := make([]int64, 0)
	seen := make(map[int64]bool)
	for _, annotation := range a {
		if annotation.AnnotationType != AnnotationTypeAutoEvaluate {
			continue
		}
		meta := annotation.GetAutoEvaluateMetadata()
		if meta == nil {
			continue
		}
		versionId := meta.EvaluatorVersionID
		if versionId <= 0 {
			continue
		} else if seen[versionId] {
			continue
		}
		seen[versionId] = true
		result = append(result, versionId)
	}
	return result
}

func (a AnnotationList) Uniq() AnnotationList {
	return lo.UniqBy(a, func(item *Annotation) string {
		return item.ID
	})
}

func NewStringValue(v string) AnnotationValue {
	return AnnotationValue{
		ValueType:   AnnotationValueTypeString,
		StringValue: v,
	}
}

func NewLongValue(v int64) AnnotationValue {
	return AnnotationValue{
		ValueType: AnnotationValueTypeLong,
		LongValue: v,
	}
}

func NewDoubleValue(v float64) AnnotationValue {
	return AnnotationValue{
		ValueType:  AnnotationValueTypeDouble,
		FloatValue: v,
	}
}

func NewBoolValue(v bool) AnnotationValue {
	return AnnotationValue{
		ValueType: AnnotationValueTypeBool,
		BoolValue: v,
	}
}
