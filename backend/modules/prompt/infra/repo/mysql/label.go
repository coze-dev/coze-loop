// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/samber/lo"
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

//go:generate mockgen -destination=mocks/label_dao.go -package=mocks . ILabelDAO
type ILabelDAO interface {
	Create(ctx context.Context, labelPO *model.PromptLabel, opts ...db.Option) error
	List(ctx context.Context, param ListLabelDAOParam, opts ...db.Option) ([]*model.PromptLabel, error)
	BatchGet(ctx context.Context, spaceID int64, labelKeys []string, opts ...db.Option) ([]*model.PromptLabel, error)
}

//go:generate mockgen -destination=mocks/commit_label_mapping_dao.go -package=mocks . ICommitLabelMappingDAO
type ICommitLabelMappingDAO interface {
	BatchCreate(ctx context.Context, mappings []*model.PromptCommitLabelMapping, opts ...db.Option) error
	BatchUpdate(ctx context.Context, mappings []*model.PromptCommitLabelMapping, opts ...db.Option) error
	BatchDelete(ctx context.Context, ids []int64, opts ...db.Option) error
	ListByPromptIDAndLabelKeys(ctx context.Context, promptID int64, labelKeys []string, opts ...db.Option) ([]*model.PromptCommitLabelMapping, error)
	ListByPromptIDAndVersions(ctx context.Context, promptID int64, versions []string, opts ...db.Option) ([]*model.PromptCommitLabelMapping, error)
	MGetPromptVersionByLabelQuery(ctx context.Context, param MGetPromptVersionByLabelQueryParam, opts ...db.Option) ([]*model.PromptCommitLabelMapping, error)
}

type ListLabelDAOParam struct {
	SpaceID      int64
	LabelKeyLike string
	Cursor       *int64
	Limit        int
}

type MGetPromptVersionByLabelParam struct {
	SpaceID    int64
	PromptKeys []string
	LabelKeys  []string
}

// 添加支持组合查询的参数结构
type PromptLabelQuery struct {
	PromptID int64
	LabelKey string
}

type MGetPromptVersionByLabelQueryParam struct {
	Queries []PromptLabelQuery
}

type LabelDAOImpl struct {
	db           db.Provider
	writeTracker platestwrite.ILatestWriteTracker
}

func NewLabelDAO(db db.Provider, redisCli redis.Cmdable) ILabelDAO {
	return &LabelDAOImpl{
		db:           db,
		writeTracker: platestwrite.NewLatestWriteTracker(redisCli),
	}
}

func (d *LabelDAOImpl) Create(ctx context.Context, labelPO *model.PromptLabel, opts ...db.Option) error {
	if labelPO == nil {
		return errorx.New("labelPO is empty")
	}

	q := query.Use(d.db.NewSession(ctx, opts...)).WithContext(ctx)
	err := q.PromptLabel.Create(labelPO)
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return errorx.WrapByCode(err, prompterr.CommonResourceDuplicatedCode)
		}
		return errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	d.writeTracker.SetWriteFlag(ctx, platestwrite.ResourceTypePromptLabel, labelPO.ID, platestwrite.SetWithSearchParam(strconv.FormatInt(labelPO.SpaceID, 10)))
	return nil
}

func (d *LabelDAOImpl) List(ctx context.Context, param ListLabelDAOParam, opts ...db.Option) ([]*model.PromptLabel, error) {
	if param.SpaceID <= 0 || param.Limit <= 0 {
		return nil, errorx.New("param(SpaceID or Limit) is invalid, param = %s", json.Jsonify(param))
	}
	if d.writeTracker.CheckWriteFlagBySearchParam(ctx, platestwrite.ResourceTypePromptLabel, strconv.FormatInt(param.SpaceID, 10)) {
		opts = append(opts, db.WithMaster())
	}

	q := query.Use(d.db.NewSession(ctx, opts...))
	tx := q.WithContext(ctx).PromptLabel
	tx = tx.Where(q.PromptLabel.SpaceID.Eq(param.SpaceID))

	if !lo.IsEmpty(param.LabelKeyLike) {
		tx = tx.Where(q.PromptLabel.LabelKey.Like(fmt.Sprintf("%%%s%%", param.LabelKeyLike)))
	}

	if param.Cursor != nil {
		tx = tx.Where(q.PromptLabel.ID.Lte(*param.Cursor))
	}

	tx = tx.Order(q.PromptLabel.ID.Desc()).Limit(param.Limit)
	labelPOs, err := tx.Find()
	if err != nil {
		return nil, errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	return labelPOs, nil
}

func (d *LabelDAOImpl) BatchGet(ctx context.Context, spaceID int64, labelKeys []string, opts ...db.Option) ([]*model.PromptLabel, error) {
	if len(labelKeys) <= 0 {
		return nil, nil
	}

	q := query.Use(d.db.NewSession(ctx, opts...))
	tx := q.WithContext(ctx).PromptLabel
	tx = tx.Where(q.PromptLabel.SpaceID.Eq(spaceID), q.PromptLabel.LabelKey.In(labelKeys...))
	labelPOs, err := tx.Find()
	if err != nil {
		return nil, errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	return labelPOs, nil
}

type CommitLabelMappingDAOImpl struct {
	db           db.Provider
	writeTracker platestwrite.ILatestWriteTracker
}

func NewCommitLabelMappingDAO(db db.Provider, redisCli redis.Cmdable) ICommitLabelMappingDAO {
	return &CommitLabelMappingDAOImpl{
		db:           db,
		writeTracker: platestwrite.NewLatestWriteTracker(redisCli),
	}
}

func (d *CommitLabelMappingDAOImpl) BatchCreate(ctx context.Context, mappings []*model.PromptCommitLabelMapping, opts ...db.Option) error {
	if len(mappings) == 0 {
		return nil
	}

	q := query.Use(d.db.NewSession(ctx, opts...)).WithContext(ctx)
	for _, mapping := range mappings {
		mapping.CreatedAt = time.Time{}
		mapping.UpdatedAt = time.Time{}
	}

	err := q.PromptCommitLabelMapping.CreateInBatches(mappings, 100)
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return errorx.WrapByCode(err, prompterr.CommonResourceDuplicatedCode)
		}
		return errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}

	for _, mapping := range mappings {
		d.writeTracker.SetWriteFlag(ctx, platestwrite.ResourceTypePromptCommitLabelMapping, mapping.ID,
			platestwrite.SetWithSearchParam(fmt.Sprintf("%d", mapping.PromptID)))
	}
	return nil
}

