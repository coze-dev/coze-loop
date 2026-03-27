package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/samber/lo"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity/toolmgmt"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql/convertor"
	mysqlmodel "github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql/gorm_gen/model"
	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

type ToolRepoImpl struct {
	db    db.Provider
	idgen idgen.IIDGenerator

	toolBasicDAO  mysql.IToolBasicDAO
	toolCommitDAO mysql.IToolCommitDAO
}

func NewToolRepo(
	db db.Provider,
	idgen idgen.IIDGenerator,
	toolBasicDAO mysql.IToolBasicDAO,
	toolCommitDAO mysql.IToolCommitDAO,
) repo.IToolRepo {
	return &ToolRepoImpl{
		db:            db,
		idgen:         idgen,
		toolBasicDAO:  toolBasicDAO,
		toolCommitDAO: toolCommitDAO,
	}
}

func (d *ToolRepoImpl) CreateTool(ctx context.Context, tool *toolmgmt.Tool) (toolID int64, err error) {
	if tool == nil || tool.ToolBasic == nil {
		return 0, errorx.New("tool or tool.ToolBasic is empty")
	}
	toolID, err = d.idgen.GenID(ctx)
	if err != nil {
		return 0, err
	}
	return toolID, d.db.Transaction(ctx, func(tx *gorm.DB) error {
		opt := db.WithTransaction(tx)

		basicPO := convertor.ToolDO2BasicPO(tool)
		basicPO.ID = toolID
		if err := d.toolBasicDAO.Create(ctx, basicPO, opt); err != nil {
			return err
		}

		var draftContent string
		if tool.ToolCommit != nil && tool.ToolCommit.ToolDetail != nil {
			draftContent = tool.ToolCommit.ToolDetail.Content
		}
		draftPO := &mysqlmodel.ToolCommit{
			SpaceID:     tool.SpaceID,
			ToolID:      toolID,
			Version:     toolmgmt.PublicDraftVersion,
			BaseVersion: "",
			CommittedBy: tool.ToolBasic.CreatedBy,
			Content:     lo.ToPtr(draftContent),
			Description: nil,
		}
		if err := d.toolCommitDAO.UpsertDraft(ctx, draftPO, time.Now(), opt); err != nil {
			return err
		}
		return nil
	})
}

func (d *ToolRepoImpl) DeleteTool(ctx context.Context, toolID int64) (err error) {
	if toolID <= 0 {
		return errorx.New("toolID is invalid, toolID = %d", toolID)
	}
	basicPO, err := d.toolBasicDAO.Get(ctx, toolID)
	if err != nil {
		return err
	}
	if basicPO == nil {
		return errorx.NewByCode(prompterr.ResourceNotFoundCode, errorx.WithExtraMsg(fmt.Sprintf("tool is not found, tool id = %d", toolID)))
	}
	return d.toolBasicDAO.Delete(ctx, toolID, basicPO.SpaceID)
}

