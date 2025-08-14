// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"fmt"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

func NewExptTurnResultRepo(idgen idgen.IIDGenerator, exptTurnResultDAO mysql.ExptTurnResultDAO, exptTurnEvaluatorResultRefDAO mysql.IExptTurnEvaluatorResultRefDAO) repo.IExptTurnResultRepo {
	return &ExptTurnResultRepoImpl{
		idgen:                         idgen,
		exptTurnResultDAO:             exptTurnResultDAO,
		exptTurnEvaluatorResultRefDAO: exptTurnEvaluatorResultRefDAO,
	}
}

type ExptTurnResultRepoImpl struct {
	idgen                         idgen.IIDGenerator
	exptTurnResultDAO             mysql.ExptTurnResultDAO
	exptTurnEvaluatorResultRefDAO mysql.IExptTurnEvaluatorResultRefDAO
}

func (r *ExptTurnResultRepoImpl) UpdateTurnResultsWithItemIDs(ctx context.Context, exptID int64, itemIDs []int64, spaceID int64, ufields map[string]any) error {
	return r.exptTurnResultDAO.UpdateTurnResultsWithItemIDs(ctx, exptID, itemIDs, spaceID, ufields)
}

func (r *ExptTurnResultRepoImpl) UpdateTurnResults(ctx context.Context, exptID int64, itemTurnIDs []*entity.ItemTurnID, spaceID int64, ufields map[string]any) error {
	return r.exptTurnResultDAO.UpdateTurnResults(ctx, exptID, itemTurnIDs, spaceID, ufields)
}

func (r *ExptTurnResultRepoImpl) ScanTurnResults(ctx context.Context, exptID int64, status []int32, cursor, limit, spaceID int64) ([]*entity.ExptTurnResult, int64, error) {
	exptTurnResultPOs, ncursor, err := r.exptTurnResultDAO.ScanTurnResults(ctx, exptID, status, cursor, limit, spaceID)
	if err != nil {
		return nil, 0, errorx.Wrapf(err, "ScanTurnResults fail, exptID=%d, cursor=%d", exptID, cursor)
	}

	exptTurnResultsDOs := make([]*entity.ExptTurnResult, 0, len(exptTurnResultPOs))
	for _, exptTurnResultPO := range exptTurnResultPOs {
		exptTurnResultDO := convert.NewExptTurnResultConvertor().PO2DO(exptTurnResultPO, nil)
		exptTurnResultsDOs = append(exptTurnResultsDOs, exptTurnResultDO)
	}

	return exptTurnResultsDOs, ncursor, nil
}

func (r *ExptTurnResultRepoImpl) ScanTurnRunLogs(ctx context.Context, exptID, cursor, limit, spaceID int64) ([]*entity.ExptTurnResultRunLog, int64, error) {
	exptTurnResultRunLogPOs, ncursor, err := r.exptTurnResultDAO.ScanTurnRunLogs(ctx, exptID, cursor, limit, spaceID)
	if err != nil {
		return nil, 0, errorx.Wrapf(err, "ScanTurnResults fail, exptID=%d, cursor=%d", exptID, cursor)
	}
	exptTurnResultRunLogDOs := make([]*entity.ExptTurnResultRunLog, 0, len(exptTurnResultRunLogPOs))
	for _, exptTurnResultRunLogPO := range exptTurnResultRunLogPOs {
		exptTurnResultRunLogDO, err := convert.NewExptTurnResultRunLogConvertor().PO2DO(exptTurnResultRunLogPO)
		if err != nil {
			return nil, 0, errorx.Wrapf(err, "PO2DO fail, ExptTurnResultRunLogPO: %v", json.Jsonify(exptTurnResultRunLogPO))
		}
		exptTurnResultRunLogDOs = append(exptTurnResultRunLogDOs, exptTurnResultRunLogDO)
	}

	return exptTurnResultRunLogDOs, ncursor, nil
}

func (r *ExptTurnResultRepoImpl) BatchCreateNX(ctx context.Context, turnResults []*entity.ExptTurnResult) error {
	turnResultPOs := make([]*model.ExptTurnResult, 0, len(turnResults))
	for _, turnResult := range turnResults {
		turnResultPO := convert.NewExptTurnResultConvertor().DO2PO(turnResult)
		turnResultPOs = append(turnResultPOs, turnResultPO)
	}
	err := r.exptTurnResultDAO.BatchCreateNX(ctx, turnResultPOs)
	if err != nil {
		return errorx.Wrapf(err, "BatchCreateNX fail, turnResults: %v", json.Jsonify(turnResults))
	}
	return nil
}

