// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql/gorm_gen/query"
	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

type ToolCommitQuery struct {
	ToolID  int64
	Version string
}

type IToolCommitDAO interface {
	Create(ctx context.Context, commitPO *model.ToolCommit, opts ...db.Option) error
	Get(ctx context.Context, toolID int64, version string, opts ...db.Option) (*model.ToolCommit, error)
	GetLatestCommit(ctx context.Context, toolID int64, opts ...db.Option) (*model.ToolCommit, error)
	MGet(ctx context.Context, queries []ToolCommitQuery, opts ...db.Option) (map[ToolCommitQuery]*model.ToolCommit, error)
	List(ctx context.Context, toolID int64, pageSize int, pageToken *int64, asc bool, opts ...db.Option) ([]*model.ToolCommit, error)
	Delete(ctx context.Context, toolID int64, version string, opts ...db.Option) error
	Upsert(ctx context.Context, commitPO *model.ToolCommit, opts ...db.Option) error
}

func NewToolCommitDAO(db db.Provider) IToolCommitDAO {
	return &ToolCommitDAOImpl{db: db}
}

type ToolCommitDAOImpl struct {
	db db.Provider
}

func (d *ToolCommitDAOImpl) Create(ctx context.Context, commitPO *model.ToolCommit, opts ...db.Option) error {
	if commitPO == nil {
		return errorx.New("commitPO is empty")
	}
	q := query.Use(d.db.NewSession(ctx, opts...)).WithContext(ctx)
	err := q.ToolCommit.Create(commitPO)
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return errorx.WrapByCode(err, prompterr.PromptSubmitVersionExistCode)
		}
		return errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	return nil
}

func (d *ToolCommitDAOImpl) Get(ctx context.Context, toolID int64, version string, opts ...db.Option) (*model.ToolCommit, error) {
	q := query.Use(d.db.NewSession(ctx, opts...))
	commitPO, err := q.WithContext(ctx).ToolCommit.Where(
		q.ToolCommit.ToolID.Eq(toolID),
		q.ToolCommit.Version.Eq(version),
	).First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	return commitPO, nil
}

func (d *ToolCommitDAOImpl) GetLatestCommit(ctx context.Context, toolID int64, opts ...db.Option) (*model.ToolCommit, error) {
	q := query.Use(d.db.NewSession(ctx, opts...))
	commitPO, err := q.WithContext(ctx).ToolCommit.Where(
		q.ToolCommit.ToolID.Eq(toolID),
		q.ToolCommit.Version.Neq(entity.ToolPublicDraftVersion),
	).Order(q.ToolCommit.CreatedAt.Desc()).First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	return commitPO, nil
}

func (d *ToolCommitDAOImpl) MGet(ctx context.Context, queries []ToolCommitQuery, opts ...db.Option) (map[ToolCommitQuery]*model.ToolCommit, error) {
	if len(queries) == 0 {
		return nil, nil
	}

	toolIDs := make([]int64, 0, len(queries))
	for _, cq := range queries {
		toolIDs = append(toolIDs, cq.ToolID)
	}

	q := query.Use(d.db.NewSession(ctx, opts...))
	commitPOs, err := q.WithContext(ctx).ToolCommit.Where(q.ToolCommit.ToolID.In(toolIDs...)).Find()
	if err != nil {
		return nil, errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}

	indexed := make(map[ToolCommitQuery]*model.ToolCommit, len(commitPOs))
	for _, po := range commitPOs {
		indexed[ToolCommitQuery{ToolID: po.ToolID, Version: po.Version}] = po
	}

	result := make(map[ToolCommitQuery]*model.ToolCommit, len(queries))
	for _, cq := range queries {
		if po, ok := indexed[cq]; ok {
			result[cq] = po
		}
	}
	return result, nil
}

func (d *ToolCommitDAOImpl) List(ctx context.Context, toolID int64, pageSize int, pageToken *int64, asc bool, opts ...db.Option) ([]*model.ToolCommit, error) {
	q := query.Use(d.db.NewSession(ctx, opts...))
	tx := q.WithContext(ctx).ToolCommit.Where(
		q.ToolCommit.ToolID.Eq(toolID),
		q.ToolCommit.Version.Neq(entity.ToolPublicDraftVersion),
	)

	if pageToken != nil {
		if asc {
			tx = tx.Where(q.ToolCommit.ID.Gt(*pageToken))
		} else {
			tx = tx.Where(q.ToolCommit.ID.Lt(*pageToken))
		}
	}

	if asc {
		tx = tx.Order(q.ToolCommit.ID.Asc())
	} else {
		tx = tx.Order(q.ToolCommit.ID.Desc())
	}

	commitPOs, err := tx.Limit(pageSize + 1).Find()
	if err != nil {
		return nil, errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	return commitPOs, nil
}

func (d *ToolCommitDAOImpl) Delete(ctx context.Context, toolID int64, version string, opts ...db.Option) error {
	q := query.Use(d.db.NewSession(ctx, opts...))
	_, err := q.WithContext(ctx).ToolCommit.Where(
		q.ToolCommit.ToolID.Eq(toolID),
		q.ToolCommit.Version.Eq(version),
	).Delete()
	if err != nil {
		return errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	return nil
}

func (d *ToolCommitDAOImpl) Upsert(ctx context.Context, commitPO *model.ToolCommit, opts ...db.Option) error {
	if commitPO == nil {
		return errorx.New("commitPO is empty")
	}
	sess := d.db.NewSession(ctx, opts...)
	err := sess.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tool_id"}, {Name: "version"}},
		DoUpdates: clause.AssignmentColumns([]string{"content", "base_version", "committed_by", "description"}),
	}).Create(commitPO).Error
	if err != nil {
		return errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	return nil
}
