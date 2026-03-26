// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"fmt"

	"golang.org/x/exp/maps"

	tool_manage "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/tool_manage"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/application/convertor"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/service"

	rpc "github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/consts"
	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"

	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
)

type ToolManageApplicationImpl struct {
	toolRepo        repo.IToolRepo
	toolService     service.IToolService
	authRPCProvider rpc.IAuthProvider
	userRPCProvider rpc.IUserProvider
}

func NewToolManageApplication(
	toolRepo repo.IToolRepo,
	toolService service.IToolService,
	authRPCProvider rpc.IAuthProvider,
	userRPCProvider rpc.IUserProvider,
) tool_manage.ToolManageService {
	return &ToolManageApplicationImpl{
		toolRepo:        toolRepo,
		toolService:     toolService,
		authRPCProvider: authRPCProvider,
		userRPCProvider: userRPCProvider,
	}
}

func (app *ToolManageApplicationImpl) CreateTool(ctx context.Context, request *tool_manage.CreateToolRequest) (r *tool_manage.CreateToolResponse, err error) {
	r = tool_manage.NewCreateToolResponse()

	userID, ok := session.UserIDInCtx(ctx)
	if !ok {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found"))
	}

	err = app.authRPCProvider.CheckSpacePermission(ctx, request.GetWorkspaceID(), consts.ActionWorkspaceCreateLoopPrompt)
	if err != nil {
		return r, err
	}

	toolDO := &entity.CommonTool{
		SpaceID: request.GetWorkspaceID(),
		ToolBasic: &entity.CommonToolBasic{
			Name:        request.GetToolName(),
			Description: request.GetToolDescription(),
			CreatedBy:   userID,
			UpdatedBy:   userID,
		},
	}

	// 如果有初始草稿
	if request.DraftDetail != nil {
		toolDO.ToolCommit = &entity.CommonToolCommit{
			ToolDetail: convertor.ToolDetailDTO2DO(request.DraftDetail),
			CommitInfo: &entity.CommonToolCommitInfo{
				Version:     entity.ToolPublicDraftVersion,
				CommittedBy: userID,
			},
		}
	}

	toolID, err := app.toolService.CreateTool(ctx, toolDO)
	if err != nil {
		return r, err
	}

	r.ToolID = &toolID
	return r, nil
}

func (app *ToolManageApplicationImpl) GetToolDetail(ctx context.Context, request *tool_manage.GetToolDetailRequest) (r *tool_manage.GetToolDetailResponse, err error) {
	r = tool_manage.NewGetToolDetailResponse()

	_, ok := session.UserIDInCtx(ctx)
	if !ok {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found"))
	}

	err = app.authRPCProvider.CheckSpacePermission(ctx, request.GetWorkspaceID(), consts.ActionLoopPromptRead)
	if err != nil {
		return r, err
	}

	param := repo.GetToolParam{
		ToolID:        request.GetToolID(),
		SpaceID:       request.GetWorkspaceID(),
		WithCommit:    request.GetWithCommit(),
		CommitVersion: request.GetCommitVersion(),
		WithDraft:     request.GetWithDraft(),
	}

	toolDO, err := app.toolRepo.GetTool(ctx, param)
	if err != nil {
		return r, err
	}

	r.Tool = convertor.CommonToolDO2DTO(toolDO)
	return r, nil
}

func (app *ToolManageApplicationImpl) ListTool(ctx context.Context, request *tool_manage.ListToolRequest) (r *tool_manage.ListToolResponse, err error) {
	r = tool_manage.NewListToolResponse()

	_, ok := session.UserIDInCtx(ctx)
	if !ok {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found"))
	}

	err = app.authRPCProvider.CheckSpacePermission(ctx, request.GetWorkspaceID(), consts.ActionWorkspaceListLoopPrompt)
	if err != nil {
		return r, err
	}

	var orderBy repo.ListToolOrderBy
	switch request.GetOrderBy() {
	case tool_manage.ListToolOrderByCommittedAt:
		orderBy = repo.ListToolOrderByCommittedAt
	default:
		orderBy = repo.ListToolOrderByCreatedAt
	}

	param := repo.ListToolParam{
		SpaceID:       request.GetWorkspaceID(),
		KeyWord:       request.GetKeyWord(),
		CreatedBys:    request.GetCreatedBys(),
		CommittedOnly: request.GetCommittedOnly(),
		PageNum:       int(request.GetPageNum()),
		PageSize:      int(request.GetPageSize()),
		OrderBy:       orderBy,
		Asc:           request.GetAsc(),
	}

	result, err := app.toolRepo.ListTool(ctx, param)
	if err != nil {
		return r, err
	}

	r.Tools = convertor.BatchCommonToolDO2DTO(result.ToolDOs)
	total := int32(result.Total)
	r.Total = &total

	// 获取用户信息
	userIDSet := make(map[string]struct{})
	for _, toolDO := range result.ToolDOs {
		if toolDO.ToolBasic != nil {
			userIDSet[toolDO.ToolBasic.CreatedBy] = struct{}{}
			userIDSet[toolDO.ToolBasic.UpdatedBy] = struct{}{}
		}
	}
	if len(userIDSet) > 0 {
		userDOs, err := app.userRPCProvider.MGetUserInfo(ctx, maps.Keys(userIDSet))
		if err != nil {
			return r, err
		}
		r.Users = convertor.BatchUserInfoDO2DTO(userDOs)
	}

	return r, nil
}

