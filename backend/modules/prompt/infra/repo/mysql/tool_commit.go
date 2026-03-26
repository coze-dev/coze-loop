// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"errors"
	"time"

	"gorm.io/gen/field"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql/gorm_gen/query"
	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

//go:generate mockgen -destination=mocks/tool_commit_dao.go -package=mocks . IToolCommitDAO
type IToolCommitDAO interface {
	Create(ctx context.Context, commitPO *model.ToolCommit, timeNow time.Time, opts ...db.Option) (err error)
	Get(ctx context.Context, toolID int64, version string, opts ...db.Option) (commitPO *model.ToolCommit, err error)
	MGet(ctx context.Context, pairs []ToolIDVersionPair, opts ...db.Option) (pairCommitPOMap map[ToolIDVersionPair]*model.ToolCommit, err error)
	Upsert(ctx context.Context, commitPO *model.ToolCommit, opts ...db.Option) error
	Delete(ctx context.Context, toolID int64, version string, opts ...db.Option) error
	List(ctx context.Context, param ListToolCommitDAOParam, opts ...db.Option) (commitPOs []*model.ToolCommit, err error)
}

type ToolIDVersionPair struct {
	ToolID  int64
	Version string
}

type ListToolCommitDAOParam struct {
	ToolID int64
	Cursor *int64
	Limit  int
	Asc    bool
}

func NewToolCommitDAO(db db.Provider) IToolCommitDAO {
	return &ToolCommitDAOImpl{db: db}
}

type ToolCommitDAOImpl struct {
	db db.Provider
}

func (d *ToolCommitDAOImpl) Create(ctx context.Context, commitPO *model.ToolCommit, timeNow time.Time, opts ...db.Option) (err error) {
	if commitPO == nil {
		return errorx.New("commitPO is empty")
	}
	q := query.Use(d.db.NewSession(ctx, opts...)).WithContext(ctx)
	commitPO.CreatedAt = timeNow
	commitPO.UpdatedAt = timeNow
	err = q.ToolCommit.Create(commitPO)
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return errorx.WrapByCode(err, prompterr.PromptSubmitVersionExistCode)
		}
		return errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	return nil
}

func (d *ToolCommitDAOImpl) Get(ctx context.Context, toolID int64, version string, opts ...db.Option) (commitPO *model.ToolCommit, err error) {
	if toolID <= 0 {
		return nil, errorx.New("toolID is invalid, toolID = %d", toolID)
	}

	q := query.Use(d.db.NewSession(ctx, opts...))
	tx := q.WithContext(ctx).ToolCommit
	tx = tx.Where(q.ToolCommit.ToolID.Eq(toolID))
	tx = tx.Where(q.ToolCommit.Version.Eq(version))
	commitPOs, err := tx.Find()
	if err != nil {
		return nil, errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	if len(commitPOs) <= 0 {
		return nil, nil
	}
	return commitPOs[0], nil
}

func (d *ToolCommitDAOImpl) MGet(ctx context.Context, pairs []ToolIDVersionPair, opts ...db.Option) (pairCommitPOMap map[ToolIDVersionPair]*model.ToolCommit, err error) {
	if len(pairs) <= 0 {
		return nil, nil
	}
	q := query.Use(d.db.NewSession(ctx, opts...))
	tx := q.WithContext(ctx).ToolCommit

	conditions := make([]field.Expr, 0, len(pairs))
	for _, pair := range pairs {
		conditions = append(conditions,
			field.And(q.ToolCommit.ToolID.Eq(pair.ToolID), q.ToolCommit.Version.Eq(pair.Version)))
	}
	tx = tx.Where(field.Or(conditions...))
	commitPOs, err := tx.Find()
	if err != nil {
		return nil, errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}

	result := make(map[ToolIDVersionPair]*model.ToolCommit, len(commitPOs))
	for _, po := range commitPOs {
		result[ToolIDVersionPair{ToolID: po.ToolID, Version: po.Version}] = po
	}
	return result, nil
}

func (d *ToolCommitDAOImpl) Upsert(ctx context.Context, commitPO *model.ToolCommit, opts ...db.Option) error {
	if commitPO == nil {
		return errorx.New("commitPO is empty")
	}

	session := d.db.NewSession(ctx, opts...)
	q := query.Use(session).WithContext(ctx)

	// 先查是否存在
	existing, err := d.Get(ctx, commitPO.ToolID, commitPO.Version, opts...)
	if err != nil {
		return err
	}

	now := time.Now()
	if existing == nil {
		commitPO.CreatedAt = now
		commitPO.UpdatedAt = now
		return q.ToolCommit.Create(commitPO)
	}

	// 更新
	updateMap := map[string]interface{}{
		"content":      commitPO.Content,
		"base_version": commitPO.BaseVersion,
		"committed_by": commitPO.CommittedBy,
		"description":  commitPO.Description,
		"updated_at":   now,
	}
	qr := query.Use(session)
	tx2 := qr.WithContext(ctx).ToolCommit
	tx2 = tx2.Where(qr.ToolCommit.ToolID.Eq(commitPO.ToolID))
	tx2 = tx2.Where(qr.ToolCommit.Version.Eq(commitPO.Version))
	_, err = tx2.Updates(updateMap)
	if err != nil {
		return errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	return nil
}

func (d *ToolCommitDAOImpl) Delete(ctx context.Context, toolID int64, version string, opts ...db.Option) error {
	if toolID <= 0 {
		return errorx.New("toolID is invalid, toolID = %d", toolID)
	}

	q := query.Use(d.db.NewSession(ctx, opts...))
	tx := q.WithContext(ctx).ToolCommit
	tx = tx.Where(q.ToolCommit.ToolID.Eq(toolID))
	tx = tx.Where(q.ToolCommit.Version.Eq(version))
	_, err := tx.Delete()
	if err != nil {
		return errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	return nil
}

func (d *ToolCommitDAOImpl) List(ctx context.Context, param ListToolCommitDAOParam, opts ...db.Option) (commitPOs []*model.ToolCommit, err error) {
	q := query.Use(d.db.NewSession(ctx, opts...))
	tx := q.WithContext(ctx).ToolCommit
	tx = tx.Where(q.ToolCommit.ToolID.Eq(param.ToolID))
	// 排除草稿版本
	tx = tx.Where(q.ToolCommit.Version.Neq("$PublicDraft"))

	if param.Cursor != nil {
		cursorTime := time.UnixMilli(*param.Cursor)
		if param.Asc {
			tx = tx.Where(q.ToolCommit.CreatedAt.Gte(cursorTime))
		} else {
			tx = tx.Where(q.ToolCommit.CreatedAt.Lte(cursorTime))
		}
	}

	if param.Asc {
		tx = tx.Order(q.ToolCommit.CreatedAt)
	} else {
		tx = tx.Order(q.ToolCommit.CreatedAt.Desc())
	}

	tx = tx.Limit(param.Limit)
	commitPOs, err = tx.Find()
	if err != nil {
		return nil, errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	return commitPOs, nil
}
