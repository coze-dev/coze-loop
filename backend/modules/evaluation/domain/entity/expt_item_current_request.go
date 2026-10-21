// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"time"
)

// ExptItemCurrentRequest is one primary database observation of the canonical item and run log.
type ExptItemCurrentRequest struct {
	PublicationObserved                           bool
	Publication                                   *ExptItemResultPublishSource
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

	// OnlyIfQueueingUnreserved rejects a superseded pre-execution observation.
	// Its zero value preserves ordinary completion, yield and retry writes.
	OnlyIfQueueingUnreserved bool
}

// SameCurrentEvaluatorRefs compares invocation identity across JSON round trips.
// JSON omitempty makes an empty slice/map and nil the same persisted source.
func SameCurrentEvaluatorRefs(a, b *EvaluatorResults) bool {
	if a == nil {
		a = &EvaluatorResults{}
	}
	if b == nil {
		b = &EvaluatorResults{}
	}
	if len(a.EvalVerIDToResID) != len(b.EvalVerIDToResID) || len(a.Registered) != len(b.Registered) || len(a.Inline) != len(b.Inline) {
		return false
	}
	for version, id := range a.EvalVerIDToResID {
		if b.EvalVerIDToResID[version] != id {
			return false
		}
	}
	for index, ref := range a.Registered {
		other := b.Registered[index]
		if ref == nil || other == nil {
			if ref != other {
				return false
			}
			continue
		}
		if *ref != *other {
			return false
		}
	}
	for index, ref := range a.Inline {
		other := b.Inline[index]
		if ref == nil || other == nil {
			if ref != other {
				return false
			}
			continue
		}
		if *ref != *other {
			return false
		}
	}
	return true
}

// TurnError preserves the existing result reader's turn-level error contract.
// A yielding retry leaves prior error text unchanged; terminal completion replaces it.
func (w *ExptCurrentItemRunWrite) TurnError(status TurnRunState) []byte {
	if status == TurnRunState_Terminal {
		return []byte(errno.SerializeErr(errno.NewItemManuallyTerminatedErr()))
	}
	if status != TurnRunState_Fail {
		return nil
	}
	reason := errno.DeserializeErr(w.ErrMsg)
	return []byte(errno.SerializeErr(errno.NewTurnOtherErr(reason.Error(), reason)))
}
