// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_set"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/eval_target"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

type GetEvaluationSetReq struct {
	WorkspaceID     int64
	EvaluationSetID int64
}
type CreateEvaluationSetReq struct {
	EvaluationSet *entity.EvaluationSet
	Session       *common.Session
}
type SubmitExperimentReq struct {
	WorkspaceID           int64
	EvalSetVersionID      *int64
	TargetVersionID       *int64
	EvaluatorVersionIds   []int64
	Name                  *string
	Desc                  *string
	EvalSetID             *int64
	TargetID              *int64
	TargetFieldMapping    *expt.TargetFieldMapping
	EvaluatorFieldMapping []*expt.EvaluatorFieldMapping
	ItemConcurNum         *int32
	EvaluatorsConcurNum   *int32
	CreateEvalTargetParam *eval_target.CreateEvalTargetParam
	ExptType              *expt.ExptType
	MaxAliveTime          *int64
	SourceType            *expt.SourceType
	SourceID              *string
	Session               *common.Session
}
type InvokeExperimentReq struct {
	WorkspaceID     int64
	EvaluationSetID int64
	Items           []*eval_set.EvaluationSetItem
	// items 中存在无效数据时，默认不会写入任何数据；设置 skipInvalidItems=true 会跳过无效数据，写入有效数据      // items 中存在无效数据时，默认不会写入任何数据；设置 skipInvalidItems=true 会跳过无效数据，写入有效数据
	SkipInvalidItems *bool
	// 批量写入 items 如果超出数据集容量限制，默认不会写入任何数据；设置 partialAdd=true 会写入不超出容量限制的前 N 条
	AllowPartialAdd *bool
	ExperimentID    *int64
	ExperimentRunID *int64
	Ext             map[string]string
	Session         *common.Session
}
type FinishExperimentReq struct {
	WorkspaceID     int64
	ExperimentID    int64
	ExperimentRunID int64
	Session         *common.Session
}
type IEvaluationRPCAdapter interface {
	SubmitExperiment(ctx context.Context, param *SubmitExperimentReq) (exptID, exptRunID int64, err error)
	InvokeExperiment(ctx context.Context, param *InvokeExperimentReq) (addedItems int64, err error)
	FinishExperiment(ctx context.Context, param *FinishExperimentReq) (err error)
}
