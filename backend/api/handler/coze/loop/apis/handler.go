// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package apis

import (
	"context"
	"fmt"
	"net/http"
	"reflect"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/kitex/client/callopt"
	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/kerrors"

	"github.com/coze-dev/coze-loop/backend/infra/i18n"
	cachemw "github.com/coze-dev/coze-loop/backend/infra/middleware/ctxcache"
	logmw "github.com/coze-dev/coze-loop/backend/infra/middleware/logs"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/validator"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/dataset"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/tag"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/eval_set"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/eval_target"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/evaluator"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/auth"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/authn"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/file"
	foundationopenapi "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/openapi"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/space"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/user"
	llmmanage "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/llm/manage"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/llm/runtime"
	traceopenapi "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/openapi"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/trace"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/debug"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/execute"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/manage"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/openapi"
	"github.com/coze-dev/coze-loop/backend/loop_gen/coze/loop/data/lodataset"
	"github.com/coze-dev/coze-loop/backend/loop_gen/coze/loop/data/lotag"
	"github.com/coze-dev/coze-loop/backend/loop_gen/coze/loop/evaluation/loeval_set"
	"github.com/coze-dev/coze-loop/backend/loop_gen/coze/loop/evaluation/loeval_target"
	"github.com/coze-dev/coze-loop/backend/loop_gen/coze/loop/evaluation/loevaluator"
	"github.com/coze-dev/coze-loop/backend/loop_gen/coze/loop/evaluation/loexpt"
	"github.com/coze-dev/coze-loop/backend/loop_gen/coze/loop/foundation/loauthn"
	foundationlofile "github.com/coze-dev/coze-loop/backend/loop_gen/coze/loop/foundation/lofile"
	foundationloopenapi "github.com/coze-dev/coze-loop/backend/loop_gen/coze/loop/foundation/loopenapi"
	"github.com/coze-dev/coze-loop/backend/loop_gen/coze/loop/foundation/lospace"
	"github.com/coze-dev/coze-loop/backend/loop_gen/coze/loop/foundation/louser"
	lollmmanage "github.com/coze-dev/coze-loop/backend/loop_gen/coze/loop/llm/lomanage"
	looptraceopenapi "github.com/coze-dev/coze-loop/backend/loop_gen/coze/loop/observability/loopenapi"
	"github.com/coze-dev/coze-loop/backend/loop_gen/coze/loop/observability/lotrace"
	"github.com/coze-dev/coze-loop/backend/loop_gen/coze/loop/prompt/lodebug"
	"github.com/coze-dev/coze-loop/backend/loop_gen/coze/loop/prompt/lomanage"
	"github.com/coze-dev/coze-loop/backend/loop_gen/coze/loop/prompt/loopenapi"
	dataapp "github.com/coze-dev/coze-loop/backend/modules/data/application"
	evalapp "github.com/coze-dev/coze-loop/backend/modules/evaluation/application"
	"github.com/coze-dev/coze-loop/backend/modules/foundation/pkg/errno"
	obapp "github.com/coze-dev/coze-loop/backend/modules/observability/application"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/goroutine"
)

type APIHandler struct {
	*PromptHandler
	*LLMHandler
	*EvaluationHandler
	*DataHandler
	*ObservabilityHandler
	*FoundationHandler
	Translater i18n.ITranslater
}

func (a *APIHandler) GetTranslater() i18n.ITranslater {
	return a.Translater
}

type EvaluationHandler struct {
	evalapp.IExperimentApplication
	evaluation.EvaluatorService
	evaluation.EvaluationSetService
	evaluation.EvalTargetService
}

type FoundationHandler struct {
	auth.AuthService
	authn.AuthNService
	space.SpaceService
	user.UserService
	file.FileService
	foundationopenapi.FoundationOpenAPIService
}

func NewFoundationHandler(
	authApp auth.AuthService,
	authnApp authn.AuthNService,
	spaceApp space.SpaceService,
	userApp user.UserService,
	fileApp file.FileService,
	foundationOpenApiApp foundationopenapi.FoundationOpenAPIService,
) *FoundationHandler {
	h := &FoundationHandler{
		AuthService:              authApp,
		AuthNService:             authnApp,
		SpaceService:             spaceApp,
		UserService:              userApp,
		FileService:              fileApp,
		FoundationOpenAPIService: foundationOpenApiApp,
	}
	bindLocalCallClient(foundationopenapi.FoundationOpenAPIService(h), &foundationOpenAPIClient, foundationloopenapi.NewLocalFoundationOpenAPIService)
	bindLocalCallClient(file.FileService(h), &foundationFileClient, foundationlofile.NewLocalFileService)
	bindLocalCallClient(space.SpaceService(h), &localSpaceClient, lospace.NewLocalSpaceService)
	bindLocalCallClient(user.UserService(h), &localUserClient, louser.NewLocalUserService)
	bindLocalCallClient(authn.AuthNService(h), &localAuthNClient, loauthn.NewLocalAuthNService)
	return h
}

