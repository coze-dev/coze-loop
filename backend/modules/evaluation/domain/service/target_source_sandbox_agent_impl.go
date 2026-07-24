// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"strconv"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	metricscomp "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func NewSandboxAgentSourceEvalTargetServiceImpl(idgen idgen.IIDGenerator, sandboxSchedulerAdapter rpc.ISandboxSchedulerAdapter, sandboxAgentMetrics metricscomp.SandboxAgentMetrics) ISourceEvalTargetOperateService {
	return &SandboxAgentSourceEvalTargetServiceImpl{
		idgen:                   idgen,
		sandboxSchedulerAdapter: sandboxSchedulerAdapter,
		sandboxAgentMetrics:     sandboxAgentMetrics,
	}
}

type SandboxAgentSourceEvalTargetServiceImpl struct {
	idgen                   idgen.IIDGenerator
	sandboxSchedulerAdapter rpc.ISandboxSchedulerAdapter
	sandboxAgentMetrics     metricscomp.SandboxAgentMetrics
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
	t.emitInvokeStarted(invokeID, param)
	return invokeID, "sandbox_agent", nil, nil
}

// emitInvokeStarted 提交侧打点：evaluation_target_sandbox_agent.invoke_started
// 触发时机：invokeID 生成成功后立即上报，代表"评测开始执行"。
func (t *SandboxAgentSourceEvalTargetServiceImpl) emitInvokeStarted(invokeID int64, param *entity.ExecuteEvalTargetParam) {
	if t.sandboxAgentMetrics == nil || param == nil {
		return
	}
	tags := metricscomp.SandboxAgentInvokeTags{
		ExperimentID: param.ExptID,
		InvokeID:     strconv.FormatInt(invokeID, 10),
		TargetID:     param.TargetID,
	}
	if param.ItemMeta != nil {
		tags.DatasetID = parseInt64OrZero(param.ItemMeta.EvalSetID)
		tags.DatasetVersion = parseInt64OrZero(param.ItemMeta.EvalSetVersionID)
		tags.ItemKey = param.ItemMeta.ItemKey
		tags.DatasetKey = param.ItemMeta.DatasetKey
	}
	if param.EvalSetItemID != nil {
		tags.ItemID = *param.EvalSetItemID
	}
	t.sandboxAgentMetrics.EmitInvokeStarted(tags)
}

// parseInt64OrZero 尝试把 string id 解析为 int64；异常场景返回 0（tag 层会填占位符）。
func parseInt64OrZero(s string) int64 {
	if s == "" {
		return 0
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return v
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
