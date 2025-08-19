// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"time"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

func EvalTargetDO2PO(do *entity.EvalTarget) (po *model.Target) {
	po = &model.Target{
		ID:             do.ID,
		SpaceID:        do.SpaceID,
		SourceTargetID: do.SourceTargetID,
		TargetType:     int32(do.EvalTargetType),
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

func EvalTargetVersionDO2PO(do *entity.EvalTargetVersion) (po *model.TargetVersion, err error) {
	// 序列化Metainfo（整个DO）
	var meta []byte
	var inputSchema []byte
	var outputSchema []byte
	switch do.EvalTargetType {
	case entity.EvalTargetTypeCozeBot:
		meta, err = json.Marshal(do.CozeBot)
		if err != nil {
			return nil, err
		}
	case entity.EvalTargetTypeLoopPrompt:
		meta, err = json.Marshal(do.Prompt)
		if err != nil {
			return nil, err
		}
	case entity.EvalTargetTypeCozeWorkflow:
		meta, err = json.Marshal(do.CozeWorkflow)
		if err != nil {
			return nil, err
		}
	}
	if do.InputSchema != nil {
		inputSchema, err = json.Marshal(do.InputSchema)
		if err != nil {
			return nil, err
		}
	}
	if do.OutputSchema != nil {
		outputSchema, err = json.Marshal(do.OutputSchema)
		if err != nil {
			return nil, err
		}
	}
	po = &model.TargetVersion{
		ID:                  do.ID,
		SpaceID:             do.SpaceID,
		TargetID:            do.TargetID,
		SourceTargetVersion: do.SourceTargetVersion,
		TargetMeta:          &meta,
		InputSchema:         &inputSchema,
		OutputSchema:        &outputSchema,
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
	return po, nil
}

func EvalTargetPO2DOs(targetPOs []*model.Target) (targetDOs []*entity.EvalTarget) {
	if targetPOs == nil {
		return nil
	}
	targetDOs = make([]*entity.EvalTarget, 0)
	for _, po := range targetPOs {
		targetDOs = append(targetDOs, EvalTargetPO2DO(po))
	}
	return targetDOs
}

func EvalTargetPO2DO(targetPO *model.Target) (targetDO *entity.EvalTarget) {
	if targetPO == nil {
		return
	}
	targetDO = &entity.EvalTarget{}
	targetDO.ID = targetPO.ID
	targetDO.SpaceID = targetPO.SpaceID
	targetDO.SourceTargetID = targetPO.SourceTargetID
	targetDO.EvalTargetType = entity.EvalTargetType(targetPO.TargetType)

	targetDO.BaseInfo = &entity.BaseInfo{
		CreatedBy: &entity.UserInfo{
			UserID: gptr.Of(targetPO.CreatedBy),
		},
		UpdatedBy: &entity.UserInfo{
			UserID: gptr.Of(targetPO.UpdatedBy),
		},
		CreatedAt: gptr.Of(targetPO.CreatedAt.UnixMilli()),
		UpdatedAt: gptr.Of(targetPO.UpdatedAt.UnixMilli()),
	}
	if targetPO.DeletedAt.Valid {
		targetDO.BaseInfo.DeletedAt = gptr.Of(targetPO.DeletedAt.Time.UnixMilli())
	}

	return
}

func EvalTargetVersionPO2DO(targetVersionPO *model.TargetVersion, targetType entity.EvalTargetType) (targetVersionDO *entity.EvalTargetVersion) {
	if targetVersionPO == nil {
		return
	}
	targetVersionDO = &entity.EvalTargetVersion{}
	targetVersionDO.ID = targetVersionPO.ID
	targetVersionDO.SpaceID = targetVersionPO.SpaceID
	targetVersionDO.TargetID = targetVersionPO.TargetID
	targetVersionDO.SourceTargetVersion = targetVersionPO.SourceTargetVersion

	targetVersionDO.BaseInfo = &entity.BaseInfo{
		CreatedBy: &entity.UserInfo{
			UserID: gptr.Of(targetVersionPO.CreatedBy),
		},
		UpdatedBy: &entity.UserInfo{
			UserID: gptr.Of(targetVersionPO.UpdatedBy),
		},
		CreatedAt: gptr.Of(targetVersionPO.CreatedAt.UnixMilli()),
		UpdatedAt: gptr.Of(targetVersionPO.UpdatedAt.UnixMilli()),
	}
	if targetVersionPO.DeletedAt.Valid {
		targetVersionDO.BaseInfo.DeletedAt = gptr.Of(targetVersionPO.DeletedAt.Time.UnixMilli())
	}

	if targetVersionPO.InputSchema != nil {
		schema := make([]*entity.ArgsSchema, 0)
		if err := json.Unmarshal(*targetVersionPO.InputSchema, &schema); err != nil {
			return
		}
		targetVersionDO.InputSchema = schema
	}
	if targetVersionPO.OutputSchema != nil {
		schema := make([]*entity.ArgsSchema, 0)
		if err := json.Unmarshal(*targetVersionPO.OutputSchema, &schema); err == nil {
			targetVersionDO.OutputSchema = schema
		}
	}
	if targetVersionPO.TargetMeta != nil {
		switch targetType {
		case entity.EvalTargetTypeCozeBot:
			meta := &entity.CozeBot{}
			if err := json.Unmarshal(*targetVersionPO.TargetMeta, meta); err == nil {
				targetVersionDO.CozeBot = meta
			}
		case entity.EvalTargetTypeLoopPrompt:
			meta := &entity.LoopPrompt{}
			if err := json.Unmarshal(*targetVersionPO.TargetMeta, meta); err == nil {
				targetVersionDO.Prompt = meta
			}
		case entity.EvalTargetTypeCozeWorkflow:
			meta := &entity.CozeWorkflow{}
			if err := json.Unmarshal(*targetVersionPO.TargetMeta, meta); err == nil {
				targetVersionDO.CozeWorkflow = meta
			}
		default:
			// todo
		}
	}
	return targetVersionDO
}
