// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package target

import (
	"time"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_target"
	commonconvertor "github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/common"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func EvalTargetRecordDO2DTO(src *entity.EvalTargetRecord) *eval_target.EvalTargetRecord {
	if src == nil {
		return nil
	}

	res := &eval_target.EvalTargetRecord{
		ID:                   &src.ID,
		WorkspaceID:          &src.SpaceID,
		TargetID:             &src.TargetID,
		TargetVersionID:      &src.TargetVersionID,
		ExperimentRunID:      &src.ExperimentRunID,
		ItemID:               &src.ItemID,
		TurnID:               &src.TurnID,
		TraceID:              &src.TraceID,
		LogID:                &src.LogID,
		EvalTargetInputData:  InputDO2DTO(src.EvalTargetInputData),
		EvalTargetOutputData: OutputDO2DTO(src.EvalTargetOutputData),
		Status:               StatusDO2DTO(src.Status),
		BaseInfo: &common.BaseInfo{
			// TODO
			// CreatedBy: src.BaseInfo.CreatedBy,
			// UpdatedBy: src.BaseInfo.UpdatedBy,
			CreatedAt: src.BaseInfo.CreatedAt,
			UpdatedAt: src.BaseInfo.UpdatedAt,
			DeletedAt: src.BaseInfo.DeletedAt,
		},
	}

	return res
}

// RecordDTO2DO 将DTO层结构转换回DO层结构
func RecordDTO2DO(src *eval_target.EvalTargetRecord) *entity.EvalTargetRecord {
	if src == nil {
		return nil
	}

	return &entity.EvalTargetRecord{
		ID:                   getInt64Value(src.ID),
		SpaceID:              getInt64Value(src.WorkspaceID),
		TargetID:             getInt64Value(src.TargetID),
		TargetVersionID:      getInt64Value(src.TargetVersionID),
		ExperimentRunID:      getInt64Value(src.ExperimentRunID),
		ItemID:               getInt64Value(src.ItemID),
		TurnID:               getInt64Value(src.TurnID),
		TraceID:              getStringValue(src.TraceID),
		LogID:                getStringValue(src.LogID),
		EvalTargetInputData:  InputDTO2ToDO(src.EvalTargetInputData),
		EvalTargetOutputData: OutputDTO2ToDO(src.EvalTargetOutputData),
		Status:               StatusDTO2DO(src.Status),
		BaseInfo: &entity.BaseInfo{
			// todo
			// CreatedBy: src.GetBaseInfo().GetCreatedBy(),
			// UpdatedBy: src.GetBaseInfo().GetUpdatedBy(),
			CreatedAt: src.BaseInfo.CreatedAt,
			UpdatedAt: src.BaseInfo.UpdatedAt,
			DeletedAt: src.BaseInfo.DeletedAt,
		},
	}
}

func UnixMsPtr2Time(ms *int64) time.Time {
	if ms == nil {
		return time.Time{}
	}
	if *ms < 0 {
		return time.Time{}
	}
	return time.Unix(0, *ms*int64(time.Millisecond))
}

// DO->DTO转换函数组
func InputDO2DTO(src *entity.EvalTargetInputData) *eval_target.EvalTargetInputData {
	if src == nil {
		return nil
	}
	return &eval_target.EvalTargetInputData{
		HistoryMessages: MessagesDO2DTOs(src.HistoryMessages),
		InputFields:     ContentDOToDTOs(src.InputFields),
		Ext:             src.Ext,
	}
}

func OutputDO2DTO(src *entity.EvalTargetOutputData) *eval_target.EvalTargetOutputData {
	if src == nil {
		return nil
	}
	return &eval_target.EvalTargetOutputData{
		OutputFields:       ContentDOToDTOs(src.OutputFields),
		EvalTargetUsage:    UsageDO2DTO(src.EvalTargetUsage),
		EvalTargetRunError: RunErrorDO2DTO(src.EvalTargetRunError),
		TimeConsumingMs:    src.TimeConsumingMS,
	}
}

// DTO->DO转换函数组
func InputDTO2ToDO(src *eval_target.EvalTargetInputData) *entity.EvalTargetInputData {
	if src == nil {
		return nil
	}
	return &entity.EvalTargetInputData{
		HistoryMessages: MessagesDTO2DO(src.HistoryMessages),
		InputFields:     ContentDTO2DOs(src.InputFields),
		Ext:             src.Ext,
	}
}

func OutputDTO2ToDO(src *eval_target.EvalTargetOutputData) *entity.EvalTargetOutputData {
	if src == nil {
		return nil
	}
	return &entity.EvalTargetOutputData{
		OutputFields:       ContentDTO2DOs(src.OutputFields),
		EvalTargetUsage:    UsageDTO2DO(src.EvalTargetUsage),
		EvalTargetRunError: RunErrorDTO2DO(src.EvalTargetRunError),
		TimeConsumingMS:    src.TimeConsumingMs,
	}
}

// 状态枚举转换
func StatusDO2DTO(src *entity.EvalTargetRunStatus) *eval_target.EvalTargetRunStatus {
	if src == nil {
		return nil
	}
	status := eval_target.EvalTargetRunStatus(*src)
	return &status
}

func StatusDTO2DO(src *eval_target.EvalTargetRunStatus) *entity.EvalTargetRunStatus {
	if src == nil {
		return nil
	}
	status := entity.EvalTargetRunStatus(*src)
	return &status
}

// 类型安全转换（假设底层类型相同）
func MessagesDO2DTOs(src []*entity.Message) []*common.Message {
	res := make([]*common.Message, 0)
	if len(src) == 0 {
		return res
	}
	for _, message := range src {
		res = append(res, commonconvertor.ConvertMessageDO2DTO(message))
	}
	return res
}

func ContentDOToDTOs(src map[string]*entity.Content) map[string]*common.Content {
	res := make(map[string]*common.Content)
	if len(src) == 0 {
		return res
	}
	for k, v := range src {
		res[k] = commonconvertor.ConvertContentDO2DTO(v)
	}
	return res
}

func MessagesDTO2DO(src []*common.Message) []*entity.Message {
	res := make([]*entity.Message, 0)
	if len(src) == 0 {
		return res
	}
	for _, message := range src {
		if message == nil {
			continue
		}
		res = append(res, commonconvertor.ConvertMessageDTO2DO(message))
	}

	return res
}

func ContentDTO2DOs(src map[string]*common.Content) map[string]*entity.Content {
	res := make(map[string]*entity.Content)
	if len(src) == 0 {
		return res
	}
	for k, v := range src {
		if v == nil {
			res[k] = nil
			continue
		}
		res[k] = commonconvertor.ConvertContentDTO2DO(v)
	}
	return res
}

// 辅助函数保持不变
func getInt64Value(ptr *int64) int64 {
	if ptr != nil {
		return *ptr
	}
	return 0
}

func getStringValue(ptr *string) string {
	if ptr != nil {
		return *ptr
	}
	return ""
}

// 其他嵌套类型转换
func UsageDO2DTO(src *entity.EvalTargetUsage) *eval_target.EvalTargetUsage {
	if src == nil {
		return nil
	}
	return &eval_target.EvalTargetUsage{
		InputTokens:  src.InputTokens,
		OutputTokens: src.OutputTokens,
	}
}

func RunErrorDO2DTO(src *entity.EvalTargetRunError) *eval_target.EvalTargetRunError {
	if src == nil {
		return nil
	}
	return &eval_target.EvalTargetRunError{
		Code:    &src.Code,
		Message: &src.Message,
	}
}

func UsageDTO2DO(src *eval_target.EvalTargetUsage) *entity.EvalTargetUsage {
	if src == nil {
		return nil
	}
	return &entity.EvalTargetUsage{
		InputTokens:  src.InputTokens,
		OutputTokens: src.OutputTokens,
	}
}

func RunErrorDTO2DO(src *eval_target.EvalTargetRunError) *entity.EvalTargetRunError {
	if src == nil {
		return nil
	}
	return &entity.EvalTargetRunError{
		Code:    getInt32Value(src.Code),
		Message: getStringValue(src.Message),
	}
}

func getInt32Value(ptr *int32) int32 {
	if ptr != nil {
		return *ptr
	}
	return 0
}
