// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package entity

import "context"

type ExptRunFinishDisposition int

const (
	ExptRunFinishFinished ExptRunFinishDisposition = iota + 1
	ExptRunFinishAlreadyFinished
	ExptRunFinishPending
	ExptRunFinishNonCurrent
)

// Final counts and the admission mode are read from the locked SQL rows.
type ExptRunFinishRequest struct {
	SpaceID, ExptID, RunID int64
	// BeforeCommit runs only after current-run validation, before terminal visibility.
	BeforeCommit func(context.Context, ExptRunMode) error `json:"-"`
}

type ExptRunFinishResult struct {
	Disposition ExptRunFinishDisposition
	Experiment  *Experiment
}

// ExptRunTerminationRequest is a frozen control operation, never an execution writer.
type ExptRunTerminationRequest struct {
	SpaceID, ExptID, RunID int64
	Begin                  bool
	Finalize               bool
	SetupAbort             bool
	Status                 ExptStatus
	StatusMessage          string
	NoCompleteItemTurn     bool
	ExpectedSource         *ExptCurrentItemRunSource
	Items                  []*ExptCurrentItemRunSource
	MissingTurnLogs        []*ExptTurnResultRunLog
	BeforeCommit           func(context.Context, ExptRunMode) error `json:"-"`
}
type ExptRunTerminationResult struct {
	Applied    bool
	NonCurrent bool
	Experiment *Experiment
}
