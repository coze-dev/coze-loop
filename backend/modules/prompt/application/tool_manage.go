package application

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Masterminds/semver/v3"
	"github.com/samber/lo"
	"golang.org/x/exp/maps"

	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/domain/tool"
	toolmanage "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/tool/manage"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/application/convertor"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity/toolmgmt"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/consts"
	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func NewToolManageApplication(
	toolRepo repo.IToolRepo,
	authRPCProvider rpc.IAuthProvider,
	userRPCProvider rpc.IUserProvider,
) toolmanage.ToolManageService {
	return &ToolManageApplicationImpl{
		toolRepo:        toolRepo,
		authRPCProvider: authRPCProvider,
		userRPCProvider: userRPCProvider,
	}
}

type ToolManageApplicationImpl struct {
	toolRepo        repo.IToolRepo
	authRPCProvider rpc.IAuthProvider
	userRPCProvider rpc.IUserProvider
}

func (app *ToolManageApplicationImpl) CreateTool(ctx context.Context, request *toolmanage.CreateToolRequest) (r *toolmanage.CreateToolResponse, err error) {
	r = toolmanage.NewCreateToolResponse()

	userID, ok := session.UserIDInCtx(ctx)
	if !ok || lo.IsEmpty(userID) {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found"))
	}

	if request.GetWorkspaceID() <= 0 {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("Workspace ID is required"))
	}

	err = app.authRPCProvider.CheckSpacePermission(ctx, request.GetWorkspaceID(), consts.ActionWorkspaceCreateLoopPrompt)
	if err != nil {
		return r, err
	}

	toolDO := &toolmgmt.Tool{
		SpaceID: request.GetWorkspaceID(),
		ToolBasic: &toolmgmt.ToolBasic{
			Name:        request.GetToolName(),
			Description: request.GetToolDescription(),
			CreatedBy:   userID,
			UpdatedBy:   userID,
		},
		ToolCommit: &toolmgmt.ToolCommit{
			ToolDetail: &toolmgmt.ToolDetail{
				Content: func() string {
					if request.DraftDetail == nil {
						return ""
					}
					return request.DraftDetail.GetContent()
				}(),
			},
			CommitInfo: &toolmgmt.CommitInfo{
				Version:     toolmgmt.PublicDraftVersion,
				BaseVersion: "",
				Description: "",
				CommittedBy: userID,
			},
		},
	}

	toolID, err := app.toolRepo.CreateTool(ctx, toolDO)
	if err != nil {
		return r, err
	}
	r.ToolID = ptr.Of(toolID)
	return r, nil
}

func (app *ToolManageApplicationImpl) GetToolDetail(ctx context.Context, request *toolmanage.GetToolDetailRequest) (r *toolmanage.GetToolDetailResponse, err error) {
	r = toolmanage.NewGetToolDetailResponse()

	_, ok := session.UserIDInCtx(ctx)
	if !ok {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found"))
	}

	if request.GetToolID() <= 0 {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("Tool ID is required"))
	}
	if request.GetWorkspaceID() <= 0 {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("Workspace ID is required"))
	}
	err = app.authRPCProvider.CheckSpacePermission(ctx, request.GetWorkspaceID(), consts.ActionLoopPromptRead)
	if err != nil {
		return r, err
	}

	getParam := repo.GetToolParam{
		ToolID:        request.GetToolID(),
		WithCommit:    request.GetWithCommit(),
		WithDraft:     request.GetWithDraft(),
		CommitVersion: request.GetCommitVersion(),
	}
	toolDO, err := app.toolRepo.GetTool(ctx, getParam)
	if err != nil {
		return r, err
	}
	if toolDO == nil {
		return r, errorx.NewByCode(prompterr.ResourceNotFoundCode, errorx.WithExtraMsg(fmt.Sprintf("tool id = %d", request.GetToolID())))
	}
	if toolDO.SpaceID != request.GetWorkspaceID() {
		return r, errorx.NewByCode(prompterr.ResourceNotFoundCode, errorx.WithExtraMsg(fmt.Sprintf("tool id = %d", request.GetToolID())))
	}
	r.Tool = convertor.ToolMgmtDO2DTO(toolDO)
	return r, nil
}

