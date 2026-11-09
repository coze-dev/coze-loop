// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
)

func NewExptStatsRepo(exptStatsDAO mysql.IExptStatsDAO) repo.IExptStatsRepo {
	return &exptStatsRepo{exptStatsDAO: exptStatsDAO}
}

type exptStatsRepo struct {
	exptStatsDAO mysql.IExptStatsDAO
}

func (e *exptStatsRepo) Create(ctx context.Context, stats *entity.ExptStats) error {
	return e.exptStatsDAO.Create(ctx, convert.NewExptStatsConverter().DO2PO(stats))
}

func (e *exptStatsRepo) Get(ctx context.Context, exptID, spaceID int64) (*entity.ExptStats, error) {
	po, err := e.exptStatsDAO.Get(ctx, exptID, spaceID)
	if err != nil {
		return nil, err
	}
	return convert.NewExptStatsConverter().PO2DO(po), nil
}

func (e *exptStatsRepo) MGet(ctx context.Context, exptIDs []int64, spaceID int64) ([]*entity.ExptStats, error) {
	pos, err := e.exptStatsDAO.MGet(ctx, exptIDs, spaceID)
	if err != nil {
		return nil, err
	}

	dos := make([]*entity.ExptStats, 0)
	for _, po := range pos {
		dos = append(dos, convert.NewExptStatsConverter().PO2DO(po))
	}
	return dos, nil
}

func (e *exptStatsRepo) UpdateByExptID(ctx context.Context, exptID, spaceID int64, stats *entity.ExptStats) error {
	err := e.exptStatsDAO.UpdateByExptID(ctx, exptID, spaceID, convert.NewExptStatsConverter().DO2PO(stats))
	if err != nil {
		return err
	}
	return nil
}

func (e *exptStatsRepo) ArithOperateCount(ctx context.Context, exptID, spaceID int64, cntArithOp *entity.StatsCntArithOp) error {
	err := e.exptStatsDAO.ArithOperateCount(ctx, exptID, spaceID, cntArithOp)
	if err != nil {
		return err
	}
	return nil
}

func (e *exptStatsRepo) Save(ctx context.Context, stats *entity.ExptStats) error {
	err := e.exptStatsDAO.Save(ctx, convert.NewExptStatsConverter().DO2PO(stats))
	if err != nil {
		return err
	}
	return nil
}
