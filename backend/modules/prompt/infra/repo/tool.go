// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql/convertor"
	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
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

func (r *ToolRepoImpl) CreateTool(ctx context.Context, toolDO *entity.CommonTool) (toolID int64, err error) {
	if toolDO == nil || toolDO.ToolBasic == nil {
		return 0, errorx.New("toolDO or toolDO.ToolBasic is empty")
	}

	basicPO := convertor.ToolDO2BasicPO(toolDO)

	err = r.db.Transaction(ctx, func(tx *gorm.DB) error {
		opt := db.WithTransaction(tx)

		if err := r.toolBasicDAO.Create(ctx, basicPO, opt); err != nil {
			return err
		}

		// 如果有初始草稿，创建草稿记录
		if toolDO.ToolCommit != nil {
			commitPO := convertor.ToolDO2CommitPO(toolDO, basicPO.ID, basicPO.SpaceID)
			if commitPO != nil {
				commitPO.Version = entity.ToolPublicDraftVersion
				if err := r.toolCommitDAO.Upsert(ctx, commitPO, opt); err != nil {
					return err
				}
			}
		}

		return nil
	})
	if err != nil {
		return 0, err
	}

	return basicPO.ID, nil
}

func (r *ToolRepoImpl) GetTool(ctx context.Context, param repo.GetToolParam) (*entity.CommonTool, error) {
	basicPO, err := r.toolBasicDAO.Get(ctx, param.ToolID)
	if err != nil {
		return nil, err
	}
	if basicPO == nil {
		return nil, errorx.NewByCode(prompterr.ResourceNotFoundCode)
	}

	if param.WithDraft {
		// 获取草稿
		draftPO, err := r.toolCommitDAO.Get(ctx, param.ToolID, entity.ToolPublicDraftVersion)
		if err != nil {
			return nil, err
		}
		return convertor.ToolPO2DO(basicPO, draftPO), nil
	}

	if param.WithCommit && param.CommitVersion != "" {
		// 获取指定版本
		versionPO, err := r.toolCommitDAO.Get(ctx, param.ToolID, param.CommitVersion)
		if err != nil {
			return nil, err
		}
		return convertor.ToolPO2DO(basicPO, versionPO), nil
	}

	return convertor.ToolPO2DO(basicPO, nil), nil
}

func (r *ToolRepoImpl) MGetTool(ctx context.Context, queries []repo.MGetToolQuery) (map[repo.MGetToolQuery]*entity.CommonTool, error) {
	if len(queries) == 0 {
		return nil, nil
	}

	// 收集所有 toolID
	toolIDSet := make(map[int64]struct{})
	for _, q := range queries {
		toolIDSet[q.ToolID] = struct{}{}
	}
	toolIDs := make([]int64, 0, len(toolIDSet))
	for id := range toolIDSet {
		toolIDs = append(toolIDs, id)
	}

	// 批量获取 basic
	basicMap, err := r.toolBasicDAO.MGet(ctx, toolIDs)
	if err != nil {
		return nil, err
	}

	// 构建 commit 查询列表
	commitPairs := make([]mysql.ToolIDVersionPair, 0, len(queries))
	for _, q := range queries {
		version := q.Version
		if version == "" {
			// 空版本则取最新提交版本
			if basic, ok := basicMap[q.ToolID]; ok && basic.LatestCommittedVersion != nil && *basic.LatestCommittedVersion != "" {
				version = *basic.LatestCommittedVersion
			} else {
				continue
			}
		}
		commitPairs = append(commitPairs, mysql.ToolIDVersionPair{
			ToolID:  q.ToolID,
			Version: version,
		})
	}

	// 批量获取 commit
	commitResultMap, err := r.toolCommitDAO.MGet(ctx, commitPairs)
	if err != nil {
		return nil, err
	}

	// 组装结果
	result := make(map[repo.MGetToolQuery]*entity.CommonTool, len(queries))
	for _, q := range queries {
		basic, ok := basicMap[q.ToolID]
		if !ok {
			continue
		}
		version := q.Version
		if version == "" && basic.LatestCommittedVersion != nil {
			version = *basic.LatestCommittedVersion
		}
		commitPO := commitResultMap[mysql.ToolIDVersionPair{ToolID: q.ToolID, Version: version}]
		result[q] = convertor.ToolPO2DO(basic, commitPO)
	}

	return result, nil
}