func (r *ExptTurnResultRepoImpl) CreateTurnEvaluatorRefs(ctx context.Context, refs []*entity.ExptTurnEvaluatorResultRef) error {
	pos := make([]*model.ExptTurnEvaluatorResultRef, 0, len(refs))
	for _, ref := range refs {
		po := convert.NewExptTurnEvaluatorResultRefConvertor().DO2PO(ref)
		pos = append(pos, po)
	}
	return r.exptTurnResultDAO.CreateTurnEvaluatorRefs(ctx, pos)
}

func (r *ExptTurnResultRepoImpl) BatchGet(ctx context.Context, spaceID int64, exptID int64, itemIDs []int64) ([]*entity.ExptTurnResult, error) {
	exptTurnResultPOs, err := r.exptTurnResultDAO.BatchGet(ctx, spaceID, exptID, itemIDs)
	if err != nil {
		return nil, errorx.Wrapf(err, "BatchGet fail, spaceID: %v, exptID: %v, itemIDs: %v", spaceID, exptID, itemIDs)
	}
	exptTurnResults := make([]*entity.ExptTurnResult, 0, len(exptTurnResultPOs))
	for _, exptTurnResultPO := range exptTurnResultPOs {
		exptTurnResult := convert.NewExptTurnResultConvertor().PO2DO(exptTurnResultPO, nil)
		exptTurnResults = append(exptTurnResults, exptTurnResult)
	}
	return exptTurnResults, nil
}

func (r *ExptTurnResultRepoImpl) SaveTurnResults(ctx context.Context, turnResults []*entity.ExptTurnResult) error {
	logs.CtxInfo(ctx, "SaveTurnResults: %v", json.Jsonify(turnResults))

	turnResultPOs := make([]*model.ExptTurnResult, 0, len(turnResults))
	for _, turnResult := range turnResults {
		turnResultPO := convert.NewExptTurnResultConvertor().DO2PO(turnResult)
		turnResultPOs = append(turnResultPOs, turnResultPO)
	}
	err := r.exptTurnResultDAO.SaveTurnResults(ctx, turnResultPOs)
	if err != nil {
		return errorx.Wrapf(err, "SaveTurnResults fail, turnResults: %v", json.Jsonify(turnResults))
	}
	return nil
}

func (r *ExptTurnResultRepoImpl) SaveTurnRunLogs(ctx context.Context, runLogs []*entity.ExptTurnResultRunLog) error {
	exptTurnResultRunLogPOs := make([]*model.ExptTurnResultRunLog, 0, len(runLogs))
	for _, runLog := range runLogs {
		runLogPO, err := convert.NewExptTurnResultRunLogConvertor().DO2PO(runLog)
		if err != nil {
			return errorx.Wrapf(err, "DO2PO fail, ExptTurnResultRunLog: %v", json.Jsonify(runLog))
		}
		exptTurnResultRunLogPOs = append(exptTurnResultRunLogPOs, runLogPO)
	}
	err := r.exptTurnResultDAO.SaveTurnRunLogs(ctx, exptTurnResultRunLogPOs)
	if err != nil {
		return errorx.Wrapf(err, "SaveTurnRunLogs fail, runLogs: %v", json.Jsonify(runLogs))
	}

	return nil
}

func (r *ExptTurnResultRepoImpl) UpdateTurnRunLogWithItemIDs(ctx context.Context, spaceID, exptID, exptRunID int64, itemIDs []int64, ufields map[string]any) error {
	return r.exptTurnResultDAO.UpdateTurnRunLogWithItemIDs(ctx, spaceID, exptID, exptRunID, itemIDs, ufields)
}

func (r *ExptTurnResultRepoImpl) CreateOrUpdateItemsTurnRunLogStatus(ctx context.Context, spaceID, exptID, exptRunID int64, itemIDs []int64, status entity.TurnRunState) error {
	// runlog might be not created, creating ignore existed
	turnResults, err := r.exptTurnResultDAO.BatchGet(ctx, spaceID, exptID, itemIDs)
	if err != nil {
		return err
	}

	ids, err := r.idgen.GenMultiIDs(ctx, len(turnResults))
	if err != nil {
		return err
	}
	runlogs := make([]*model.ExptTurnResultRunLog, 0, len(turnResults))
	for idx := range turnResults {
		runlogs = append(runlogs, &model.ExptTurnResultRunLog{
			ID:        ids[idx],
			SpaceID:   spaceID,
			ExptID:    exptID,
			ExptRunID: exptRunID,
			ItemID:    turnResults[idx].ItemID,
			TurnID:    turnResults[idx].TurnID,
			Status:    int32(status),
			ErrMsg:    gptr.Of([]byte(errno.SerializeErr(errno.NewTurnOtherErr("turn status not updated for long interval", fmt.Errorf("turn result failure with timeout"))))),
		})
	}

	if err := r.exptTurnResultDAO.BatchCreateNXRunLog(ctx, runlogs); err != nil {
		return err
	}

	if err := r.UpdateTurnRunLogWithItemIDs(ctx, spaceID, exptID, exptRunID, itemIDs, map[string]any{"status": int32(status)}); err != nil {
		return err
	}

	return nil
}

