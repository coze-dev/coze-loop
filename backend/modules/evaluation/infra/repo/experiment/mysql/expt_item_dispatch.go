// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"errors"
	"fmt"

	"github.com/bytedance/gg/gptr"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

//go:generate mockgen -destination=mocks/expt_item_dispatch.go -package=mocks . IExptItemDispatchDAO

type IExptItemDispatchDAO interface {
	ClaimQuotaReserved(ctx context.Context, spaceID, exptID, exptRunID int64, itemIDs []int64, itemRequestIDs map[int64]string) ([]repo.ClaimedDispatchItem, error)
	ResetQuotaReserved(ctx context.Context, spaceID, exptID, exptRunID int64, itemIDs []int64, itemRequestIDs map[int64]string) ([]int64, error)
	LoadDispatchRuntime(ctx context.Context, spaceID, exptID, exptRunID int64, candidateLimit int) (*repo.ExptDispatchRuntime, error)
	StartReservedItem(ctx context.Context, spaceID, exptID, exptRunID, itemID int64, expectedRequestID string) (repo.StartReservedItemResult, error)
	RequeueProcessingItem(ctx context.Context, spaceID, exptID, exptRunID, itemID int64, expectedRequestID string) (bool, error)
	MGetDispatchObservations(ctx context.Context, spaceID, exptID, exptRunID int64, itemIDs []int64) ([]*repo.ExptDispatchObservation, error)
}

func NewExptItemDispatchDAO(provider db.Provider) IExptItemDispatchDAO {
	return &exptItemDispatchDAOImpl{db: provider}
}

type exptItemDispatchDAOImpl struct{ db db.Provider }

func dispatchRequest(ext []byte, runID int64) (string, *entity.ManualRetryContext, error) {
	parsed, err := entity.ParseExptItemResultExt(ext)
	if err != nil {
		return "", nil, err
	}
	manual, err := entity.ParseManualRetryContext(parsed)
	if err != nil {
		return "", nil, err
	}
	if manual == nil {
		return "", nil, nil
	}
	if manual.RunID != runID {
		return "", nil, fmt.Errorf("dispatch manual retry run mismatch")
	}
	return manual.RequestID, manual, nil
}

