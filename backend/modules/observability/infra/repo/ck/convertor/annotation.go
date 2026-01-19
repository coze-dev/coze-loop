// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package convertor

import (
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/ck/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/dao"
)

func AnnotationListCKModels2PO(annotations []*model.ObservabilityAnnotation) []*dao.Annotation {
	ret := make([]*dao.Annotation, len(annotations))
	for i, annotation := range annotations {
		ret[i] = AnnotationCKModel2PO(annotation)
	}
	return ret
}

func AnnotationListPO2CKModels(annotations []*dao.Annotation) []*model.ObservabilityAnnotation {
	ret := make([]*model.ObservabilityAnnotation, len(annotations))
	for i, annotation := range annotations {
		ret[i] = AnnotationPO2CKModel(annotation)
	}
	return ret
}

func AnnotationPO2CKModel(anno *dao.Annotation) *model.ObservabilityAnnotation {
	if anno == nil {
		return nil
	}
	return &model.ObservabilityAnnotation{
		ID:              anno.ID,
		SpanID:          anno.SpanID,
		TraceID:         anno.TraceID,
		StartTime:       anno.StartTime,
		SpaceID:         anno.SpaceID,
		AnnotationType:  anno.AnnotationType,
		AnnotationIndex: anno.AnnotationIndex,
		Key:             anno.Key,
		ValueType:       anno.ValueType,
		ValueFloat:      anno.ValueFloat,
		ValueString:     anno.ValueString,
		ValueBool:       anno.ValueBool,
		ValueLong:       anno.ValueLong,
		Reasoning:       anno.Reasoning,
		Correction:      anno.Correction,
		Metadata:        anno.Metadata,
		Status:          anno.Status,
		CreatedBy:       anno.CreatedBy,
		CreatedAt:       anno.CreatedAt,
		UpdatedBy:       anno.UpdatedBy,
		UpdatedAt:       anno.UpdatedAt,
		DeletedAt:       anno.DeletedAt,
		StartDate:       anno.StartDate,
	}
}

func AnnotationCKModel2PO(anno *model.ObservabilityAnnotation) *dao.Annotation {
	if anno == nil {
		return nil
	}
	return &dao.Annotation{
		ID:              anno.ID,
		SpanID:          anno.SpanID,
		TraceID:         anno.TraceID,
		StartTime:       anno.StartTime,
		SpaceID:         anno.SpaceID,
		AnnotationType:  anno.AnnotationType,
		AnnotationIndex: anno.AnnotationIndex,
		Key:             anno.Key,
		ValueType:       anno.ValueType,
		ValueFloat:      anno.ValueFloat,
		ValueString:     anno.ValueString,
		ValueBool:       anno.ValueBool,
		ValueLong:       anno.ValueLong,
		Reasoning:       anno.Reasoning,
		Correction:      anno.Correction,
		Metadata:        anno.Metadata,
		Status:          anno.Status,
		CreatedBy:       anno.CreatedBy,
		CreatedAt:       anno.CreatedAt,
		UpdatedBy:       anno.UpdatedBy,
		UpdatedAt:       anno.UpdatedAt,
		DeletedAt:       anno.DeletedAt,
		StartDate:       anno.StartDate,
	}
}
