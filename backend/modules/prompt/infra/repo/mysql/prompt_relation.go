// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"strconv"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/platestwrite"
	"github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql/gorm_gen/query"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

//go:generate mockgen -destination=mocks/prompt_relation_dao.go -package=mocks . IPromptRelationDAO
type IPromptRelationDAO interface {
	// BatchCreate creates multiple prompt relations in batch
	BatchCreate(ctx context.Context, relationPOs []*model.PromptRelation, opts ...db.Option) error

	// DeleteByMainPrompt deletes all relations for a main prompt
	DeleteByMainPrompt(ctx context.Context, mainPromptID int64, mainPromptVersion string, mainDraftUserID string, opts ...db.Option) error

	// BatchDeleteByIDs deletes relations by their IDs
	BatchDeleteByIDs(ctx context.Context, relationIDs []int64, opts ...db.Option) error

	// List lists prompt relations with optional filtering parameters
	List(ctx context.Context, param ListPromptRelationParam, opts ...db.Option) ([]*model.PromptRelation, error)
}

// ListPromptRelationParam unified parameter for listing prompt relations
// All fields are optional - only non-zero values will be used for filtering
type ListPromptRelationParam struct {
	// Main prompt filtering
	MainPromptID       *int64
	MainPromptVersions []string
	MainDraftUserID    *string

	// Sub prompt filtering
	SubPromptID       *int64
	SubPromptVersions []string

	// Pagination
	Limit  *int
	Offset *int
}

func NewPromptRelationDAO(db db.Provider, redisCli redis.Cmdable) IPromptRelationDAO {
	return &PromptRelationDAOImpl{
		db:           db,
		writeTracker: platestwrite.NewLatestWriteTracker(redisCli),
	}
}

type PromptRelationDAOImpl struct {
	db           db.Provider
	writeTracker platestwrite.ILatestWriteTracker
}

func (d *PromptRelationDAOImpl) BatchCreate(ctx context.Context, relationPOs []*model.PromptRelation, opts ...db.Option) error {
	if len(relationPOs) == 0 {
		return nil
	}

	q := query.Use(d.db.NewSession(ctx, opts...)).WithContext(ctx)
	err := q.PromptRelation.CreateInBatches(relationPOs, len(relationPOs))
	if err != nil {
		return err
	}

	// 批量设置写标志，处理主从延迟
	mainPromptIDs := make(map[int64]bool)
	subPromptIDs := make(map[int64]bool)
	for _, relationPO := range relationPOs {
		if relationPO != nil {
			mainPromptIDs[relationPO.MainPromptID] = true
			subPromptIDs[relationPO.SubPromptID] = true
			d.writeTracker.SetWriteFlag(ctx, platestwrite.ResourceTypePromptRelation, relationPO.ID)
		}
	}
	for mainPromptID := range mainPromptIDs {
		d.writeTracker.SetWriteFlag(ctx, platestwrite.ResourceTypePromptRelation, 0,
			platestwrite.SetWithSearchParam(strconv.FormatInt(mainPromptID, 10)))
	}
	for subPromptID := range subPromptIDs {
		d.writeTracker.SetWriteFlag(ctx, platestwrite.ResourceTypePromptRelation, 0,
			platestwrite.SetWithSearchParam(strconv.FormatInt(subPromptID, 10)))
	}

	return nil
}

func (d *PromptRelationDAOImpl) DeleteByMainPrompt(ctx context.Context, mainPromptID int64, mainPromptVersion string, mainDraftUserID string, opts ...db.Option) error {
	if mainPromptID <= 0 {
		return errorx.New("mainPromptID is invalid, mainPromptID = %d", mainPromptID)
	}

	q := query.Use(d.db.NewSession(ctx, opts...))
	tx := q.WithContext(ctx).PromptRelation
	tx = tx.Where(
		q.PromptRelation.MainPromptID.Eq(mainPromptID),
		q.PromptRelation.MainPromptVersion.Eq(mainPromptVersion),
		q.PromptRelation.MainDraftUserID.Eq(mainDraftUserID),
	)
	_, err := tx.Delete()
	if err != nil {
		return err
	}

	// 设置写标志，处理主从延迟
	d.writeTracker.SetWriteFlag(ctx, platestwrite.ResourceTypePromptRelation, 0,
		platestwrite.SetWithSearchParam(strconv.FormatInt(mainPromptID, 10)))

	return nil
}

func (d *PromptRelationDAOImpl) BatchDeleteByIDs(ctx context.Context, relationIDs []int64, opts ...db.Option) error {
	if len(relationIDs) == 0 {
		return nil
	}

	q := query.Use(d.db.NewSession(ctx, opts...))
	_, err := q.PromptRelation.WithContext(ctx).Where(q.PromptRelation.ID.In(relationIDs...)).Delete()
	if err != nil {
		return err
	}

	// 批量设置写标志，处理主从延迟
	for _, relationID := range relationIDs {
		d.writeTracker.SetWriteFlag(ctx, platestwrite.ResourceTypePromptRelation, relationID)
	}

	return nil
}

func (d *PromptRelationDAOImpl) List(ctx context.Context, param ListPromptRelationParam, opts ...db.Option) ([]*model.PromptRelation, error) {
	// 检查主从延迟写标志
	if param.MainPromptID != nil && d.writeTracker.CheckWriteFlagBySearchParam(ctx,
		platestwrite.ResourceTypePromptRelation, strconv.FormatInt(*param.MainPromptID, 10)) {
		opts = append(opts, db.WithMaster())
	}

	q := query.Use(d.db.NewSession(ctx, opts...))
	tx := q.WithContext(ctx).PromptRelation

	// Apply filters only when parameters are provided
	if param.MainPromptID != nil {
		tx = tx.Where(q.PromptRelation.MainPromptID.Eq(*param.MainPromptID))
	}
	if len(param.MainPromptVersions) > 0 {
		tx = tx.Where(q.PromptRelation.MainPromptVersion.In(param.MainPromptVersions...))
	}
	if param.MainDraftUserID != nil {
		tx = tx.Where(q.PromptRelation.MainDraftUserID.Eq(*param.MainDraftUserID))
	}
	if param.SubPromptID != nil {
		tx = tx.Where(q.PromptRelation.SubPromptID.Eq(*param.SubPromptID))
	}
	if len(param.SubPromptVersions) > 0 {
		tx = tx.Where(q.PromptRelation.SubPromptVersion.In(param.SubPromptVersions...))
	}

	// Apply pagination if provided
	if param.Limit != nil && *param.Limit > 0 {
		tx = tx.Limit(*param.Limit)
	}
	if param.Offset != nil && *param.Offset > 0 {
		tx = tx.Offset(*param.Offset)
	}

	return tx.Find()
}