func (r *ExptTurnResultRepoImpl) GetItemTurnResults(ctx context.Context, exptID, itemID, spaceID int64) ([]*entity.ExptTurnResult, error) {
	exptTurnResultPOs, err := r.exptTurnResultDAO.GetItemTurnResults(ctx, exptID, itemID, spaceID)
	if err != nil {
		return nil, errorx.Wrapf(err, "GetItemTurnResults fail, exptID: %v, itemID: %v", exptID, itemID)
	}

	exptTurnResults := make([]*entity.ExptTurnResult, 0, len(exptTurnResultPOs))
	for _, exptTurnResultPO := range exptTurnResultPOs {
		exptTurnResult := convert.NewExptTurnResultConvertor().PO2DO(exptTurnResultPO, nil)
		exptTurnResults = append(exptTurnResults, exptTurnResult)
	}
	return exptTurnResults, nil
}

func (r *ExptTurnResultRepoImpl) GetItemTurnRunLogs(ctx context.Context, exptID, exptRunID, itemID, spaceID int64) ([]*entity.ExptTurnResultRunLog, error) {
	exptTurnResultRunLogPOs, err := r.exptTurnResultDAO.GetItemTurnRunLogs(ctx, exptID, exptRunID, itemID, spaceID)
	if err != nil {
		return nil, errorx.Wrapf(err, "GetItemTurnRunLogs fail, exptID: %v, exptRunID: %v, itemID: %v", exptID, exptRunID, itemID)
	}
	exptTurnResultRunLogs := make([]*entity.ExptTurnResultRunLog, 0, len(exptTurnResultRunLogPOs))
	for _, exptTurnResultRunLogPO := range exptTurnResultRunLogPOs {
		exptTurnResultRunLog, err := convert.NewExptTurnResultRunLogConvertor().PO2DO(exptTurnResultRunLogPO)
		if err != nil {
			return nil, errorx.Wrapf(err, "PO2DO fail, ExptTurnResultRunLogPO: %v", json.Jsonify(exptTurnResultRunLogPO))
		}
		exptTurnResultRunLogs = append(exptTurnResultRunLogs, exptTurnResultRunLog)
	}

	return exptTurnResultRunLogs, nil
}

func (r *ExptTurnResultRepoImpl) MGetItemTurnRunLogs(ctx context.Context, exptID, exptRunID int64, itemIDs []int64, spaceID int64) ([]*entity.ExptTurnResultRunLog, error) {
	exptTurnResultRunLogPOs, err := r.exptTurnResultDAO.MGetItemTurnRunLogs(ctx, exptID, exptRunID, itemIDs, spaceID)
	if err != nil {
		return nil, errorx.Wrapf(err, "MGetItemTurnRunLogs fail, exptID: %v, exptRunID: %v, itemIDs: %v", exptID, exptRunID, itemIDs)
	}

	exptTurnResultRunLogs := make([]*entity.ExptTurnResultRunLog, 0, len(exptTurnResultRunLogPOs))
	for _, exptTurnResultRunLogPO := range exptTurnResultRunLogPOs {
		exptTurnResultRunLog, err := convert.NewExptTurnResultRunLogConvertor().PO2DO(exptTurnResultRunLogPO)
		if err != nil {
			return nil, errorx.Wrapf(err, "PO2DO fail, ExptTurnResultRunLogPO: %v", json.Jsonify(exptTurnResultRunLogPO))
		}
		exptTurnResultRunLogs = append(exptTurnResultRunLogs, exptTurnResultRunLog)
	}

	return exptTurnResultRunLogs, nil
}

func (r *ExptTurnResultRepoImpl) BatchCreateNXRunLog(ctx context.Context, exptTurnResultRunLogs []*entity.ExptTurnResultRunLog) error {
	exptTurnResultRunLogPOs := make([]*model.ExptTurnResultRunLog, 0, len(exptTurnResultRunLogs))
	for _, turnResult := range exptTurnResultRunLogs {
		turnResultPO, err := convert.NewExptTurnResultRunLogConvertor().DO2PO(turnResult)
		if err != nil {
			return errorx.Wrapf(err, "DO2PO fail, ExptTurnResultRunLog: %v", json.Jsonify(turnResult))
		}
		exptTurnResultRunLogPOs = append(exptTurnResultRunLogPOs, turnResultPO)
	}
	err := r.exptTurnResultDAO.BatchCreateNXRunLog(ctx, exptTurnResultRunLogPOs)
	if err != nil {
		return errorx.Wrapf(err, "BatchCreateNXRunLog fail, exptTurnResultRunLogs: %v", json.Jsonify(exptTurnResultRunLogs))
	}
	return nil
}

