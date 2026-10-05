// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"bytes"
	"encoding/json"
	"unicode/utf8"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/hook"
)

// Protocol validation preserves receiver-owned display data; it is not content redaction.
func hookOperationSummary(row hookSummaryRow) (entity.HookRunSummary, error) {
	bad := func() (entity.HookRunSummary, error) {
		return entity.HookRunSummary{}, entity.ErrHookSummaryUnavailable
	}
	if row.OperationID == "" || len(row.OperationID) > 128 || row.UpdatedAt == nil || row.UpdatedAt.IsZero() || row.Attempt < 0 || row.Attempt > 11 {
		return bad()
	}
	for _, c := range row.OperationID {
		if c < 33 || c > 126 {
			return bad()
		}
	}
	out := entity.HookRunSummary{Status: entity.HookOperationStatus(row.Status), OperationID: row.OperationID, Attempt: row.Attempt, UpdatedAt: row.UpdatedAt}
	switch out.Status {
	case entity.HookOperationPending:
		if row.Attempt != 0 {
			return bad()
		}
	case entity.HookOperationRunning, entity.HookOperationRetryWait:
		if row.Attempt == 0 {
			return bad()
		}
	case entity.HookOperationSucceeded:
		if row.Attempt == 0 || len(row.ResultRedacted) > 32768 {
			return bad()
		}
		// Storage holds only the validated display map, not the original HTTP body.
		result := bytes.TrimSpace(row.ResultRedacted)
		if len(row.ResultRedacted) == 0 {
			result = []byte("null")
		}
		if !json.Valid(result) {
			return bad()
		}
		body := append([]byte(`{"status":"succeeded","result":`), result...)
		body = append(body, '}')
		response, outcome := hook.DecodeResponse(200, "application/json", bytes.NewReader(body))
		if response == nil || outcome.Code != entity.HookSucceeded {
			return bad()
		}
		out.Response = response
	case entity.HookOperationFailed:
		code, message := gptr.Indirect(row.ErrorCode), gptr.Indirect(row.ErrorMessage)
		if len(code) > 128 || len(message) > 2048 || !utf8.ValidString(code) || !utf8.ValidString(message) {
			return bad()
		}
		// Cancellation and timeout recovery can terminate without an HTTP error body.
		if code == "" {
			code = string(entity.HookFailed)
		}
		if message == "" {
			message = "Hook execution failed"
		}
		out.Error = &spi.HookError{Code: &code, Message: &message, Retryable: gptr.Of(false)}
	default:
		return bad()
	}
	return out, nil
}

func hookSummaryRunState(row hookSummaryRow, key entity.HookRunKey, status entity.ExptStatus) (*entity.HookRunState, error) {
	if row.Gate < 0 || row.Gate > 2 || row.PlanState < 0 || row.PlanState > 1 || row.FinalizeState < 0 || row.FinalizeState > 2 ||
		(row.Gate == 0 && !row.BeforeEnabled) || (row.PlanState == 1 && !row.PlanHashValid) || (row.FinalizeState != 0 && row.TerminalAt == nil) {
		return nil, entity.ErrHookSummaryUnavailable
	}
	state := &entity.HookRunState{Key: key, Status: status,
		Gate:     []entity.HookGateState{entity.HookGateWaiting, entity.HookGateReady, entity.HookGateClosed}[row.Gate],
		Finalize: []entity.HookFinalizeState{entity.HookFinalizeNone, entity.HookFinalizePending, entity.HookFinalizeCommitted}[row.FinalizeState],
		Before:   entity.HookOperation{Status: entity.HookOperationDisabled}, After: entity.HookOperation{Status: entity.HookOperationDisabled},
		Intent: entity.HookTerminalIntent{Status: entity.ExptStatus(gptr.Indirect(row.TerminalStatus))},
	}
	if row.HasTerminalReason {
		state.Intent.Reason = "present"
	}
	return state, nil
}

func hookSummaryOperationState(row hookSummaryRow) entity.HookOperation {
	status := entity.HookOperationStatus(row.Status)
	active := row.Activated && (status == entity.HookOperationPending || status == entity.HookOperationRunning || status == entity.HookOperationRetryWait)
	return entity.HookOperation{ID: row.OperationID, Status: status, Activated: active, Attempt: row.Attempt, Generation: row.LeaseGeneration,
		LeaseUntil: gptr.Indirect(row.LeaseUntil), AttemptDeadline: gptr.Indirect(row.AttemptDeadline), Deadline: gptr.Indirect(row.OperationDeadline)}
}
