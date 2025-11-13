// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/platestwrite"
	"github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql/gorm_gen/query"
	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

//go:generate mockgen -destination=mocks/prompt_commit_dao.go -package=mocks . IPromptCommitDAO
type IPromptCommitDAO interface {
	Create(ctx context.Context, promptCommitPO *model.PromptCommit, time time.Time, opts ...db.Option) (err error)
	Get(ctx context.Context, promptID int64, commitVersion string, opts ...db.Option) (promptCommitPO *model.PromptCommit, err error)
	MGet(ctx context.Context, pairs []PromptIDCommitVersionPair, opts ...db.Option) (pairCommitPOMap map[PromptIDCommitVersionPair]*model.PromptCommit, err error)
	List(ctx context.Context, param ListCommitParam, opts ...db.Option) (commitPOs []*model.PromptCommit, err error)
}

type ListCommitParam struct {
	PromptID int64

	Cursor *int64
	Limit  int
	Asc    bool
}

type PromptCommitDAOImpl struct {
	db           db.Provider
	writeTracker platestwrite.ILatestWriteTracker
}

func NewPromptCommitDAO(db db.Provider, redisCli redis.Cmdable) IPromptCommitDAO {
	return &PromptCommitDAOImpl{
		db:           db,
		writeTracker: platestwrite.NewLatestWriteTracker(redisCli),
	}
}

type PromptIDCommitVersionPair struct {
	PromptID      int64
	CommitVersion string
}

func (d *PromptCommitDAOImpl) Create(ctx context.Context, promptCommitPO *model.PromptCommit, timeNow time.Time, opts ...db.Option) (err error) {
	if promptCommitPO == nil {
		return errorx.New("promptCommitPO is empty")
	}
	q := query.Use(d.db.NewSession(ctx, opts...)).WithContext(ctx)
	promptCommitPO.CreatedAt = timeNow
	promptCommitPO.UpdatedAt = timeNow
	err = q.PromptCommit.Create(promptCommitPO)
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return errorx.WrapByCode(err, prompterr.CommonResourceDuplicatedCode)
		}
		return errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	d.writeTracker.SetWriteFlag(ctx, platestwrite.ResourceTypePromptCommit, promptCommitPO.PromptID, platestwrite.SetWithSearchParam(fmt.Sprintf("%d:%s", promptCommitPO.PromptID, promptCommitPO.Version)))
	return nil
}

func (d *PromptCommitDAOImpl) Get(ctx context.Context, promptID int64, commitVersion string, opts ...db.Option) (promptCommitPO *model.PromptCommit, err error) {
	if promptID <= 0 {
		return nil, errorx.New("promptID is invalid, promptID = %d", promptID)
	}
	d.writeTracker.CheckWriteFlagBySearchParam(ctx, platestwrite.ResourceTypePromptCommit, fmt.Sprintf("%d:%s", promptID, commitVersion))

	q := query.Use(d.db.NewSession(ctx, opts...))
	tx := q.WithContext(ctx).PromptCommit
	tx = tx.Where(q.PromptCommit.PromptID.Eq(promptID))
	tx = tx.Where(q.PromptCommit.Version.Eq(commitVersion))
	promptCommitPOs, err := tx.Find()
	if err != nil {
		return nil, errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	if len(promptCommitPOs) <= 0 {
		return nil, nil
	}
	return promptCommitPOs[0], nil
}

func (d *PromptCommitDAOImpl) MGet(ctx context.Context, pairs []PromptIDCommitVersionPair, opts ...db.Option) (pairCommitPOMap map[PromptIDCommitVersionPair]*model.PromptCommit, err error) {
	if len(pairs) <= 0 {
		return nil, errorx.New("invalid param")
	}
	q := query.Use(d.db.NewSession(ctx, opts...).Debug())
	tx := q.WithContext(ctx).PromptCommit
	oriTx := tx
	for _, pair := range pairs {
		subCon := oriTx.Where(q.PromptCommit.PromptID.Eq(pair.PromptID), q.PromptCommit.Version.Eq(pair.CommitVersion))
		tx = tx.Or(subCon)
	}
	promptCommitPOs, err := tx.Find()
	if err != nil {
		return nil, err
	}
	if len(promptCommitPOs) <= 0 {
		return nil, nil
	}
	pairCommitPOMap = make(map[PromptIDCommitVersionPair]*model.PromptCommit)
	for _, po := range promptCommitPOs {
		pairCommitPOMap[PromptIDCommitVersionPair{
			PromptID:      po.PromptID,
			CommitVersion: po.Version,
		}] = po
	}
	return pairCommitPOMap, nil
}

func (d *PromptCommitDAOImpl) List(ctx context.Context, param ListCommitParam, opts ...db.Option) (commitPOs []*model.PromptCommit, err error) {
	if param.PromptID <= 0 || param.Limit <= 0 {
		return nil, errorx.New("Param(PromptID or List or Cursor) is invalid, param = %s", json.Jsonify(param))
	}
	if d.writeTracker.CheckWriteFlagByID(ctx, platestwrite.ResourceTypePromptCommit, param.PromptID) {
		opts = append(opts, db.WithMaster())
	}

	q := query.Use(d.db.NewSession(ctx, opts...))
	tx := q.WithContext(ctx).PromptCommit
	tx = tx.Where(q.PromptCommit.PromptID.Eq(param.PromptID))
	if param.Cursor == nil {
		if param.Asc {
			tx = tx.Order(q.PromptCommit.ID.Asc())
		} else {
			tx = tx.Order(q.PromptCommit.ID.Desc())
		}
	} else {
		if param.Asc {
			tx = tx.Where(q.PromptCommit.ID.Gte(*param.Cursor)).Order(q.PromptCommit.ID.Asc())
		} else {
			tx = tx.Where(q.PromptCommit.ID.Lte(*param.Cursor)).Order(q.PromptCommit.ID.Desc())
		}
	}
	tx = tx.Limit(param.Limit)
	commitPOs, err = tx.Find()
	if err != nil {
		return nil, errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	if len(commitPOs) <= 0 {
		return nil, nil
	}
	return commitPOs, nil
}
