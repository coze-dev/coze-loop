// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Masterminds/semver/v3"
	"github.com/samber/lo"
	"golang.org/x/exp/maps"

	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/domain/prompt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/manage"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/application/convertor"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/conf"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/service"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/consts"
	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func NewPromptManageApplication(
	promptManageRepo repo.IManageRepo,
	promptService service.IPromptService,
	authRPCProvider rpc.IAuthProvider,
	userRPCProvider rpc.IUserProvider,
	auditRPCProvider rpc.IAuditProvider,
	configProvider conf.IConfigProvider,
) manage.PromptManageService {
	return &PromptManageApplicationImpl{
		manageRepo:       promptManageRepo,
		promptService:    promptService,
		authRPCProvider:  authRPCProvider,
		userRPCProvider:  userRPCProvider,
		auditRPCProvider: auditRPCProvider,
		configProvider:   configProvider,
	}
}

type PromptManageApplicationImpl struct {
	manageRepo       repo.IManageRepo
	promptService    service.IPromptService
	authRPCProvider  rpc.IAuthProvider
	userRPCProvider  rpc.IUserProvider
	auditRPCProvider rpc.IAuditProvider
	configProvider   conf.IConfigProvider
}

func (app *PromptManageApplicationImpl) CreatePrompt(ctx context.Context, request *manage.CreatePromptRequest) (r *manage.CreatePromptResponse, err error) {
	r = manage.NewCreatePromptResponse()

	// 用户
	userID, ok := session.UserIDInCtx(ctx)
	if !ok || lo.IsEmpty(userID) {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found"))
	}

	// 权限
	err = app.authRPCProvider.CheckSpacePermission(ctx, request.GetWorkspaceID(), consts.ActionWorkspaceCreateLoopPrompt)
	if err != nil {
		return r, err
	}

	// create prompt
	promptDTO := &prompt.Prompt{
		WorkspaceID: request.WorkspaceID,
		PromptKey:   request.PromptKey,
		PromptBasic: &prompt.PromptBasic{
			DisplayName: request.PromptName,
			Description: request.PromptDescription,
			CreatedBy:   ptr.Of(userID),
			UpdatedBy:   ptr.Of(userID),
		},
		PromptDraft: func() *prompt.PromptDraft {
			if request.DraftDetail == nil {
				return nil
			}
			return &prompt.PromptDraft{
				DraftInfo: &prompt.DraftInfo{
					UserID:     ptr.Of(userID),
					IsModified: ptr.Of(true),
				},
				Detail: request.DraftDetail,
			}
		}(),
	}
	promptDO := convertor.PromptDTO2DO(promptDTO)

	// 审核
	err = app.auditRPCProvider.AuditPrompt(ctx, promptDO)
	if err != nil {
		return r, err
	}

	// create prompt
	promptID, err := app.manageRepo.CreatePrompt(ctx, promptDO)
	if err != nil {
		return r, err
	}
	r.PromptID = ptr.Of(promptID)
	return r, nil
}

func (app *PromptManageApplicationImpl) ClonePrompt(ctx context.Context, request *manage.ClonePromptRequest) (r *manage.ClonePromptResponse, err error) {
	r = manage.NewClonePromptResponse()

	// 用户
	userID, ok := session.UserIDInCtx(ctx)
	if !ok {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found"))
	}

	// prompt
	getPromptParam := repo.GetPromptParam{
		PromptID:      request.GetPromptID(),
		WithCommit:    true,
		CommitVersion: request.GetCommitVersion(),
	}
	promptDO, err := app.manageRepo.GetPrompt(ctx, getPromptParam)
	if err != nil {
		return r, err
	}

	// 权限
	err = app.authRPCProvider.MCheckPromptPermission(ctx, promptDO.SpaceID, []int64{request.GetPromptID()}, consts.ActionLoopPromptRead)
	if err != nil {
		return r, err
	}
	err = app.authRPCProvider.CheckSpacePermission(ctx, promptDO.SpaceID, consts.ActionWorkspaceCreateLoopPrompt)
	if err != nil {
		return r, err
	}

	// clone prompt
	clonedPromptDO := promptDO.CloneDetail()
	clonedPromptDO.PromptKey = request.GetClonedPromptKey()
	clonedPromptDO.PromptBasic = &entity.PromptBasic{
		DisplayName: request.GetClonedPromptName(),
		Description: request.GetClonedPromptDescription(),
		CreatedBy:   userID,
	}
	clonedPromptDO.PromptDraft = &entity.PromptDraft{
		DraftInfo: &entity.DraftInfo{
			UserID:     userID,
			IsModified: true,
		},
		PromptDetail: clonedPromptDO.PromptCommit.PromptDetail,
	}
	clonedPromptDO.PromptCommit = nil
	clonedPromptID, err := app.manageRepo.CreatePrompt(ctx, clonedPromptDO)
	if err != nil {
		return r, err
	}
	r.ClonedPromptID = ptr.Of(clonedPromptID)
	return r, nil
}