// The grant map is required even for original empty-token items. Validate the
// whole input first so a missing entry cannot silently become an empty wildcard.
func dispatchIDs(itemIDs []int64, expected map[int64]string) ([]int64, error) {
	seen := make(map[int64]bool, len(itemIDs))
	ids := make([]int64, 0, len(itemIDs))
	for _, id := range itemIDs {
		if _, ok := expected[id]; !ok {
			return nil, fmt.Errorf("dispatch expected request ID missing for item %d", id)
		}
		if !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	return ids, nil
}

type lockedDispatchItem struct {
	item    model.ExptItemResult
	log     model.ExptItemResultRunLog
	manual  *entity.ManualRetryContext
	request string
	mode    entity.ExptRunMode
}

func lockDispatchItem(tx *gorm.DB, spaceID, exptID, runID, itemID int64, expected string) (*lockedDispatchItem, error) {
	s := &lockedDispatchItem{}
	lock := clause.Locking{Strength: "UPDATE"}
	err := tx.Clauses(lock).Where("space_id=? AND expt_id=? AND item_id=? AND deleted_at IS NULL", spaceID, exptID, itemID).Take(&s.item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if s.item.ExptRunID != runID {
		return nil, nil
	}
	s.request, s.manual, err = dispatchRequest(gptr.Indirect(s.item.Ext), runID)
	if err != nil {
		return nil, err
	}
	if s.request != expected {
		return nil, nil
	}
	// Admission locks experiment/run before canonical. Never lock those rows
	// after canonical here: current-source control reads follow the existing
	// execution path; run-tick lifecycle serialization is owned by the caller.
	var run model.ExptRunLog
	if err = tx.Where("space_id=? AND expt_id=? AND expt_run_id=? AND deleted_at IS NULL", spaceID, exptID, runID).Take(&run).Error; err != nil {
		return nil, err
	}
	if run.Status == nil {
		return nil, fmt.Errorf("dispatch source run status missing")
	}
	status := entity.ExptStatus(*run.Status)
	if entity.IsExptFinished(status) || entity.IsExptFinishing(status) {
		return nil, nil
	}
	if status != entity.ExptStatus_Pending && status != entity.ExptStatus_Processing {
		return nil, fmt.Errorf("dispatch invalid run status")
	}
	var expt model.Experiment
	if err = tx.Where("space_id=? AND id=? AND deleted_at IS NULL", spaceID, exptID).Take(&expt).Error; err != nil {
		return nil, err
	}
	if expt.LatestRunID != runID || entity.IsExptFinished(entity.ExptStatus(expt.Status)) || entity.IsExptFinishing(entity.ExptStatus(expt.Status)) {
		return nil, nil
	}
	s.mode = entity.ExptRunMode(gptr.Indirect(run.Mode))
	if s.mode < entity.EvaluationModeSubmit || s.mode > entity.EvaluationModeTrialRun {
		return nil, fmt.Errorf("dispatch invalid source run mode")
	}
	if s.manual != nil {
		s.mode = s.manual.Mode
	}
	if err = tx.Clauses(lock).Where("space_id=? AND expt_id=? AND expt_run_id=? AND item_id=? AND deleted_at IS NULL", spaceID, exptID, runID, itemID).Take(&s.log).Error; err != nil {
		return nil, err
	}
	if s.item.ItemVersionID != s.log.ItemVersionID || s.log.RetryTimes < 0 || !validCurrentItemStatus(entity.ItemRunState(s.item.Status)) || !validCurrentItemStatus(entity.ItemRunState(s.log.Status)) || gptr.Indirect(s.log.ResultState) < 0 || gptr.Indirect(s.log.ResultState) > int32(entity.ExptItemResultStateLogged) {
		return nil, fmt.Errorf("dispatch damaged item binding/state")
	}
	if entity.IsItemRunFinished(entity.ItemRunState(s.item.Status)) || entity.IsItemRunFinished(entity.ItemRunState(s.log.Status)) {
		return nil, nil
	}
	return s, nil
}

func (d *exptItemDispatchDAOImpl) ClaimQuotaReserved(ctx context.Context, spaceID, exptID, runID int64, itemIDs []int64, expected map[int64]string) ([]repo.ClaimedDispatchItem, error) {
	ids, err := dispatchIDs(itemIDs, expected)
	if err != nil {
		return nil, err
	}
	claimed := make([]repo.ClaimedDispatchItem, 0, len(ids))
	for _, id := range ids {
		var payload *repo.ClaimedDispatchItem
		err = d.db.Transaction(ctx, func(tx *gorm.DB) error {
			s, err := lockDispatchItem(tx, spaceID, exptID, runID, id, expected[id])
			if err != nil || s == nil {
				return err
			}
			if s.log.Status != int32(entity.ItemRunState_Queueing) || s.log.QuotaReservationState != int32(entity.QuotaReservationStateNone) {
				return nil
			}
			result := tx.Model(&model.ExptItemResultRunLog{}).Where("id=? AND status=? AND quota_reservation_state=?", s.log.ID, int32(entity.ItemRunState_Queueing), int32(entity.QuotaReservationStateNone)).Update("quota_reservation_state", int32(entity.QuotaReservationStateReserved))
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 1 {
				payload = &repo.ClaimedDispatchItem{ItemID: id, RequestID: s.request, Mode: s.mode, RetryTimes: s.log.RetryTimes}
			}
			return nil
		}, db.WithMaster())
		if err != nil {
			return claimed, err
		}
		if payload != nil {
			claimed = append(claimed, *payload)
		}
	}
	return claimed, nil
}

func (d *exptItemDispatchDAOImpl) ResetQuotaReserved(ctx context.Context, spaceID, exptID, runID int64, itemIDs []int64, expected map[int64]string) ([]int64, error) {
	ids, err := dispatchIDs(itemIDs, expected)
	if err != nil {
		return nil, err
	}
	reset := make([]int64, 0, len(ids))
	for _, id := range ids {
		changed := false
		err = d.db.Transaction(ctx, func(tx *gorm.DB) error {
			s, err := lockDispatchItem(tx, spaceID, exptID, runID, id, expected[id])
			if err != nil || s == nil {
				return err
			}
			if s.log.Status != int32(entity.ItemRunState_Queueing) || s.log.QuotaReservationState != int32(entity.QuotaReservationStateReserved) {
				return nil
			}
			result := tx.Model(&model.ExptItemResultRunLog{}).Where("id=? AND status=? AND quota_reservation_state=?", s.log.ID, int32(entity.ItemRunState_Queueing), int32(entity.QuotaReservationStateReserved)).Update("quota_reservation_state", int32(entity.QuotaReservationStateNone))
			changed = result.RowsAffected == 1
			return result.Error
		}, db.WithMaster())
		if err != nil {
			return reset, err
		}
		if changed {
			reset = append(reset, id)
		}
	}
	return reset, nil
}

func (d *exptItemDispatchDAOImpl) dispatchObservations(ctx context.Context, spaceID, exptID, runID int64, itemIDs []int64, activeOnly bool) ([]*repo.ExptDispatchObservation, error) {
	// Filter canonical soft deletion in ON, not WHERE: a missing/deleted right
	// row must remain visible as an incomplete binding, never a fake empty run.
	session := d.db.NewSession(ctx, db.WithMaster()).Table("expt_item_result_run_log AS l").
		Select("l.id,l.item_id,l.item_version_id,l.status,l.result_state,l.retry_times,l.quota_reservation_state,c.id AS canonical_id,c.expt_run_id AS canonical_run_id,c.status AS canonical_status,c.item_version_id AS canonical_item_version_id,c.ext,r.mode,r.status AS run_status").
		Joins("LEFT JOIN expt_item_result AS c ON c.space_id=l.space_id AND c.expt_id=l.expt_id AND c.item_id=l.item_id AND c.deleted_at IS NULL").
		Joins("LEFT JOIN expt_run_log AS r ON r.space_id=l.space_id AND r.expt_id=l.expt_id AND r.expt_run_id=l.expt_run_id AND r.deleted_at IS NULL").
		Where("l.space_id=? AND l.expt_id=? AND l.expt_run_id=? AND l.deleted_at IS NULL", spaceID, exptID, runID)
	if itemIDs != nil {
		session = session.Where("l.item_id IN ?", itemIDs)
	}
	if activeOnly {
		session = session.Where("l.status IN ?", []int32{int32(entity.ItemRunState_Queueing), int32(entity.ItemRunState_Processing)})
	}
	var rows []currentRequestRow
	if err := session.Order("l.id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*repo.ExptDispatchObservation, 0, len(rows))
	for _, r := range rows {
		if r.CanonicalID == 0 || r.CanonicalRunID != runID || r.CanonicalItemVersionID != r.ItemVersionID {
			return nil, fmt.Errorf("dispatch incomplete canonical binding for item %d", r.ItemID)
		}
		request, _, err := dispatchRequest(r.Ext, runID)
		if err != nil {
			return nil, err
		}
		if r.RunStatus == nil || r.Mode == nil || entity.ExptRunMode(*r.Mode) < entity.EvaluationModeSubmit || entity.ExptRunMode(*r.Mode) > entity.EvaluationModeTrialRun {
			return nil, fmt.Errorf("dispatch incomplete run binding")
		}
		if !validCurrentItemStatus(entity.ItemRunState(r.Status)) || !validCurrentItemStatus(entity.ItemRunState(r.CanonicalStatus)) || r.RetryTimes < 0 || gptr.Indirect(r.ResultState) < 0 || gptr.Indirect(r.ResultState) > int32(entity.ExptItemResultStateLogged) {
			return nil, fmt.Errorf("dispatch damaged observation")
		}
		runStatus := entity.ExptStatus(*r.RunStatus)
		if runStatus != entity.ExptStatus_Pending && runStatus != entity.ExptStatus_Processing && !entity.IsExptFinished(runStatus) && !entity.IsExptFinishing(runStatus) {
			return nil, fmt.Errorf("dispatch invalid source run status")
		}
		status := r.Status
		if entity.IsItemRunFinished(entity.ItemRunState(r.CanonicalStatus)) {
			status = r.CanonicalStatus
		}
		out = append(out, &repo.ExptDispatchObservation{ItemID: r.ItemID, Status: status, QuotaReservationState: entity.QuotaReservationState(r.QuotaReservationState), RequestID: request})
	}
	if itemIDs != nil {
		wanted := make(map[int64]bool, len(itemIDs))
		for _, id := range itemIDs {
			wanted[id] = true
		}
		for _, row := range rows {
			delete(wanted, row.ItemID)
		}
		if len(wanted) != 0 {
			return nil, fmt.Errorf("dispatch item log observation incomplete")
		}
	}
	return out, nil
}

func (d *exptItemDispatchDAOImpl) LoadDispatchRuntime(ctx context.Context, spaceID, exptID, runID int64, candidateLimit int) (*repo.ExptDispatchRuntime, error) {
	if candidateLimit <= 0 {
		candidateLimit = defaultLimit
	}
	observations, err := d.dispatchObservations(ctx, spaceID, exptID, runID, nil, true)
	if err != nil {
		return nil, err
	}
	return classifyDispatchRuntime(observations, candidateLimit), nil
}

func classifyDispatchRuntime(observations []*repo.ExptDispatchObservation, candidateLimit int) *repo.ExptDispatchRuntime {
	runtime := &repo.ExptDispatchRuntime{ItemRequestIDs: make(map[int64]string)}
	for _, o := range observations {
		if o == nil {
			continue
		}
		switch {
		case o.Status == int32(entity.ItemRunState_Processing):
			runtime.OccupiedItemIDs = append(runtime.OccupiedItemIDs, o.ItemID)
		case o.Status != int32(entity.ItemRunState_Queueing):
			continue
		case o.QuotaReservationState.IsQuotaReserved():
			runtime.OccupiedItemIDs = append(runtime.OccupiedItemIDs, o.ItemID)
		default:
			if candidateLimit > 0 && len(runtime.CandidateItemIDs) >= candidateLimit {
				continue
			}
			runtime.CandidateItemIDs = append(runtime.CandidateItemIDs, o.ItemID)
		}
		runtime.ItemRequestIDs[o.ItemID] = o.RequestID
	}
	return runtime
}

// Both item/log and pending canonical/log turns move before the one stats
// delta. Success turns and all invocation refs/scores remain untouched.
func transitionDispatchItem(tx *gorm.DB, s *lockedDispatchItem, next entity.ItemRunState) error {
	var turns []model.ExptTurnResult
	var logs []model.ExptTurnResultRunLog
	lock := clause.Locking{Strength: "UPDATE"}
	where := "space_id=? AND expt_id=? AND expt_run_id=? AND item_id=? AND deleted_at IS NULL"
	args := []any{s.item.SpaceID, s.item.ExptID, s.item.ExptRunID, s.item.ItemID}
	if err := tx.Clauses(lock).Where(where, args...).Order("id ASC").Find(&turns).Error; err != nil {
		return err
	}
	if err := tx.Clauses(lock).Where(where, args...).Order("id ASC").Find(&logs).Error; err != nil {
		return err
	}
	byTurn := make(map[int64]model.ExptTurnResultRunLog, len(logs))
	for _, log := range logs {
		if _, duplicate := byTurn[log.TurnID]; duplicate {
			return fmt.Errorf("dispatch duplicate turn log")
		}
		if _, err := convert.NewExptTurnResultRunLogConvertor().PO2DO(&log); err != nil {
			return err
		}
		if err := validateManualRetryTurnExt(log.Ext); err != nil {
			return err
		}
		byTurn[log.TurnID] = log
	}
	if s.manual != nil && (len(turns) == 0 || len(turns) != len(logs)) {
		return fmt.Errorf("dispatch frozen turns incomplete")
	}
	pendingTurns, pendingLogs := []int64{}, []int64{}
	matchedLogs := 0
	fromTurn, toTurn := int32(entity.TurnRunState_Queueing), int32(entity.TurnRunState_Processing)
	if next == entity.ItemRunState_Queueing {
		fromTurn, toTurn = toTurn, fromTurn
	}
	for _, turn := range turns {
		log, exists := byTurn[turn.TurnID]
		if exists {
			matchedLogs++
		}
		if turn.ItemVersionID != s.item.ItemVersionID || turn.TurnID < 0 || exists && log.ItemVersionID != turn.ItemVersionID {
			return fmt.Errorf("dispatch turn binding mismatch")
		}
		if s.manual != nil && !exists {
			return fmt.Errorf("dispatch frozen turn log missing")
		}
		if turn.Status == int32(entity.TurnRunState_Success) {
			if exists && log.Status != turn.Status {
				return fmt.Errorf("dispatch success turn mismatch")
			}
			continue
		}
		if turn.Status < 0 || turn.Status > int32(entity.TurnRunState_Terminal) || exists && (log.Status < 0 || log.Status > int32(entity.TurnRunState_Terminal)) {
			return fmt.Errorf("dispatch invalid turn state")
		}
		if s.manual != nil && (turn.Status != fromTurn && turn.Status != toTurn || log.Status != fromTurn && log.Status != toTurn) {
			return fmt.Errorf("dispatch invalid pending turn state")
		}
		if turn.Status == fromTurn {
			pendingTurns = append(pendingTurns, turn.ID)
		}
		if exists && log.Status == fromTurn {
			pendingLogs = append(pendingLogs, log.ID)
		}
	}
	if matchedLogs != len(logs) {
		return fmt.Errorf("dispatch orphaned turn log")
	}
	var stats model.ExptStats
	if err := tx.Clauses(lock).Where("space_id=? AND expt_id=? AND deleted_at IS NULL", s.item.SpaceID, s.item.ExptID).Take(&stats).Error; err != nil {
		return err
	}
	old := entity.ItemRunState(s.item.Status)
	if old != next && (old == entity.ItemRunState_Queueing && stats.PendingCnt <= 0 || old == entity.ItemRunState_Processing && stats.ProcessingCnt <= 0) {
		return fmt.Errorf("dispatch stats underflow")
	}
	// UpdateColumns skips GORM timestamp hooks; explicit original timestamps
	// also prevent MySQL ON UPDATE from restarting the zombie clock on requeue.
	update := func(value any, ids []int64, fields map[string]any) error {
		if len(ids) == 0 {
			return nil
		}
		if next == entity.ItemRunState_Queueing {
			fields["updated_at"] = gorm.Expr("updated_at")
		}
		query := tx.Model(value).Where("id IN ?", ids)
		var result *gorm.DB
		if next == entity.ItemRunState_Queueing {
			result = query.UpdateColumns(fields)
		} else {
			result = query.Updates(fields)
		}
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != int64(len(ids)) {
			return fmt.Errorf("dispatch transition write lost")
		}
		return nil
	}
	if old != next {
		if err := update(&model.ExptItemResult{}, []int64{s.item.ID}, map[string]any{"status": int32(next)}); err != nil {
			return err
		}
	}
	if err := update(&model.ExptItemResultRunLog{}, []int64{s.log.ID}, map[string]any{"status": int32(next), "quota_reservation_state": int32(entity.QuotaReservationStateNone)}); err != nil {
		return err
	}
	if err := update(&model.ExptTurnResult{}, pendingTurns, map[string]any{"status": toTurn}); err != nil {
		return err
	}
	if err := update(&model.ExptTurnResultRunLog{}, pendingLogs, map[string]any{"status": toTurn}); err != nil {
		return err
	}
	if old != next {
		delta := -1
		if next == entity.ItemRunState_Queueing {
			delta = 1
		}
		result := tx.Model(&model.ExptStats{}).Where("id=?", stats.ID).Updates(map[string]any{"pending_cnt": gorm.Expr("pending_cnt + ?", delta), "processing_cnt": gorm.Expr("processing_cnt - ?", delta)})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("dispatch stats update lost")
		}
	}
	return nil
}

func (d *exptItemDispatchDAOImpl) StartReservedItem(ctx context.Context, spaceID, exptID, runID, itemID int64, expected string) (repo.StartReservedItemResult, error) {
	result := repo.StartReservedItemResult{}
	err := d.db.Transaction(ctx, func(tx *gorm.DB) error {
		s, err := lockDispatchItem(tx, spaceID, exptID, runID, itemID, expected)
		if err != nil || s == nil {
			return err
		}
		result.CurrentRequest = true
		if s.log.Status != int32(entity.ItemRunState_Queueing) || s.log.QuotaReservationState != int32(entity.QuotaReservationStateReserved) {
			return nil
		}
		if err = transitionDispatchItem(tx, s, entity.ItemRunState_Processing); err != nil {
			return err
		}
		result.Started = true
		return nil
	}, db.WithMaster())
	if err != nil {
		return repo.StartReservedItemResult{}, err
	}
	return result, nil
}

func (d *exptItemDispatchDAOImpl) RequeueProcessingItem(ctx context.Context, spaceID, exptID, runID, itemID int64, expected string) (bool, error) {
	changed := false
	err := d.db.Transaction(ctx, func(tx *gorm.DB) error {
		s, err := lockDispatchItem(tx, spaceID, exptID, runID, itemID, expected)
		if err != nil || s == nil {
			return err
		}
		if s.log.Status != int32(entity.ItemRunState_Processing) || s.log.QuotaReservationState != int32(entity.QuotaReservationStateNone) {
			return nil
		}
		if err = transitionDispatchItem(tx, s, entity.ItemRunState_Queueing); err != nil {
			return err
		}
		changed = true
		return nil
	}, db.WithMaster())
	return changed && err == nil, err
}

func (d *exptItemDispatchDAOImpl) MGetDispatchObservations(ctx context.Context, spaceID, exptID, runID int64, itemIDs []int64) ([]*repo.ExptDispatchObservation, error) {
	if len(itemIDs) == 0 {
		return nil, nil
	}
	return d.dispatchObservations(ctx, spaceID, exptID, runID, itemIDs, false)
}

func chunkInt64(in []int64, size int) [][]int64 {
	if size <= 0 || len(in) == 0 {
		return nil
	}
	out := make([][]int64, 0, (len(in)+size-1)/size)
	for i := 0; i < len(in); i += size {
		end := i + size
		if end > len(in) {
			end = len(in)
		}
		out = append(out, in[i:end])
	}
	return out
}
