// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

type PreparedRetryItem struct {
	PreviousRunID     int64
	PreviousRequestID string
	ItemVersionID     int64
	SourceItem        *ExptItemResult
	CurrentItemLog    *ExptItemResultRunLog
	FrozenItemRef     *ExptItemRef
	SourceTurnLogs    []*ExptTurnResultRunLog
	Turns             []*PreparedRetryTurn
}

type PreparedRetryTurn struct {
	SourceTurn       *ExptTurnResult
	CurrentRunLog    *ExptTurnResultRunLog
	SourceRefs       []*ExptTurnEvaluatorResultRef
	TargetResultID   int64
	EvaluatorResults *EvaluatorResults
	KeepRefIDs       []int64
	PreserveSuccess  bool
}

type ManualRetryPreparation struct {
	Items         []*PreparedRetryItem
	RejectedItems []*ManualRetryRejectedItem
	SnapshotAtMS  int64
}

type RequeueTerminalItemsRequest struct {
	SpaceID       int64
	ExptID        int64
	RunID         int64
	RequestID     string
	PayloadHash   string
	Operator      string
	Mode          ExptRunMode
	SnapshotAtMS  int64
	Items         []*PreparedRetryItem
	RejectedItems []*ManualRetryRejectedItem
}

type RetryInCurrentRunRequest struct {
	SpaceID       int64
	ExptID        int64
	ExpectedRunID int64
	RequestID     string
	Mode          ExptRunMode
	ItemIDs       []int64
	Ext           map[string]string
	Session       *Session
}