func (app *ToolManageApplicationImpl) SaveToolDetail(ctx context.Context, request *tool_manage.SaveToolDetailRequest) (r *tool_manage.SaveToolDetailResponse, err error) {
	r = tool_manage.NewSaveToolDetailResponse()

	userID, ok := session.UserIDInCtx(ctx)
	if !ok {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found"))
	}

	err = app.authRPCProvider.CheckSpacePermission(ctx, request.GetWorkspaceID(), consts.ActionLoopPromptEdit)
	if err != nil {
		return r, err
	}

	toolDO := &entity.CommonTool{
		ID:      request.GetToolID(),
		SpaceID: request.GetWorkspaceID(),
		ToolCommit: &entity.CommonToolCommit{
			ToolDetail: convertor.ToolDetailDTO2DO(request.ToolDetail),
			CommitInfo: &entity.CommonToolCommitInfo{
				Version:     entity.ToolPublicDraftVersion,
				BaseVersion: request.GetBaseVersion(),
				CommittedBy: userID,
			},
		},
	}

	err = app.toolRepo.SaveDraft(ctx, toolDO)
	if err != nil {
		return r, err
	}

	return r, nil
}

func (app *ToolManageApplicationImpl) CommitToolDraft(ctx context.Context, request *tool_manage.CommitToolDraftRequest) (r *tool_manage.CommitToolDraftResponse, err error) {
	r = tool_manage.NewCommitToolDraftResponse()

	userID, ok := session.UserIDInCtx(ctx)
	if !ok {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found"))
	}

	err = app.authRPCProvider.CheckSpacePermission(ctx, request.GetWorkspaceID(), consts.ActionLoopPromptEdit)
	if err != nil {
		return r, err
	}

	param := repo.CommitToolDraftParam{
		ToolID:            request.GetToolID(),
		SpaceID:           request.GetWorkspaceID(),
		CommitVersion:     request.GetCommitVersion(),
		CommitDescription: request.GetCommitDescription(),
		BaseVersion:       request.GetBaseVersion(),
		CommittedBy:       userID,
	}

	err = app.toolRepo.CommitDraft(ctx, param)
	if err != nil {
		return r, err
	}

	return r, nil
}

func (app *ToolManageApplicationImpl) ListToolCommit(ctx context.Context, request *tool_manage.ListToolCommitRequest) (r *tool_manage.ListToolCommitResponse, err error) {
	r = tool_manage.NewListToolCommitResponse()

	_, ok := session.UserIDInCtx(ctx)
	if !ok {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found"))
	}

	err = app.authRPCProvider.CheckSpacePermission(ctx, request.GetWorkspaceID(), consts.ActionLoopPromptRead)
	if err != nil {
		return r, err
	}

	var pageToken *int64
	if request.GetPageToken() != "" {
		// 解析 page_token 为 int64 时间戳
		// page_token 是上一页最后一条记录的 created_at 时间戳字符串
		// 此处先传 nil 让 DAO 从头查
	}

	param := repo.ListToolCommitParam{
		ToolID:           request.GetToolID(),
		WithCommitDetail: request.GetWithCommitDetail(),
		PageSize:         int(request.GetPageSize()),
		PageToken:        pageToken,
		Asc:              request.GetAsc(),
	}

	result, err := app.toolRepo.ListToolCommitInfo(ctx, param)
	if err != nil {
		return r, err
	}

	r.ToolCommitInfos = convertor.BatchCommitInfoDO2ToolDTO(result.CommitInfoDOs)
	r.ToolCommitDetailMapping = convertor.ToolDetailDOMap2DTOMap(result.CommitDetailMapping)
	r.HasMore = &result.HasMore

	if result.HasMore {
		tokenStr := fmt.Sprintf("%d", result.NextPageToken)
		r.NextPageToken = &tokenStr
	}

	// 获取用户信息
	userIDSet := make(map[string]struct{})
	for _, info := range result.CommitInfoDOs {
		if info != nil && info.CommittedBy != "" {
			userIDSet[info.CommittedBy] = struct{}{}
		}
	}
	if len(userIDSet) > 0 {
		userDOs, err := app.userRPCProvider.MGetUserInfo(ctx, maps.Keys(userIDSet))
		if err != nil {
			return r, err
		}
		r.Users = convertor.BatchUserInfoDO2DTO(userDOs)
	}

	return r, nil
}

func (app *ToolManageApplicationImpl) BatchGetTools(ctx context.Context, request *tool_manage.BatchGetToolsRequest) (r *tool_manage.BatchGetToolsResponse, err error) {
	r = tool_manage.NewBatchGetToolsResponse()
	// 内部接口不鉴权

	queries := make([]repo.MGetToolQuery, 0, len(request.GetQueries()))
	queryMap := make(map[repo.MGetToolQuery]*tool_manage.ToolQuery)
	for _, q := range request.GetQueries() {
		if q == nil {
			continue
		}
		query := repo.MGetToolQuery{
			ToolID:  q.GetToolID(),
			Version: q.GetVersion(),
		}
		queries = append(queries, query)
		queryMap[query] = q
	}

	toolMap, err := app.toolRepo.MGetTool(ctx, queries)
	if err != nil {
		return r, err
	}

	for query, toolDO := range toolMap {
		r.Items = append(r.Items, &tool_manage.ToolResult_{
			Query: queryMap[query],
			Tool:  convertor.CommonToolDO2DTO(toolDO),
		})
	}

	return r, nil
}
