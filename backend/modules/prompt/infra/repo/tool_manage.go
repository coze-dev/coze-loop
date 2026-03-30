// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql/convertor"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql/gorm_gen/model"
	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

type ToolManageRepoImpl struct {
	db    db.Provider
	idgen idgen.IIDGenerator

	toolBasicDAO  mysql.IToolBasicDAO
	toolCommitDAO mysql.IToolCommitDAO
}

func NewToolManageRepo(
	db db.Provider,
	idgen idgen.IIDGenerator,
	toolBasicDAO mysql.IToolBasicDAO,
	toolCommitDAO mysql.IToolCommitDAO,
) repo.IToolManageRepo {
	return &ToolManageRepoImpl{
		db:            db,
		idgen:         idgen,
		toolBasicDAO:  toolBasicDAO,
		toolCommitDAO: toolCommitDAO,
	}
}

func (d *ToolManageRepoImpl) CreateTool(ctx context.Context, toolDO *entity.CommonTool) (toolID int64, err error) {
	if toolDO == nil || toolDO.ToolBasic == nil {
		return 0, errorx.New("toolDO or toolDO.ToolBasic is empty")
	}

	toolID, err = d.idgen.GenID(ctx)
	if err != nil {
		return 0, err
	}

	return toolID, d.db.Transaction(ctx, func(tx *gorm.DB) error {
		opt := db.WithTransaction(tx)

		basicPO := convertor.ToolDO2BasicPO(toolDO)
		basicPO.ID = toolID
		if err := d.toolBasicDAO.Create(ctx, basicPO, opt); err != nil {
			return err
		}

		// Create initial draft if provided
		if toolDO.ToolCommit != nil {
			commitID, err := d.idgen.GenID(ctx)
			if err != nil {
				return err
			}
			commitPO := convertor.ToolDO2CommitPO(toolDO, toolID)
			commitPO.ID = commitID
			commitPO.Version = entity.ToolPublicDraftVersion
			if err := d.toolCommitDAO.Create(ctx, commitPO, opt); err != nil {
				return err
			}
		}

		return nil
	})
}

func (d *ToolManageRepoImpl) GetTool(ctx context.Context, param repo.GetToolParam) (toolDO *entity.CommonTool, err error) {
	if param.ToolID <= 0 {
		return nil, errorx.New("param.ToolID is invalid")
	}

	basicPO, err := d.toolBasicDAO.Get(ctx, param.ToolID)
	if err != nil {
		return nil, err
	}
	if basicPO == nil {
		return nil, errorx.NewByCode(prompterr.ResourceNotFoundCode,
			errorx.WithExtraMsg(fmt.Sprintf("tool id = %d", param.ToolID)))
	}

	var commitPO *model.ToolCommit
	if param.WithDraft {
		commitPO, err = d.toolCommitDAO.Get(ctx, param.ToolID, entity.ToolPublicDraftVersion)
		if err != nil {
			return nil, err
		}
	} else if param.WithCommit {
		version := param.CommitVersion
		if version == "" {
			if basicPO.LatestCommittedVersion != nil && *basicPO.LatestCommittedVersion != "" {
				version = *basicPO.LatestCommittedVersion
			}
		}
		if version != "" {
			commitPO, err = d.toolCommitDAO.Get(ctx, param.ToolID, version)
			if err != nil {
				return nil, err
			}
			if commitPO == nil {
				return nil, errorx.NewByCode(prompterr.PromptVersionNotExistCode,
					errorx.WithExtraMsg(fmt.Sprintf("tool id = %d, version = %s", param.ToolID, version)))
			}
		}
	}

	return convertor.ToolPO2DO(basicPO, commitPO), nil
}

func (d *ToolManageRepoImpl) MGetTool(ctx context.Context, queries []repo.GetToolParam) (map[repo.GetToolParam]*entity.CommonTool, error) {
	result := make(map[repo.GetToolParam]*entity.CommonTool)
	if len(queries) == 0 {
		return result, nil
	}

	toolIDs := make([]int64, 0, len(queries))
	for _, q := range queries {
		toolIDs = append(toolIDs, q.ToolID)
	}

	basicMap, err := d.toolBasicDAO.MGet(ctx, toolIDs)
	if err != nil {
		return nil, err
	}

	resolvedVersions := make(map[int64]string, len(queries))
	var commitQueries []mysql.ToolCommitQuery
	for _, q := range queries {
		basic, ok := basicMap[q.ToolID]
		if !ok {
			continue
		}
		version := resolveCommitVersion(q, basic)
		resolvedVersions[q.ToolID] = version
		if version != "" {
			commitQueries = append(commitQueries, mysql.ToolCommitQuery{
				ToolID:  q.ToolID,
				Version: version,
			})
		}
	}

	commitMap := make(map[mysql.ToolCommitQuery]*model.ToolCommit)
	if len(commitQueries) > 0 {
		commitMap, err = d.toolCommitDAO.MGet(ctx, commitQueries)
		if err != nil {
			return nil, err
		}
	}

	for _, q := range queries {
		basic, ok := basicMap[q.ToolID]
		if !ok {
			continue
		}
		version := resolvedVersions[q.ToolID]
		var commitPO *model.ToolCommit
		if version != "" {
			commitPO = commitMap[mysql.ToolCommitQuery{ToolID: q.ToolID, Version: version}]
		}
		result[q] = convertor.ToolPO2DO(basic, commitPO)
	}

	return result, nil
}

func resolveCommitVersion(q repo.GetToolParam, basic *model.ToolBasic) string {
	if q.CommitVersion != "" {
		return q.CommitVersion
	}
	if q.WithCommit && basic.LatestCommittedVersion != nil && *basic.LatestCommittedVersion != "" {
		return *basic.LatestCommittedVersion
	}
	return ""
}