func NewEvaluationHandler(
	exptApp evalapp.IExperimentApplication,
	evaluatorApp evaluation.EvaluatorService,
	evaluationSetApp evaluation.EvaluationSetService,
	evalTargetService evaluation.EvalTargetService,
) *EvaluationHandler {
	h := &EvaluationHandler{
		EvaluatorService:       evaluatorApp,
		IExperimentApplication: exptApp,
		EvaluationSetService:   evaluationSetApp,
		EvalTargetService:      evalTargetService,
	}
	bindLocalCallClient(expt.ExperimentService(h), &localExptSvc, loexpt.NewLocalExperimentService)
	bindLocalCallClient(evaluator.EvaluatorService(h), &localEvaluatorSvc, loevaluator.NewLocalEvaluatorService)
	bindLocalCallClient(eval_set.EvaluationSetService(h), &localEvalSetSvc, loeval_set.NewLocalEvaluationSetService)
	bindLocalCallClient(eval_target.EvalTargetService(h), &localEvalTargetSvc, loeval_target.NewLocalEvalTargetService)
	return h
}

type DataHandler struct {
	dataapp.IDatasetApplication
	tag.TagService
}

func NewDataHandler(dataApp dataapp.IDatasetApplication, tagApp tag.TagService) *DataHandler {
	h := &DataHandler{IDatasetApplication: dataApp, TagService: tagApp}
	bindLocalCallClient(dataset.DatasetService(h), &localDataSvc, lodataset.NewLocalDatasetService)
	bindLocalCallClient(tag.TagService(h), &localTagClient, lotag.NewLocalTagService)
	return h
}

type PromptHandler struct {
	manage.PromptManageService
	debug.PromptDebugService
	execute.PromptExecuteService
	openapi.PromptOpenAPIService
}

func NewPromptHandler(
	manageApp manage.PromptManageService,
	debugApp debug.PromptDebugService,
	executeApp execute.PromptExecuteService,
	openAPIApp openapi.PromptOpenAPIService,
) *PromptHandler {
	h := &PromptHandler{
		PromptManageService:  manageApp,
		PromptDebugService:   debugApp,
		PromptExecuteService: executeApp,
		PromptOpenAPIService: openAPIApp,
	}
	bindLocalCallClient(manage.PromptManageService(h), &promptManageSvc, lomanage.NewLocalPromptManageService)
	bindLocalCallClient(debug.PromptDebugService(h), &promptDebugSvc, lodebug.NewLocalPromptDebugService)
	bindLocalCallClient(openapi.PromptOpenAPIService(h), &promptOpenAPISvc, loopenapi.NewLocalPromptOpenAPIService)
	return h
}

type LLMHandler struct {
	llmmanage.LLMManageService
	runtime.LLMRuntimeService
}

func NewLLMHandler(
	manageApp llmmanage.LLMManageService,
	runtimeApp runtime.LLMRuntimeService,
) *LLMHandler {
	h := &LLMHandler{
		LLMManageService:  manageApp,
		LLMRuntimeService: runtimeApp,
	}
	bindLocalCallClient(llmmanage.LLMManageService(h), &llmManageSvc, lollmmanage.NewLocalLLMManageService)
	return h
}

type ObservabilityHandler struct {
	obapp.ITraceApplication
	obapp.ITraceIngestionApplication
	obapp.IObservabilityOpenAPIApplication
}

func NewObservabilityHandler(
	traceApp obapp.ITraceApplication,
	ingestApp obapp.ITraceIngestionApplication,
	openAPIApp obapp.IObservabilityOpenAPIApplication,
) *ObservabilityHandler {
	h := &ObservabilityHandler{
		ITraceApplication:                traceApp,
		ITraceIngestionApplication:       ingestApp,
		IObservabilityOpenAPIApplication: openAPIApp,
	}
	bindLocalCallClient(trace.TraceService(h), &observabilityClient, lotrace.NewLocalTraceService)
	bindLocalCallClient(traceopenapi.OpenAPIService(h), &observabilityOpenAPIClient, looptraceopenapi.NewLocalOpenAPIService)
	return h
}

func bindLocalCallClient[T, K any](svc T, cli any, provider func(t T, mds ...endpoint.Middleware) K) {
	v := reflect.ValueOf(cli)
	if v.Kind() != reflect.Ptr {
		panic("cli must be a pointer")
	}
	c := provider(svc, defaultKiteXMiddlewares()...)
	v.Elem().Set(reflect.ValueOf(c))
}

func defaultKiteXMiddlewares() []endpoint.Middleware {
	return []endpoint.Middleware{
		logmw.LogTrafficMW,
		validator.KiteXValidatorMW,
		cachemw.CtxCacheMW,
	}
}

func invokeAndRender[T, K any](
	ctx context.Context, c *app.RequestContext,
	callable func(ctx context.Context, req T, callOptions ...callopt.Option) (K, error),
) {
	render := func(c *app.RequestContext, fn func() (any, error)) {
		resp, err := fn()
		if err == nil {
			c.JSON(http.StatusOK, resp)
			return
		}

		_ = c.Error(err)
	}

	render(c, func() (r any, err error) {
		defer goroutine.Recover(ctx, &err)

		var req T
		typ := reflect.TypeOf(req)
		if typ.Kind() != reflect.Ptr || typ.Elem().Kind() != reflect.Struct {
			return nil, kerrors.NewBizStatusError(errno.CommonInternalErrorCode, "callable must be KiteX service method, found invalid request")
		}
		ins := reflect.New(typ.Elem()).Interface().(T)
		if err := c.BindAndValidate(ins); err != nil {
			return nil, kerrors.NewBizStatusError(errno.CommonBadRequestCode, fmt.Sprintf("invalid request, err: %s", err.Error()))
		}
		return callable(ctx, ins)
	})
}
