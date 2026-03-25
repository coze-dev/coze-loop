// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
)

func NewExptRunLogRepo(exptRunLogDAO mysql.IExptRunLogDAO) repo.IExptRunLogRepo {
	return &ExptRunLogImpl{
		exptRunLogDAO: exptRunLogDAO,
	}
}

type ExptRunLogImpl struct {
	exptRunLogDAO mysql.IExptRunLogDAO
}

func (e *ExptRunLogImpl) Get(ctx context.Context, exptID, exptRunID int64) (*entity.ExptRunLog, error) {
	po, err := e.exptRunLogDAO.Get(ctx, exptID, exptRunID)
	if err != nil {
		return nil, err
	}
	return convert.NewExptRunLogConvertor().PO2DO(po)
}

func (e *ExptRunLogImpl) Create(ctx context.Context, exptRunLog *entity.ExptRunLog) error {
	po, err := convert.NewExptRunLogConvertor().DO2PO(exptRunLog)
	if err != nil {
		return err
	}
	po.CreatedAt = time.Now()

	if err := e.exptRunLogDAO.Create(ctx, po); err != nil {
		return err
	}

	return nil
}

func (e *ExptRunLogImpl) Save(ctx context.Context, exptRunLog *entity.ExptRunLog) error {
	po, err := convert.NewExptRunLogConvertor().DO2PO(exptRunLog)
	if err != nil {
		return err
	}
	po.UpdatedAt = time.Now()

	if err := e.exptRunLogDAO.Save(ctx, po); err != nil {
		return err
	}
	return nil
}

func (e *ExptRunLogImpl) Update(ctx context.Context, exptID, exptRunID int64, ufields map[string]any) error {
	ufields["updated_at"] = time.Now()
	err := e.exptRunLogDAO.Update(ctx, exptID, exptRunID, ufields)
	if err != nil {
		return err
	}
	return nil
}