func (r *ToolRepoImpl) ListTool(ctx context.Context, param repo.ListToolParam) (*repo.ListToolResult, error) {
	daoParam := mysql.ListToolBasicParam{
		SpaceID:       param.SpaceID,
		KeyWord:       param.KeyWord,
		CreatedBys:    param.CreatedBys,
		CommittedOnly: param.CommittedOnly,
		Offset:        (param.PageNum - 1) * param.PageSize,
		Limit:         param.PageSize,
		Asc:           param.Asc,
	}

	switch param.OrderBy {
	case repo.ListToolOrderByCommittedAt:
		daoParam.OrderBy = mysql.ListToolBasicOrderByLatestCommittedAt
	default:
		daoParam.OrderBy = mysql.ListToolBasicOrderByCreatedAt
	}

	basicPOs, total, err := r.toolBasicDAO.List(ctx, daoParam)
	if err != nil {
		return nil, err
	}

	toolDOs := convertor.BatchBasicPO2ToolDO(basicPOs)

	return &repo.ListToolResult{
		Total:   total,
		ToolDOs: toolDOs,
	}, nil
}

func (r *ToolRepoImpl) SaveDraft(ctx context.Context, toolDO *entity.CommonTool) error {
	if toolDO == nil || toolDO.ToolCommit == nil {
		return errorx.New("toolDO or toolDO.ToolCommit is empty")
	}

	commitPO := convertor.ToolDO2CommitPO(toolDO, toolDO.ID, toolDO.SpaceID)
	if commitPO == nil {
		return errorx.New("failed to convert toolDO to commitPO")
	}
	commitPO.Version = entity.ToolPublicDraftVersion

	return r.toolCommitDAO.Upsert(ctx, commitPO)
}

func (r *ToolRepoImpl) CommitDraft(ctx context.Context, param repo.CommitToolDraftParam) error {
	return r.db.Transaction(ctx, func(tx *gorm.DB) error {
		opt := db.WithTransaction(tx)

		// 获取当前草稿
		draftPO, err := r.toolCommitDAO.Get(ctx, param.ToolID, entity.ToolPublicDraftVersion, opt)
		if err != nil {
			return err
		}

		// 创建新的 commit 记录
		now := time.Now()
		var content *string
		if draftPO != nil {
			content = draftPO.Content
		}
		commitPO := convertor.ToolDO2CommitPO(&entity.CommonTool{
			ToolCommit: &entity.CommonToolCommit{
				ToolDetail: &entity.CommonToolDetail{
					Content: func() string {
						if content != nil {
							return *content
						}
						return ""
					}(),
				},
				CommitInfo: &entity.CommonToolCommitInfo{
					Version:     param.CommitVersion,
					BaseVersion: param.BaseVersion,
					Description: param.CommitDescription,
					CommittedBy: param.CommittedBy,
				},
			},
		}, param.ToolID, param.SpaceID)

		if err := r.toolCommitDAO.Create(ctx, commitPO, now, opt); err != nil {
			return err
		}

		// 删除草稿（如果存在）
		if draftPO != nil {
			if err := r.toolCommitDAO.Delete(ctx, param.ToolID, entity.ToolPublicDraftVersion, opt); err != nil {
				return err
			}
		}

		// 更新 basic 的 latest_committed_version
		updateFields := map[string]interface{}{
			"latest_committed_version": param.CommitVersion,
			"updated_by":              param.CommittedBy,
		}
		if err := r.toolBasicDAO.Update(ctx, param.ToolID, updateFields, opt); err != nil {
			return err
		}

		return nil
	})
}

func (r *ToolRepoImpl) ListToolCommitInfo(ctx context.Context, param repo.ListToolCommitParam) (*repo.ListToolCommitResult, error) {
	daoParam := mysql.ListToolCommitDAOParam{
		ToolID: param.ToolID,
		Cursor: param.PageToken,
		Limit:  param.PageSize + 1,
		Asc:    param.Asc,
	}

	commitPOs, err := r.toolCommitDAO.List(ctx, daoParam)
	if err != nil {
		return nil, err
	}

	hasMore := len(commitPOs) > param.PageSize
	if hasMore {
		commitPOs = commitPOs[:param.PageSize]
	}

	commitInfos := convertor.BatchCommitInfoDOFromCommitPO(commitPOs)

	// 构建 detail mapping
	var detailMapping map[string]*entity.CommonToolDetail
	if param.WithCommitDetail && len(commitPOs) > 0 {
		detailMapping = make(map[string]*entity.CommonToolDetail, len(commitPOs))
		for _, po := range commitPOs {
			detailMapping[po.Version] = convertor.ToolDetailDOFromCommitPO(po)
		}
	}

	var nextPageToken int64
	if hasMore && len(commitPOs) > 0 {
		lastPO := commitPOs[len(commitPOs)-1]
		nextPageToken = lastPO.CreatedAt.UnixMilli()
	}

	return &repo.ListToolCommitResult{
		CommitInfoDOs:       commitInfos,
		CommitDetailMapping: detailMapping,
		NextPageToken:       nextPageToken,
		HasMore:             hasMore,
	}, nil
}