func (app *PromptManageApplicationImpl) DeletePrompt(ctx context.Context, request *manage.DeletePromptRequest) (r *manage.DeletePromptResponse, err error) {
	r = manage.NewDeletePromptResponse()

	// 用户
	_, ok := session.UserIDInCtx(ctx)
	if !ok {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found"))
	}

	// prompt
	getPromptParam := repo.GetPromptParam{
		PromptID: request.GetPromptID(),
	}
	promptDO, err := app.manageRepo.GetPrompt(ctx, getPromptParam)
	if err != nil {
		return r, err
	}

	// 权限
	err = app.authRPCProvider.MCheckPromptPermission(ctx, promptDO.SpaceID, []int64{request.GetPromptID()}, consts.ActionLoopPromptEdit)
	if err != nil {
		return r, err
	}

	// delete prompt
	err = app.manageRepo.DeletePrompt(ctx, request.GetPromptID())
	return r, err
}

func (app *PromptManageApplicationImpl) GetPrompt(ctx context.Context, request *manage.GetPromptRequest) (r *manage.GetPromptResponse, err error) {
	r = manage.NewGetPromptResponse()

	// 用户
	userID, ok := session.UserIDInCtx(ctx)
	if !ok {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found"))
	}

	// commit default version
	commitVersion := request.GetCommitVersion()
	if request.GetWithCommit() && lo.IsEmpty(commitVersion) {
		getPromptParam := repo.GetPromptParam{
			PromptID: request.GetPromptID(),
		}
		promptDO, err := app.manageRepo.GetPrompt(ctx, getPromptParam)
		if err != nil {
			return r, err
		}
		commitVersion = promptDO.PromptBasic.LatestVersion
	}

	// prompt
	getPromptParam := repo.GetPromptParam{
		PromptID: request.GetPromptID(),

		WithCommit:    !lo.IsEmpty(commitVersion),
		CommitVersion: commitVersion,

		WithDraft: request.GetWithDraft(),
		UserID:    userID,
	}
	promptDO, err := app.manageRepo.GetPrompt(ctx, getPromptParam)
	if err != nil {
		return r, err
	}

	// 权限
	err = app.authRPCProvider.MCheckPromptPermission(ctx, promptDO.SpaceID, []int64{request.GetPromptID()}, consts.ActionLoopPromptRead)
	if err != nil {
		return r, err
	}

	// 空间权限
	if request.GetWorkspaceID() > 0 && request.GetWorkspaceID() != promptDO.SpaceID {
		return r, errorx.NewByCode(prompterr.ResourceNotFoundCode, errorx.WithExtraMsg("WorkspaceID not match"))
	}

	// 返回
	r.Prompt = convertor.PromptDO2DTO(promptDO)

	// 返回默认配置
	if request.GetWithDefaultConfig() {
		defaultConfig, err := app.configProvider.GetPromptDefaultConfig(ctx)
		if err != nil {
			return r, err
		}
		r.DefaultConfig = defaultConfig
	}
	return r, err
}

