// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package entity

type ExptRunFinishDisposition int

const (
	ExptRunFinishFinished ExptRunFinishDisposition = iota + 1
	ExptRunFinishAlreadyFinished
	ExptRunFinishPending
	ExptRunFinishNonCurrent
)

// ExptRunFinishRequest carries identities only; final counts are read in SQL.
type ExptRunFinishRequest struct{ SpaceID, ExptID, RunID int64 }
type ExptRunFinishResult struct {
	Disposition ExptRunFinishDisposition
	Experiment  *Experiment
}