func (r *ExptTurnResultRepoImpl) ListTurnResult(ctx context.Context, spaceID, exptID int64, filter *entity.ExptTurnResultFilter, page entity.Page, desc bool) ([]*entity.ExptTurnResult, int64, error) {
	exptTurnResultPOs, total, err := r.exptTurnResultDAO.ListTurnResult(ctx, spaceID, exptID, filter, page, desc)
	if err != nil {
		return nil, 0, errorx.Wrapf(err, "ListTurnResult fail, spaceID: %v, exptID: %v, filters: %v, page: %v", spaceID, exptID, filter, page)
	}

	exptTurnResults := make([]*entity.ExptTurnResult, 0, len(exptTurnResultPOs))
	for _, exptTurnResultPO := range exptTurnResultPOs {
		exptTurnResult := convert.NewExptTurnResultConvertor().PO2DO(exptTurnResultPO, nil)
		exptTurnResults = append(exptTurnResults, exptTurnResult)
	}
	return exptTurnResults, total, nil
}

// nolint: byted_s_too_many_lines_in_func
func (r *ExptTurnResultRepoImpl) ListTurnResultByItemIDs(ctx context.Context, spaceID, exptID int64, itemIDs []int64, page entity.Page, desc bool) ([]*entity.ExptTurnResult, int64, error) {
	exptTurnResultPOs, total, err := r.exptTurnResultDAO.ListTurnResultByItemIDs(ctx, spaceID, exptID, itemIDs, page, desc)
	if err != nil {
		return nil, 0, errorx.Wrapf(err, "ListTurnResult fail, spaceID: %v, exptID: %v, itemIDs: %v", spaceID, exptID, itemIDs)
	}

	exptTurnResults := make([]*entity.ExptTurnResult, 0, len(exptTurnResultPOs))
	for _, exptTurnResultPO := range exptTurnResultPOs {
		exptTurnResult := convert.NewExptTurnResultConvertor().PO2DO(exptTurnResultPO, nil)
		exptTurnResults = append(exptTurnResults, exptTurnResult)
	}
	return exptTurnResults, total, nil
}

func (e *ExptTurnResultRepoImpl) BatchGetTurnEvaluatorResultRef(ctx context.Context, spaceID int64, exptTurnResultIDs []int64) ([]*entity.ExptTurnEvaluatorResultRef, error) {
	pos, err := e.exptTurnEvaluatorResultRefDAO.BatchGet(ctx, spaceID, exptTurnResultIDs)
	if err != nil {
		return nil, err
	}
	dos := make([]*entity.ExptTurnEvaluatorResultRef, 0)
	for _, po := range pos {
		dos = append(dos, convert.NewExptTurnEvaluatorResultRefConvertor().PO2DO(po))
	}
	return dos, nil
}

func (e *ExptTurnResultRepoImpl) GetTurnEvaluatorResultRefByExptID(ctx context.Context, spaceID, exptID int64) ([]*entity.ExptTurnEvaluatorResultRef, error) {
	pos, err := e.exptTurnEvaluatorResultRefDAO.GetByExptID(ctx, spaceID, exptID)
	if err != nil {
		return nil, err
	}
	dos := make([]*entity.ExptTurnEvaluatorResultRef, 0)
	for _, po := range pos {
		dos = append(dos, convert.NewExptTurnEvaluatorResultRefConvertor().PO2DO(po))
	}
	return dos, nil
}

func (e *ExptTurnResultRepoImpl) GetTurnEvaluatorResultRefByEvaluatorVersionID(ctx context.Context, spaceID, exptID, evaluatorVersionID int64) ([]*entity.ExptTurnEvaluatorResultRef, error) {
	pos, err := e.exptTurnEvaluatorResultRefDAO.GetByExptEvaluatorVersionID(ctx, spaceID, exptID, evaluatorVersionID)
	if err != nil {
		return nil, err
	}
	dos := make([]*entity.ExptTurnEvaluatorResultRef, 0)
	for _, po := range pos {
		dos = append(dos, convert.NewExptTurnEvaluatorResultRefConvertor().PO2DO(po))
	}
	return dos, nil
}
