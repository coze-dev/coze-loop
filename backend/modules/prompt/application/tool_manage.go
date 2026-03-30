// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"fmt"
	"strconv"

	"github.com/samber/lo"
	"golang.org/x/exp/maps"

	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/domain/tool"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/tool_manage"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/application/convertor"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/consts"
	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func NewToolManageApplication(
	toolManageRepo repo.IToolManageRepo,
	authRPCProvider rpc.IAuthProvider,
	userRPCProvider rpc.IUserProvider,
) tool_manage.ToolManageService {
	return &ToolManageApplicationImpl{
		toolManageRepo:  toolManageRepo,
		authRPCProvider: authRPCProvider,
		userRPCProvider: userRPCProvider,
	}
}

type ToolManageApplicationImpl struct {
	toolManageRepo  repo.IToolManageRepo
	authRPCProvider rpc.IAuthProvider
	userRPCProvider rpc.IUserProvider
}

func (app *ToolManageApplicationImpl) CreateTool(ctx context.Context, request *tool_manage.CreateToolRequest) (r *tool_manage.CreateToolResponse, err error) {
	r = tool_manage.NewCreateToolResponse()

	userID, ok := session.UserIDInCtx(ctx)
	if !ok || lo.IsEmpty(userID) {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found"))
	}

	err = app.authRPCProvider.CheckSpacePermission(ctx, request.GetWorkspaceID(), consts.ActionLoopPromptEdit)
	if err != nil {
		return r, err
	}

	toolDO := &entity.CommonTool{
		SpaceID: request.GetWorkspaceID(),
		ToolBasic: &entity.ToolBasic{
			Name:        request.GetToolName(),
			Description: request.GetToolDescription(),
			CreatedBy:   userID,
			UpdatedBy:   userID,
		},
	}

	if request.DraftDetail != nil {
		toolDO.ToolCommit = &entity.ToolCommit{
			ToolDetail: convertor.CommonToolDetailDTO2DO(request.DraftDetail),
			CommitInfo: &entity.ToolCommitInfo{
				Version:     entity.ToolPublicDraftVersion,
				CommittedBy: userID,
			},
		}
	}

	toolID, err := app.toolManageRepo.CreateTool(ctx, toolDO)
	if err != nil {
		return r, err
	}
	r.ToolID = ptr.Of(toolID)
	return r, nil
}

func (app *ToolManageApplicationImpl) GetToolDetail(ctx context.Context, request *tool_manage.GetToolDetailRequest) (r *tool_manage.GetToolDetailResponse, err error) {
	r = tool_manage.NewGetToolDetailResponse()

	userID, ok := session.UserIDInCtx(ctx)
	if !ok || lo.IsEmpty(userID) {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found"))
	}

	getParam := repo.GetToolParam{
		ToolID:        request.GetToolID(),
		WithCommit:    request.GetWithCommit(),
		CommitVersion: request.GetCommitVersion(),
		WithDraft:     request.GetWithDraft(),
	}
	toolDO, err := app.toolManageRepo.GetTool(ctx, getParam)
	if err != nil {
		return r, err
	}

	err = app.authRPCProvider.CheckSpacePermission(ctx, toolDO.SpaceID, consts.ActionLoopPromptRead)
	if err != nil {
		return r, err
	}

	r.Tool = convertor.CommonToolDO2DTO(toolDO)
	return r, nil
}