func (d *CommitLabelMappingDAOImpl) BatchUpdate(ctx context.Context, mappings []*model.PromptCommitLabelMapping, opts ...db.Option) error {
	if len(mappings) == 0 {
		return nil
	}

	q := query.Use(d.db.NewSession(ctx, opts...))

	for _, mapping := range mappings {
		mapping.UpdatedAt = time.Time{}
		_, err := q.WithContext(ctx).PromptCommitLabelMapping.Where(q.PromptCommitLabelMapping.ID.Eq(mapping.ID)).Updates(mapping)
		if err != nil {
			return errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
		}

		d.writeTracker.SetWriteFlag(ctx, platestwrite.ResourceTypePromptCommitLabelMapping, mapping.ID,
			platestwrite.SetWithSearchParam(fmt.Sprintf("%d", mapping.PromptID)))
	}
	return nil
}

func (d *CommitLabelMappingDAOImpl) BatchDelete(ctx context.Context, ids []int64, opts ...db.Option) error {
	if len(ids) == 0 {
		return nil
	}

	q := query.Use(d.db.NewSession(ctx, opts...))
	_, err := q.WithContext(ctx).PromptCommitLabelMapping.Where(
		q.PromptCommitLabelMapping.ID.In(ids...),
	).Delete(&model.PromptCommitLabelMapping{})
	if err != nil {
		return errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}

	return nil
}

func (d *CommitLabelMappingDAOImpl) ListByPromptIDAndLabelKeys(ctx context.Context, promptID int64, labelKeys []string, opts ...db.Option) ([]*model.PromptCommitLabelMapping, error) {
	if len(labelKeys) == 0 {
		return nil, nil
	}

	if d.writeTracker.CheckWriteFlagBySearchParam(ctx, platestwrite.ResourceTypePromptCommitLabelMapping, fmt.Sprintf("%d", promptID)) {
		opts = append(opts, db.WithMaster())
	}

	q := query.Use(d.db.NewSession(ctx, opts...))
	tx := q.WithContext(ctx).PromptCommitLabelMapping
	tx = tx.Where(
		q.PromptCommitLabelMapping.PromptID.Eq(promptID),
		q.PromptCommitLabelMapping.LabelKey.In(labelKeys...),
	)
	mappings, err := tx.Find()
	if err != nil {
		return nil, errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	return mappings, nil
}

func (d *CommitLabelMappingDAOImpl) ListByPromptIDAndVersions(ctx context.Context, promptID int64, versions []string, opts ...db.Option) ([]*model.PromptCommitLabelMapping, error) {
	if d.writeTracker.CheckWriteFlagBySearchParam(ctx, platestwrite.ResourceTypePromptCommitLabelMapping, fmt.Sprintf("%d", promptID)) {
		opts = append(opts, db.WithMaster())
	}
	q := query.Use(d.db.NewSession(ctx, opts...))
	tx := q.WithContext(ctx).PromptCommitLabelMapping
	tx = tx.Where(
		q.PromptCommitLabelMapping.PromptID.Eq(promptID),
		q.PromptCommitLabelMapping.PromptVersion.In(versions...),
	)
	mappings, err := tx.Find()
	if err != nil {
		return nil, errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	return mappings, nil
}

func (d *CommitLabelMappingDAOImpl) MGetPromptVersionByLabelQuery(ctx context.Context, param MGetPromptVersionByLabelQueryParam, opts ...db.Option) ([]*model.PromptCommitLabelMapping, error) {
	if len(param.Queries) == 0 {
		return nil, nil
	}

	q := query.Use(d.db.NewSession(ctx, opts...))
	tx := q.WithContext(ctx).PromptCommitLabelMapping
	oriTx := tx

	// 构建OR条件，每个查询组合作为一个AND条件
	for _, query := range param.Queries {
		subCon := oriTx.Where(
			q.PromptCommitLabelMapping.PromptID.Eq(query.PromptID),
			q.PromptCommitLabelMapping.LabelKey.Eq(query.LabelKey),
		)
		tx = tx.Or(subCon)
	}

	mappings, err := tx.Find()
	if err != nil {
		return nil, errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	return mappings, nil
}