func (app *PromptManageApplicationImpl) BatchGetPrompt(ctx context.Context, request *manage.BatchGetPromptRequest) (r *manage.BatchGetPromptResponse, err error) {
	r = manage.NewBatchGetPromptResponse()
	// 内部接口不鉴权
	paramMap := make(map[repo.GetPromptParam]*manage.PromptQuery)
	for _, query := range request.Queries {
		if query == nil {
			continue
		}
		paramMap[repo.GetPromptParam{
			PromptID:      query.GetPromptID(),
			WithCommit:    query.GetWithCommit(),
			CommitVersion: query.GetCommitVersion(),
		}] = query
	}
	promptMap, err := app.manageRepo.MGetPrompt(ctx, maps.Keys(paramMap))
	if err != nil {
		return r, err
	}
	for query, promptDO := range promptMap {
		r.Results = append(r.Results, &manage.PromptResult_{
			Query:  paramMap[query],
			Prompt: convertor.PromptDO2DTO(promptDO),
		})
	}
	return r, err
}

func (app *PromptManageApplicationImpl) ListPrompt(ctx context.Context, request *manage.ListPromptRequest) (r *manage.ListPromptResponse, err error) {
	r = manage.NewListPromptResponse()

	// 用户
	userID, ok := session.UserIDInCtx(ctx)
	if !ok {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found"))
	}

	// 权限
	err = app.authRPCProvider.CheckSpacePermission(ctx, request.GetWorkspaceID(), consts.ActionWorkspaceListLoopPrompt)
	if err != nil {
		return r, err
	}

	// list prompt
	listPromptParam := repo.ListPromptParam{
		SpaceID: request.GetWorkspaceID(),

		KeyWord:       request.GetKeyWord(),
		CreatedBys:    request.GetCreatedBys(),
		UserID:        userID,
		CommittedOnly: request.GetCommittedOnly(),

		PageNum:  int(request.GetPageNum()),
		PageSize: int(request.GetPageSize()),
		OrderBy:  app.listPromptOrderBy(request.OrderBy),
		Asc:      request.GetAsc(),
	}
	listPromptResult, err := app.manageRepo.ListPrompt(ctx, listPromptParam)
	if err != nil {
		return r, err
	}
	if listPromptResult == nil {
		return r, nil
	}
	r.Total = ptr.Of(int32(listPromptResult.Total))
	r.Prompts = convertor.BatchPromptDO2DTO(listPromptResult.PromptDOs)
	userIDSet := make(map[string]struct{})
	for _, promptDTO := range r.Prompts {
		if promptDTO == nil || promptDTO.PromptBasic == nil || lo.IsEmpty(promptDTO.PromptBasic.GetCreatedBy()) {
			continue
		}
		userIDSet[promptDTO.PromptBasic.GetCreatedBy()] = struct{}{}
	}
	userDOs, err := app.userRPCProvider.MGetUserInfo(ctx, maps.Keys(userIDSet))
	if err != nil {
		return r, err
	}
	r.Users = convertor.BatchUserInfoDO2DTO(userDOs)
	return r, err
}

func (app *PromptManageApplicationImpl) UpdatePrompt(ctx context.Context, request *manage.UpdatePromptRequest) (r *manage.UpdatePromptResponse, err error) {
	r = manage.NewUpdatePromptResponse()

	// 用户
	userID, ok := session.UserIDInCtx(ctx)
	if !ok {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found"))
	}

	// prompt
	getPromptParam := repo.GetPromptParam{
		PromptID: request.GetPromptID(),
	}
	promptDO, err := app.manageRepo.GetPrompt(ctx, getPromptParam)
	if err != nil {
		return r, err
	}

	// 权限
	err = app.authRPCProvider.MCheckPromptPermission(ctx, promptDO.SpaceID, []int64{request.GetPromptID()}, consts.ActionLoopPromptEdit)
	if err != nil {
		return r, err
	}

	// 审核
	err = app.auditRPCProvider.AuditPrompt(ctx, &entity.Prompt{
		ID: request.GetPromptID(),
		PromptBasic: &entity.PromptBasic{
			DisplayName: request.GetPromptName(),
			Description: request.GetPromptDescription(),
		},
	})
	if err != nil {
		return r, err
	}

	// update prompt
	updatePromptParam := repo.UpdatePromptParam{
		PromptID:  request.GetPromptID(),
		UpdatedBy: userID,

		PromptName:        request.GetPromptName(),
		PromptDescription: request.GetPromptDescription(),
	}
	return r, app.manageRepo.UpdatePrompt(ctx, updatePromptParam)
}

