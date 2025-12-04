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
	labelRepo repo.ILabelRepo,
	promptService service.IPromptService,
	authRPCProvider rpc.IAuthProvider,
	userRPCProvider rpc.IUserProvider,
	auditRPCProvider rpc.IAuditProvider,
	configProvider conf.IConfigProvider,
) manage.PromptManageService {
	return &PromptManageApplicationImpl{
		manageRepo:       promptManageRepo,
		labelRepo:        labelRepo,
		promptService:    promptService,
		authRPCProvider:  authRPCProvider,
		userRPCProvider:  userRPCProvider,
		auditRPCProvider: auditRPCProvider,
		configProvider:   configProvider,
	}
}

type PromptManageApplicationImpl struct {
	manageRepo       repo.IManageRepo
	labelRepo        repo.ILabelRepo
	promptService    service.IPromptService
	authRPCProvider  rpc.IAuthProvider
	userRPCProvider  rpc.IUserProvider
	auditRPCProvider rpc.IAuditProvider
	configProvider   conf.IConfigProvider
}

func (app *PromptManageApplicationImpl) ListParentPrompt(ctx context.Context, request *manage.ListParentPromptRequest) (r *manage.ListParentPromptResponse, err error) {
	r = manage.NewListParentPromptResponse()

	// 用户验证
	userID, ok := session.UserIDInCtx(ctx)
	if !ok || lo.IsEmpty(userID) {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found"))
	}

	// 权限检查
	err = app.authRPCProvider.CheckSpacePermission(ctx, request.GetWorkspaceID(), consts.ActionLoopPromptRead)
	if err != nil {
		return r, err
	}

	// 参数验证
	if request.GetPromptID() <= 0 {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("Prompt ID is required"))
	}

	// 调用repository层查询父prompt
	result, err := app.manageRepo.ListParentPrompt(ctx, repo.ListParentPromptParam{
		SubPromptID:       request.GetPromptID(),
		SubPromptVersions: request.GetCommitVersions(),
	})
	if err != nil {
		return r, err
	}

	// 转换结果
	parentPrompts := make(map[string][]*prompt.PromptCommitVersions)
	for version, promptCommitVersions := range result {
		promptVersionDTOs := make([]*prompt.PromptCommitVersions, 0, len(promptCommitVersions))
		for _, promptCommitVersion := range promptCommitVersions {
			promptVersionDTO := &prompt.PromptCommitVersions{
				ID:             ptr.Of(promptCommitVersion.PromptID),
				WorkspaceID:    ptr.Of(promptCommitVersion.SpaceID),
				PromptKey:      ptr.Of(promptCommitVersion.PromptKey),
				PromptBasic:    convertor.PromptBasicDO2DTO(promptCommitVersion.PromptBasic),
				CommitVersions: promptCommitVersion.CommitVersions,
			}
			promptVersionDTOs = append(promptVersionDTOs, promptVersionDTO)
		}
		parentPrompts[version] = promptVersionDTOs
	}

	r.ParentPrompts = parentPrompts
	return r, nil
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

	if request.PromptType == nil {
		request.PromptType = ptr.Of(prompt.PromptTypeNormal)
	}
	// create prompt
	promptDTO := &prompt.Prompt{
		WorkspaceID: request.WorkspaceID,
		PromptKey:   request.PromptKey,
		PromptBasic: &prompt.PromptBasic{
			PromptType:  request.PromptType,
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

	// create prompt using domain service with snippet validation
	var promptID int64
	promptID, err = app.promptService.CreatePrompt(ctx, promptDO)
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

	promptType := entity.PromptTypeNormal
	if promptDO.PromptBasic != nil && promptDO.PromptBasic.PromptType != "" {
		promptType = promptDO.PromptBasic.PromptType
	}

	clonedPromptDO.PromptBasic = &entity.PromptBasic{
		DisplayName: request.GetClonedPromptName(),
		Description: request.GetClonedPromptDescription(),
		CreatedBy:   userID,
		PromptType:  promptType,
	}
	clonedPromptDO.PromptDraft = &entity.PromptDraft{
		DraftInfo: &entity.DraftInfo{
			UserID:     userID,
			IsModified: true,
		},
		PromptDetail: clonedPromptDO.PromptCommit.PromptDetail,
	}
	clonedPromptDO.PromptCommit = nil
	clonedPromptID, err := app.promptService.CreatePrompt(ctx, clonedPromptDO)
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
	if promptDO.PromptBasic.PromptType == entity.PromptTypeSnippet {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("Snippet prompt can not be deleted"))
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
	getPromptParam := service.GetPromptParam{
		PromptID: request.GetPromptID(),

		WithCommit:    !lo.IsEmpty(commitVersion),
		CommitVersion: commitVersion,

		WithDraft:     request.GetWithDraft(),
		UserID:        userID,
		ExpandSnippet: request.GetExpandSnippet(),
	}
	promptDO, err := app.promptService.GetPrompt(ctx, getPromptParam)
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

	// [prompt片段]返回被引用总次数
	if promptDO.PromptBasic != nil && promptDO.PromptBasic.PromptType == entity.PromptTypeSnippet {
		var commitVersionParams []string
		if request.GetWithCommit() && lo.IsNotEmpty(request.GetCommitVersion()) {
			commitVersionParams = append(commitVersionParams, commitVersion)
		} else {
			commitVersions, err := app.manageRepo.MGetVersionsByPromptID(ctx, request.GetPromptID())
			if err != nil {
				return r, err
			}
			commitVersionParams = append(commitVersionParams, commitVersions...)
		}
		if len(commitVersionParams) > 0 {
			parentPromptCommitVersions, err := app.manageRepo.ListParentPrompt(ctx, repo.ListParentPromptParam{
				SubPromptID:       request.GetPromptID(),
				SubPromptVersions: commitVersionParams,
			})
			if err != nil {
				return r, err
			}
			if len(parentPromptCommitVersions) > 0 {
				var total int32
				for _, parents := range parentPromptCommitVersions {
					for _, parent := range parents {
						total += int32(len(parent.CommitVersions))
					}
				}
				r.TotalParentReferences = ptr.Of(total)
			}
		}
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

	// Default filtering behavior: if no filter_prompt_types specified, only show normal prompts
	filterPromptTypes := request.GetFilterPromptTypes()
	if len(filterPromptTypes) == 0 {
		filterPromptTypes = []prompt.PromptType{prompt.PromptTypeNormal}
	}

	// Convert prompt.PromptType to entity.PromptType
	var entityFilterPromptTypes []entity.PromptType
	for _, pt := range filterPromptTypes {
		entityFilterPromptTypes = append(entityFilterPromptTypes, convertor.PromptTypeDTO2DO(pt))
	}

	listPromptParam := repo.ListPromptParam{
		SpaceID: request.GetWorkspaceID(),

		KeyWord:           request.GetKeyWord(),
		CreatedBys:        request.GetCreatedBys(),
		UserID:            userID,
		CommittedOnly:     request.GetCommittedOnly(),
		FilterPromptTypes: entityFilterPromptTypes,

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
		if lo.IsNotEmpty(promptDTO.PromptBasic.GetUpdatedBy()) {
			userIDSet[promptDTO.PromptBasic.GetUpdatedBy()] = struct{}{}
		}
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
	draftInfoDO, err := app.promptService.SaveDraft(ctx, savingPromptDO)
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

	// 验证label是否存在（如果有提供label）
	var labelKeys []string
	if len(request.GetLabelKeys()) > 0 {
		// 使用labelService验证label是否存在
		err = app.promptService.ValidateLabelsExist(ctx, promptDO.SpaceID, request.GetLabelKeys())
		if err != nil {
			return r, err
		}

		labelKeys = request.GetLabelKeys()
	}

	// commit
	commitDraftParam := repo.CommitDraftParam{
		PromptID: request.GetPromptID(),

		UserID: userID,

		CommitVersion:     request.GetCommitVersion(),
		CommitDescription: request.GetCommitDescription(),
		LabelKeys:         labelKeys,
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
	if request.GetWithCommitDetail() {
		commitDTOs := convertor.BatchPromptCommitDO2DTO(listCommitResult.CommitDOs)
		promptCommitDetailMap := make(map[string]*prompt.PromptDetail)
		for _, commitDTO := range commitDTOs {
			if commitDTO == nil || commitDTO.CommitInfo == nil || lo.IsEmpty(commitDTO.CommitInfo.Version) {
				continue
			}
			promptCommitDetailMap[commitDTO.GetCommitInfo().GetVersion()] = commitDTO.Detail
		}
		r.PromptCommitDetailMapping = promptCommitDetailMap
	}
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

	if len(r.PromptCommitInfos) > 0 {
		var commitVersions []string
		for _, commitInfo := range r.PromptCommitInfos {
			if commitInfo != nil && commitInfo.Version != nil {
				commitVersions = append(commitVersions, commitInfo.GetVersion())
			}
		}

		// 填充commit版本标签映射
		if len(commitVersions) > 0 {
			// 查询这些版本的标签映射，使用labelService
			commitLabelMapping, err := app.promptService.BatchGetCommitLabels(ctx, request.GetPromptID(), commitVersions)
			if err != nil {
				return r, err
			}

			// 构建版本到标签的映射
			commitVersionLabelMapping := make(map[string][]*prompt.Label)
			for version, labelKeys := range commitLabelMapping {
				var labelDTOs []*prompt.Label
				for _, labelKey := range labelKeys {
					labelDTOs = append(labelDTOs, &prompt.Label{
						Key: ptr.Of(labelKey),
					})
				}
				commitVersionLabelMapping[version] = labelDTOs
			}

			r.CommitVersionLabelMapping = commitVersionLabelMapping
		}
		// 填充被引用次数映射
		if len(commitVersions) > 0 && promptDO.PromptBasic != nil && promptDO.PromptBasic.PromptType == entity.PromptTypeSnippet {
			// 查询这些版本的被引用次数，使用labelService
			parentPromptCommitVersions, err := app.manageRepo.ListParentPrompt(ctx, repo.ListParentPromptParam{
				SubPromptID:       request.GetPromptID(),
				SubPromptVersions: commitVersions,
			})
			if err != nil {
				return r, err
			}

			// 构建版本到被引用次数的映射
			commitVersionReferencesMapping := make(map[string]int32)
			for version, parents := range parentPromptCommitVersions {
				for _, parent := range parents {
					if parent == nil {
						continue
					}
					commitVersionReferencesMapping[version] += int32(len(parent.CommitVersions))
				}
			}

			r.ParentReferencesMapping = commitVersionReferencesMapping
		}
	}

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
	_, err = app.promptService.SaveDraft(ctx, promptDO)
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

// CreateLabel creates a new label in the workspace
func (app *PromptManageApplicationImpl) CreateLabel(ctx context.Context, request *manage.CreateLabelRequest) (r *manage.CreateLabelResponse, err error) {
	r = manage.NewCreateLabelResponse()

	// 用户
	userID, ok := session.UserIDInCtx(ctx)
	if !ok || lo.IsEmpty(userID) {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found"))
	}

	// 权限检查
	err = app.authRPCProvider.CheckSpacePermission(ctx, request.GetWorkspaceID(), consts.ActionWorkspaceCreateLoopPrompt)
	if err != nil {
		return r, err
	}

	// 使用labelService创建Label，包含预置标签重复校验
	labelDO := &entity.PromptLabel{
		SpaceID:   request.GetWorkspaceID(),
		LabelKey:  request.GetLabel().GetKey(),
		CreatedBy: userID,
		UpdatedBy: userID,
	}
	err = app.promptService.CreateLabel(ctx, labelDO)
	if err != nil {
		return r, err
	}

	return r, nil
}

// ListLabel lists labels in the workspace with pagination
func (app *PromptManageApplicationImpl) ListLabel(ctx context.Context, request *manage.ListLabelRequest) (r *manage.ListLabelResponse, err error) {
	r = manage.NewListLabelResponse()

	// 权限检查
	err = app.authRPCProvider.CheckSpacePermission(ctx, request.GetWorkspaceID(), consts.ActionWorkspaceListLoopPrompt)
	if err != nil {
		return r, err
	}

	// 当需要查询prompt版本映射时，验证参数
	if request.GetWithPromptVersionMapping() {
		if request.GetPromptID() <= 0 {
			return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("PromptID must be provided when WithPromptVersionMapping is true"))
		}
	}

	// 构建查询参数
	param := service.ListLabelParam{
		SpaceID:      request.GetWorkspaceID(),
		LabelKeyLike: request.GetLabelKeyLike(),
		PageSize:     int(request.GetPageSize()),
	}

	// 处理分页token
	if request.GetPageToken() != "" {
		pageToken, parseErr := strconv.ParseInt(request.GetPageToken(), 10, 64)
		if parseErr != nil {
			return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("Invalid page token"))
		}
		param.PageToken = &pageToken
	}

	// 调用domain层服务查询Label列表
	labels, nextPageToken, err := app.promptService.ListLabel(ctx, param)
	if err != nil {
		return r, err
	}

	// 转换结果
	r.Labels = convertor.BatchLabelDO2DTO(labels)
	if nextPageToken != nil {
		r.NextPageToken = ptr.Of(strconv.FormatInt(*nextPageToken, 10))
		r.HasMore = ptr.Of(true)
	} else {
		r.HasMore = ptr.Of(false)
	}

	// 当需要查询prompt版本映射时，查询每个label关联该prompt的版本
	if request.GetWithPromptVersionMapping() && len(r.Labels) > 0 {
		// 构建查询列表
		queries := make([]service.PromptLabelQuery, 0, len(r.Labels))
		for _, label := range r.Labels {
			if label != nil && label.Key != nil {
				queries = append(queries, service.PromptLabelQuery{
					PromptID: request.GetPromptID(),
					LabelKey: label.GetKey(),
				})
			}
		}

		// 调用服务层方法
		promptVersionMapping, err := app.promptService.BatchGetLabelMappingPromptVersion(ctx, queries)
		if err != nil {
			return r, err
		}

		// 转换结果格式，从 map[PromptLabelQuery]string 转为 map[string]string
		resultMapping := make(map[string]string)
		for key, version := range promptVersionMapping {
			resultMapping[key.LabelKey] = version
		}
		r.PromptVersionMapping = resultMapping
	}

	return r, nil
}

// BatchGetLabel retrieves labels by their keys
func (app *PromptManageApplicationImpl) BatchGetLabel(ctx context.Context, request *manage.BatchGetLabelRequest) (r *manage.BatchGetLabelResponse, err error) {
	r = manage.NewBatchGetLabelResponse()
	// 权限检查
	err = app.authRPCProvider.CheckSpacePermission(ctx, request.GetWorkspaceID(), consts.ActionWorkspaceListLoopPrompt)
	if err != nil {
		return r, err
	}

	labels, err := app.labelRepo.BatchGetLabel(ctx, request.GetWorkspaceID(), request.GetLabelKeys())
	if err != nil {
		return nil, err
	}
	r.Labels = convertor.BatchLabelDO2DTO(labels)
	return r, nil
}

// UpdateCommitLabels updates labels for a specific commit version
func (app *PromptManageApplicationImpl) UpdateCommitLabels(ctx context.Context, request *manage.UpdateCommitLabelsRequest) (r *manage.UpdateCommitLabelsResponse, err error) {
	r = manage.NewUpdateCommitLabelsResponse()

	// 用户
	userID, ok := session.UserIDInCtx(ctx)
	if !ok || lo.IsEmpty(userID) {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found"))
	}

	// 权限检查
	err = app.authRPCProvider.MCheckPromptPermission(ctx, request.GetWorkspaceID(), []int64{request.GetPromptID()}, consts.ActionLoopPromptEdit)
	if err != nil {
		return r, err
	}

	// 构建更新参数
	param := service.UpdateCommitLabelsParam{
		PromptID:      request.GetPromptID(),
		CommitVersion: request.GetCommitVersion(),
		LabelKeys:     request.GetLabelKeys(),
		UpdatedBy:     userID,
	}

	// 更新commit的labels
	err = app.promptService.UpdateCommitLabels(ctx, param)
	if err != nil {
		return r, err
	}

	return r, nil
}
