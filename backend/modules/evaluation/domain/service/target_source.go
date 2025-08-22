// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

//go:generate mockgen -destination=mocks/target_source.go -package=mocks . ISourceEvalTargetOperateService
type ISourceEvalTargetOperateService interface {
	EvalType() entity.EvalTargetType
	// BuildBySource 根据source target构建eval target实体
	BuildBySource(ctx context.Context, spaceID int64, sourceTargetID, sourceTargetVersion string, opts ...entity.Option) (*entity.EvalTarget, error)
	// ListSource 查询source target列表
	ListSource(ctx context.Context, param *entity.ListSourceParam) (targets []*entity.EvalTarget, nextCursor string, hasMore bool, err error)
	// BatchGetSource 查询source target列表
	BatchGetSource(ctx context.Context, spaceID int64, ids []string) (targets []*entity.EvalTarget, err error)
	// ListSourceVersion 查询source target版本列表
	ListSourceVersion(ctx context.Context, param *entity.ListSourceVersionParam) (versions []*entity.EvalTargetVersion, nextCursor string, hasMore bool, err error)
	// PackSourceInfo 拼装源信息
	PackSourceInfo(ctx context.Context, spaceID int64, dos []*entity.EvalTarget) (err error)
	// PackSourceVersionInfo 拼装源版本信息
	PackSourceVersionInfo(ctx context.Context, spaceID int64, dos []*entity.EvalTarget) (err error)
	// ValidateInput
	ValidateInput(ctx context.Context, spaceID int64, inputSchema []*entity.ArgsSchema, input *entity.EvalTargetInputData) error
	// Execute
	Execute(ctx context.Context, spaceID int64, param *entity.ExecuteEvalTargetParam) (outputData *entity.EvalTargetOutputData, status entity.EvalTargetRunStatus, err error)
	RuntimeParam() entity.IRuntimeParam
}

//type Option func(option *Opt)
//
//type Opt struct {
//	PublishVersion *string
//	BotInfoType    entity.CozeBotInfoType
//}
//
//func WithCozeBotPublishVersion(publishVersion *string) Option {
//	return func(option *Opt) {
//		option.PublishVersion = publishVersion
//	}
//}
//
//func WithCozeBotInfoType(botInfoType entity.CozeBotInfoType) Option {
//	return func(option *Opt) {
//		option.BotInfoType = botInfoType
//	}
//}
//
//type ExecuteEvalTargetParam struct {
//	TargetID            int64
//	SourceTargetID      string
//	SourceTargetVersion string
//	Input               *entity.EvalTargetInputData
//	TargetType          entity.EvalTargetType
//}