func (app *PromptManageApplicationImpl) SaveDraft(ctx context.Context, request *manage.SaveDraftRequest) (r *manage.SaveDraftResponse, err error) {
	r = manage.NewSaveDraftResponse()

	// 用户
	userID, ok := session.UserIDInCtx(ctx)
	if !ok {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found"))
	}

	// 校验
	if request.PromptDraft.DraftInfo == nil || request.PromptDraft.Detail == nil {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("Draft is not specified"))
	}

	// prompt
	getPromptParam := repo.GetPromptParam{
		PromptID: request.GetPromptID(),
	}
	promptDO, err := app.manageRepo.GetPrompt(ctx, getPromptParam)
	if err != nil {
		return r, err
	}

	// 权限
	err = app.authRPCProvider.MCheckPromptPermission(ctx, promptDO.SpaceID, []int64{request.GetPromptID()}, consts.ActionLoopPromptEdit)
	if err != nil {
		return r, err
	}

	// prepare
	savingPromptDTO := &prompt.Prompt{
		ID:          request.PromptID,
		PromptDraft: request.PromptDraft,
	}
	savingPromptDTO.PromptDraft.DraftInfo.UserID = ptr.Of(userID)
	savingPromptDO := convertor.PromptDTO2DO(savingPromptDTO)

	// 审核
	err = app.auditRPCProvider.AuditPrompt(ctx, savingPromptDO)
	if err != nil {
		return r, err
	}

	// save draft
	draftInfoDO, err := app.manageRepo.SaveDraft(ctx, savingPromptDO)
	if err != nil {
		return r, err
	}
	r.DraftInfo = convertor.DraftInfoDO2DTO(draftInfoDO)
	return r, nil
}

func (app *PromptManageApplicationImpl) CommitDraft(ctx context.Context, request *manage.CommitDraftRequest) (r *manage.CommitDraftResponse, err error) {
	r = manage.NewCommitDraftResponse()

	// 用户
	userID, ok := session.UserIDInCtx(ctx)
	if !ok {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found"))
	}

	// 校验
	_, err = semver.StrictNewVersion(request.GetCommitVersion())
	if err != nil {
		return r, err
	}

	// prompt
	getPromptParam := repo.GetPromptParam{
		PromptID: request.GetPromptID(),
	}
	promptDO, err := app.manageRepo.GetPrompt(ctx, getPromptParam)
	if err != nil {
		return r, err
	}

	// 权限
	err = app.authRPCProvider.MCheckPromptPermission(ctx, promptDO.SpaceID, []int64{request.GetPromptID()}, consts.ActionLoopPromptEdit)
	if err != nil {
		return r, err
	}

	// commit
	commitDraftParam := repo.CommitDraftParam{
		PromptID: request.GetPromptID(),

		UserID: userID,

		CommitVersion:     request.GetCommitVersion(),
		CommitDescription: request.GetCommitDescription(),
	}
	return r, app.manageRepo.CommitDraft(ctx, commitDraftParam)
}