func (app *ToolManageApplicationImpl) ListTool(ctx context.Context, request *tool_manage.ListToolRequest) (r *tool_manage.ListToolResponse, err error) {
	r = tool_manage.NewListToolResponse()

	userID, ok := session.UserIDInCtx(ctx)
	if !ok || lo.IsEmpty(userID) {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found"))
	}

	err = app.authRPCProvider.CheckSpacePermission(ctx, request.GetWorkspaceID(), consts.ActionLoopPromptRead)
	if err != nil {
		return r, err
	}

	listParam := repo.ListToolParam{
		SpaceID:       request.GetWorkspaceID(),
		KeyWord:       request.GetKeyWord(),
		CreatedBys:    request.GetCreatedBys(),
		CommittedOnly: request.GetCommittedOnly(),
		PageNum:       int(request.GetPageNum()),
		PageSize:      int(request.GetPageSize()),
		OrderBy:       app.listToolOrderBy(request.OrderBy),
		Asc:           request.GetAsc(),
	}
	result, err := app.toolManageRepo.ListTool(ctx, listParam)
	if err != nil {
		return r, err
	}
	if result == nil {
		return r, nil
	}
	r.Total = ptr.Of(int32(result.Total))
	r.Tools = convertor.BatchCommonToolDO2DTO(result.ToolDOs)

	userIDSet := make(map[string]struct{})
	for _, toolDTO := range r.Tools {
		if toolDTO == nil || toolDTO.ToolBasic == nil {
			continue
		}
		if lo.IsNotEmpty(toolDTO.ToolBasic.GetCreatedBy()) {
			userIDSet[toolDTO.ToolBasic.GetCreatedBy()] = struct{}{}
		}
		if lo.IsNotEmpty(toolDTO.ToolBasic.GetUpdatedBy()) {
			userIDSet[toolDTO.ToolBasic.GetUpdatedBy()] = struct{}{}
		}
	}
	userDOs, err := app.userRPCProvider.MGetUserInfo(ctx, maps.Keys(userIDSet))
	if err != nil {
		return r, err
	}
	r.Users = convertor.BatchUserInfoDO2DTO(userDOs)
	return r, nil
}

func (app *ToolManageApplicationImpl) SaveToolDetail(ctx context.Context, request *tool_manage.SaveToolDetailRequest) (r *tool_manage.SaveToolDetailResponse, err error) {
	r = tool_manage.NewSaveToolDetailResponse()

	userID, ok := session.UserIDInCtx(ctx)
	if !ok || lo.IsEmpty(userID) {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found"))
	}

	getParam := repo.GetToolParam{ToolID: request.GetToolID()}
	toolDO, err := app.toolManageRepo.GetTool(ctx, getParam)
	if err != nil {
		return r, err
	}

	err = app.authRPCProvider.CheckSpacePermission(ctx, toolDO.SpaceID, consts.ActionLoopPromptEdit)
	if err != nil {
		return r, err
	}

	savingToolDO := &entity.CommonTool{
		ID:      request.GetToolID(),
		SpaceID: toolDO.SpaceID,
		ToolCommit: &entity.ToolCommit{
			ToolDetail: convertor.CommonToolDetailDTO2DO(request.ToolDetail),
			CommitInfo: &entity.ToolCommitInfo{
				Version:     entity.ToolPublicDraftVersion,
				BaseVersion: request.GetBaseVersion(),
				CommittedBy: userID,
			},
		},
	}

	return r, app.toolManageRepo.SaveDraft(ctx, savingToolDO)
}

func (app *ToolManageApplicationImpl) CommitToolDraft(ctx context.Context, request *tool_manage.CommitToolDraftRequest) (r *tool_manage.CommitToolDraftResponse, err error) {
	r = tool_manage.NewCommitToolDraftResponse()

	userID, ok := session.UserIDInCtx(ctx)
	if !ok || lo.IsEmpty(userID) {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found"))
	}

	getParam := repo.GetToolParam{ToolID: request.GetToolID()}
	toolDO, err := app.toolManageRepo.GetTool(ctx, getParam)
	if err != nil {
		return r, err
	}

	err = app.authRPCProvider.CheckSpacePermission(ctx, toolDO.SpaceID, consts.ActionLoopPromptEdit)
	if err != nil {
		return r, err
	}

	commitParam := repo.CommitToolDraftParam{
		ToolID:            request.GetToolID(),
		UserID:            userID,
		CommitVersion:     request.GetCommitVersion(),
		CommitDescription: request.GetCommitDescription(),
		BaseVersion:       request.GetBaseVersion(),
	}
	return r, app.toolManageRepo.CommitDraft(ctx, commitParam)
}

