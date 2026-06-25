// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func NewSandboxAgentSourceEvalTargetServiceImpl(idgen idgen.IIDGenerator, sandboxSchedulerAdapter rpc.ISandboxSchedulerAdapter) ISourceEvalTargetOperateService {
	return &SandboxAgentSourceEvalTargetServiceImpl{
		idgen:                   idgen,
		sandboxSchedulerAdapter: sandboxSchedulerAdapter,
	}
}

type SandboxAgentSourceEvalTargetServiceImpl struct {
	idgen                   idgen.IIDGenerator
	sandboxSchedulerAdapter rpc.ISandboxSchedulerAdapter
}

func (t *SandboxAgentSourceEvalTargetServiceImpl) EvalType() entity.EvalTargetType {
	return entity.EvalTargetTypeSandboxAgent
}

func (t *SandboxAgentSourceEvalTargetServiceImpl) RuntimeParam() entity.IRuntimeParam {
	return entity.NewGenericJSONRuntimeParam()
}

func (t *SandboxAgentSourceEvalTargetServiceImpl) ValidateInput(ctx context.Context, spaceID int64, inputSchema []*entity.ArgsSchema, input *entity.EvalTargetInputData) error {
	if input == nil {
		return nil
	}
	return input.ValidateInputSchema(inputSchema)
}

func (t *SandboxAgentSourceEvalTargetServiceImpl) Execute(ctx context.Context, spaceID int64, param *entity.ExecuteEvalTargetParam) (*entity.EvalTargetOutputData, entity.EvalTargetRunStatus, error) {
	return nil, entity.EvalTargetRunStatusFail, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("SandboxAgent target only supports async execute, use ReportEvalTargetInvokeResult to report"))
}

// AsyncExecute 仅生成 invokeID 占位，实际执行由外部沙箱通过 ReportEvalTargetInvokeResult 回调上报。
func (t *SandboxAgentSourceEvalTargetServiceImpl) AsyncExecute(ctx context.Context, spaceID int64, param *entity.ExecuteEvalTargetParam) (int64, string, map[string]string, error) {
	invokeID, err := t.idgen.GenID(ctx)
	if err != nil {
		return 0, "", nil, err
	}
	return invokeID, "sandbox_agent", nil, nil
}

func (t *SandboxAgentSourceEvalTargetServiceImpl) BuildBySource(ctx context.Context, spaceID int64, sourceTargetID, sourceTargetVersion string, opts ...entity.Option) (*entity.EvalTarget, error) {
	o := &entity.Opt{}
	for _, opt := range opts {
		opt(o)
	}
	if o.SandboxAgent == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("SandboxAgent config is required"))
	}
	userIDInContext := session.UserIDInCtxOrEmpty(ctx)
	userInfo := &entity.UserInfo{UserID: gptr.Of(userIDInContext)}
	return &entity.EvalTarget{
		SpaceID:        spaceID,
		SourceTargetID: sourceTargetID,
		EvalTargetType: entity.EvalTargetTypeSandboxAgent,
		EvalTargetVersion: &entity.EvalTargetVersion{
			SpaceID:             spaceID,
			SourceTargetVersion: sourceTargetVersion,
			EvalTargetType:      entity.EvalTargetTypeSandboxAgent,
			SandboxAgent:        o.SandboxAgent,
			RuntimeParamDemo:    gptr.Of(entity.NewGenericJSONRuntimeParam().GetJSONDemo()),
			BaseInfo: &entity.BaseInfo{
				CreatedBy: userInfo,
				UpdatedBy: userInfo,
			},
		},
		BaseInfo: &entity.BaseInfo{
			CreatedBy: userInfo,
			UpdatedBy: userInfo,
		},
	}, nil
}

func (t *SandboxAgentSourceEvalTargetServiceImpl) ListSource(ctx context.Context, param *entity.ListSourceParam) ([]*entity.EvalTarget, string, bool, error) {
	return nil, "", false, nil
}

func (t *SandboxAgentSourceEvalTargetServiceImpl) BatchGetSource(ctx context.Context, spaceID int64, ids []string) ([]*entity.EvalTarget, error) {
	return nil, nil
}

func (t *SandboxAgentSourceEvalTargetServiceImpl) ListSourceVersion(ctx context.Context, param *entity.ListSourceVersionParam) ([]*entity.EvalTargetVersion, string, bool, error) {
	return nil, "", false, nil
}

func (t *SandboxAgentSourceEvalTargetServiceImpl) PackSourceInfo(ctx context.Context, spaceID int64, dos []*entity.EvalTarget) error {
	return nil
}

func (t *SandboxAgentSourceEvalTargetServiceImpl) PackSourceVersionInfo(ctx context.Context, spaceID int64, dos []*entity.EvalTarget) error {
	return nil
}

func (t *SandboxAgentSourceEvalTargetServiceImpl) SearchCustomEvalTarget(ctx context.Context, param *entity.SearchCustomEvalTargetParam) ([]*entity.CustomEvalTarget, string, bool, error) {
	return nil, "", false, nil
}
