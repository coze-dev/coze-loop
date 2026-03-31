// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gen/field"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/platestwrite"
	"github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql/gorm_gen/query"
	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

type IToolCommitDAO interface {
	Create(ctx context.Context, toolCommitPO *model.ToolCommit, timeNow time.Time, opts ...db.Option) (err error)
	UpsertDraft(ctx context.Context, toolCommitPO *model.ToolCommit, timeNow time.Time, opts ...db.Option) (err error)
	Get(ctx context.Context, toolID int64, version string, opts ...db.Option) (toolCommitPO *model.ToolCommit, err error)
	BatchGet(ctx context.Context, toolIDVersionPairs []ToolIDVersionPair, opts ...db.Option) (commitPOs []*model.ToolCommit, err error)
	List(ctx context.Context, param ListToolCommitParam, opts ...db.Option) (commitPOs []*model.ToolCommit, err error)
}

type ToolIDVersionPair struct {
	ToolID  int64
	Version string
}

type ListToolCommitParam struct {
	ToolID int64

	Cursor *int64
	Limit  int
	Asc    bool

	ExcludeVersion string
}

type ToolCommitDAOImpl struct {
	db           db.Provider
	writeTracker platestwrite.ILatestWriteTracker
}

func NewToolCommitDAO(db db.Provider, redisCli redis.Cmdable) IToolCommitDAO {
	return &ToolCommitDAOImpl{
		db:           db,
		writeTracker: platestwrite.NewLatestWriteTracker(redisCli),
	}
}

func (d *ToolCommitDAOImpl) Create(ctx context.Context, toolCommitPO *model.ToolCommit, timeNow time.Time, opts ...db.Option) (err error) {
	if toolCommitPO == nil {
		return errorx.New("toolCommitPO is empty")
	}
	q := query.Use(d.db.NewSession(ctx, opts...)).WithContext(ctx)
	toolCommitPO.CreatedAt = timeNow
	toolCommitPO.UpdatedAt = timeNow
	err = q.ToolCommit.Create(toolCommitPO)
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return errorx.WrapByCode(err, prompterr.CommonResourceDuplicatedCode)
		}
		return errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	d.writeTracker.SetWriteFlag(ctx, platestwrite.ResourceTypeToolCommit, toolCommitPO.ToolID, platestwrite.SetWithSearchParam(fmt.Sprintf("%d:%s", toolCommitPO.ToolID, toolCommitPO.Version)))
	return nil
}

func (d *ToolCommitDAOImpl) UpsertDraft(ctx context.Context, toolCommitPO *model.ToolCommit, timeNow time.Time, opts ...db.Option) (err error) {
	if toolCommitPO == nil {
		return errorx.New("toolCommitPO is empty")
	}
	q := query.Use(d.db.NewSession(ctx, opts...)).WithContext(ctx)
	toolCommitPO.UpdatedAt = timeNow
	toolCommitPO.CreatedAt = timeNow

	tx := q.ToolCommit.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "tool_id"},
			{Name: "version"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"content",
			"base_version",
			"committed_by",
			"description",
			"updated_at",
		}),
	})
	err = tx.Create(toolCommitPO)
	if err != nil {
		return errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	d.writeTracker.SetWriteFlag(ctx, platestwrite.ResourceTypeToolCommit, toolCommitPO.ToolID, platestwrite.SetWithSearchParam(fmt.Sprintf("%d:%s", toolCommitPO.ToolID, toolCommitPO.Version)))
	return nil
}

func (d *ToolCommitDAOImpl) Get(ctx context.Context, toolID int64, version string, opts ...db.Option) (toolCommitPO *model.ToolCommit, err error) {
	if toolID <= 0 {
		return nil, errorx.New("toolID is invalid, toolID = %d", toolID)
	}
	d.writeTracker.CheckWriteFlagBySearchParam(ctx, platestwrite.ResourceTypeToolCommit, fmt.Sprintf("%d:%s", toolID, version))

	q := query.Use(d.db.NewSession(ctx, opts...))
	tx := q.WithContext(ctx).ToolCommit
	tx = tx.Where(q.ToolCommit.ToolID.Eq(toolID), q.ToolCommit.Version.Eq(version))
	toolCommitPOs, err := tx.Find()
	if err != nil {
		return nil, errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	if len(toolCommitPOs) == 0 {
		return nil, nil
	}
	return toolCommitPOs[0], nil
}

func (d *ToolCommitDAOImpl) BatchGet(ctx context.Context, toolIDVersionPairs []ToolIDVersionPair, opts ...db.Option) (commitPOs []*model.ToolCommit, err error) {
	if len(toolIDVersionPairs) == 0 {
		return nil, nil
	}
	q := query.Use(d.db.NewSession(ctx, opts...))
	tx := q.WithContext(ctx).ToolCommit

	conditions := make([]field.Expr, 0, len(toolIDVersionPairs))
	for _, pair := range toolIDVersionPairs {
		conditions = append(conditions, field.And(q.ToolCommit.ToolID.Eq(pair.ToolID), q.ToolCommit.Version.Eq(pair.Version)))
	}
	tx = tx.Where(field.Or(conditions...))
	commitPOs, err = tx.Find()
	if err != nil {
		return nil, errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	return commitPOs, nil
}

func (d *ToolCommitDAOImpl) List(ctx context.Context, param ListToolCommitParam, opts ...db.Option) (commitPOs []*model.ToolCommit, err error) {
	if param.ToolID <= 0 || param.Limit <= 0 {
		return nil, errorx.New("Param(ToolID or Limit) is invalid, param = %s", json.Jsonify(param))
	}
	if d.writeTracker.CheckWriteFlagByID(ctx, platestwrite.ResourceTypeToolCommit, param.ToolID) {
		opts = append(opts, db.WithMaster())
	}

	q := query.Use(d.db.NewSession(ctx, opts...))
	tx := q.WithContext(ctx).ToolCommit
	tx = tx.Where(q.ToolCommit.ToolID.Eq(param.ToolID))
	if param.ExcludeVersion != "" {
		tx = tx.Where(q.ToolCommit.Version.Neq(param.ExcludeVersion))
	}

	if param.Cursor == nil {
		if param.Asc {
			tx = tx.Order(q.ToolCommit.CreatedAt.Asc())
		} else {
			tx = tx.Order(q.ToolCommit.CreatedAt.Desc())
		}
	} else {
		if param.Asc {
			tx = tx.Where(q.ToolCommit.CreatedAt.Gte(time.Unix(*param.Cursor, 0))).Order(q.ToolCommit.CreatedAt.Asc())
		} else {
			tx = tx.Where(q.ToolCommit.CreatedAt.Lte(time.Unix(*param.Cursor, 0))).Order(q.ToolCommit.CreatedAt.Desc())
		}
	}
	tx = tx.Limit(param.Limit)
	commitPOs, err = tx.Find()
	if err != nil {
		return nil, errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	return commitPOs, nil
}