func (app *ToolManageApplicationImpl) ListTool(ctx context.Context, request *toolmanage.ListToolRequest) (r *toolmanage.ListToolResponse, err error) {
	r = toolmanage.NewListToolResponse()

	_, ok := session.UserIDInCtx(ctx)
	if !ok {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found"))
	}

	if request.GetWorkspaceID() <= 0 {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("Workspace ID is required"))
	}
	if request.GetPageNum() <= 0 || request.GetPageSize() <= 0 {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("PageNum or PageSize is invalid"))
	}

	err = app.authRPCProvider.CheckSpacePermission(ctx, request.GetWorkspaceID(), consts.ActionWorkspaceListLoopPrompt)
	if err != nil {
		return r, err
	}

	listParam := repo.ListToolParam{
		SpaceID: request.GetWorkspaceID(),

		KeyWord:       request.GetKeyWord(),
		CreatedBys:    request.GetCreatedBys(),
		CommittedOnly: request.GetCommittedOnly(),

		PageNum:  int(request.GetPageNum()),
		PageSize: int(request.GetPageSize()),
		OrderBy:  app.listToolOrderBy(request.OrderBy),
		Asc:      request.GetAsc(),
	}
	listResult, err := app.toolRepo.ListTool(ctx, listParam)
	if err != nil {
		return r, err
	}
	if listResult == nil {
		return r, nil
	}
	r.Total = ptr.Of(int32(listResult.Total))
	r.Tools = convertor.BatchToolMgmtDO2DTO(listResult.Tools)

	userIDSet := make(map[string]struct{})
	for _, toolDTO := range r.Tools {
		if toolDTO == nil || toolDTO.ToolBasic == nil || lo.IsEmpty(toolDTO.ToolBasic.GetCreatedBy()) {
			continue
		}
		userIDSet[toolDTO.ToolBasic.GetCreatedBy()] = struct{}{}
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

func (app *ToolManageApplicationImpl) SaveToolDetail(ctx context.Context, request *toolmanage.SaveToolDetailRequest) (r *toolmanage.SaveToolDetailResponse, err error) {
	r = toolmanage.NewSaveToolDetailResponse()

	userID, ok := session.UserIDInCtx(ctx)
	if !ok || lo.IsEmpty(userID) {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found"))
	}

	if request.GetToolID() <= 0 || request.ToolDetail == nil {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("Tool ID or ToolDetail is required"))
	}
	if request.GetWorkspaceID() <= 0 {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("Workspace ID is required"))
	}
	err = app.authRPCProvider.CheckSpacePermission(ctx, request.GetWorkspaceID(), consts.ActionLoopPromptEdit)
	if err != nil {
		return r, err
	}

	toolDO, err := app.toolRepo.GetTool(ctx, repo.GetToolParam{ToolID: request.GetToolID()})
	if err != nil {
		return r, err
	}
	if toolDO == nil {
		return r, errorx.NewByCode(prompterr.ResourceNotFoundCode, errorx.WithExtraMsg(fmt.Sprintf("tool id = %d", request.GetToolID())))
	}
	if toolDO.SpaceID != request.GetWorkspaceID() {
		return r, errorx.NewByCode(prompterr.ResourceNotFoundCode, errorx.WithExtraMsg(fmt.Sprintf("tool id = %d", request.GetToolID())))
	}

	err = app.toolRepo.SaveToolDetail(ctx, repo.SaveToolDetailParam{
		ToolID:      request.GetToolID(),
		BaseVersion: request.GetBaseVersion(),
		Content:     request.ToolDetail.GetContent(),
		UpdatedBy:   userID,
	})
	if err != nil {
		return r, err
	}
	return r, nil
}

func (app *ToolManageApplicationImpl) CommitToolDraft(ctx context.Context, request *toolmanage.CommitToolDraftRequest) (r *toolmanage.CommitToolDraftResponse, err error) {
	r = toolmanage.NewCommitToolDraftResponse()

	userID, ok := session.UserIDInCtx(ctx)
	if !ok || lo.IsEmpty(userID) {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found"))
	}

	_, err = semver.StrictNewVersion(request.GetCommitVersion())
	if err != nil {
		return r, err
	}

	if request.GetToolID() <= 0 {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("Tool ID is required"))
	}
	if request.GetWorkspaceID() <= 0 {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("Workspace ID is required"))
	}
	err = app.authRPCProvider.CheckSpacePermission(ctx, request.GetWorkspaceID(), consts.ActionLoopPromptEdit)
	if err != nil {
		return r, err
	}

	toolDO, err := app.toolRepo.GetTool(ctx, repo.GetToolParam{ToolID: request.GetToolID()})
	if err != nil {
		return r, err
	}
	if toolDO == nil {
		return r, errorx.NewByCode(prompterr.ResourceNotFoundCode, errorx.WithExtraMsg(fmt.Sprintf("tool id = %d", request.GetToolID())))
	}
	if toolDO.SpaceID != request.GetWorkspaceID() {
		return r, errorx.NewByCode(prompterr.ResourceNotFoundCode, errorx.WithExtraMsg(fmt.Sprintf("tool id = %d", request.GetToolID())))
	}

	err = app.toolRepo.CommitToolDraft(ctx, repo.CommitToolDraftParam{
		ToolID:            request.GetToolID(),
		CommitVersion:     request.GetCommitVersion(),
		CommitDescription: request.GetCommitDescription(),
		BaseVersion:       request.GetBaseVersion(),
		CommittedBy:       userID,
	})
	if err != nil {
		return r, err
	}
	return r, nil
}

