// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

func NewExptItemRefRepo(dao mysql.IExptItemRefDAO) repo.IExptItemRefRepo {
	return &ExptItemRefRepoImpl{dao: dao}
}

type ExptItemRefRepoImpl struct {
	dao mysql.IExptItemRefDAO
}

func (r *ExptItemRefRepoImpl) BatchCreate(ctx context.Context, items []*entity.ExptItemRef) error {
	if len(items) == 0 {
		return nil
	}
	conv := convert.NewExptItemRefConvertor()
	pos := make([]*model.ExptItemRef, 0, len(items))
	for _, item := range items {
		pos = append(pos, conv.DO2PO(item))
	}
	return r.dao.BatchCreateNX(ctx, pos)
}

func (r *ExptItemRefRepoImpl) ListByExptID(ctx context.Context, spaceID, exptID int64, cursor, limit int64) ([]*entity.ExptItemRef, int64, error) {
	pos, nextCursor, err := r.dao.ListByExptID(ctx, spaceID, exptID, cursor, limit)
	if err != nil {
		return nil, 0, err
	}
	conv := convert.NewExptItemRefConvertor()
	dos := make([]*entity.ExptItemRef, 0, len(pos))
	for _, po := range pos {
		dos = append(dos, conv.PO2DO(po))
	}
	return dos, nextCursor, nil
}

func (r *ExptItemRefRepoImpl) GetByExptIDAndItemID(ctx context.Context, spaceID, exptID, itemID int64) (*entity.ExptItemRef, error) {
	po, err := r.dao.GetByExptIDAndItemID(ctx, spaceID, exptID, itemID)
	if err != nil {
		return nil, err
	}
	return convert.NewExptItemRefConvertor().PO2DO(po), nil
}

func (r *ExptItemRefRepoImpl) MGetByExptIDAndItemIDs(ctx context.Context, spaceID, exptID int64, itemIDs []int64) ([]*entity.ExptItemRef, error) {
	pos, err := r.dao.MGetByExptIDAndItemIDs(ctx, spaceID, exptID, itemIDs)
	if err != nil {
		return nil, err
	}
	conv := convert.NewExptItemRefConvertor()
	dos := make([]*entity.ExptItemRef, 0, len(pos))
	for _, po := range pos {
		dos = append(dos, conv.PO2DO(po))
	}
	return dos, nil
}

func (r *ExptItemRefRepoImpl) CountByEvalSetGrouped(ctx context.Context, spaceID int64, exptIDs []int64) (map[int64][]*entity.ExptEvalSetItemCount, error) {
	counts, err := r.dao.CountByEvalSetGrouped(ctx, spaceID, exptIDs)
	if err != nil {
		return nil, err
	}
	result := make(map[int64][]*entity.ExptEvalSetItemCount)
	for _, c := range counts {
		result[c.ExptID] = append(result[c.ExptID], &entity.ExptEvalSetItemCount{
			ExptID:           c.ExptID,
			EvalSetID:        c.EvalSetID,
			EvalSetVersionID: c.EvalSetVersionID,
			ItemCount:        c.ItemCount,
		})
	}
	return result, nil
}
