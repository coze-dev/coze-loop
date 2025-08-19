// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"time"

	"github.com/bytedance/gg/gptr"
	"gorm.io/gorm"

	evaluatordo "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/js_conv"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func ConvertEvaluatorDO2PO(do *evaluatordo.Evaluator) *model.Evaluator {
	if do == nil {
		return nil
	}
	po := &model.Evaluator{
		ID:             do.ID,
		SpaceID:        do.SpaceID,
		Name:           ptr.Of(do.Name),
		Description:    ptr.Of(do.Description),
		DraftSubmitted: ptr.Of(do.DraftSubmitted),
		EvaluatorType:  int32(do.EvaluatorType),
		LatestVersion:  do.LatestVersion,
	}
	if do.BaseInfo != nil {
		if do.BaseInfo.CreatedBy != nil {
			po.CreatedBy = gptr.Indirect(do.BaseInfo.CreatedBy.UserID) // ignore_security_alert SQL_INJECTION
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
	return po
}

// ConvertEvaluatorPO2DO 将 Evaluator 的 PO 对象转换为 DO 对象
func ConvertEvaluatorPO2DO(po *model.Evaluator) *evaluatordo.Evaluator {
	if po == nil {
		return nil
	}
	do := &evaluatordo.Evaluator{
		ID:             po.ID,
		SpaceID:        po.SpaceID,
		Name:           gptr.Indirect(po.Name),
		Description:    gptr.Indirect(po.Description),
		DraftSubmitted: gptr.Indirect(po.DraftSubmitted),
		EvaluatorType:  evaluatordo.EvaluatorType(po.EvaluatorType),
		LatestVersion:  po.LatestVersion,
	}
	do.BaseInfo = &evaluatordo.BaseInfo{
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
		do.BaseInfo.DeletedAt = ptr.Of(po.DeletedAt.Time.UnixMilli())
	}

	return do
}

func ConvertEvaluatorVersionDO2PO(do *evaluatordo.Evaluator) (*model.EvaluatorVersion, error) {
	if do == nil || 
		(do.EvaluatorType == evaluatordo.EvaluatorTypePrompt && do.PromptEvaluatorVersion == nil) ||
		(do.EvaluatorType == evaluatordo.EvaluatorTypeCode && do.CodeEvaluatorVersion == nil) {
		return nil, nil
	}

	po := &model.EvaluatorVersion{
		ID:            do.GetEvaluatorVersionID(),
		SpaceID:       do.SpaceID,
		Version:       do.GetVersion(),
		EvaluatorType: ptr.Of(int32(do.EvaluatorType)),
		EvaluatorID:   do.ID,
		Description:   ptr.Of(do.GetEvaluatorVersionDescription()),
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
	switch do.EvaluatorType {
	case evaluatordo.EvaluatorTypePrompt:
		// 序列化Metainfo（整个DO）
		metaInfoByte, err := json.Marshal(do.PromptEvaluatorVersion)
		if err != nil {
			return nil, err
		}

		// 序列化InputSchema
		inputSchemaByte, err := json.Marshal(do.PromptEvaluatorVersion.InputSchemas)
		if err != nil {
			return nil, err
		}
		po.InputSchema = ptr.Of(inputSchemaByte)
		po.Metainfo = ptr.Of(metaInfoByte)
		po.ReceiveChatHistory = do.PromptEvaluatorVersion.ReceiveChatHistory
		po.ID = do.PromptEvaluatorVersion.ID
	case evaluatordo.EvaluatorTypeCode:
		// 序列化Metainfo（整个CodeEvaluatorVersion）
		metaInfoByte, err := json.Marshal(do.CodeEvaluatorVersion)
		if err != nil {
			return nil, err
		}
		
		// Code evaluator不需要InputSchema，设置为nil
		po.InputSchema = nil
		po.Metainfo = ptr.Of(metaInfoByte)
		// Code evaluator不需要chat history，设置为nil
		po.ReceiveChatHistory = nil
		po.ID = do.CodeEvaluatorVersion.ID
	}
	return po, nil
}

// ConvertEvaluatorVersionPO2DO 将 EvaluatorVersion 的 PO 对象转换为 DO 对象
func ConvertEvaluatorVersionPO2DO(po *model.EvaluatorVersion) (*evaluatordo.Evaluator, error) {
	if po == nil {
		return nil, nil
	}
	do := &evaluatordo.Evaluator{
		EvaluatorType: evaluatordo.EvaluatorType(gptr.Indirect(po.EvaluatorType)), // ignore_security_alert SQL_INJECTION
	}
	switch do.EvaluatorType {
	case evaluatordo.EvaluatorTypePrompt:
		do.PromptEvaluatorVersion = &evaluatordo.PromptEvaluatorVersion{}
		// 反序列化Metainfo获取完整配置
		if po.Metainfo != nil {
			var meta struct {
				PromptSourceType  evaluatordo.PromptSourceType `json:"prompt_source_type"`
				PromptTemplateKey string                       `json:"prompt_template_key"`
				MessageList       []*evaluatordo.Message       `json:"message_list"`
				ModelConfig       *evaluatordo.ModelConfig     `json:"model_config"`
				Tools             []*evaluatordo.Tool          `json:"tools"`
			}
			if err := js_conv.GetUnmarshaler()(*po.Metainfo, &meta); err == nil {
				do.PromptEvaluatorVersion.PromptSourceType = meta.PromptSourceType
				do.PromptEvaluatorVersion.PromptTemplateKey = meta.PromptTemplateKey
				do.PromptEvaluatorVersion.MessageList = meta.MessageList
				do.PromptEvaluatorVersion.ModelConfig = meta.ModelConfig
				do.PromptEvaluatorVersion.Tools = meta.Tools
			} else {
				return nil, errorx.Wrapf(err, "evaluator version metainfo json unmarshal fail, evluator_version_id: %v", po.ID)
			}
			if po.InputSchema != nil {
				var schema []*evaluatordo.ArgsSchema
				if err := json.Unmarshal(*po.InputSchema, &schema); err == nil {
					do.PromptEvaluatorVersion.InputSchemas = schema
				}
			}
		}
	case evaluatordo.EvaluatorTypeCode:
		do.CodeEvaluatorVersion = &evaluatordo.CodeEvaluatorVersion{}
		// 反序列化Metainfo获取完整的CodeEvaluatorVersion对象
		if po.Metainfo != nil {
			if err := json.Unmarshal(*po.Metainfo, do.CodeEvaluatorVersion); err != nil {
				return nil, err
			}
		}
	}
	do.SetEvaluatorVersionID(po.ID)
	do.SetVersion(po.Version)
	do.SetSpaceID(po.SpaceID)
	do.SetEvaluatorID(po.EvaluatorID)
	if po.Description != nil {
		do.SetEvaluatorVersionDescription(gptr.Indirect(po.Description))
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