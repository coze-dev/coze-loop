// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"golang.org/x/exp/maps"

	"github.com/coze-dev/coze-loop/backend/infra/limiter"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/openapi"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/application/convertor"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/conf"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/service"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/collector"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/consts"
	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

func NewPromptOpenAPIApplication(
	promptService service.IPromptService,
	promptManageRepo repo.IManageRepo,
	config conf.IConfigProvider,
	auth rpc.IAuthProvider,
	factory limiter.IRateLimiterFactory,
	collector collector.ICollectorProvider,
) (openapi.PromptOpenAPIService, error) {
	return &PromptOpenAPIApplicationImpl{
		promptService:    promptService,
		promptManageRepo: promptManageRepo,
		config:           config,
		auth:             auth,
		rateLimiter:      factory.NewRateLimiter(),
		collector:        collector,
	}, nil
}

type PromptOpenAPIApplicationImpl struct {
	promptService    service.IPromptService
	promptManageRepo repo.IManageRepo
	config           conf.IConfigProvider
	auth             rpc.IAuthProvider
	rateLimiter      limiter.IRateLimiter
	collector        collector.ICollectorProvider
}

func (p *PromptOpenAPIApplicationImpl) BatchGetPromptByPromptKey(ctx context.Context, req *openapi.BatchGetPromptByPromptKeyRequest) (r *openapi.BatchGetPromptByPromptKeyResponse, err error) {
	r = openapi.NewBatchGetPromptByPromptKeyResponse()
	if req.GetWorkspaceID() == 0 {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtra(map[string]string{"invalid_param": "workspace_id参数为空"}))
	}
	defer func() {
		if err != nil {
			logs.CtxError(ctx, "openapi get prompts failed, err=%v", err)
		}
	}()

	// 限流检查
	if !p.AllowBySpace(ctx, req.GetWorkspaceID()) {
		return r, errorx.NewByCode(prompterr.PromptHubQPSLimitCode, errorx.WithExtraMsg("qps limit exceeded"))
	}

	// 查询prompt id并鉴权
	var promptKeys []string
	for _, q := range req.Queries {
		if q == nil {
			continue
		}
		promptKeys = append(promptKeys, q.GetPromptKey())
	}
	promptKeyIDMap, err := p.promptService.MGetPromptIDs(ctx, req.GetWorkspaceID(), promptKeys)
	if err != nil {
		return r, err
	}
	// 执行权限检查
	if err = p.auth.MCheckPromptPermission(ctx, req.GetWorkspaceID(), maps.Values(promptKeyIDMap), consts.ActionLoopPromptRead); err != nil {
		return nil, err
	}

	// 获取提示详细信息
	return p.fetchPromptResults(ctx, req, promptKeyIDMap)
}

// fetchPromptResults 构建返回结果
func (p *PromptOpenAPIApplicationImpl) fetchPromptResults(ctx context.Context, req *openapi.BatchGetPromptByPromptKeyRequest, promptKeyIDMap map[string]int64) (*openapi.BatchGetPromptByPromptKeyResponse, error) {
	// 准备查询参数
	var mgetParams []repo.GetPromptParam
	var pairs []service.PromptKeyVersionPair
	for _, q := range req.Queries {
		if q == nil {
			continue
		}
		pairs = append(pairs, service.PromptKeyVersionPair{
			PromptKey: q.GetPromptKey(),
			Version:   q.GetVersion(),
		})
	}
	// 解析具体的提交版本
	promptKeyCommitVersionMap, err := p.promptService.MParseCommitVersionByPromptKey(ctx, req.GetWorkspaceID(), pairs)
	if err != nil {
		return nil, err
	}
	for _, query := range req.Queries {
		if query == nil {
			continue
		}
		mgetParams = append(mgetParams, repo.GetPromptParam{
			PromptID:      promptKeyIDMap[query.GetPromptKey()],
			WithCommit:    true,
			CommitVersion: promptKeyCommitVersionMap[service.PromptKeyVersionPair{PromptKey: query.GetPromptKey(), Version: query.GetVersion()}],
		})
	}

	// 获取prompt详细信息
	prompts, err := p.promptManageRepo.MGetPrompt(ctx, mgetParams, repo.WithPromptCacheEnable())
	if err != nil {
		if bizErr, ok := errorx.FromStatusError(err); ok && bizErr.Code() == prompterr.PromptVersionNotExistCode {
			extra := bizErr.Extra()
			for promptKey, promptID := range promptKeyIDMap {
				if extra["prompt_id"] == strconv.FormatInt(promptID, 10) {
					extra["prompt_key"] = promptKey
					break
				}
			}
			bizErr.WithExtra(extra)
		}
		return nil, err
	}

	// 构建版本映射
	promptMap := make(map[service.PromptKeyVersionPair]*entity.Prompt)
	for _, prompt := range maps.Values(prompts) {
		promptMap[service.PromptKeyVersionPair{
			PromptKey: prompt.PromptKey,
			Version:   prompt.GetVersion(),
		}] = prompt
	}

	// 构建响应
	r := openapi.NewBatchGetPromptByPromptKeyResponse()
	r.Data = openapi.NewPromptResultData()

	for _, q := range req.Queries {
		if q == nil {
			continue
		}
		// 找到具体的版本
		commitVersion := promptKeyCommitVersionMap[service.PromptKeyVersionPair{PromptKey: q.GetPromptKey(), Version: q.GetVersion()}]
		promptDTO := convertor.OpenAPIPromptDO2DTO(promptMap[service.PromptKeyVersionPair{PromptKey: q.GetPromptKey(), Version: commitVersion}])
		if promptDTO == nil {
			return nil, errorx.NewByCode(prompterr.PromptVersionNotExistCode,
				errorx.WithExtraMsg("prompt version not exist"),
				errorx.WithExtra(map[string]string{"prompt_key": q.GetPromptKey(), "version": q.GetVersion()}))
		}

		r.Data.Items = append(r.Data.Items, &openapi.PromptResult_{
			Query:  q,
			Prompt: promptDTO,
		})
	}

	if len(promptMap) > 0 {
		p.collector.CollectPromptHubEvent(ctx, req.GetWorkspaceID(), maps.Values(promptMap))
	}

	return r, nil
}

