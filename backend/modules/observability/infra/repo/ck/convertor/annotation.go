// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"fmt"
	"time"

	"github.com/bytedance/sonic"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/ck/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

func AnnotationPO2DO(annotation *model.ObservabilityAnnotation) *loop_span.Annotation {
	if annotation == nil {
		return nil
	}
	ret := &loop_span.Annotation{
		ID:              annotation.ID,
		SpanID:          annotation.SpanID,
		TraceID:         annotation.TraceID,
		StartTime:       time.UnixMicro(annotation.StartTime),
		WorkspaceID:     annotation.SpaceID,
		AnnotationType:  loop_span.AnnotationType(annotation.AnnotationType),
		AnnotationIndex: annotation.AnnotationIndex,
		Key:             annotation.Key,
		Reasoning:       annotation.Reasoning,
		Status:          loop_span.AnnotationStatus(annotation.Status),
		CreatedBy:       annotation.CreatedBy,
		CreatedAt:       time.UnixMicro(int64(annotation.CreatedAt)),
		UpdatedBy:       annotation.UpdatedBy,
		UpdatedAt:       time.UnixMicro(int64(annotation.UpdatedAt)),
	}
	ret.Value = loop_span.AnnotationValue{
		ValueType: loop_span.AnnotationValueType(annotation.ValueType),
	}
	switch ret.Value.ValueType {
	case loop_span.AnnotationValueTypeString:
		ret.Value.StringValue = annotation.ValueString
	case loop_span.AnnotationValueTypeLong:
		ret.Value.LongValue = annotation.ValueLong
	case loop_span.AnnotationValueTypeDouble:
		ret.Value.FloatValue = annotation.ValueFloat
	case loop_span.AnnotationValueTypeBool:
		ret.Value.BoolValue = annotation.ValueBool
	}
	if annotation.Metadata != "" {
		switch ret.AnnotationType {
		case loop_span.AnnotationTypeAutoEvaluate:
			var metadata loop_span.AutoEvaluateMetadata
			err := json.Unmarshal([]byte(annotation.Metadata), &metadata)
			if err != nil {
				logs.Error("json unmarshal metadata error: %v", err)
			} else {
				ret.Metadata = metadata
			}
		case loop_span.AnnotationTypeManualEvaluationSet, loop_span.AnnotationTypeManualDataset:
			var metadata loop_span.ManualDatasetMetadata
			err := json.Unmarshal([]byte(annotation.Metadata), &metadata)
			if err != nil {
				logs.Error("json unmarshal metadata error: %v", err)
			} else {
				ret.Metadata = metadata
			}
		}
	}
	if annotation.Correction != "" {
		var corrections []loop_span.AnnotationCorrection
		err := json.Unmarshal([]byte(annotation.Correction), &corrections)
		if err != nil {
			logs.Error("json unmarshal correction error: %v", err)
		} else {
			ret.Corrections = corrections
		}
	}
	if annotation.DeletedAt > 0 {
		ret.IsDeleted = true
	}
	return ret
}

func AnnotationDO2PO(annotation *loop_span.Annotation) (*model.ObservabilityAnnotation, error) {
	ret := &model.ObservabilityAnnotation{
		ID:              annotation.ID,
		SpanID:          annotation.SpanID,
		TraceID:         annotation.TraceID,
		StartTime:       annotation.StartTime.UnixMicro(),
		SpaceID:         annotation.WorkspaceID,
		AnnotationType:  string(annotation.AnnotationType),
		AnnotationIndex: annotation.AnnotationIndex,
		Reasoning:       annotation.Reasoning,
		Key:             annotation.Key,
		ValueType:       string(annotation.Value.ValueType),
		Status:          string(annotation.Status),
		CreatedBy:       annotation.CreatedBy,
		CreatedAt:       uint64(annotation.CreatedAt.UnixMicro()),
		UpdatedBy:       annotation.UpdatedBy,
		UpdatedAt:       uint64(annotation.UpdatedAt.UnixMicro()),
	}
	if annotation.IsDeleted {
		ret.DeletedAt = uint64(time.Now().UnixMicro())
	}
	if len(annotation.Corrections) > 0 {
		corrections, err := sonic.MarshalString(annotation.Corrections)
		if err != nil {
			return nil, fmt.Errorf("fail to marshal corrections %v, %v", annotation.Corrections, err)
		}
		ret.Correction = corrections
	}
	if annotation.Metadata != nil {
		metadata, err := sonic.MarshalString(annotation.Metadata)
		if err != nil {
			return nil, fmt.Errorf("fail to marshal metadata %v, %v", annotation.Metadata, err)
		}
		ret.Metadata = metadata
	}
	switch annotation.Value.ValueType {
	case loop_span.AnnotationValueTypeString:
		ret.ValueString = annotation.Value.StringValue
	case loop_span.AnnotationValueTypeLong:
		ret.ValueLong = annotation.Value.LongValue
	case loop_span.AnnotationValueTypeDouble:
		ret.ValueFloat = annotation.Value.FloatValue
	case loop_span.AnnotationValueTypeBool:
		ret.ValueBool = annotation.Value.BoolValue
	}
	ret.StartDate = getStartDate(annotation.StartTime)
	return ret, nil
}

func AnnotationListPO2DO(annotations []*model.ObservabilityAnnotation) loop_span.AnnotationList {
	ret := make(loop_span.AnnotationList, len(annotations))
	for i, annotation := range annotations {
		ret[i] = AnnotationPO2DO(annotation)
	}
	return ret
}

func getStartDate(st time.Time) string {
	const layout = "2006-01-02"
	return st.Format(layout)
}