func (d *ToolManageRepoImpl) ListTool(ctx context.Context, param repo.ListToolParam) (*repo.ListToolResult, error) {
	listParam := mysql.ListToolBasicParam{
		SpaceID:       param.SpaceID,
		KeyWord:       param.KeyWord,
		CreatedBys:    param.CreatedBys,
		CommittedOnly: param.CommittedOnly,
		Offset:        (param.PageNum - 1) * param.PageSize,
		Limit:         param.PageSize,
		OrderBy:       param.OrderBy,
		Asc:           param.Asc,
	}

	basicPOs, total, err := d.toolBasicDAO.List(ctx, listParam)
	if err != nil {
		return nil, err
	}

	return &repo.ListToolResult{
		Total:   total,
		ToolDOs: convertor.BatchToolBasicPO2DO(basicPOs),
	}, nil
}

func (d *ToolManageRepoImpl) SaveDraft(ctx context.Context, toolDO *entity.CommonTool) error {
	if toolDO == nil || toolDO.ToolCommit == nil {
		return errorx.New("toolDO or toolDO.ToolCommit is empty")
	}

	return d.db.Transaction(ctx, func(tx *gorm.DB) error {
		opt := db.WithTransaction(tx)

		// Verify tool exists
		basicPO, err := d.toolBasicDAO.Get(ctx, toolDO.ID, opt)
		if err != nil {
			return err
		}
		if basicPO == nil {
			return errorx.NewByCode(prompterr.ResourceNotFoundCode,
				errorx.WithExtraMsg(fmt.Sprintf("tool id = %d", toolDO.ID)))
		}

		// Upsert draft
		commitPO := convertor.ToolDO2CommitPO(toolDO, toolDO.ID)
		commitPO.SpaceID = basicPO.SpaceID
		commitPO.Version = entity.ToolPublicDraftVersion
		if err := d.toolCommitDAO.Upsert(ctx, commitPO, opt); err != nil {
			return err
		}

		// Update basic updated_by
		if toolDO.ToolCommit.CommitInfo != nil && toolDO.ToolCommit.CommitInfo.CommittedBy != "" {
			if err := d.toolBasicDAO.Update(ctx, toolDO.ID, map[string]interface{}{
				"updated_by": toolDO.ToolCommit.CommitInfo.CommittedBy,
			}, opt); err != nil {
				return err
			}
		}

		return nil
	})
}

func (d *ToolManageRepoImpl) CommitDraft(ctx context.Context, param repo.CommitToolDraftParam) error {
	return d.db.Transaction(ctx, func(tx *gorm.DB) error {
		opt := db.WithTransaction(tx)

		// Get basic with lock
		basicPO, err := d.toolBasicDAO.Get(ctx, param.ToolID, opt, db.WithSelectForUpdate())
		if err != nil {
			return err
		}
		if basicPO == nil {
			return errorx.NewByCode(prompterr.ResourceNotFoundCode,
				errorx.WithExtraMsg(fmt.Sprintf("tool id = %d", param.ToolID)))
		}

		// Get draft
		draftPO, err := d.toolCommitDAO.Get(ctx, param.ToolID, entity.ToolPublicDraftVersion, opt)
		if err != nil {
			return err
		}
		if draftPO == nil {
			return errorx.NewByCode(prompterr.ResourceNotFoundCode,
				errorx.WithExtraMsg(fmt.Sprintf("tool draft not found, tool id = %d", param.ToolID)))
		}

		// Create commit record
		commitID, err := d.idgen.GenID(ctx)
		if err != nil {
			return err
		}
		commitPO := &model.ToolCommit{
			ID:          commitID,
			SpaceID:     basicPO.SpaceID,
			ToolID:      param.ToolID,
			Content:     draftPO.Content,
			Version:     param.CommitVersion,
			BaseVersion: param.BaseVersion,
			CommittedBy: param.UserID,
			Description: ptr.Of(param.CommitDescription),
		}
		if err := d.toolCommitDAO.Create(ctx, commitPO, opt); err != nil {
			return err
		}

		// Delete draft
		if err := d.toolCommitDAO.Delete(ctx, param.ToolID, entity.ToolPublicDraftVersion, opt); err != nil {
			return err
		}

		// Update basic
		if err := d.toolBasicDAO.Update(ctx, param.ToolID, map[string]interface{}{
			"latest_committed_version": param.CommitVersion,
			"updated_by":              param.UserID,
		}, opt); err != nil {
			return err
		}

		return nil
	})
}

func (d *ToolManageRepoImpl) ListToolCommitInfo(ctx context.Context, param repo.ListToolCommitParam) (*repo.ListToolCommitResult, error) {
	commitPOs, err := d.toolCommitDAO.List(ctx, param.ToolID, param.PageSize, param.PageToken, param.Asc)
	if err != nil {
		return nil, err
	}

	result := &repo.ListToolCommitResult{}

	hasMore := len(commitPOs) > param.PageSize
	if hasMore {
		commitPOs = commitPOs[:param.PageSize]
	}

	for _, commitPO := range commitPOs {
		commitDO := convertor.ToolCommitPO2DO(commitPO)
		if commitDO == nil {
			continue
		}
		result.CommitInfoDOs = append(result.CommitInfoDOs, commitDO.CommitInfo)
		result.CommitDOs = append(result.CommitDOs, commitDO)
	}

	if hasMore && len(commitPOs) > 0 {
		lastPO := commitPOs[len(commitPOs)-1]
		result.NextPageToken = lastPO.ID
	}

	return result, nil
}