func (app *ToolManageApplicationImpl) ListToolCommit(ctx context.Context, request *toolmanage.ListToolCommitRequest) (r *toolmanage.ListToolCommitResponse, err error) {
	r = toolmanage.NewListToolCommitResponse()

	_, ok := session.UserIDInCtx(ctx)
	if !ok {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found"))
	}

	if request.GetToolID() <= 0 {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("Tool ID is required"))
	}
	if request.GetWorkspaceID() <= 0 {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("Workspace ID is required"))
	}
	err = app.authRPCProvider.CheckSpacePermission(ctx, request.GetWorkspaceID(), consts.ActionLoopPromptRead)
	if err != nil {
		return r, err
	}

	toolDO, err := app.toolRepo.GetTool(ctx, repo.GetToolParam{ToolID: request.GetToolID()})
	if err != nil {
		return r, err
	}
	if toolDO == nil {
		return r, errorx.NewByCode(prompterr.ResourceNotFoundCode, errorx.WithExtraMsg(fmt.Sprintf("tool id = %d", request.GetToolID())))
	}
	if toolDO.SpaceID != request.GetWorkspaceID() {
		return r, errorx.NewByCode(prompterr.ResourceNotFoundCode, errorx.WithExtraMsg(fmt.Sprintf("tool id = %d", request.GetToolID())))
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

	listResult, err := app.toolRepo.ListToolCommit(ctx, repo.ListToolCommitParam{
		ToolID:           request.GetToolID(),
		PageSize:         int(request.GetPageSize()),
		PageToken:        pageTokenPtr,
		Asc:              request.GetAsc(),
		WithCommitDetail: request.GetWithCommitDetail(),
	})
	if err != nil {
		return r, err
	}
	if listResult == nil {
		return r, nil
	}
	if listResult.NextPageToken > 0 {
		r.NextPageToken = ptr.Of(strconv.FormatInt(listResult.NextPageToken, 10))
		r.HasMore = ptr.Of(true)
	}

	r.ToolCommitInfos = make([]*tool.CommitInfo, 0, len(listResult.CommitInfos))
	userIDSet := make(map[string]struct{})
	for _, ci := range listResult.CommitInfos {
		if ci == nil {
			continue
		}
		r.ToolCommitInfos = append(r.ToolCommitInfos, convertor.ToolMgmtCommitInfoDO2DTO(ci))
		if lo.IsNotEmpty(ci.CommittedBy) {
			userIDSet[ci.CommittedBy] = struct{}{}
		}
	}
	if request.GetWithCommitDetail() && len(listResult.CommitDetails) > 0 {
		mapping := make(map[string]*tool.ToolDetail, len(listResult.CommitDetails))
		for version, detail := range listResult.CommitDetails {
			mapping[version] = convertor.ToolMgmtDetailDO2DTO(detail)
		}
		r.ToolCommitDetailMapping = mapping
	}

	userDOs, err := app.userRPCProvider.MGetUserInfo(ctx, maps.Keys(userIDSet))
	if err != nil {
		return toolmanage.NewListToolCommitResponse(), err
	}
	r.Users = convertor.BatchUserInfoDO2DTO(userDOs)
	return r, nil
}

func (app *ToolManageApplicationImpl) BatchGetTools(ctx context.Context, request *toolmanage.BatchGetToolsRequest) (r *toolmanage.BatchGetToolsResponse, err error) {
	r = toolmanage.NewBatchGetToolsResponse()

	_, ok := session.UserIDInCtx(ctx)
	if !ok {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found"))
	}

	if request.GetWorkspaceID() <= 0 {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("Workspace ID is required"))
	}
	if len(request.GetQueries()) == 0 {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("Queries is required"))
	}

	err = app.authRPCProvider.CheckSpacePermission(ctx, request.GetWorkspaceID(), consts.ActionLoopPromptRead)
	if err != nil {
		return r, err
	}

	queries := make([]repo.BatchGetToolsQuery, 0, len(request.GetQueries()))
	for _, q := range request.GetQueries() {
		if q == nil {
			continue
		}
		queries = append(queries, repo.BatchGetToolsQuery{
			ToolID:  q.GetToolID(),
			Version: q.GetVersion(),
		})
	}

	results, err := app.toolRepo.BatchGetTools(ctx, repo.BatchGetToolsParam{
		SpaceID: request.GetWorkspaceID(),
		Queries: queries,
	})
	if err != nil {
		return r, err
	}

	items := make([]*toolmanage.ToolResult_, 0, len(results))
	for _, result := range results {
		if result == nil || result.Tool == nil {
			continue
		}
		items = append(items, &toolmanage.ToolResult_{
			Query: &toolmanage.ToolQuery{
				ToolID:  ptr.Of(result.Query.ToolID),
				Version: ptr.Of(result.Query.Version),
			},
			Tool: convertor.ToolMgmtDO2DTO(result.Tool),
		})
	}
	r.Items = items
	return r, nil
}

func (app *ToolManageApplicationImpl) listToolOrderBy(orderBy *toolmanage.ListToolOrderBy) int {
	if orderBy == nil {
		return mysql.ListToolBasicOrderByID
	}
	switch *orderBy {
	case toolmanage.ListToolOrderByCommittedAt:
		return mysql.ListToolBasicOrderByCommittedAt
	case toolmanage.ListToolOrderByCreatedAt:
		return mysql.ListToolBasicOrderByCreatedAt
	default:
		return mysql.ListToolBasicOrderByID
	}
}
