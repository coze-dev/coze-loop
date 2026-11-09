// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package component

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// ICentralItemDispatcher extends the injected reservation guard without replacing the Ticker's selection algorithm.
type ICentralItemDispatcher interface {
	ReplayPendingItems(ctx context.Context, event *entity.ExptScheduleEvent, expt *entity.Experiment) error
	// SubmitItems returns the number of persisted dispatch claims, including unknown Submit outcomes.
	SubmitItems(ctx context.Context, event *entity.ExptScheduleEvent, expt *entity.Experiment, items []*entity.ExptEvalItem) (int, error)
}

// ICentralItemExecution validates the immutable ItemRun handshake, matching Task and Held Allocation under the Item mutex.
type ICentralItemExecution interface {
	ConfirmItemRunning(ctx context.Context, expt *entity.Experiment, item *entity.ExptItemResultRunLog) (bool, error)
	// PrepareItemResources returns a context derived from ctx carrying the granted plan for the original execution chain; nil is invalid.
	PrepareItemResources(ctx context.Context, eiec *entity.ExptItemEvalCtx) (context.Context, error)
}

// ICentralItemLifecycle is a best-effort notification after durable business state, not a second state machine.
type ICentralItemLifecycle interface {
	// Implementations verify the handshake and re-read primary state; Terminal requires actual stop AND start prevention before Cancel.
	OnItemTerminal(ctx context.Context, item *entity.ExptItemResultRunLog) error
}

// ICentralHistoricalItemLifecycle resolves generations from immutable persistent bindings, never the current item or context RequestID.
type ICentralHistoricalItemLifecycle interface {
	// Called under the item lease after Terminal is re-read and all historical run/item attempts are strictly stopped.
	// Missing/conflicting bindings or an unknown finalization outcome must return an error to block deletion.
	OnHistoricalItemTerminal(ctx context.Context, item *entity.ExptItemResultRunLog) error
}

// ICentralSandboxTaskPolicy extends the Sandbox adapter at experiment Submit/Retry/Update; direct Init remains the budget-authorized entry.
type ICentralSandboxTaskPolicy interface {
	DeferSandboxTaskInit(ctx context.Context, exptID, spaceID int64) (bool, error)
}

// ICentralEvaluationPolicy overrides retired EvalX admission; only non-EvalX SandboxAgent creation consults the new protocol.
type ICentralEvaluationPolicy interface {
	SelectEvaluationScheduling(ctx context.Context, subject CentralAdmissionSubject) (CentralAdmissionDecision, error)
}
