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
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func NewExptRunLogRepo(exptRunLogDAO mysql.IExptRunLogDAO) repo.IExptRunLogRepo {
	return &ExptRunLogImpl{
		exptRunLogDAO: exptRunLogDAO,
	}
}

type ExptRunLogImpl struct {
	exptRunLogDAO mysql.IExptRunLogDAO
}

func (e *ExptRunLogImpl) AppendRunItemIDs(ctx context.Context, exptID, exptRunID, spaceID int64, itemIDs []int64) error {
	return e.exptRunLogDAO.AppendRunItemIDs(ctx, exptID, exptRunID, spaceID, itemIDs)
}

func (e *ExptRunLogImpl) UpdateRunStats(ctx context.Context, exptID, exptRunID, spaceID int64, run *entity.ExptRunLog, stats *entity.ExptStats) error {
	fields := map[string]any{
		"status": run.Status, "pending_cnt": run.PendingCnt, "processing_cnt": run.ProcessingCnt,
		"success_cnt": run.SuccessCnt, "fail_cnt": run.FailCnt, "terminated_cnt": run.TerminatedCnt,
		"credit_cost": run.CreditCost, "token_cost": run.TokenCost, "updated_at": time.Now(),
	}
	return e.exptRunLogDAO.UpdateRunStats(ctx, exptID, exptRunID, spaceID, fields, convert.NewExptStatsConverter().DO2PO(stats))
}

func (e *ExptRunLogImpl) Get(ctx context.Context, exptID, exptRunID int64) (*entity.ExptRunLog, error) {
	po, err := e.exptRunLogDAO.Get(ctx, exptID, exptRunID)
	if err != nil {
		return nil, err
	}
	return convert.NewExptRunLogConvertor().PO2DO(po)
}

func (e *ExptRunLogImpl) MarkProcessingIfPending(ctx context.Context, exptID, runID, spaceID int64) (bool, error) {
	marker, ok := e.exptRunLogDAO.(mysql.IExptRunActivationDAO)
	if !ok {
		return false, errorx.New("run activation DAO missing")
	}
	return marker.MarkProcessingIfPending(ctx, exptID, runID, spaceID)
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

func (e *ExptRunLogImpl) Update(ctx context.Context, exptID, exptRunID, spaceID int64, ufields map[string]any) error {
	ufields["updated_at"] = time.Now()
	err := e.exptRunLogDAO.Update(ctx, exptID, exptRunID, spaceID, ufields)
	if err != nil {
		return err
	}
	return nil
}

func (e *ExptRunLogImpl) UpdateStatusIfCurrent(ctx context.Context, exptID, exptRunID, spaceID, expectedStatus, status int64) (bool, error) {
	updater, ok := e.exptRunLogDAO.(mysql.IExptRunLogTerminalDAO)
	if !ok {
		return false, errorx.New("run terminal status CAS missing")
	}
	return updater.UpdateStatusIfCurrent(ctx, exptID, exptRunID, spaceID, expectedStatus, status)
}

func (e *ExptRunLogImpl) UpdateIfLatestRun(ctx context.Context, exptID, exptRunID, spaceID int64, ufields map[string]any) (bool, error) {
	ufields["updated_at"] = time.Now()
	updater, ok := e.exptRunLogDAO.(mysql.IExptRunLogLatestRunDAO)
	if !ok {
		// Legacy/mock DAO without the CAS capability: keep the unconditional write.
		if err := e.exptRunLogDAO.Update(ctx, exptID, exptRunID, spaceID, ufields); err != nil {
			return false, err
		}
		return true, nil
	}
	return updater.UpdateIfLatestRun(ctx, exptID, exptRunID, spaceID, ufields)
}
