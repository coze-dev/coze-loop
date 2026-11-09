// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
)

func resourceTurnStartValue(execution *currentItemExecution, claimed bool) string {
	value, _ := json.Marshal(entity.ResourceTurnStart{ItemRunLogID: execution.ItemRunLogID,
		RequestID: execution.Source.Current.RequestID, Claimed: claimed})
	return string(value)
}

func resourceTurnHasPendingStart(execution *currentItemExecution, turn *entity.ExptTurnResultRunLog) bool {
	return execution != nil && execution.Source != nil && execution.ItemRunLogID > 0 &&
		turn != nil && turn.Ext[entity.ResourceTurnStartExtKey] == resourceTurnStartValue(execution, false)
}

// Persist before dispatch changes every pending turn to Processing; that projection is not an invocation claim.
func (e *ExptItemEventEvalServiceImpl) prepareResourceTurnStarts(ctx context.Context) error {
	execution := executionSource(ctx)
	port, ok := e.exptItemResultRepo.(repo.IExptCurrentResultRepo)
	if !ok || execution == nil || execution.Source == nil || !execution.SourceValidated || execution.LockLost || execution.ItemRunLogID <= 0 {
		return fmt.Errorf("%w: resource turn source unavailable", errCentralItemAdmission)
	}
	for id, turn := range execution.Source.Turns {
		if turn == nil || turn.Status != entity.TurnRunState_Queueing || turn.TargetResultID != 0 ||
			!entity.SameCurrentEvaluatorRefs(turn.EvaluatorResultIds, nil) {
			continue
		}
		if turn.Ext[entity.ResourceTurnStartExtKey] == resourceTurnStartValue(execution, true) || resourceTurnHasPendingStart(execution, turn) {
			continue
		}
		prepared := *turn
		prepared.Ext = maps.Clone(turn.Ext)
		if prepared.Ext == nil {
			prepared.Ext = make(map[string]string)
		}
		prepared.Ext[entity.ResourceTurnStartExtKey] = resourceTurnStartValue(execution, false)
		applied, err := port.StoreCurrentTurnRunResult(ctx, execution.Source, &prepared)
		if err := checkedCurrentWrite(ctx, applied, err); err != nil {
			return err
		}
		execution.Source.Turns[id] = &prepared
	}
	return nil
}
