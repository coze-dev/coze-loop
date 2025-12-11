// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"strconv"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/ck/gorm_gen/model"
)

// ExptTurnResultFilterEntity2PO 将 ExptTurnResultFilterEntity 转换为 model.ExptTurnResultFilterAccelerator
func ExptTurnResultFilterEntity2PO(filterEntity *entity.ExptTurnResultFilterEntity) *model.ExptTurnResultFilter {
	if filterEntity == nil {
		return nil
	}

	annotationBool := make(map[string]int8)
	for k, v := range filterEntity.AnnotationBool {
		if v {
			annotationBool[k] = 1
		} else {
			annotationBool[k] = 0
		}
	}

	exptTurnResultFilter := &model.ExptTurnResultFilter{
		SpaceID:          strconv.FormatInt(filterEntity.SpaceID, 10),
		ExptID:           strconv.FormatInt(filterEntity.ExptID, 10),
		ItemID:           strconv.FormatInt(filterEntity.ItemID, 10),
		ItemIdx:          filterEntity.ItemIdx,
		TurnID:           strconv.FormatInt(filterEntity.TurnID, 10),
		Status:           int32(filterEntity.Status),
		EvalTargetData:   filterEntity.EvalTargetData,
		EvaluatorScore:   filterEntity.EvaluatorScore,
		AnnotationFloat:  filterEntity.AnnotationFloat,
		AnnotationBool:   annotationBool,
		AnnotationString: filterEntity.AnnotationString,
		EvalTargetMetrics: filterEntity.EvalTargetMetrics,
		CreatedDate:      filterEntity.CreatedDate,
		EvalSetVersionID: strconv.FormatInt(filterEntity.EvalSetVersionID, 10),
		UpdatedAt:        filterEntity.UpdatedAt,
	}
	if filterEntity.EvaluatorScoreCorrected {
		exptTurnResultFilter.EvaluatorScoreCorrected = 1
	}

	return exptTurnResultFilter
}

// ExptTurnResultFilterPO2Entity 将 model.ExptTurnResultFilterAccelerator 转换为 ExptTurnResultFilterEntity
func ExptTurnResultFilterPO2Entity(filterPO *model.ExptTurnResultFilter) *entity.ExptTurnResultFilterEntity {
	if filterPO == nil {
		return nil
	}

	annotationBool := make(map[string]bool)
	for k, v := range filterPO.AnnotationBool {
		annotationBool[k] = v > 0
	}

	exptTurnResultFilterEntity := &entity.ExptTurnResultFilterEntity{
		SpaceID:          ParseStringToInt64(filterPO.SpaceID),
		ExptID:           ParseStringToInt64(filterPO.ExptID),
		ItemID:           ParseStringToInt64(filterPO.ItemID),
		ItemIdx:          filterPO.ItemIdx,
		TurnID:           ParseStringToInt64(filterPO.TurnID),
		Status:           entity.ItemRunState(filterPO.Status),
		EvalTargetData:   filterPO.EvalTargetData,
		EvaluatorScore:   filterPO.EvaluatorScore,
		AnnotationFloat:  filterPO.AnnotationFloat,
		AnnotationBool:   annotationBool,
		AnnotationString: filterPO.AnnotationString,
		EvalTargetMetrics: filterPO.EvalTargetMetrics,
		CreatedDate:      filterPO.CreatedDate,
		EvalSetVersionID: ParseStringToInt64(filterPO.EvalSetVersionID),
	}
	if filterPO.EvaluatorScoreCorrected > 0 {
		exptTurnResultFilterEntity.EvaluatorScoreCorrected = true
	}
	return exptTurnResultFilterEntity
}

// ParseStringToInt64 将 string 转换为 int64
func ParseStringToInt64(s string) int64 {
	if s == "" {
		return 0
	}
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return i
}
