// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import "time"

// ExptItemCurrentRequest is one primary database observation of the canonical item and run log.
type ExptItemCurrentRequest struct {
	Source                                        *ExptCurrentItemRunSource
	SpaceID, ExptID, RunID, ItemID, ItemVersionID int64
	RequestID                                     string
	Mode                                          ExptRunMode
	Status                                        ItemRunState
	ResultState                                   ExptItemResultState
	RetryTimes                                    int32
	UpdatedAt                                     *time.Time
	QuotaReservationState                         QuotaReservationState
}

func (s *ExptItemCurrentRequest) Matches(runID int64, requestID string) bool {
	return s != nil && s.RunID == runID && s.RequestID == requestID && !IsItemRunFinished(s.Status)
}

// ExptCurrentItemRunSource freezes the item and invocation references before execution.
// It is internal and is never serialized into provider requests or MQ messages.
type ExptCurrentItemRunSource struct {
	CanonicalTurns []*ExptTurnResult
	CurrentRefs    []*ExptTurnEvaluatorResultRef
	Current        ExptItemCurrentRequest
	Turns          map[int64]*ExptTurnResultRunLog
}

// ExptCurrentItemRunWrite describes execution logging, not result publication.
type ExptCurrentItemRunWrite struct {
	Status           ItemRunState
	ErrMsg           []byte
	RetryTimes       int32
	Yield            bool
	RetryOnly        bool
	FailPendingTurns bool
	MissingTurnLogs  []*ExptTurnResultRunLog
}