func (app *PromptManageApplicationImpl) ListCommit(ctx context.Context, request *manage.ListCommitRequest) (r *manage.ListCommitResponse, err error) {
	r = manage.NewListCommitResponse()

	// 用户
	_, ok := session.UserIDInCtx(ctx)
	if !ok {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found"))
	}

	// prompt
	getPromptParam := repo.GetPromptParam{
		PromptID: request.GetPromptID(),
	}
	promptDO, err := app.manageRepo.GetPrompt(ctx, getPromptParam)
	if err != nil {
		return r, err
	}

	// 权限
	err = app.authRPCProvider.MCheckPromptPermission(ctx, promptDO.SpaceID, []int64{request.GetPromptID()}, consts.ActionLoopPromptRead)
	if err != nil {
		return r, err
	}

	// 校验
	var pageTokenPtr *int64
	if request.PageToken != nil {
		pageToken, err := strconv.ParseInt(request.GetPageToken(), 10, 64)
		if err != nil {
			return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg(
				fmt.Sprintf("Page token is invalid, page token = %s", request.GetPageToken())))
		}
		pageTokenPtr = ptr.Of(pageToken)
	}

	// list commit
	listCommitParam := repo.ListCommitInfoParam{
		PromptID: request.GetPromptID(),

		PageSize:  int(request.GetPageSize()),
		PageToken: pageTokenPtr,
		Asc:       request.GetAsc(),
	}
	listCommitResult, err := app.manageRepo.ListCommitInfo(ctx, listCommitParam)
	if err != nil {
		return r, err
	}
	if listCommitResult == nil {
		return r, nil
	}
	if listCommitResult.NextPageToken > 0 {
		r.NextPageToken = ptr.Of(strconv.FormatInt(listCommitResult.NextPageToken, 10))
		r.HasMore = ptr.Of(true)
	}
	r.PromptCommitInfos = convertor.BatchCommitInfoDO2DTO(listCommitResult.CommitInfoDOs)
	userIDSet := make(map[string]struct{})
	for _, commitInfoDTO := range r.PromptCommitInfos {
		if commitInfoDTO == nil || lo.IsEmpty(commitInfoDTO.GetCommittedBy()) {
			continue
		}
		userIDSet[commitInfoDTO.GetCommittedBy()] = struct{}{}
	}
	userDOs, err := app.userRPCProvider.MGetUserInfo(ctx, maps.Keys(userIDSet))
	if err != nil {
		return manage.NewListCommitResponse(), err
	}
	r.Users = convertor.BatchUserInfoDO2DTO(userDOs)
	return r, nil
}

func (app *PromptManageApplicationImpl) RevertDraftFromCommit(ctx context.Context, request *manage.RevertDraftFromCommitRequest) (r *manage.RevertDraftFromCommitResponse, err error) {
	r = manage.NewRevertDraftFromCommitResponse()

	// 用户
	userID, ok := session.UserIDInCtx(ctx)
	if !ok {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found"))
	}

	// prompt
	getPromptParam := repo.GetPromptParam{
		PromptID: request.GetPromptID(),

		WithCommit:    true,
		CommitVersion: request.GetCommitVersionRevertingFrom(),
	}
	promptDO, err := app.manageRepo.GetPrompt(ctx, getPromptParam)
	if err != nil {
		return r, err
	}
	if promptDO == nil || promptDO.PromptCommit == nil {
		return r, errorx.New("Prompt or commit not found, prompt id = %d, commit version = %s",
			request.GetPromptID(), request.GetCommitVersionRevertingFrom())
	}

	// 权限
	err = app.authRPCProvider.MCheckPromptPermission(ctx, promptDO.SpaceID, []int64{request.GetPromptID()}, consts.ActionLoopPromptEdit)
	if err != nil {
		return r, err
	}

	// save draft
	promptDO.PromptDraft = &entity.PromptDraft{
		DraftInfo: &entity.DraftInfo{
			UserID:      userID,
			BaseVersion: promptDO.PromptCommit.CommitInfo.Version,
		},
		PromptDetail: promptDO.PromptCommit.PromptDetail,
	}
	_, err = app.manageRepo.SaveDraft(ctx, promptDO)
	return r, err
}

func (app *PromptManageApplicationImpl) listPromptOrderBy(dtoEnum *manage.ListPromptOrderBy) int {
	if dtoEnum == nil {
		return mysql.ListPromptBasicOrderByID
	}
	switch *dtoEnum {
	case manage.ListPromptOrderByCreatedAt:
		return mysql.ListPromptBasicOrderByCreatedAt
	case manage.ListPromptOrderByCommitedAt:
		return mysql.ListPromptBasicOrderByLatestCommittedAt
	default:
		return mysql.ListPromptBasicOrderByID
	}
}
