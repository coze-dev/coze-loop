package application

import (
	"context"
	stdjson "encoding/json"
	"errors"
	"fmt"
	"github.com/bytedance/gg/gptr"
	"strconv"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/base"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/openapi"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"gorm.io/gorm"
)

func (e *EvalOpenAPIApplication) GetFluxWorkflowRun(ctx context.Context, req *openapi.GetFluxWorkflowRunRequest) (*openapi.GetFluxWorkflowRunResponse, error) {
	if req == nil || req.GetWorkspaceID() <= 0 || req.GetInvokeID() <= 0 {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode)
	}
	run, err := e.fluxItemRunRepo.GetFluxItemRun(ctx, req.GetWorkspaceID(), req.GetInvokeID())
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("target record not yet created"))
		}
		return nil, err
	}
	if _, _, err = e.authorizeFluxWorkflowRun(ctx, req.GetWorkspaceID(), req.GetInvokeID(), run); err != nil {
		return nil, err
	}
	resp := &openapi.GetFluxWorkflowRunResponse{BaseResp: base.NewBaseResp()}
	if run != nil {
		encoded, err := stdjson.Marshal(run)
		if err != nil {
			return nil, err
		}
		resp.RunJSON = gptr.Of(string(encoded))
		resp.Version = gptr.Of(run.Version)
	}
	return resp, nil
}

func (e *EvalOpenAPIApplication) CompareAndSwapFluxWorkflowRun(ctx context.Context, req *openapi.CompareAndSwapFluxWorkflowRunRequest) (*openapi.CompareAndSwapFluxWorkflowRunResponse, error) {
	if req == nil || req.GetWorkspaceID() <= 0 || req.GetInvokeID() <= 0 || req.GetExpectedVersion() < 0 || req.GetRunJSON() == "" {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode)
	}
	var next entity.FluxItemRun
	if err := stdjson.Unmarshal([]byte(req.GetRunJSON()), &next); err != nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("invalid run_json"))
	}
	current, err := e.fluxItemRunRepo.GetFluxItemRun(ctx, req.GetWorkspaceID(), req.GetInvokeID())
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("target record not yet created"))
		}
		return nil, err
	}
	exptID, bindingID, err := e.authorizeFluxWorkflowRun(ctx, req.GetWorkspaceID(), req.GetInvokeID(), current)
	if err != nil {
		return nil, err
	}
	if next.SpaceID != req.GetWorkspaceID() || next.InvokeID != req.GetInvokeID() || next.ExperimentID != exptID || next.BindingID != bindingID {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("run identity mismatch"))
	}
	updated, latest, err := e.fluxItemRunRepo.CompareAndSwapFluxItemRun(ctx, req.GetWorkspaceID(), req.GetInvokeID(), req.GetExpectedVersion(), &next)
	if err != nil {
		return nil, err
	}
	resp := &openapi.CompareAndSwapFluxWorkflowRunResponse{Updated: gptr.Of(updated), BaseResp: base.NewBaseResp()}
	if latest != nil {
		encoded, err := stdjson.Marshal(latest)
		if err != nil {
			return nil, err
		}
		resp.CurrentJSON, resp.CurrentVersion = gptr.Of(string(encoded)), gptr.Of(latest.Version)
	}
	return resp, nil
}

// The private copy is served only by the commercial storage wrapper.
func (e *EvalOpenAPIApplication) GetFluxWorkflowCopyYAML(_ context.Context, _ *openapi.GetFluxWorkflowCopyYAMLRequest) (*openapi.GetFluxWorkflowCopyYAMLResponse, error) {
	return nil, errorx.NewByCode(errno.ResourceNotFoundCode)
}

func (e *EvalOpenAPIApplication) authorizeFluxWorkflowRun(ctx context.Context, spaceID, invokeID int64, current *entity.FluxItemRun) (int64, string, error) {
	var exptID int64
	actx, err := e.asyncRepo.GetEvalAsyncCtx(ctx, strconv.FormatInt(invokeID, 10))
	if err == nil && actx != nil && actx.Event != nil && actx.Event.SpaceID == spaceID && actx.Callee == sandboxAgentAsyncCallee {
		exptID = actx.Event.ExptID
	}
	if exptID == 0 && current != nil && current.SpaceID == spaceID && current.InvokeID == invokeID {
		exptID = current.ExperimentID
	}
	if exptID == 0 {
		return 0, "", errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg(fmt.Sprintf("Flux Workflow invoke %d not ready", invokeID)))
	}
	if current != nil && current.ExperimentID != exptID {
		return 0, "", errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("Flux Workflow experiment mismatch"))
	}
	binding, err := e.fluxBindingRepo.GetFluxWorkflowBinding(ctx, spaceID, exptID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, "", errorx.NewByCode(errno.ResourceNotFoundCode)
	}
	if err != nil {
		return 0, "", err
	}
	if binding == nil || binding.State != "ready" {
		return 0, "", errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("Flux Workflow binding not ready"))
	}
	if current != nil && current.BindingID != binding.BindingID {
		return 0, "", errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("Flux Workflow binding mismatch"))
	}
	return exptID, binding.BindingID, nil
}