func (p *PromptOpenAPIApplicationImpl) AllowBySpace(ctx context.Context, workspaceID int64) bool {
	maxQPS, err := p.config.GetPromptHubMaxQPSBySpace(ctx, workspaceID)
	if err != nil {
		logs.CtxError(ctx, "get prompt hub max qps failed, err=%v, space_id=%d", err, workspaceID)
		return true
	}
	result, err := p.rateLimiter.AllowN(ctx, fmt.Sprintf("prompt_hub:qps:space_id:%d", workspaceID), 1,
		limiter.WithLimit(&limiter.Limit{
			Rate:   maxQPS,
			Burst:  maxQPS,
			Period: time.Second,
		}))
	if err != nil {
		logs.CtxError(ctx, "allow rate limit failed, err=%v", err)
		return true
	}
	if result == nil || result.Allowed {
		return true
	}
	return false
}

// ValidateTemplate 验证Jinja2模板语法
func (p *PromptOpenAPIApplicationImpl) ValidateTemplate(ctx context.Context, req *openapi.ValidateTemplateRequest) (*openapi.ValidateTemplateResponse, error) {
	r := openapi.NewValidateTemplateResponse()

	// 参数验证
	if req.GetTemplate() == "" {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("template参数不能为空"))
	}

	if req.GetTemplateType() != "jinja2" {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("目前只支持jinja2模板类型"))
	}

	defer func() {
		if err := recover(); err != nil {
			logs.CtxError(ctx, "template validation panic, err=%v", err)
			r.Code = 500
			r.Msg = "Internal server error"
			r.Data = &openapi.ValidateTemplateData{
				IsValid:     false,
				ErrorMessage: "Template validation failed due to internal error",
			}
		}
	}()

	// 创建Jinja2引擎进行语法验证
	engine := entity.NewJinja2Engine()

	// 使用空变量进行语法验证
	_, err := engine.Execute(req.GetTemplate(), map[string]interface{}{})

	r.Code = 200
	r.Msg = "Success"
	r.Data = &openapi.ValidateTemplateData{
		IsValid: err == nil,
	}

	if err != nil {
		r.Data.ErrorMessage = err.Error()
		logs.CtxInfo(ctx, "template validation failed, template=%s, error=%v", req.GetTemplate(), err)
	}

	return r, nil
}

// PreviewTemplate 预览Jinja2模板渲染结果
func (p *PromptOpenAPIApplicationImpl) PreviewTemplate(ctx context.Context, req *openapi.PreviewTemplateRequest) (*openapi.PreviewTemplateResponse, error) {
	r := openapi.NewPreviewTemplateResponse()

	// 参数验证
	if req.GetTemplate() == "" {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("template参数不能为空"))
	}

	if req.GetTemplateType() != "jinja2" {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("目前只支持jinja2模板类型"))
	}

	defer func() {
		if err := recover(); err != nil {
			logs.CtxError(ctx, "template preview panic, err=%v", err)
			r.Code = 500
			r.Msg = "Internal server error"
			r.Data = &openapi.PreviewTemplateData{
				Result: "Template preview failed due to internal error",
			}
		}
	}()

	// 创建Jinja2引擎进行模板渲染
	engine := entity.NewJinja2Engine()

	// 转换变量类型
	variables := make(map[string]interface{})
	for key, value := range req.GetVariables() {
		variables[key] = value
	}

	// 执行模板渲染
	result, err := engine.Execute(req.GetTemplate(), variables)

	r.Code = 200
	r.Msg = "Success"

	if err != nil {
		r.Code = 400
		r.Msg = "Template execution failed"
		r.Data = &openapi.PreviewTemplateData{
			Result: fmt.Sprintf("Error: %s", err.Error()),
		}
		logs.CtxError(ctx, "template preview failed, template=%s, variables=%v, error=%v", req.GetTemplate(), variables, err)
	} else {
		r.Data = &openapi.PreviewTemplateData{
			Result: result,
		}
		logs.CtxInfo(ctx, "template preview success, template=%s, variables=%v, result_length=%d", req.GetTemplate(), variables, len(result))
	}

	return r, nil
}
