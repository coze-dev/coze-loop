// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"time"

	"github.com/bytedance/gg/gptr"
	"gorm.io/gorm"

	evaluatordo "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

// ConvertEvaluatorTemplateDO2PO 将 EvaluatorTemplate 的 DO 对象转换为 PO 对象
func ConvertEvaluatorTemplateDO2PO(do *evaluatordo.EvaluatorTemplate) (*model.EvaluatorTemplate, error) {
	if do == nil {
		return nil, nil
	}

	po := &model.EvaluatorTemplate{
		ID:                 do.ID,
		SpaceID:            ptr.Of(do.SpaceID),
		Name:               ptr.Of(do.Name),
		Description:        ptr.Of(do.Description),
		EvaluatorType:      ptr.Of(int32(do.EvaluatorType)),
		ReceiveChatHistory: do.ReceiveChatHistory,
		Popularity:         do.Popularity,
	}
	if do.EvaluatorInfo != nil {
		b, err := json.Marshal(do.EvaluatorInfo)
		if err == nil {
			po.EvaluatorInfo = ptr.Of(b)
		}
	}

	// 序列化InputSchema
	if len(do.InputSchemas) > 0 {
		inputSchemaByte, err := json.Marshal(do.InputSchemas)
		if err != nil {
			return nil, err
		}
		po.InputSchema = ptr.Of(inputSchemaByte)
	}

	// 序列化OutputSchema
	if len(do.OutputSchemas) > 0 {
		outputSchemaByte, err := json.Marshal(do.OutputSchemas)
		if err != nil {
			return nil, err
		}
		po.OutputSchema = ptr.Of(outputSchemaByte)
	}

	// 根据EvaluatorType序列化具体内容
	switch do.EvaluatorType {
	case evaluatordo.EvaluatorTypePrompt:
		if do.PromptEvaluatorContent != nil {
			metainfoByte, err := json.Marshal(do.PromptEvaluatorContent)
			if err != nil {
				return nil, err
			}
			po.Metainfo = ptr.Of(metainfoByte)
		}
	case evaluatordo.EvaluatorTypeCode:
		if do.CodeEvaluatorContent != nil {
			metainfoByte, err := json.Marshal(do.CodeEvaluatorContent)
			if err != nil {
				return nil, err
			}
			po.Metainfo = ptr.Of(metainfoByte)
		}
	}

	if do.GetBaseInfo() != nil {
		if do.GetBaseInfo().CreatedBy != nil {
			po.CreatedBy = gptr.Indirect(do.GetBaseInfo().CreatedBy.UserID)
		}
		if do.GetBaseInfo().UpdatedBy != nil {
			po.UpdatedBy = gptr.Indirect(do.GetBaseInfo().UpdatedBy.UserID)
		}
		if do.GetBaseInfo().CreatedAt != nil {
			po.CreatedAt = time.UnixMilli(gptr.Indirect(do.GetBaseInfo().CreatedAt))
		}
		if do.GetBaseInfo().UpdatedAt != nil {
			po.UpdatedAt = time.UnixMilli(gptr.Indirect(do.GetBaseInfo().UpdatedAt))
		}
		if do.GetBaseInfo().DeletedAt != nil {
			po.DeletedAt = gorm.DeletedAt{
				Time:  time.UnixMilli(gptr.Indirect(do.GetBaseInfo().DeletedAt)),
				Valid: true,
			}
		}
	}

	return po, nil
}

// ConvertEvaluatorTemplatePO2DO 将 EvaluatorTemplate 的 PO 对象转换为 DO 对象
func ConvertEvaluatorTemplatePO2DO(po *model.EvaluatorTemplate) (*evaluatordo.EvaluatorTemplate, error) {
	if po == nil {
		return nil, nil
	}

	do := &evaluatordo.EvaluatorTemplate{
		ID:                 po.ID,
		SpaceID:            gptr.Indirect(po.SpaceID),
		Name:               gptr.Indirect(po.Name),
		Description:        gptr.Indirect(po.Description),
		EvaluatorType:      evaluatordo.EvaluatorType(gptr.Indirect(po.EvaluatorType)),
		ReceiveChatHistory: po.ReceiveChatHistory,
		Popularity:         po.Popularity,
		Tags:               make(map[evaluatordo.EvaluatorTagLangType]map[evaluatordo.EvaluatorTagKey][]string),
	}
	if po.EvaluatorInfo != nil {
		var info evaluatordo.EvaluatorInfo
		if err := json.Unmarshal(*po.EvaluatorInfo, &info); err == nil {
			do.EvaluatorInfo = &info
		}
	}

	// 反序列化InputSchema
	if po.InputSchema != nil {
		var inputSchemas []*evaluatordo.ArgsSchema
		if err := json.Unmarshal(*po.InputSchema, &inputSchemas); err == nil {
			do.InputSchemas = inputSchemas
		}
	}

	// 反序列化OutputSchema
	if po.OutputSchema != nil {
		var outputSchemas []*evaluatordo.ArgsSchema
		if err := json.Unmarshal(*po.OutputSchema, &outputSchemas); err == nil {
			do.OutputSchemas = outputSchemas
		}
	}

	// 根据EvaluatorType反序列化具体内容
	if po.Metainfo != nil {
		switch do.EvaluatorType {
		case evaluatordo.EvaluatorTypePrompt:
			do.PromptEvaluatorContent = &evaluatordo.PromptEvaluatorContent{}
			if err := json.Unmarshal(*po.Metainfo, do.PromptEvaluatorContent); err != nil {
				return nil, err
			}
		case evaluatordo.EvaluatorTypeCode:
			do.CodeEvaluatorContent = &evaluatordo.CodeEvaluatorContent{}
			if err := json.Unmarshal(*po.Metainfo, do.CodeEvaluatorContent); err != nil {
				return nil, err
			}
		}
	}

	return do, nil
}

// ConvertEvaluatorTemplatePO2DOWithBaseInfo 将 EvaluatorTemplate 的 PO 对象转换为 DO 对象（包含基础信息）
func ConvertEvaluatorTemplatePO2DOWithBaseInfo(po *model.EvaluatorTemplate) (*evaluatordo.EvaluatorTemplate, error) {
	do, err := ConvertEvaluatorTemplatePO2DO(po)
	if err != nil {
		return nil, err
	}
	if do == nil {
		return nil, nil
	}

	baseInfo := &evaluatordo.BaseInfo{
		CreatedBy: &evaluatordo.UserInfo{
			UserID: ptr.Of(po.CreatedBy),
		},
		UpdatedBy: &evaluatordo.UserInfo{
			UserID: ptr.Of(po.UpdatedBy),
		},
		CreatedAt: ptr.Of(po.CreatedAt.UnixMilli()),
		UpdatedAt: ptr.Of(po.UpdatedAt.UnixMilli()),
	}
	if po.DeletedAt.Valid {
		baseInfo.DeletedAt = ptr.Of(po.DeletedAt.Time.UnixMilli())
	}
	do.SetBaseInfo(baseInfo)

	return do, nil
}