func (d *ToolRepoImpl) GetTool(ctx context.Context, param repo.GetToolParam) (tool *toolmgmt.Tool, err error) {
	if param.ToolID <= 0 {
		return nil, errorx.New("param.ToolID is invalid, param = %s", json.Jsonify(param))
	}
	if param.WithCommit && lo.IsEmpty(param.CommitVersion) {
		return nil, errorx.New("Get with commit, but param.CommitVersion is empty, param = %s", json.Jsonify(param))
	}

	err = d.db.Transaction(ctx, func(tx *gorm.DB) error {
		opt := db.WithTransaction(tx)

		basicPO, err := d.toolBasicDAO.Get(ctx, param.ToolID, opt)
		if err != nil {
			return err
		}
		if basicPO == nil {
			return errorx.NewByCode(prompterr.ResourceNotFoundCode, errorx.WithExtraMsg(fmt.Sprintf("tool id = %d", param.ToolID)))
		}

		var commitPO *mysqlmodel.ToolCommit
		if param.WithCommit {
			commitPO, err = d.toolCommitDAO.Get(ctx, param.ToolID, param.CommitVersion, opt)
			if err != nil {
				return err
			}
			if commitPO == nil {
				return errorx.NewByCode(prompterr.ResourceNotFoundCode, errorx.WithExtraMsg(fmt.Sprintf("tool commit is not found, tool id = %d, version = %s", param.ToolID, param.CommitVersion)))
			}
		} else if param.WithDraft {
			commitPO, err = d.toolCommitDAO.Get(ctx, param.ToolID, toolmgmt.PublicDraftVersion, opt)
			if err != nil {
				return err
			}
		}
		tool = convertor.ToolPO2DO(basicPO, commitPO)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return tool, nil
}

func (d *ToolRepoImpl) ListTool(ctx context.Context, param repo.ListToolParam) (result *repo.ListToolResult, err error) {
	if param.SpaceID <= 0 || param.PageNum <= 0 || param.PageSize <= 0 {
		return nil, errorx.New("param is invalid, param = %s", json.Jsonify(param))
	}
	offset := (param.PageNum - 1) * param.PageSize
	basicPOs, total, err := d.toolBasicDAO.List(ctx, mysql.ListToolBasicParam{
		SpaceID:       param.SpaceID,
		KeyWord:       param.KeyWord,
		CreatedBys:    param.CreatedBys,
		CommittedOnly: param.CommittedOnly,
		Offset:        offset,
		Limit:         param.PageSize,
		OrderBy:       param.OrderBy,
		Asc:           param.Asc,
	})
	if err != nil {
		return nil, err
	}
	tools := make([]*toolmgmt.Tool, 0, len(basicPOs))
	for _, po := range basicPOs {
		if po == nil {
			continue
		}
		tools = append(tools, convertor.ToolPO2DO(po, nil))
	}
	return &repo.ListToolResult{
		Total: total,
		Tools: tools,
	}, nil
}

func (d *ToolRepoImpl) SaveToolDetail(ctx context.Context, param repo.SaveToolDetailParam) (err error) {
	if param.ToolID <= 0 {
		return errorx.New("param.ToolID is invalid, param = %s", json.Jsonify(param))
	}
	return d.db.Transaction(ctx, func(tx *gorm.DB) error {
		opt := db.WithTransaction(tx)

		basicPO, err := d.toolBasicDAO.Get(ctx, param.ToolID, opt)
		if err != nil {
			return err
		}
		if basicPO == nil {
			return errorx.NewByCode(prompterr.ResourceNotFoundCode, errorx.WithExtraMsg(fmt.Sprintf("tool id = %d", param.ToolID)))
		}

		if err := d.toolCommitDAO.UpsertDraft(ctx, &mysqlmodel.ToolCommit{
			SpaceID:     basicPO.SpaceID,
			ToolID:      param.ToolID,
			Version:     toolmgmt.PublicDraftVersion,
			BaseVersion: param.BaseVersion,
			CommittedBy: param.UpdatedBy,
			Content:     lo.ToPtr(param.Content),
			Description: nil,
		}, time.Now(), opt); err != nil {
			return err
		}

		if !lo.IsEmpty(param.UpdatedBy) {
			if err := d.toolBasicDAO.Update(ctx, param.ToolID, map[string]interface{}{
				"updated_by": param.UpdatedBy,
			}, opt); err != nil {
				return err
			}
		}
		return nil
	})
}

func (d *ToolRepoImpl) CommitToolDraft(ctx context.Context, param repo.CommitToolDraftParam) (err error) {
	if param.ToolID <= 0 || lo.IsEmpty(param.CommitVersion) {
		return errorx.New("param is invalid, param = %s", json.Jsonify(param))
	}
	if param.CommitVersion == toolmgmt.PublicDraftVersion {
		return errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("commit_version is invalid"))
	}

	return d.db.Transaction(ctx, func(tx *gorm.DB) error {
		opt := db.WithTransaction(tx)

		basicPO, err := d.toolBasicDAO.Get(ctx, param.ToolID, opt)
		if err != nil {
			return err
		}
		if basicPO == nil {
			return errorx.NewByCode(prompterr.ResourceNotFoundCode, errorx.WithExtraMsg(fmt.Sprintf("tool id = %d", param.ToolID)))
		}

		draftPO, err := d.toolCommitDAO.Get(ctx, param.ToolID, toolmgmt.PublicDraftVersion, opt)
		if err != nil {
			return err
		}
		if draftPO == nil {
			return errorx.NewByCode(prompterr.ResourceNotFoundCode, errorx.WithExtraMsg(fmt.Sprintf("tool draft is not found, tool id = %d", param.ToolID)))
		}

		commitPO := &mysqlmodel.ToolCommit{
			SpaceID:     basicPO.SpaceID,
			ToolID:      param.ToolID,
			Version:     param.CommitVersion,
			BaseVersion: param.BaseVersion,
			CommittedBy: param.CommittedBy,
			Content:     draftPO.Content,
			Description: lo.ToPtr(param.CommitDescription),
		}
		if err := d.toolCommitDAO.Create(ctx, commitPO, time.Now(), opt); err != nil {
			return err
		}
		if err := d.toolBasicDAO.Update(ctx, param.ToolID, map[string]interface{}{
			"latest_committed_version": param.CommitVersion,
			"latest_committed_at":      time.Now(),
			"updated_by":               param.CommittedBy,
		}, opt); err != nil {
			return err
		}
		return nil
	})
}

func (d *ToolRepoImpl) BatchGetTools(ctx context.Context, param repo.BatchGetToolsParam) (result []*repo.BatchGetToolsResult, err error) {
	if len(param.Queries) == 0 {
		return nil, nil
	}

	toolIDSet := make(map[int64]struct{}, len(param.Queries))
	for _, q := range param.Queries {
		toolIDSet[q.ToolID] = struct{}{}
	}
	toolIDs := make([]int64, 0, len(toolIDSet))
	for id := range toolIDSet {
		toolIDs = append(toolIDs, id)
	}

	basicPOs, err := d.toolBasicDAO.BatchGet(ctx, toolIDs)
	if err != nil {
		return nil, err
	}
	basicMap := make(map[int64]*mysqlmodel.ToolBasic, len(basicPOs))
	for _, po := range basicPOs {
		if po == nil {
			continue
		}
		basicMap[po.ID] = po
	}

	var commitPairs []mysql.ToolIDVersionPair
	for _, q := range param.Queries {
		basicPO, ok := basicMap[q.ToolID]
		if !ok {
			continue
		}
		version := q.Version
		if lo.IsEmpty(version) {
			version = basicPO.LatestCommittedVersion
		}
		if lo.IsEmpty(version) {
			continue
		}
		commitPairs = append(commitPairs, mysql.ToolIDVersionPair{ToolID: q.ToolID, Version: version})
	}

	commitMap := make(map[int64]*mysqlmodel.ToolCommit)
	if len(commitPairs) > 0 {
		commitPOs, err := d.toolCommitDAO.BatchGet(ctx, commitPairs)
		if err != nil {
			return nil, err
		}
		for _, po := range commitPOs {
			if po == nil {
				continue
			}
			commitMap[po.ToolID] = po
		}
	}

	result = make([]*repo.BatchGetToolsResult, 0, len(param.Queries))
	for _, q := range param.Queries {
		basicPO, ok := basicMap[q.ToolID]
		if !ok || (param.SpaceID > 0 && basicPO.SpaceID != param.SpaceID) {
			continue
		}
		tool := convertor.ToolPO2DO(basicPO, commitMap[q.ToolID])
		result = append(result, &repo.BatchGetToolsResult{
			Query: q,
			Tool:  tool,
		})
	}
	return result, nil
}

func (d *ToolRepoImpl) ListToolCommit(ctx context.Context, param repo.ListToolCommitParam) (result *repo.ListToolCommitResult, err error) {
	if param.ToolID <= 0 || param.PageSize <= 0 {
		return nil, errorx.New("param is invalid, param = %s", json.Jsonify(param))
	}
	commitPOs, err := d.toolCommitDAO.List(ctx, mysql.ListToolCommitParam{
		ToolID:         param.ToolID,
		Cursor:         param.PageToken,
		Limit:          param.PageSize,
		Asc:            param.Asc,
		ExcludeVersion: toolmgmt.PublicDraftVersion,
	})
	if err != nil {
		return nil, err
	}
	commitInfos := make([]*toolmgmt.CommitInfo, 0, len(commitPOs))
	var commitDetails map[string]*toolmgmt.ToolDetail
	if param.WithCommitDetail {
		commitDetails = make(map[string]*toolmgmt.ToolDetail, len(commitPOs))
	}
	var nextToken int64
	for _, po := range commitPOs {
		if po == nil {
			continue
		}
		nextToken = po.ID
		commitInfos = append(commitInfos, &toolmgmt.CommitInfo{
			Version:     po.Version,
			BaseVersion: po.BaseVersion,
			Description: lo.FromPtr(po.Description),
			CommittedBy: po.CommittedBy,
			CommittedAt: po.CreatedAt,
		})
		if param.WithCommitDetail {
			commitDetails[po.Version] = &toolmgmt.ToolDetail{
				Content: lo.FromPtr(po.Content),
			}
		}
	}
	return &repo.ListToolCommitResult{
		CommitInfos:   commitInfos,
		CommitDetails: commitDetails,
		NextPageToken: nextToken,
	}, nil
}
