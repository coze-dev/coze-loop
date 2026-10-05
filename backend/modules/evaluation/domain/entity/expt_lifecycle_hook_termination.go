// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

// Cancellation uses only frozen identities and existing execution evidence.
type HookTerminationItem struct {
	Manifest   HookExecutionManifest
	Item       *ExptItemResultRunLog
	Turns      []*ExptTurnResultRunLog
	Targets    []*EvalTargetRecord
	Evaluators []*EvaluatorRecord
	// Derived from the decoded original Run, not the current experiment configuration.
	Execution *Experiment
}

type HookItemArchiveInput struct {
	Key            HookRunKey
	ExecutionScope string
	ItemID         int64
	Cancellation   bool
	// Prepared is required for cancellation; ordinary archival re-reads under the gate lock.
	Prepared *HookTerminationItem
	Refs     []*ExptTurnEvaluatorResultRef
	Scores   map[int64]*float64
}