func (app *ToolManageApplicationImpl) ListToolCommit(ctx context.Context, request *tool_manage.ListToolCommitRequest) (r *tool_manage.ListToolCommitResponse, err error) {
	r = tool_manage.NewListToolCommitResponse()

	userID, ok := session.UserIDInCtx(ctx)
	if !ok || lo.IsEmpty(userID) {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found"))
	}

	getParam := repo.GetToolParam{ToolID: request.GetToolID()}
	toolDO, err := app.toolManageRepo.GetTool(ctx, getParam)
	if err != nil {
		return r, err
	}

	err = app.authRPCProvider.CheckSpacePermission(ctx, toolDO.SpaceID, consts.ActionLoopPromptRead)
	if err != nil {
		return r, err
	}

	var pageTokenPtr *int64
	if request.PageToken != nil {
		pageToken, err := strconv.ParseInt(request.GetPageToken(), 10, 64)
		if err != nil {
			return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg(
				fmt.Sprintf("Page token is invalid, page token = %s", request.GetPageToken())))
		}
		pageTokenPtr = ptr.Of(pageToken)
	}

	listParam := repo.ListToolCommitParam{
		ToolID:    request.GetToolID(),
		PageSize:  int(request.GetPageSize()),
		PageToken: pageTokenPtr,
		Asc:       request.GetAsc(),
	}
	result, err := app.toolManageRepo.ListToolCommitInfo(ctx, listParam)
	if err != nil {
		return r, err
	}
	if result == nil {
		return r, nil
	}
	if result.NextPageToken > 0 {
		r.NextPageToken = ptr.Of(strconv.FormatInt(result.NextPageToken, 10))
		r.HasMore = ptr.Of(true)
	}
	r.ToolCommitInfos = convertor.BatchToolCommitInfoDO2DTO(result.CommitInfoDOs)

	if request.GetWithCommitDetail() {
		commitDetailMap := make(map[string]*tool.ToolDetail)
		for _, commitDO := range result.CommitDOs {
			if commitDO == nil || commitDO.CommitInfo == nil || commitDO.CommitInfo.Version == "" {
				continue
			}
			commitDetailMap[commitDO.CommitInfo.Version] = convertor.CommonToolDetailDO2DTO(commitDO.ToolDetail)
		}
		r.ToolCommitDetailMapping = commitDetailMap
	}

	userIDSet := make(map[string]struct{})
	for _, commitInfo := range r.ToolCommitInfos {
		if commitInfo != nil && lo.IsNotEmpty(commitInfo.GetCommittedBy()) {
			userIDSet[commitInfo.GetCommittedBy()] = struct{}{}
		}
	}
	userDOs, err := app.userRPCProvider.MGetUserInfo(ctx, maps.Keys(userIDSet))
	if err != nil {
		return r, err
	}
	r.Users = convertor.BatchUserInfoDO2DTO(userDOs)
	return r, nil
}

func (app *ToolManageApplicationImpl) BatchGetTools(ctx context.Context, request *tool_manage.BatchGetToolsRequest) (r *tool_manage.BatchGetToolsResponse, err error) {
	r = tool_manage.NewBatchGetToolsResponse()

	userID, ok := session.UserIDInCtx(ctx)
	if !ok || lo.IsEmpty(userID) {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found"))
	}

	err = app.authRPCProvider.CheckSpacePermission(ctx, request.GetWorkspaceID(), consts.ActionLoopPromptRead)
	if err != nil {
		return r, err
	}

	type queryPair struct {
		query *tool_manage.ToolQuery
		param repo.GetToolParam
	}
	var pairs []queryPair
	var params []repo.GetToolParam
	for _, q := range request.GetQueries() {
		if q == nil {
			continue
		}
		p := repo.GetToolParam{
			ToolID:        q.GetToolID(),
			WithCommit:    true,
			CommitVersion: q.GetVersion(),
		}
		pairs = append(pairs, queryPair{query: q, param: p})
		params = append(params, p)
	}

	toolMap, err := app.toolManageRepo.MGetTool(ctx, params)
	if err != nil {
		return r, err
	}

	for _, pair := range pairs {
		r.Items = append(r.Items, &tool_manage.ToolResult_{
			Query: pair.query,
			Tool:  convertor.CommonToolDO2DTO(toolMap[pair.param]),
		})
	}
	return r, nil
}

func (app *ToolManageApplicationImpl) listToolOrderBy(orderBy *tool_manage.ListToolOrderBy) int {
	if orderBy == nil {
		return mysql.ListToolBasicOrderByID
	}
	switch *orderBy {
	case tool_manage.ListToolOrderByCreatedAt:
		return mysql.ListToolBasicOrderByCreatedAt
	default:
		return mysql.ListToolBasicOrderByID
	}
}
