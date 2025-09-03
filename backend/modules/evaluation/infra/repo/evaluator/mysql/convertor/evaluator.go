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
	if do == nil || do.GetEvaluatorVersion() == nil {
		return nil, nil
	}

	po := &model.EvaluatorVersion{
		ID:            do.GetEvaluatorVersion().GetID(),
		SpaceID:       do.SpaceID,
		Version:       do.GetEvaluatorVersion().GetVersion(),
		EvaluatorType: ptr.Of(int32(do.EvaluatorType)),
		EvaluatorID:   do.ID,
		Description:   ptr.Of(do.GetEvaluatorVersion().GetDescription()),
	}
	if do.GetEvaluatorVersion().GetBaseInfo() != nil {
		if do.GetEvaluatorVersion().GetBaseInfo().CreatedBy != nil {
			po.CreatedBy = gptr.Indirect(do.GetEvaluatorVersion().GetBaseInfo().CreatedBy.UserID)
		}
		if do.GetEvaluatorVersion().GetBaseInfo().UpdatedBy != nil {
			po.UpdatedBy = gptr.Indirect(do.GetEvaluatorVersion().GetBaseInfo().UpdatedBy.UserID)
		}
		if do.GetEvaluatorVersion().GetBaseInfo().CreatedAt != nil {
			po.CreatedAt = time.UnixMilli(gptr.Indirect(do.GetEvaluatorVersion().GetBaseInfo().CreatedAt))
		}
		if do.GetEvaluatorVersion().GetBaseInfo().UpdatedAt != nil {
			po.UpdatedAt = time.UnixMilli(gptr.Indirect(do.GetEvaluatorVersion().GetBaseInfo().UpdatedAt))
		}
		if do.GetEvaluatorVersion().GetBaseInfo().DeletedAt != nil {
			po.DeletedAt = gorm.DeletedAt{
				Time:  time.UnixMilli(gptr.Indirect(do.GetEvaluatorVersion().GetBaseInfo().DeletedAt)),
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
	}
	do.GetEvaluatorVersion().SetID(po.ID)
	do.GetEvaluatorVersion().SetVersion(po.Version)
	do.GetEvaluatorVersion().SetSpaceID(po.SpaceID)
	do.GetEvaluatorVersion().SetEvaluatorID(po.EvaluatorID)
	if po.Description != nil {
		do.GetEvaluatorVersion().SetDescription(gptr.Indirect(po.Description))
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
	do.GetEvaluatorVersion().SetBaseInfo(baseInfo)

	return do, nil
}
