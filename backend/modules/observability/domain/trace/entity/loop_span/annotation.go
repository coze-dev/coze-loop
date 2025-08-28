// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package loop_span

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/apaxa-go/helper/strconvh"
	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/annotation"
	domain_common "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/common"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
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

func (a *Annotation) CorrectAutoEvaluateScore(score float64, reasoning string, updateBy string) {
	if a.Corrections == nil {
		// 首次修改时，先记录一下LLM的原始值
		a.Corrections = make([]AnnotationCorrection, 0)
		a.Corrections = append(a.Corrections, AnnotationCorrection{
			Reasoning: a.Reasoning,
			Value:     a.Value,
			Type:      AnnotationCorrectionTypeLLM,
			UpdateAt:  a.UpdatedAt,
			UpdatedBy: a.UpdatedBy,
		})
	}
	// 增加人工修改的记录
	a.Corrections = append(a.Corrections, AnnotationCorrection{
		Reasoning: reasoning,
		Value:     NewDoubleValue(score),
		Type:      AnnotationCorrectionTypeManual,
		UpdateAt:  time.Now(),
		UpdatedBy: updateBy,
	})
	// 更新当前值
	a.Reasoning = reasoning
	a.Value = NewDoubleValue(score)
	a.UpdatedBy = updateBy
	a.UpdatedAt = time.Now()
}

func (a *Annotation) ToFornaxAnnotation(ctx context.Context) (fa *annotation.Annotation) {
	fa = &annotation.Annotation{}
	fa.ID = lo.ToPtr(a.ID)
	fa.Type = lo.ToPtr(string(a.AnnotationType))
	fa.Key = lo.ToPtr(a.Key)

	fa.Value = lo.ToPtr(a.Value.StringValue)
	switch a.Value.ValueType {
	case annotation.ValueTypeString:
		fa.ValueType = lo.ToPtr(annotation.ValueTypeString)
		fa.Value = lo.ToPtr(a.Value.StringValue)
	case annotation.ValueTypeLong:
		fa.ValueType = lo.ToPtr(annotation.ValueTypeLong)
		fa.Value = lo.ToPtr(strconvh.FormatInt64(a.Value.LongValue))
	case annotation.ValueTypeDouble:
		fa.ValueType = lo.ToPtr(annotation.ValueTypeDouble)
		fa.Value = lo.ToPtr(strconvh.FormatFloat64(a.Value.FloatValue))
	case annotation.ValueTypeBool:
		fa.ValueType = lo.ToPtr(annotation.ValueTypeBool)
		fa.Value = lo.ToPtr(strconvh.FormatBool(a.Value.BoolValue))
	default:
		logs.CtxWarn(ctx, "toFornaxAnnotation invalid ValueType", "ValueType", a.Value.ValueType)
	}
	switch a.AnnotationType {
	case annotation.AnnotationTypeAutoEvaluate:
		fa.AutoEvaluate = a.toAutoEvaluate()
	default:
		logs.CtxWarn(ctx, "toFornaxAnnotation invalid AnnotationType", "AnnotationType", a.AnnotationType)
	}
	fa.SetBaseInfo(&domain_common.BaseInfo{
		CreatedBy: &domain_common.UserInfo{UserID: gptr.Of(a.CreatedBy)},
		UpdatedBy: &domain_common.UserInfo{UserID: gptr.Of(a.UpdatedBy)},
		CreatedAt: gptr.Of(a.CreatedAt.UnixMilli()),
		UpdatedAt: gptr.Of(a.UpdatedAt.UnixMilli()),
	})
	return fa
}
func (a *Annotation) toAutoEvaluate() *annotation.AutoEvaluate {
	metadata := a.GetAutoEvaluateMetadata()
	if metadata == nil {
		return nil
	}
	res := annotation.NewAutoEvaluate()
	res.EvaluatorVersionID = metadata.EvaluatorVersionID
	res.TaskID = strconv.FormatInt(metadata.TaskID, 10)
	res.RecordID = metadata.EvaluatorRecordID
	res.EvaluatorResult_ = annotation.NewEvaluatorResult_()
	res.EvaluatorResult_.Score = lo.ToPtr(a.Value.FloatValue)
	res.EvaluatorResult_.Reasoning = lo.ToPtr(a.Reasoning)
	if len(a.Corrections) > 0 {
		// 取最后一个人工修改的记录
		manualCorrections := lo.Filter(a.Corrections, func(item AnnotationCorrection, index int) bool {
			return item.Type == AnnotationCorrectionTypeManual
		})
		if len(manualCorrections) > 0 {
			manualCorrection := manualCorrections[len(manualCorrections)-1]
			res.EvaluatorResult_.Correction = annotation.NewCorrection()
			res.EvaluatorResult_.Correction.Score = lo.ToPtr(manualCorrection.Value.FloatValue)
			res.EvaluatorResult_.Correction.Explain = lo.ToPtr(manualCorrection.Reasoning)
		}
	}
	return res
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
