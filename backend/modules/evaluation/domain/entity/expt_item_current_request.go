// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import "time"

// ExptItemCurrentRequest is one primary database observation of the canonical item and run log.
type ExptItemCurrentRequest struct {
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
