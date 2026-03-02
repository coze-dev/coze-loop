// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"time"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

func ConvertEvaluatorRecordDO2PO(do *entity.EvaluatorRecord) *model.EvaluatorRecord {
	// 若 do 为 nil，直接返回 nil
	if do == nil {
		return nil
	}
	po := &model.EvaluatorRecord{
		ID:                 do.ID,
		SpaceID:            do.SpaceID,
		ExperimentID:       gptr.Of(do.ExperimentID),
		ExperimentRunID:    do.ExperimentRunID,
		ItemID:             do.ItemID,
		EvaluatorVersionID: do.EvaluatorVersionID,
		TurnID:             do.TurnID,
		LogID:              gptr.Of(do.LogID),
		TraceID:            do.TraceID,
		Status:             int32(do.Status),
	}

	// 大字段已在 Save 时剪裁并放回结构体
	if do.EvaluatorInputData != nil {
		inputDataBytes, err := json.Marshal(do.EvaluatorInputData)
		if err != nil {
			return nil
		}
		po.InputData = gptr.Of(inputDataBytes)
	}

	if do.EvaluatorOutputData != nil {
		outputDataBytes, err := json.Marshal(do.EvaluatorOutputData)
		if err != nil {
			return nil
		}
		po.OutputData = gptr.Of(outputDataBytes)
		if do.EvaluatorOutputData.EvaluatorResult != nil {
			if do.EvaluatorOutputData.EvaluatorResult.Correction != nil {
				po.UpdatedBy = do.EvaluatorOutputData.EvaluatorResult.Correction.UpdatedBy
				po.Score = do.EvaluatorOutputData.EvaluatorResult.Correction.Score
			} else {
				po.Score = do.EvaluatorOutputData.EvaluatorResult.Score
			}
		}
	}

	if do.BaseInfo != nil {
		if do.BaseInfo.CreatedBy != nil {
			po.CreatedBy = gptr.Indirect(do.BaseInfo.CreatedBy.UserID)
		}
		if do.BaseInfo.UpdatedBy != nil {
			po.UpdatedBy = gptr.Indirect(do.BaseInfo.UpdatedBy.UserID)
		}
		if do.BaseInfo.CreatedAt != nil {
			po.CreatedAt = time.UnixMilli(gptr.Indirect(do.BaseInfo.CreatedAt))
		}
		if do.BaseInfo.UpdatedAt != nil {
			po.UpdatedAt = time.UnixMilli(gptr.Indirect(do.BaseInfo.UpdatedAt))
		}
	}

	if len(do.Ext) > 0 {
		extBytes, err := json.Marshal(do.Ext)
		if err != nil {
			return nil
		}
		po.Ext = gptr.Of(extBytes)
	}

	return po
}

// ConvertEvaluatorRecordPO2DO 将 model.EvaluatorRecord 类型的 PO 对象转换为 evaluator_record.EvaluatorRecord 类型的 DO 对象
func ConvertEvaluatorRecordPO2DO(po *model.EvaluatorRecord) (*entity.EvaluatorRecord, error) {
	// 若 po 为 nil，直接返回 nil
	if po == nil {
		return nil, nil
	}
	do := &entity.EvaluatorRecord{}

	do.ID = po.ID
	do.SpaceID = po.SpaceID
	if po.ExperimentID != nil {
		do.ExperimentID = *po.ExperimentID
	}
	do.ExperimentRunID = po.ExperimentRunID
	do.ItemID = po.ItemID
	do.EvaluatorVersionID = po.EvaluatorVersionID
	do.TraceID = po.TraceID
	if po.LogID != nil {
		do.LogID = *po.LogID
	}
	do.TurnID = po.TurnID
	do.Status = entity.EvaluatorRunStatus(po.Status)

	if po.InputData != nil {
		inputData := &entity.EvaluatorInputData{}
		err := json.Unmarshal(*po.InputData, inputData)
		if err != nil {
			return nil, err
		}
		do.EvaluatorInputData = inputData
	}

	if po.OutputData != nil {
		outputData := &entity.EvaluatorOutputData{}
		err := json.Unmarshal(*po.OutputData, outputData)
		if err != nil {
			return nil, err
		}
		do.EvaluatorOutputData = outputData
	}

	do.BaseInfo = &entity.BaseInfo{
		CreatedAt: gptr.Of(po.CreatedAt.UnixMilli()),
		UpdatedAt: gptr.Of(po.UpdatedAt.UnixMilli()),
		CreatedBy: &entity.UserInfo{UserID: gptr.Of(po.CreatedBy)},
		UpdatedBy: &entity.UserInfo{UserID: gptr.Of(po.UpdatedBy)},
	}
	if po.DeletedAt.Valid {
		do.BaseInfo.DeletedAt = gptr.Of(po.DeletedAt.Time.UnixMilli())
	}

	if po.Ext != nil {
		do.Ext = make(map[string]string)
		err := json.Unmarshal(gptr.Indirect(po.Ext), &do.Ext)
		if err != nil {
			return nil, err
		}
	}

	return do, nil
}
